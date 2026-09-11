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

func newBillTestRouter(t *testing.T) (http.Handler, *gorm.DB) {
	db := testdb.New(t, &models.User{}, &models.Ledger{}, &models.Income{}, &models.Bill{}, &models.LedgerBill{})
	router := NewRouter(db, auth.MockVerifier{DB: db})
	return router, db
}

func decodeBillResponse(t *testing.T, rec *httptest.ResponseRecorder) billResponse {
	t.Helper()
	var got billResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	return got
}

func TestCreateBill_Succeeds(t *testing.T) {
	router, _ := newBillTestRouter(t)

	dueDay := 15
	rec := doJSON(t, router, http.MethodPost, "/bills", billRequest{
		Name:      "College Fund",
		Amount:    *money.New(20000, "USD"),
		DueDay:    &dueDay,
		IsAutoPay: true,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeBillResponse(t, rec)
	if got.ID == uuid.Nil {
		t.Fatal("expected a generated id")
	}
	if got.Amount.Amount() != 20000 {
		t.Fatalf("expected amount 200 USD, got %+v", got.Amount)
	}
	if got.DueDay == nil || *got.DueDay != 15 {
		t.Fatalf("expected dueDay 15, got %+v", got.DueDay)
	}
}

func TestCreateBill_MissingName_Returns400(t *testing.T) {
	router, _ := newBillTestRouter(t)

	rec := doJSON(t, router, http.MethodPost, "/bills", billRequest{
		Amount: *money.New(20000, "USD"),
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateBill_MissingAmount_Returns400(t *testing.T) {
	router, _ := newBillTestRouter(t)

	rec := doJSON(t, router, http.MethodPost, "/bills", billRequest{
		Name: "College Fund",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateBill_InvalidDueDay_Returns400(t *testing.T) {
	router, _ := newBillTestRouter(t)

	dueDay := 32
	rec := doJSON(t, router, http.MethodPost, "/bills", billRequest{
		Name:   "College Fund",
		Amount: *money.New(20000, "USD"),
		DueDay: &dueDay,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListBills_OnlyReturnsCurrentUsersBills(t *testing.T) {
	router, db := newBillTestRouter(t)
	user := seedCurrentUser(t, db)

	mine := models.Bill{UserID: user.ID, Name: "Mine", Amount: decimal.NewFromInt(10), IsAutoPay: true}
	if err := db.Create(&mine).Error; err != nil {
		t.Fatalf("failed to seed own bill: %v", err)
	}

	otherUser := models.User{Sub: "auth0|other", Email: "other@example.com"}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatalf("failed to seed other user: %v", err)
	}
	other := models.Bill{UserID: otherUser.ID, Name: "Other", Amount: decimal.NewFromInt(20), IsAutoPay: false}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("failed to seed other bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/bills", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got []billResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 1 || got[0].ID != mine.ID {
		t.Fatalf("expected exactly the current user's bill, got %+v", got)
	}
}

func TestListBills_OrderParam_DefaultsNewestFirst(t *testing.T) {
	router, db := newBillTestRouter(t)
	user := seedCurrentUser(t, db)

	older := models.Bill{
		Base:   models.Base{CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		UserID: user.ID, Name: "Older", Amount: decimal.NewFromInt(10),
	}
	if err := db.Create(&older).Error; err != nil {
		t.Fatalf("failed to seed older bill: %v", err)
	}
	newer := models.Bill{
		Base:   models.Base{CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		UserID: user.ID, Name: "Newer", Amount: decimal.NewFromInt(20),
	}
	if err := db.Create(&newer).Error; err != nil {
		t.Fatalf("failed to seed newer bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/bills", nil)
	var got []billResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 2 || got[0].ID != newer.ID || got[1].ID != older.ID {
		t.Fatalf("expected [newer, older] by default, got %+v", got)
	}

	rec = doJSON(t, router, http.MethodGet, "/bills?order=asc", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 2 || got[0].ID != older.ID || got[1].ID != newer.ID {
		t.Fatalf("expected [older, newer] with order=asc, got %+v", got)
	}
}

func TestGetBill_IncludesLedgerBillsWithLedger(t *testing.T) {
	router, db := newBillTestRouter(t)
	user := seedCurrentUser(t, db)

	bill := models.Bill{UserID: user.ID, Name: "Mortgage", Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}
	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(), Name: strPtr("December P1"),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	lb := models.LedgerBill{LedgerID: ledger.ID, BillID: &bill.ID, Amount: decimal.NewFromInt(150000), IsPayed: true}
	if err := db.Create(&lb).Error; err != nil {
		t.Fatalf("failed to seed ledger bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/bills/"+bill.ID.String(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got billDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	if len(got.LedgerBills) != 1 {
		t.Fatalf("expected 1 nested ledger bill, got %+v", got.LedgerBills)
	}
	if got.LedgerBills[0].Ledger.ID != ledger.ID {
		t.Fatalf("expected nested ledger %s, got %+v", ledger.ID, got.LedgerBills[0].Ledger)
	}
}

func TestGetBill_BelongsToAnotherUser_Returns404(t *testing.T) {
	router, db := newBillTestRouter(t)

	otherUser := models.User{Sub: "auth0|other", Email: "other@example.com"}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatalf("failed to seed other user: %v", err)
	}
	bill := models.Bill{UserID: otherUser.ID, Name: "Other", Amount: decimal.NewFromInt(10)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodGet, "/bills/"+bill.ID.String(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateBill_Succeeds(t *testing.T) {
	router, db := newBillTestRouter(t)
	user := seedCurrentUser(t, db)

	bill := models.Bill{UserID: user.ID, Name: "Old Name", Amount: decimal.NewFromInt(10)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodPut, "/bills/"+bill.ID.String(), billRequest{
		Name:   "New Name",
		Amount: *money.New(30000, "USD"),
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeBillResponse(t, rec)
	if got.Name != "New Name" || got.Amount.Amount() != 30000 {
		t.Fatalf("expected updated fields, got %+v", got)
	}
}

func TestUpdateBill_BelongsToAnotherUser_Returns404(t *testing.T) {
	router, db := newBillTestRouter(t)

	otherUser := models.User{Sub: "auth0|other", Email: "other@example.com"}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatalf("failed to seed other user: %v", err)
	}
	bill := models.Bill{UserID: otherUser.ID, Name: "Other", Amount: decimal.NewFromInt(10)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodPut, "/bills/"+bill.ID.String(), billRequest{
		Name:   "New Name",
		Amount: *money.New(30000, "USD"),
	})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteBill_CascadesLedgerBills(t *testing.T) {
	router, db := newBillTestRouter(t)
	user := seedCurrentUser(t, db)

	bill := models.Bill{UserID: user.ID, Name: "Mortgage", Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to seed bill: %v", err)
	}
	ledger := models.Ledger{
		UserID: user.ID, Date: time.Now(),
		BankBalance: decimal.Zero, Income: decimal.Zero, Expenses: decimal.Zero,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to seed ledger: %v", err)
	}
	lb := models.LedgerBill{LedgerID: ledger.ID, BillID: &bill.ID, Amount: decimal.NewFromInt(150000)}
	if err := db.Create(&lb).Error; err != nil {
		t.Fatalf("failed to seed ledger bill: %v", err)
	}

	rec := doJSON(t, router, http.MethodDelete, "/bills/"+bill.ID.String(), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	var billCount, ledgerBillCount int64
	db.Model(&models.Bill{}).Where("id = ?", bill.ID).Count(&billCount)
	db.Model(&models.LedgerBill{}).Where("bill_id = ?", bill.ID).Count(&ledgerBillCount)
	if billCount != 0 {
		t.Fatalf("expected bill to be deleted, found %d rows", billCount)
	}
	if ledgerBillCount != 0 {
		t.Fatalf("expected ledger bill to be cascade-deleted, found %d rows", ledgerBillCount)
	}
}

func TestDeleteBill_NotFound_Returns404(t *testing.T) {
	router, _ := newBillTestRouter(t)

	rec := doJSON(t, router, http.MethodDelete, "/bills/"+uuid.NewString(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func strPtr(s string) *string { return &s }
