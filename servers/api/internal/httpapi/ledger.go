package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	money "github.com/Rhymond/go-money"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/mrcunninghamz/money-bae/servers/api/internal/auth"
	"github.com/mrcunninghamz/money-bae/servers/api/internal/models"
)

type ledgerRequest struct {
	Date        time.Time    `json:"date"`
	Name        *string      `json:"name"`
	BankBalance *money.Money `json:"bankBalance"`
	Income      *money.Money `json:"income"`
	Expenses    *money.Money `json:"expenses"`
	Notes       *string      `json:"notes"`
}

// validatedLedger is a decoded, fully-validated ledgerRequest. Net/Total
// aren't part of it — they're always server-computed (resolveNet/
// resolveTotal below), the same way tui/'s Postgres schema defined them as
// GENERATED ALWAYS AS columns the client could never set directly.
type validatedLedger struct {
	Date        time.Time
	Name        *string
	BankBalance decimal.Decimal
	Income      decimal.Decimal
	Expenses    decimal.Decimal
	Notes       *string
}

// resolveNet mirrors tui/'s `net NUMERIC GENERATED ALWAYS AS (bank_balance +
// income - expenses) STORED`.
func resolveNet(v validatedLedger) decimal.Decimal {
	return v.BankBalance.Add(v.Income).Sub(v.Expenses)
}

// resolveTotal mirrors tui/'s `total NUMERIC GENERATED ALWAYS AS
// (bank_balance + income) STORED`.
func resolveTotal(v validatedLedger) decimal.Decimal {
	return v.BankBalance.Add(v.Income)
}

// recalculateLedgerTotals mirrors tui/'s recalculate_ledger_totals()
// Postgres trigger (tui/migrations/.../add_ledger_totals_triggers), applied
// here instead of at the DB layer: sums a ledger's attached Incomes and
// LedgerBills into its income/expenses columns, then rederives net/total
// from them. Call this after any write that attaches, edits, or removes an
// Income or LedgerBill — those never touch the ledger row itself
// otherwise, so its stored totals would otherwise go stale.
// Uses SQL SUM() directly, not a Go-side loop — safe because
// decimal.Decimal columns are genuinely `numeric` (see
// internal/migrations' "decimal_columns_to_numeric" migration and the
// gorm:"type:numeric" tags in internal/models), not the untyped-text
// columns AutoMigrate defaulted to before that.
func recalculateLedgerTotals(db *gorm.DB, ledgerID uuid.UUID) error {
	var ledger models.Ledger
	if err := db.Select("bank_balance").First(&ledger, "id = ?", ledgerID).Error; err != nil {
		return err
	}

	var income decimal.Decimal
	if err := db.Model(&models.Income{}).Where("ledger_id = ?", ledgerID).
		Select("COALESCE(SUM(amount), 0)").Row().Scan(&income); err != nil {
		return err
	}

	var expenses decimal.Decimal
	if err := db.Model(&models.LedgerBill{}).Where("ledger_id = ?", ledgerID).
		Select("COALESCE(SUM(amount), 0)").Row().Scan(&expenses); err != nil {
		return err
	}

	v := validatedLedger{BankBalance: ledger.BankBalance, Income: income, Expenses: expenses}
	return db.Model(&models.Ledger{}).Where("id = ?", ledgerID).Updates(map[string]any{
		"income":   income,
		"expenses": expenses,
		"net":      resolveNet(v),
		"total":    resolveTotal(v),
	}).Error
}

// decodeOptionalMoney decodes an optional money field, defaulting to zero
// when the field is absent, but rejecting a present-but-wrong-currency value.
func decodeOptionalMoney(w http.ResponseWriter, fieldName string, m *money.Money) (decimal.Decimal, bool) {
	if m == nil {
		return decimal.Zero, true
	}
	amount, ok := moneyToDecimal(*m)
	if !ok {
		http.Error(w, fieldName+" must be in USD", http.StatusBadRequest)
		return decimal.Decimal{}, false
	}
	return amount, true
}

