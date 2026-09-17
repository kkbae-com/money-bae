package httpapi

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/mrcunninghamz/money-bae/servers/api/internal/auth"
)

func NewRouter(db *gorm.DB, verifier auth.Verifier) http.Handler {
	requireAuth := auth.RequireAuth(verifier)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(db))
	mux.Handle("GET /me", requireAuth(meHandler()))
	mux.Handle("POST /incomes", requireAuth(createIncomeHandler(db)))
	mux.Handle("GET /incomes", requireAuth(listIncomesHandler(db)))
	mux.Handle("GET /incomes/{id}", requireAuth(getIncomeHandler(db)))
	mux.Handle("PUT /incomes/{id}", requireAuth(updateIncomeHandler(db)))
	mux.Handle("DELETE /incomes/{id}", requireAuth(deleteIncomeHandler(db)))

	mux.Handle("POST /bills", requireAuth(createBillHandler(db)))
	mux.Handle("GET /bills", requireAuth(listBillsHandler(db)))
	mux.Handle("GET /bills/{id}", requireAuth(getBillHandler(db)))
	mux.Handle("PUT /bills/{id}", requireAuth(updateBillHandler(db)))
	mux.Handle("DELETE /bills/{id}", requireAuth(deleteBillHandler(db)))

	mux.Handle("GET /ledgers/current", requireAuth(currentLedgerHandler(db)))
	mux.Handle("GET /ledgers/history", requireAuth(ledgerHistoryHandler(db)))
	mux.Handle("POST /ledgers", requireAuth(createLedgerHandler(db)))
	mux.Handle("GET /ledgers", requireAuth(listLedgersHandler(db)))
	mux.Handle("GET /ledgers/{id}", requireAuth(getLedgerHandler(db)))
	mux.Handle("POST /ledgers/{id}/duplicate", requireAuth(duplicateLedgerHandler(db)))
	mux.Handle("PUT /ledgers/{id}", requireAuth(updateLedgerHandler(db)))
	mux.Handle("DELETE /ledgers/{id}", requireAuth(deleteLedgerHandler(db)))
	mux.Handle("POST /ledgers/{ledgerId}/bills", requireAuth(createLedgerBillHandler(db)))
	mux.Handle("PUT /ledgers/{ledgerId}/bills/{id}", requireAuth(updateLedgerBillHandler(db)))
	mux.Handle("DELETE /ledgers/{ledgerId}/bills/{id}", requireAuth(deleteLedgerBillHandler(db)))

	mux.Handle("GET /ptos/current", requireAuth(currentPtoHandler(db)))
	mux.Handle("POST /ptos", requireAuth(createPtoHandler(db)))
	mux.Handle("GET /ptos", requireAuth(listPtosHandler(db)))
	mux.Handle("GET /ptos/{id}", requireAuth(getPtoHandler(db)))
	mux.Handle("PUT /ptos/{id}", requireAuth(updatePtoHandler(db)))
	mux.Handle("DELETE /ptos/{id}", requireAuth(deletePtoHandler(db)))
	mux.Handle("POST /ptos/{ptoId}/plans", requireAuth(createPtoPlanHandler(db)))
	mux.Handle("PUT /ptos/{ptoId}/plans/{id}", requireAuth(updatePtoPlanHandler(db)))
	mux.Handle("DELETE /ptos/{ptoId}/plans/{id}", requireAuth(deletePtoPlanHandler(db)))
	mux.Handle("POST /ptos/{ptoId}/holidays", requireAuth(createHolidayHourHandler(db)))
	mux.Handle("PUT /ptos/{ptoId}/holidays/{id}", requireAuth(updateHolidayHourHandler(db)))
	mux.Handle("DELETE /ptos/{ptoId}/holidays/{id}", requireAuth(deleteHolidayHourHandler(db)))

	return corsMiddleware(loggingMiddleware(mux))
}
