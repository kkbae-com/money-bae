package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	money "github.com/Rhymond/go-money"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/mrcunninghamz/money-bae/servers/api/internal/auth"
	"github.com/mrcunninghamz/money-bae/servers/api/internal/models"
	"github.com/mrcunninghamz/money-bae/servers/api/internal/testdb"
)

func newLedgerTestRouter(t *testing.T) (http.Handler, *gorm.DB) {
	db := testdb.New(t, &models.User{}, &models.Ledger{}, &models.Income{}, &models.Bill{}, &models.LedgerBill{})
	router := NewRouter(db, auth.MockVerifier{DB: db})
	return router, db
}

func decodeLedgerResponse(t *testing.T, rec *httptest.ResponseRecorder) ledgerResponse {
	t.Helper()
	var got ledgerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	return got
}

func TestCreateLedger_MinimalFields_DefaultsAndComputesNet(t *testing.T) {
	router, _ := newLedgerTestRouter(t)

	rec := doJSON(t, router, http.MethodPost, "/ledgers", ledgerRequest{
		Date: time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC),
		Name: strPtr("December P1"),
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeLedgerResponse(t, rec)
	if got.BankBalance.Amount() != 0 || got.Income.Amount() != 0 || got.Expenses.Amount() != 0 {
		t.Fatalf("expected zero-defaulted money fields, got %+v", got)
	}
	if got.Net.Amount() != 0 {
		t.Fatalf("expected net 0, got %+v", got.Net)
	}
}

func TestCreateLedger_ComputesNetFromBankBalanceIncomeExpenses(t *testing.T) {
	router, _ := newLedgerTestRouter(t)

	bankBalance := money.New(500000, "USD")
	income := money.New(300000, "USD")
	expenses := money.New(200000, "USD")
	rec := doJSON(t, router, http.MethodPost, "/ledgers", ledgerRequest{
		Date:        time.Now(),
		BankBalance: bankBalance,
		Income:      income,
		Expenses:    expenses,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeLedgerResponse(t, rec)
	if got.Net.Amount() != 600000 {
		t.Fatalf("expected net 6000 USD (5000+3000-2000), got %+v", got.Net)
	}
}

func TestCreateLedger_MissingDate_Returns400(t *testing.T) {
	router, _ := newLedgerTestRouter(t)

	rec := doJSON(t, router, http.MethodPost, "/ledgers", ledgerRequest{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListLedgers_OrderParam_DefaultsNewestFirst(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	older := models.Ledger{
		UserID: user.ID, Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&older).Error; err != nil {
		t.Fatalf("failed to seed older ledger: %v", err)
	}
	newer := models.Ledger{
		UserID: user.ID, Date: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&newer).Error; err != nil {
		t.Fatalf("failed to seed newer ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers", nil)
	var got []ledgerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 2 || got[0].ID != newer.ID || got[1].ID != older.ID {
		t.Fatalf("expected [newer, older] by default, got %+v", got)
	}

	rec = doJSON(t, router, http.MethodGet, "/ledgers?order=asc", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 2 || got[0].ID != older.ID || got[1].ID != newer.ID {
		t.Fatalf("expected [older, newer] with order=asc, got %+v", got)
	}
}

func TestGetLedger_IncludesIncomesAndLedgerBillsWithBill(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	income := models.Income{UserID: user.ID, Date: time.Now(), Amount: decimal.NewFromInt(1000), LedgerID: &ledger.ID}
	if err := db.Create(&income).Error; err != nil {
		t.Fatalf("failed to seed income: %v", err)
	}
	bill := models.Bill{UserID: user.ID, Name: "Mortgage", Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}
	lb := models.LedgerBill{LedgerID: ledger.ID, BillID: &bill.ID, Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&lb).Error; err != nil {
		t.Fatalf("failed to seed ledger bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers/"+ledger.ID.String(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got ledgerDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	if len(got.Incomes) != 1 || got.Incomes[0].ID != income.ID {
		t.Fatalf("expected 1 nested income, got %+v", got.Incomes)
	}
	if len(got.LedgerBills) != 1 || got.LedgerBills[0].Bill.ID != bill.ID {
		t.Fatalf("expected 1 nested ledger bill with bill %s, got %+v", bill.ID, got.LedgerBills)
	}
}

func TestGetLedger_BelongsToAnotherUser_Returns404(t *testing.T) {
	router, db := newLedgerTestRouter(t)

	otherUser := models.User{Sub: "auth0|other", Email: "other@example.com"}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatalf("failed to seed other user: %v", err)
	}
	ledger := models.Ledger{
		UserID: otherUser.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers/"+ledger.ID.String(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateLedger_Succeeds(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(), Name: strPtr("Old"),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodPut, "/ledgers/"+ledger.ID.String(), ledgerRequest{
		Date: time.Now(),
		Name: strPtr("New"),
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeLedgerResponse(t, rec)
	if got.Name == nil || *got.Name != "New" {
		t.Fatalf("expected updated name, got %+v", got.Name)
	}
}

func TestDeleteLedger_NullsIncomeLedgerIdAndCascadesLedgerBills(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	income := models.Income{UserID: user.ID, Date: time.Now(), Amount: decimal.NewFromInt(1000), LedgerID: &ledger.ID}
	if err := db.Create(&income).Error; err != nil {
		t.Fatalf("failed to seed income: %v", err)
	}
	bill := models.Bill{UserID: user.ID, Name: "Mortgage", Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}
	lb := models.LedgerBill{LedgerID: ledger.ID, BillID: &bill.ID, Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&lb).Error; err != nil {
		t.Fatalf("failed to seed ledger bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodDelete, "/ledgers/"+ledger.ID.String(), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	var reloadedIncome models.Income
	if err := db.First(&reloadedIncome, "id = ?", income.ID).Error; err != nil {
		t.Fatalf("expected income to survive ledger delete: %v", err)
	}
	if reloadedIncome.LedgerID != nil {
		t.Fatalf("expected income's ledgerId to be nulled, got %+v", reloadedIncome.LedgerID)
	}

	var ledgerBillCount int64
	db.Model(&models.LedgerBill{}).Where("ledger_id = ?", ledger.ID).Count(&ledgerBillCount)
	if ledgerBillCount != 0 {
		t.Fatalf("expected ledger bill to be cascade-deleted, found %d rows", ledgerBillCount)
	}
}

// --- LedgerBill (nested)

func TestCreateLedgerBill_Succeeds(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	bill := models.Bill{UserID: user.ID, Name: "Mortgage", Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodPost, "/ledgers/"+ledger.ID.String()+"/bills", ledgerBillRequest{
		BillID: &bill.ID,
		Amount: *money.New(150000, "USD"),
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var got ledgerBillResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	if got.LedgerID != ledger.ID || got.BillID == nil || *got.BillID != bill.ID {
		t.Fatalf("expected ledgerId %s and billId %s, got %+v", ledger.ID, bill.ID, got)
	}
}

func TestCreateLedgerBill_BillNotOwnedByUser_Returns400(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	otherUser := models.User{Sub: "auth0|other", Email: "other@example.com"}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatalf("failed to seed other user: %v", err)
	}
	otherBill := models.Bill{UserID: otherUser.ID, Name: "Other", Amount: decimal.NewFromInt(10)}
	if err := db.Create(&otherBill).Error; err != nil {
		t.Fatalf("failed to seed other bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodPost, "/ledgers/"+ledger.ID.String()+"/bills", ledgerBillRequest{
		BillID: &otherBill.ID,
		Amount: *money.New(10000, "USD"),
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateLedgerBill_LedgerNotOwnedByUser_Returns404(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	bill := models.Bill{UserID: user.ID, Name: "Mortgage", Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}
	otherUser := models.User{Sub: "auth0|other", Email: "other@example.com"}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatalf("failed to seed other user: %v", err)
	}
	otherLedger := models.Ledger{
		UserID: otherUser.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&otherLedger).Error; err != nil {
		t.Fatalf("failed to seed other ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodPost, "/ledgers/"+otherLedger.ID.String()+"/bills", ledgerBillRequest{
		BillID: &bill.ID,
		Amount: *money.New(150000, "USD"),
	})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateLedgerBill_TogglesPaid(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	bill := models.Bill{UserID: user.ID, Name: "Mortgage", Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}
	lb := models.LedgerBill{LedgerID: ledger.ID, BillID: &bill.ID, Amount: decimal.NewFromInt(150000), IsPayed: false}
	if err := db.Create(&lb).Error; err != nil {
		t.Fatalf("failed to seed ledger bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodPut, "/ledgers/"+ledger.ID.String()+"/bills/"+lb.ID.String(), ledgerBillRequest{
		BillID:  &bill.ID,
		Amount:  *money.New(150000, "USD"),
		IsPayed: true,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got ledgerBillResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	if !got.IsPayed {
		t.Fatalf("expected isPayed true, got %+v", got)
	}
}

func TestDeleteLedgerBill_Succeeds(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	bill := models.Bill{UserID: user.ID, Name: "Mortgage", Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}
	lb := models.LedgerBill{LedgerID: ledger.ID, BillID: &bill.ID, Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&lb).Error; err != nil {
		t.Fatalf("failed to seed ledger bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodDelete, "/ledgers/"+ledger.ID.String()+"/bills/"+lb.ID.String(), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	var count int64
	db.Model(&models.LedgerBill{}).Where("id = ?", lb.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected ledger bill to be deleted, found %d rows", count)
	}
}

func TestDeleteLedgerBill_NotFound_Returns404(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodDelete, "/ledgers/"+ledger.ID.String()+"/bills/"+uuid.NewString(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCreateLedgerBill_GenericExpense_Succeeds(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodPost, "/ledgers/"+ledger.ID.String()+"/bills", ledgerBillRequest{
		Name:   strPtr("Car repair"),
		Amount: *money.New(45000, "USD"),
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var got ledgerBillResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	if got.BillID != nil {
		t.Fatalf("expected nil billId for generic expense, got %+v", got.BillID)
	}
	if got.Name == nil || *got.Name != "Car repair" {
		t.Fatalf("expected name %q, got %+v", "Car repair", got.Name)
	}
}

func TestCreateLedgerBill_NoBillIdOrName_Returns400(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodPost, "/ledgers/"+ledger.ID.String()+"/bills", ledgerBillRequest{
		Amount: *money.New(45000, "USD"),
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetLedger_GenericExpense_NullBillInResponse(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	lb := models.LedgerBill{LedgerID: ledger.ID, Name: strPtr("Car repair"), Amount: decimal.NewFromInt(450)}
	if err := db.Create(&lb).Error; err != nil {
		t.Fatalf("failed to seed generic expense ledger bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers/"+ledger.ID.String(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got ledgerDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	if len(got.LedgerBills) != 1 {
		t.Fatalf("expected 1 nested ledger bill, got %+v", got.LedgerBills)
	}
	entry := got.LedgerBills[0]
	if entry.Bill != nil {
		t.Fatalf("expected nil bill for generic expense, got %+v", entry.Bill)
	}
	if entry.Name == nil || *entry.Name != "Car repair" {
		t.Fatalf("expected name %q, got %+v", "Car repair", entry.Name)
	}
}

func TestUpdateLedgerBill_GenericExpense_UpdatesName(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	lb := models.LedgerBill{LedgerID: ledger.ID, Name: strPtr("Car repair"), Amount: decimal.NewFromInt(450)}
	if err := db.Create(&lb).Error; err != nil {
		t.Fatalf("failed to seed generic expense ledger bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodPut, "/ledgers/"+ledger.ID.String()+"/bills/"+lb.ID.String(), ledgerBillRequest{
		Name:   strPtr("Car repair (updated)"),
		Amount: *money.New(45000, "USD"),
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got ledgerBillResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	if got.Name == nil || *got.Name != "Car repair (updated)" {
		t.Fatalf("expected updated name, got %+v", got.Name)
	}
}

// --- /ledgers/current and /ledgers/history

func seedLedgerBill(t *testing.T, db *gorm.DB, userID, ledgerID uuid.UUID, amount decimal.Decimal, isPayed bool) models.LedgerBill {
	t.Helper()
	bill := models.Bill{UserID: userID, Name: "test bill", Amount: amount, IsAutoPay: false}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}
	ledgerBill := models.LedgerBill{LedgerID: ledgerID, BillID: &bill.ID, Amount: amount, IsPayed: isPayed}
	if err := db.Create(&ledgerBill).Error; err != nil {
		t.Fatalf("failed to seed ledger bill: %v", err)
	}
	return ledgerBill
}

func decodeCurrentLedgerResponse(t *testing.T, rec *httptest.ResponseRecorder) currentLedgerResponse {
	t.Helper()
	var got currentLedgerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	return got
}

func TestCurrentLedger_ComputesTotals(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.NewFromInt(1000), Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	income := models.Income{UserID: user.ID, Date: time.Now(), Amount: decimal.NewFromInt(500), LedgerID: &ledger.ID}
	if err := db.Create(&income).Error; err != nil {
		t.Fatalf("failed to seed income: %v", err)
	}
	seedLedgerBill(t, db, user.ID, ledger.ID, decimal.NewFromInt(200), true)
	seedLedgerBill(t, db, user.ID, ledger.ID, decimal.NewFromInt(300), false)

	rec := doJSON(t, router, http.MethodGet, "/ledgers/current", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	got := decodeCurrentLedgerResponse(t, rec)
	if !got.AvailableFunds.Equal(decimal.NewFromInt(1500)) {
		t.Fatalf("expected availableFunds 1500, got %s", got.AvailableFunds)
	}
	if !got.Paid.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("expected paid 200, got %s", got.Paid)
	}
	if !got.Planned.Equal(decimal.NewFromInt(300)) {
		t.Fatalf("expected planned 300, got %s", got.Planned)
	}
	if !got.Net.Equal(decimal.NewFromInt(1000)) {
		t.Fatalf("expected net 1000, got %s", got.Net)
	}
	if got.UnpaidCount != 1 {
		t.Fatalf("expected unpaidCount 1, got %d", got.UnpaidCount)
	}
	if got.CheckIn.Status != "good" {
		t.Fatalf("expected status good, got %s", got.CheckIn.Status)
	}
}

func TestCurrentLedger_IncludesName(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(), Name: strPtr("December P1"),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers/current", nil)
	got := decodeCurrentLedgerResponse(t, rec)
	if got.Name == nil || *got.Name != "December P1" {
		t.Fatalf("expected name %q, got %+v", "December P1", got.Name)
	}
}

func TestCurrentLedger_NoLedgers_Returns404(t *testing.T) {
	router, _ := newLedgerTestRouter(t)

	rec := doJSON(t, router, http.MethodGet, "/ledgers/current", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCurrentLedger_OnlyConsidersOwnLedgers(t *testing.T) {
	router, db := newLedgerTestRouter(t)

	otherUser := models.User{Sub: "auth0|other", Email: "other@example.com"}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatalf("failed to seed other user: %v", err)
	}
	otherLedger := models.Ledger{
		UserID: otherUser.ID, Date: time.Now(),
		BankBalance: decimal.NewFromInt(1000), Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&otherLedger).Error; err != nil {
		t.Fatalf("failed to seed other ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers/current", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCurrentLedger_PicksMostRecentByDate(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	older := models.Ledger{
		UserID: user.ID, Date: time.Now().AddDate(0, 0, -14),
		BankBalance: decimal.NewFromInt(1), Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&older).Error; err != nil {
		t.Fatalf("failed to seed older ledger: %v", err)
	}
	newer := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.NewFromInt(2), Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&newer).Error; err != nil {
		t.Fatalf("failed to seed newer ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers/current", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeCurrentLedgerResponse(t, rec)
	if got.ID != newer.ID {
		t.Fatalf("expected the newer ledger %s, got %s", newer.ID, got.ID)
	}
}

func TestCurrentLedger_CheckInStatus_TightAtZero(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers/current", nil)
	got := decodeCurrentLedgerResponse(t, rec)
	if got.CheckIn.Status != "tight" {
		t.Fatalf("expected status tight at net=0, got %s", got.CheckIn.Status)
	}
}

func TestCurrentLedger_CheckInStatus_GoodAtThreshold(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.NewFromInt(100), Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers/current", nil)
	got := decodeCurrentLedgerResponse(t, rec)
	if got.CheckIn.Status != "good" {
		t.Fatalf("expected status good at net=100, got %s", got.CheckIn.Status)
	}
}

func TestCurrentLedger_CheckInStatus_Negative(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.NewFromInt(-1), Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers/current", nil)
	got := decodeCurrentLedgerResponse(t, rec)
	if got.CheckIn.Status != "negative" {
		t.Fatalf("expected status negative at net=-1, got %s", got.CheckIn.Status)
	}
}

func seedLedger(t *testing.T, db *gorm.DB, userID uuid.UUID, date time.Time, bank, income decimal.Decimal) models.Ledger {
	t.Helper()
	ledger := models.Ledger{UserID: userID, Date: date, BankBalance: bank, Income: income, Expenses: decimal.Zero}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	return ledger
}

func TestLedgerHistory_OrdersOldestToNewestWithNet(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	older := seedLedger(t, db, user.ID, time.Now().AddDate(0, 0, -14), decimal.NewFromInt(100), decimal.Zero)
	newer := seedLedger(t, db, user.ID, time.Now(), decimal.NewFromInt(200), decimal.Zero)
	// older: net=100 (availableFunds=100, no bills). newer: net=150 (availableFunds=200, one $50 unpaid bill).
	seedLedgerBill(t, db, user.ID, newer.ID, decimal.NewFromInt(50), false)

	rec := doJSON(t, router, http.MethodGet, "/ledgers/history", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got []ledgerHistoryEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].ID != older.ID || got[1].ID != newer.ID {
		t.Fatalf("expected oldest-first ordering %s,%s, got %s,%s", older.ID, newer.ID, got[0].ID, got[1].ID)
	}
	if !got[0].Net.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("expected older net 100, got %s", got[0].Net)
	}
	if !got[1].Net.Equal(decimal.NewFromInt(150)) {
		t.Fatalf("expected newer net 150, got %s", got[1].Net)
	}
}

func TestLedgerHistory_ZeroAvailableFunds_NetIsZero(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)
	seedLedger(t, db, user.ID, time.Now(), decimal.Zero, decimal.Zero)

	rec := doJSON(t, router, http.MethodGet, "/ledgers/history", nil)
	var got []ledgerHistoryEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 1 || !got[0].Net.IsZero() {
		t.Fatalf("expected a single zero-net entry, got %+v", got)
	}
}

// Reproduces a real bug: a percent-of-available-funds metric forced this
// case to 0 (division by a zero denominator), hiding a badly negative
// cycle. The raw dollar net has no such blind spot.
func TestLedgerHistory_NegativeNet_ZeroAvailableFunds(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)
	ledger := seedLedger(t, db, user.ID, time.Now(), decimal.Zero, decimal.Zero)
	seedLedgerBill(t, db, user.ID, ledger.ID, decimal.NewFromInt(1723), false)

	rec := doJSON(t, router, http.MethodGet, "/ledgers/history", nil)
	var got []ledgerHistoryEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if !got[0].Net.Equal(decimal.NewFromInt(-1723)) {
		t.Fatalf("expected net -1723 even with zero available funds, got %s", got[0].Net)
	}
}

func TestLedgerHistory_RespectsLimitParam(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)

	for i := 0; i < 5; i++ {
		seedLedger(t, db, user.ID, time.Now().AddDate(0, 0, -i), decimal.NewFromInt(1), decimal.Zero)
	}

	rec := doJSON(t, router, http.MethodGet, "/ledgers/history?limit=2", nil)
	var got []ledgerHistoryEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries with limit=2, got %d", len(got))
	}
}

func TestLedgerHistory_OnlyReturnsOwnLedgers(t *testing.T) {
	router, db := newLedgerTestRouter(t)
	user := seedCurrentUser(t, db)
	seedLedger(t, db, user.ID, time.Now(), decimal.NewFromInt(1), decimal.Zero)

	otherUser := models.User{Sub: "auth0|other", Email: "other@example.com"}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatalf("failed to seed other user: %v", err)
	}
	seedLedger(t, db, otherUser.ID, time.Now(), decimal.NewFromInt(999), decimal.Zero)

	rec := doJSON(t, router, http.MethodGet, "/ledgers/history", nil)
	var got []ledgerHistoryEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the current user's ledger, got %+v", got)
	}
}