func decodeLedgerRequest(w http.ResponseWriter, r *http.Request) (validatedLedger, bool) {
	var req ledgerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return validatedLedger{}, false
	}
	if req.Date.IsZero() {
		http.Error(w, "date is required", http.StatusBadRequest)
		return validatedLedger{}, false
	}

	bankBalance, ok := decodeOptionalMoney(w, "bankBalance", req.BankBalance)
	if !ok {
		return validatedLedger{}, false
	}
	income, ok := decodeOptionalMoney(w, "income", req.Income)
	if !ok {
		return validatedLedger{}, false
	}
	expenses, ok := decodeOptionalMoney(w, "expenses", req.Expenses)
	if !ok {
		return validatedLedger{}, false
	}

	return validatedLedger{
		Date:        req.Date,
		Name:        req.Name,
		BankBalance: bankBalance,
		Income:      income,
		Expenses:    expenses,
		Notes:       req.Notes,
	}, true
}

type ledgerResponse struct {
	ID          uuid.UUID    `json:"id"`
	Date        time.Time    `json:"date"`
	Name        *string      `json:"name"`
	BankBalance money.Money  `json:"bankBalance"`
	Income      money.Money  `json:"income"`
	Expenses    money.Money  `json:"expenses"`
	Net         money.Money  `json:"net"`
	Total       *money.Money `json:"total"`
	Notes       *string      `json:"notes"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

func toLedgerResponse(ledger models.Ledger) ledgerResponse {
	net := decimal.Zero
	if ledger.Net != nil {
		net = *ledger.Net
	}
	var total *money.Money
	if ledger.Total != nil {
		t := decimalToMoney(*ledger.Total)
		total = &t
	}
	return ledgerResponse{
		ID:          ledger.ID,
		Date:        ledger.Date,
		Name:        ledger.Name,
		BankBalance: decimalToMoney(ledger.BankBalance),
		Income:      decimalToMoney(ledger.Income),
		Expenses:    decimalToMoney(ledger.Expenses),
		Net:         decimalToMoney(net),
		Total:       total,
		Notes:       ledger.Notes,
		CreatedAt:   ledger.CreatedAt,
		UpdatedAt:   ledger.UpdatedAt,
	}
}

// ledgerDetailResponse is the "detail" shape (GET /ledgers/{id} only): the
// ledger's own fields plus its navigational properties — every income and
// bill-in-cycle attached to it, each bill-in-cycle including its catalog
// Bill. List/create/update responses stay lean (ledgerResponse).
type ledgerDetailResponse struct {
	ledgerResponse
	Incomes     []incomeResponse     `json:"incomes"`
	LedgerBills []ledgerBillWithBill `json:"ledgerBills"`
}

func toLedgerDetailResponse(ledger models.Ledger) ledgerDetailResponse {
	incomes := make([]incomeResponse, len(ledger.Incomes))
	for i, income := range ledger.Incomes {
		incomes[i] = toIncomeResponse(income)
	}
	bills := make([]ledgerBillWithBill, len(ledger.LedgerBills))
	for i, lb := range ledger.LedgerBills {
		bills[i] = toLedgerBillWithBill(lb)
	}
	return ledgerDetailResponse{
		ledgerResponse: toLedgerResponse(ledger),
		Incomes:        incomes,
		LedgerBills:    bills,
	}
}

func findOwnedLedger(w http.ResponseWriter, r *http.Request, db *gorm.DB, userID uuid.UUID, pathParam string) (models.Ledger, bool) {
	var ledger models.Ledger
	id, err := uuid.Parse(r.PathValue(pathParam))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return ledger, false
	}
	err = db.Where("id = ? AND user_id = ?", id, userID).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "ledger not found", http.StatusNotFound)
		return ledger, false
	}
	if err != nil {
		internalError(w, r, "failed to look up ledger", err)
		return ledger, false
	}
	return ledger, true
}

func createLedgerHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "no authenticated user", http.StatusInternalServerError)
			return
		}

		req, ok := decodeLedgerRequest(w, r)
		if !ok {
			return
		}

		net := resolveNet(req)
		total := resolveTotal(req)
		ledger := models.Ledger{
			UserID:      principal.UserID,
			Date:        req.Date,
			Name:        req.Name,
			BankBalance: req.BankBalance,
			Income:      req.Income,
			Expenses:    req.Expenses,
			Net:         &net,
			Total:       &total,
			Notes:       req.Notes,
		}

		if err := db.Create(&ledger).Error; err != nil {
			internalError(w, r, "failed to create ledger", err)
			return
		}

		writeJSON(w, http.StatusCreated, toLedgerResponse(ledger))
	}
}

func listLedgersHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "no authenticated user", http.StatusInternalServerError)
			return
		}

		var ledgers []models.Ledger
		order := "date " + orderDirection(r)
		if err := db.Where("user_id = ?", principal.UserID).Order(order).Find(&ledgers).Error; err != nil {
			internalError(w, r, "failed to list ledgers", err)
			return
		}

		responses := make([]ledgerResponse, len(ledgers))
		for i, ledger := range ledgers {
			responses[i] = toLedgerResponse(ledger)
		}

		writeJSON(w, http.StatusOK, responses)
	}
}

func getLedgerHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "no authenticated user", http.StatusInternalServerError)
			return
		}

		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var ledger models.Ledger
		err = db.Preload("Incomes").Preload("LedgerBills.Bill").
			Where("id = ? AND user_id = ?", id, principal.UserID).First(&ledger).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "ledger not found", http.StatusNotFound)
			return
		}
		if err != nil {
			internalError(w, r, "failed to look up ledger", err)
			return
		}

		writeJSON(w, http.StatusOK, toLedgerDetailResponse(ledger))
	}
}

func updateLedgerHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "no authenticated user", http.StatusInternalServerError)
			return
		}

		ledger, ok := findOwnedLedger(w, r, db, principal.UserID, "id")
		if !ok {
			return
		}

		req, ok := decodeLedgerRequest(w, r)
		if !ok {
			return
		}

		net := resolveNet(req)
		total := resolveTotal(req)
		ledger.Date = req.Date
		ledger.Name = req.Name
		ledger.BankBalance = req.BankBalance
		ledger.Income = req.Income
		ledger.Expenses = req.Expenses
		ledger.Net = &net
		ledger.Total = &total
		ledger.Notes = req.Notes

		if err := db.Save(&ledger).Error; err != nil {
			internalError(w, r, "failed to update ledger", err)
			return
		}

		writeJSON(w, http.StatusOK, toLedgerResponse(ledger))
	}
}

func deleteLedgerHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "no authenticated user", http.StatusInternalServerError)
			return
		}

		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			var ledger models.Ledger
			if err := tx.Where("id = ? AND user_id = ?", id, principal.UserID).First(&ledger).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.Income{}).Where("ledger_id = ?", id).Update("ledger_id", nil).Error; err != nil {
				return err
			}
			if err := tx.Where("ledger_id = ?", id).Delete(&models.LedgerBill{}).Error; err != nil {
				return err
			}
			return tx.Delete(&ledger).Error
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "ledger not found", http.StatusNotFound)
			return
		}
		if err != nil {
			internalError(w, r, "failed to delete ledger", err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// --- /ledgers/current and /ledgers/history: read-only aggregate views for
// the home dashboard, computed from the same Ledger/LedgerBill rows as the
// CRUD endpoints above rather than stored separately.

// defaultLedgerHistoryLimit matches the home dashboard's "net by cycle"
// chart, which shows 8 bars.
const defaultLedgerHistoryLimit = 8

// checkInThreshold is the free-cash line between a "good" and a "tight"
// bae check-in.
var checkInThreshold = decimal.NewFromInt(100)

type checkInResponse struct {
	Status string `json:"status"`
}

type currentLedgerResponse struct {
	ID             uuid.UUID       `json:"id"`
	Date           time.Time       `json:"date"`
	Name           *string         `json:"name"`
	AvailableFunds decimal.Decimal `json:"availableFunds"`
	Paid           decimal.Decimal `json:"paid"`
	Planned        decimal.Decimal `json:"planned"`
	Net            decimal.Decimal `json:"net"`
	UnpaidCount    int             `json:"unpaidCount"`
	CheckIn        checkInResponse `json:"checkIn"`
}

// checkInStatus classifies free cash (net) into the bae check-in's three
// moods: "good" at $100 or more, "tight" from $0 up to $100, "negative"
// below $0.
func checkInStatus(net decimal.Decimal) string {
	if net.LessThan(decimal.Zero) {
		return "negative"
	}
	if net.LessThan(checkInThreshold) {
		return "tight"
	}
	return "good"
}

// cycleTotals is a ledger's cash position, derived from its stored
// BankBalance plus its associated Incomes and LedgerBills. ledger.Income is
// a stale total from the legacy import, disconnected from which Income rows
// are actually attached (they can be attached/removed after the fact via
// the ledger detail page) — sum the real rows instead of trusting it.
type cycleTotals struct {
	availableFunds decimal.Decimal
	paid           decimal.Decimal
	planned        decimal.Decimal
	net            decimal.Decimal
	unpaidCount    int
}

func computeCycleTotals(ledger models.Ledger) cycleTotals {
	income := decimal.Zero
	for _, inc := range ledger.Incomes {
		income = income.Add(inc.Amount)
	}
	availableFunds := ledger.BankBalance.Add(income)
	var paid, planned decimal.Decimal
	unpaidCount := 0
	for _, lb := range ledger.LedgerBills {
		if lb.IsPayed {
			paid = paid.Add(lb.Amount)
		} else {
			planned = planned.Add(lb.Amount)
			unpaidCount++
		}
	}
	return cycleTotals{
		availableFunds: availableFunds,
		paid:           paid,
		planned:        planned,
		net:            availableFunds.Sub(paid).Sub(planned),
		unpaidCount:    unpaidCount,
	}
}

func toCurrentLedgerResponse(ledger models.Ledger) currentLedgerResponse {
	totals := computeCycleTotals(ledger)
	return currentLedgerResponse{
		ID:             ledger.ID,
		Date:           ledger.Date,
		Name:           ledger.Name,
		AvailableFunds: totals.availableFunds,
		Paid:           totals.paid,
		Planned:        totals.planned,
		Net:            totals.net,
		UnpaidCount:    totals.unpaidCount,
		CheckIn:        checkInResponse{Status: checkInStatus(totals.net)},
	}
}

func currentLedgerHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "no authenticated user", http.StatusInternalServerError)
			return
		}

		var ledger models.Ledger
		err := db.Where("user_id = ?", principal.UserID).
			Preload("Incomes").
			Preload("LedgerBills").
			Order("date DESC").
			First(&ledger).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "no cycles found", http.StatusNotFound)
			return
		}
		if err != nil {
			internalError(w, r, "failed to look up current cycle", err)
			return
		}

		writeJSON(w, http.StatusOK, toCurrentLedgerResponse(ledger))
	}
}

type ledgerHistoryEntry struct {
	ID   uuid.UUID       `json:"id"`
	Date time.Time       `json:"date"`
	Name *string         `json:"name"`
	Net  decimal.Decimal `json:"net"`
}

func toLedgerHistoryEntry(ledger models.Ledger) ledgerHistoryEntry {
	totals := computeCycleTotals(ledger)
	return ledgerHistoryEntry{
		ID:   ledger.ID,
		Date: ledger.Date,
		Name: ledger.Name,
		Net:  totals.net,
	}
}

func ledgerHistoryHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "no authenticated user", http.StatusInternalServerError)
			return
		}

		limit := defaultLedgerHistoryLimit
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			parsed, err := strconv.Atoi(limitStr)
			if err != nil || parsed < 1 {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
			limit = parsed
		}

		var ledgers []models.Ledger
		err := db.Where("user_id = ?", principal.UserID).
			Preload("Incomes").
			Preload("LedgerBills").
			Order("date DESC").
			Limit(limit).
			Find(&ledgers).Error
		if err != nil {
			internalError(w, r, "failed to list cycle history", err)
			return
		}

		entries := make([]ledgerHistoryEntry, len(ledgers))
		for i, ledger := range ledgers {
			// ledgers is newest-first (for LIMIT to keep the most recent
			// N); the chart reads oldest-first, so reverse on the way out.
			entries[len(ledgers)-1-i] = toLedgerHistoryEntry(ledger)
		}

		writeJSON(w, http.StatusOK, entries)
	}
}

// --- LedgerBill: a bill linked into one cycle, with its own paid status.
// Nested under a ledger — no standalone GET; reads happen via
// GET /ledgers/{id}'s embedded ledgerBills.

type ledgerBillRequest struct {
	BillID  *uuid.UUID  `json:"billId"`
	Name    *string     `json:"name"`
	Amount  money.Money `json:"amount"`
	DueDay  *int        `json:"dueDay"`
	IsPayed bool        `json:"isPayed"`
	Notes   *string     `json:"notes"`
}

type ledgerBillResponse struct {
	ID        uuid.UUID   `json:"id"`
	LedgerID  uuid.UUID   `json:"ledgerId"`
	BillID    *uuid.UUID  `json:"billId"`
	Name      *string     `json:"name"`
	Amount    money.Money `json:"amount"`
	DueDay    *int        `json:"dueDay"`
	IsPayed   bool        `json:"isPayed"`
	Notes     *string     `json:"notes"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

func toLedgerBillResponse(lb models.LedgerBill) ledgerBillResponse {
	return ledgerBillResponse{
		ID:        lb.ID,
		LedgerID:  lb.LedgerID,
		BillID:    lb.BillID,
		Name:      lb.Name,
		Amount:    decimalToMoney(lb.Amount),
		DueDay:    timeToDueDay(lb.DueDay),
		IsPayed:   lb.IsPayed,
		Notes:     lb.Notes,
		CreatedAt: lb.CreatedAt,
		UpdatedAt: lb.UpdatedAt,
	}
}

// ledgerBillWithBill is what GET /ledgers/{id} embeds for each bill-in-cycle
// — the ledger-bill's own fields plus the catalog Bill it references (name,
// default amount, auto-pay, ...), since that's what the UI needs to render
// the row without a second round trip. Bill is nil for a generic expense
// (no catalog Bill behind it) — the UI falls back to the ledger-bill's own
// Name in that case.
type ledgerBillWithBill struct {
	ledgerBillResponse
	Bill *billResponse `json:"bill"`
}

func toLedgerBillWithBill(lb models.LedgerBill) ledgerBillWithBill {
	var bill *billResponse
	if lb.Bill != nil {
		b := toBillResponse(*lb.Bill)
		bill = &b
	}
	return ledgerBillWithBill{
		ledgerBillResponse: toLedgerBillResponse(lb),
		Bill:               bill,
	}
}

// ledgerBillWithLedger is the mirror image, embedded by GET /bills/{id} to
// show a bill's usage history across cycles.
type ledgerBillWithLedger struct {
	ledgerBillResponse
	Ledger ledgerResponse `json:"ledger"`
}

func toLedgerBillWithLedger(lb models.LedgerBill) ledgerBillWithLedger {
	return ledgerBillWithLedger{
		ledgerBillResponse: toLedgerBillResponse(lb),
		Ledger:             toLedgerResponse(lb.Ledger),
	}
}

func decodeLedgerBillRequest(w http.ResponseWriter, r *http.Request, db *gorm.DB, userID uuid.UUID) (models.LedgerBill, bool) {
	var req ledgerBillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return models.LedgerBill{}, false
	}
	// A generic expense (billId absent) needs its own Name, since there's
	// no catalog Bill to borrow one from; a real bill still needs its
	// ownership checked, and its Name (if any) is ignored in favor of
	// Bill.Name.
	var name *string
	if req.BillID != nil {
		if !billOwnedByUser(db, *req.BillID, userID) {
			http.Error(w, "bill not found", http.StatusBadRequest)
			return models.LedgerBill{}, false
		}
	} else {
		if req.Name == nil || *req.Name == "" {
			http.Error(w, "name is required when billId is absent", http.StatusBadRequest)
			return models.LedgerBill{}, false
		}
		name = req.Name
	}
	amount, ok := moneyToDecimal(req.Amount)
	if !ok {
		http.Error(w, "amount is required, in USD", http.StatusBadRequest)
		return models.LedgerBill{}, false
	}
	if req.DueDay != nil && (*req.DueDay < 1 || *req.DueDay > 31) {
		http.Error(w, "dueDay must be between 1 and 31", http.StatusBadRequest)
		return models.LedgerBill{}, false
	}
	return models.LedgerBill{
		BillID:  req.BillID,
		Name:    name,
		Amount:  amount,
		DueDay:  dueDayToTime(req.DueDay),
		IsPayed: req.IsPayed,
		Notes:   req.Notes,
	}, true
}

func createLedgerBillHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "no authenticated user", http.StatusInternalServerError)
			return
		}

		ledger, ok := findOwnedLedger(w, r, db, principal.UserID, "ledgerId")
		if !ok {
			return
		}

		lb, ok := decodeLedgerBillRequest(w, r, db, principal.UserID)
		if !ok {
			return
		}
		lb.LedgerID = ledger.ID

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&lb).Error; err != nil {
				return err
			}
			return recalculateLedgerTotals(tx, ledger.ID)
		})
		if err != nil {
			internalError(w, r, "failed to create ledger bill", err)
			return
		}

		writeJSON(w, http.StatusCreated, toLedgerBillResponse(lb))
	}
}

func findOwnedLedgerBill(w http.ResponseWriter, r *http.Request, db *gorm.DB, userID uuid.UUID) (models.LedgerBill, bool) {
	var lb models.LedgerBill
	ledgerID, err := uuid.Parse(r.PathValue("ledgerId"))
	if err != nil {
		http.Error(w, "invalid ledger id", http.StatusBadRequest)
		return lb, false
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return lb, false
	}
	if !ledgerOwnedByUser(db, ledgerID, userID) {
		http.Error(w, "ledger not found", http.StatusNotFound)
		return lb, false
	}
	err = db.Where("id = ? AND ledger_id = ?", id, ledgerID).First(&lb).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "ledger bill not found", http.StatusNotFound)
		return lb, false
	}
	if err != nil {
		internalError(w, r, "failed to look up ledger bill", err)
		return lb, false
	}
	return lb, true
}

func updateLedgerBillHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "no authenticated user", http.StatusInternalServerError)
			return
		}

		lb, ok := findOwnedLedgerBill(w, r, db, principal.UserID)
		if !ok {
			return
		}

		req, ok := decodeLedgerBillRequest(w, r, db, principal.UserID)
		if !ok {
			return
		}

		lb.BillID = req.BillID
		lb.Name = req.Name
		lb.Amount = req.Amount
		lb.DueDay = req.DueDay
		lb.IsPayed = req.IsPayed
		lb.Notes = req.Notes

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(&lb).Error; err != nil {
				return err
			}
			return recalculateLedgerTotals(tx, lb.LedgerID)
		})
		if err != nil {
			internalError(w, r, "failed to update ledger bill", err)
			return
		}

		writeJSON(w, http.StatusOK, toLedgerBillResponse(lb))
	}
}

func deleteLedgerBillHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "no authenticated user", http.StatusInternalServerError)
			return
		}

		ledgerID, err := uuid.Parse(r.PathValue("ledgerId"))
		if err != nil {
			http.Error(w, "invalid ledger id", http.StatusBadRequest)
			return
		}
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if !ledgerOwnedByUser(db, ledgerID, principal.UserID) {
			http.Error(w, "ledger not found", http.StatusNotFound)
			return
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			result := tx.Where("id = ? AND ledger_id = ?", id, ledgerID).Delete(&models.LedgerBill{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
			return recalculateLedgerTotals(tx, ledgerID)
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "ledger bill not found", http.StatusNotFound)
			return
		}
		if err != nil {
			internalError(w, r, "failed to delete ledger bill", err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
