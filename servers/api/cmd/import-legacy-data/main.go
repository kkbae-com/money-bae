package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mrcunninghamz/money-bae/servers/api/internal/auth"
	"github.com/mrcunninghamz/money-bae/servers/api/internal/config"
	"github.com/mrcunninghamz/money-bae/servers/api/internal/database"
	"github.com/mrcunninghamz/money-bae/servers/api/internal/migrations"
	"github.com/mrcunninghamz/money-bae/servers/api/internal/models"
)

// See issue #47 for the full design. This tool is read-only against the
// legacy database and only ever writes to the target (money_bae_api).
//
// seedUserEmail comes from internal/auth (auth.SeedUserEmail) rather than
// being redefined here, since MockVerifier must authenticate as exactly
// this row — a drift between two hardcoded copies would silently
// provision a second, empty user. Sub defaults to auth.SeedUserSub (what
// MockVerifier asserts) but can be overridden via SEED_USER_SUB so a real
// OIDC login (whose sub won't match the mock value) resolves to this same
// seeded/imported user instead of creating a new empty one.
const seedUserIDString = "01a0682f-2135-7c0a-bd1f-4d1be1918f2b"

func main() {
	cfg := config.Load() // also loads .env via godotenv, so SOURCE_DATABASE_URL below picks up the same file

	sourceDSN := os.Getenv("SOURCE_DATABASE_URL")
	if sourceDSN == "" {
		log.Fatal("SOURCE_DATABASE_URL is required (the legacy money_bae database)")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required (the target money_bae_api database)")
	}

	source, err := database.Connect(sourceDSN)
	if err != nil {
		log.Fatalf("failed to connect to source database: %v", err)
	}
	target, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to target database: %v", err)
	}

	if err := migrations.Run(target); err != nil {
		log.Fatalf("failed to migrate target schema: %v", err)
	}

	seedUserSub := auth.SeedUserSub
	if v := os.Getenv("SEED_USER_SUB"); v != "" {
		seedUserSub = v
	}

	userID, err := seedUser(target, seedUserSub)
	if err != nil {
		log.Fatalf("failed to seed user: %v", err)
	}
	log.Printf("seed user ready: %s", userID)

	billIDMap, err := importBills(source, target, userID)
	if err != nil {
		log.Fatalf("failed to import bills: %v", err)
	}
	log.Printf("imported %d bills", len(billIDMap))

	ledgerIDMap, err := importLedgers(source, target, userID)
	if err != nil {
		log.Fatalf("failed to import ledgers: %v", err)
	}
	log.Printf("imported %d ledgers", len(ledgerIDMap))

	ptoIDMap, err := importPtos(source, target, userID)
	if err != nil {
		log.Fatalf("failed to import ptos: %v", err)
	}
	log.Printf("imported %d ptos", len(ptoIDMap))

	incomeCount, err := importIncomes(source, target, userID, ledgerIDMap)
	if err != nil {
		log.Fatalf("failed to import incomes: %v", err)
	}
	log.Printf("imported %d incomes", incomeCount)

	ledgerBillCount, err := importLedgerBills(source, target, ledgerIDMap, billIDMap)
	if err != nil {
		log.Fatalf("failed to import ledger_bills: %v", err)
	}
	log.Printf("imported %d ledger_bills", ledgerBillCount)

	ptoPlanCount, err := importPtoPlans(source, target, ptoIDMap)
	if err != nil {
		log.Fatalf("failed to import pto_plan: %v", err)
	}
	log.Printf("imported %d pto_plan rows", ptoPlanCount)

	holidayHourCount, err := importHolidayHours(source, target, ptoIDMap)
	if err != nil {
		log.Fatalf("failed to import holiday_hours: %v", err)
	}
	log.Printf("imported %d holiday_hours rows", holidayHourCount)

	log.Println("migration complete")
}

func seedUser(target *gorm.DB, sub string) (uuid.UUID, error) {
	userID := uuid.MustParse(seedUserIDString)

	var user models.User
	err := target.First(&user, "id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = models.User{
			Base:  models.Base{ID: userID},
			Sub:   sub,
			Email: auth.SeedUserEmail,
		}
		if err := target.Create(&user).Error; err != nil {
			return uuid.Nil, fmt.Errorf("creating seed user: %w", err)
		}
		return user.ID, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("looking up seed user: %w", err)
	}

	if user.Sub != sub {
		user.Sub = sub
		if err := target.Save(&user).Error; err != nil {
			return uuid.Nil, fmt.Errorf("updating seed user sub: %w", err)
		}
	}
	return user.ID, nil
}

func importBills(source, target *gorm.DB, userID uuid.UUID) (map[int32]uuid.UUID, error) {
	var legacyRows []legacyBill
	if err := source.Raw("SELECT id, name, amount, due_day, is_auto_pay, created_at, notes FROM bills").Scan(&legacyRows).Error; err != nil {
		return nil, fmt.Errorf("reading legacy bills: %w", err)
	}

	idMap := make(map[int32]uuid.UUID, len(legacyRows))
	for _, old := range legacyRows {
		newRow := models.Bill{
			Base:      models.Base{CreatedAt: old.CreatedAt},
			UserID:    userID,
			Name:      old.Name,
			Amount:    old.Amount,
			DueDay:    old.DueDay,
			IsAutoPay: old.IsAutoPay,
			Notes:     old.Notes,
		}
		if err := target.Create(&newRow).Error; err != nil {
			return nil, fmt.Errorf("inserting bill (legacy id %d): %w", old.ID, err)
		}
		idMap[old.ID] = newRow.ID
	}
	return idMap, nil
}

func importLedgers(source, target *gorm.DB, userID uuid.UUID) (map[int32]uuid.UUID, error) {
	var legacyRows []legacyLedger
	if err := source.Raw("SELECT id, date, bank_balance, income, expenses, net, created_at, name, total, notes FROM ledgers").Scan(&legacyRows).Error; err != nil {
		return nil, fmt.Errorf("reading legacy ledgers: %w", err)
	}

	idMap := make(map[int32]uuid.UUID, len(legacyRows))
	for _, old := range legacyRows {
		newRow := models.Ledger{
			Base:        models.Base{CreatedAt: old.CreatedAt},
			UserID:      userID,
			Date:        old.Date,
			BankBalance: old.BankBalance,
			Income:      old.Income,
			Expenses:    old.Expenses,
			Net:         old.Net,
			Name:        old.Name,
			Total:       old.Total,
			Notes:       old.Notes,
		}
		if err := target.Create(&newRow).Error; err != nil {
			return nil, fmt.Errorf("inserting ledger (legacy id %d): %w", old.ID, err)
		}
		idMap[old.ID] = newRow.ID
	}
	return idMap, nil
}

func importPtos(source, target *gorm.DB, userID uuid.UUID) (map[int32]uuid.UUID, error) {
	var legacyRows []legacyPto
	if err := source.Raw("SELECT id, year, prev_year_hours, available_hours, hours_planned, hours_used, hours_remaining, rollover_hours, created_at FROM ptos").Scan(&legacyRows).Error; err != nil {
		return nil, fmt.Errorf("reading legacy ptos: %w", err)
	}

	idMap := make(map[int32]uuid.UUID, len(legacyRows))
	for _, old := range legacyRows {
		newRow := models.Pto{
			Base:           models.Base{CreatedAt: old.CreatedAt},
			UserID:         userID,
			Year:           int(old.Year),
			PrevYearHours:  old.PrevYearHours,
			AvailableHours: old.AvailableHours,
			HoursPlanned:   old.HoursPlanned,
			HoursUsed:      old.HoursUsed,
			HoursRemaining: old.HoursRemaining,
			RolloverHours:  old.RolloverHours,
		}
		if err := target.Create(&newRow).Error; err != nil {
			return nil, fmt.Errorf("inserting pto (legacy id %d): %w", old.ID, err)
		}
		idMap[old.ID] = newRow.ID
	}
	return idMap, nil
}

func importIncomes(source, target *gorm.DB, userID uuid.UUID, ledgerIDMap map[int32]uuid.UUID) (int, error) {
	var legacyRows []legacyIncome
	if err := source.Raw("SELECT id, date, amount, created_at, ledger_id, notes FROM incomes").Scan(&legacyRows).Error; err != nil {
		return 0, fmt.Errorf("reading legacy incomes: %w", err)
	}

	for _, old := range legacyRows {
		newRow := models.Income{
			Base:   models.Base{CreatedAt: old.CreatedAt},
			UserID: userID,
			Date:   old.Date,
			Amount: old.Amount,
			Notes:  old.Notes,
		}
		if old.LedgerID != nil {
			newLedgerID, ok := ledgerIDMap[*old.LedgerID]
			if !ok {
				return 0, fmt.Errorf("income (legacy id %d) references unknown legacy ledger id %d", old.ID, *old.LedgerID)
			}
			newRow.LedgerID = &newLedgerID
		}
		if err := target.Create(&newRow).Error; err != nil {
			return 0, fmt.Errorf("inserting income (legacy id %d): %w", old.ID, err)
		}
	}
	return len(legacyRows), nil
}

func importLedgerBills(source, target *gorm.DB, ledgerIDMap, billIDMap map[int32]uuid.UUID) (int, error) {
	var legacyRows []legacyLedgerBill
	if err := source.Raw("SELECT id, ledger_id, bill_id, amount, due_day, is_payed, created_at, notes FROM ledger_bills").Scan(&legacyRows).Error; err != nil {
		return 0, fmt.Errorf("reading legacy ledger_bills: %w", err)
	}

	for _, old := range legacyRows {
		newLedgerID, ok := ledgerIDMap[old.LedgerID]
		if !ok {
			return 0, fmt.Errorf("ledger_bill (legacy id %d) references unknown legacy ledger id %d", old.ID, old.LedgerID)
		}
		newBillID, ok := billIDMap[old.BillID]
		if !ok {
			return 0, fmt.Errorf("ledger_bill (legacy id %d) references unknown legacy bill id %d", old.ID, old.BillID)
		}
		newRow := models.LedgerBill{
			Base:     models.Base{CreatedAt: old.CreatedAt},
			LedgerID: newLedgerID,
			BillID:   &newBillID,
			Amount:   old.Amount,
			DueDay:   old.DueDay,
			IsPayed:  old.IsPayed,
			Notes:    old.Notes,
		}
		if err := target.Create(&newRow).Error; err != nil {
			return 0, fmt.Errorf("inserting ledger_bill (legacy id %d): %w", old.ID, err)
		}
	}
	return len(legacyRows), nil
}

func importPtoPlans(source, target *gorm.DB, ptoIDMap map[int32]uuid.UUID) (int, error) {
	var legacyRows []legacyPtoPlan
	if err := source.Raw("SELECT id, pto_id, start_date, end_date, name, description, hours, status, custom_hours, created_at FROM pto_plan").Scan(&legacyRows).Error; err != nil {
		return 0, fmt.Errorf("reading legacy pto_plan: %w", err)
	}

	for _, old := range legacyRows {
		newPtoID, ok := ptoIDMap[old.PtoID]
		if !ok {
			return 0, fmt.Errorf("pto_plan (legacy id %d) references unknown legacy pto id %d", old.ID, old.PtoID)
		}
		newRow := models.PtoPlan{
			Base:        models.Base{CreatedAt: old.CreatedAt},
			PtoID:       newPtoID,
			StartDate:   old.StartDate,
			EndDate:     old.EndDate,
			Name:        old.Name,
			Description: old.Description,
			Hours:       old.Hours,
			Status:      old.Status,
			CustomHours: old.CustomHours,
		}
		if err := target.Create(&newRow).Error; err != nil {
			return 0, fmt.Errorf("inserting pto_plan (legacy id %d): %w", old.ID, err)
		}
	}
	return len(legacyRows), nil
}

func importHolidayHours(source, target *gorm.DB, ptoIDMap map[int32]uuid.UUID) (int, error) {
	var legacyRows []legacyHolidayHours
	if err := source.Raw("SELECT id, pto_id, date, name, hours, created_at FROM holiday_hours").Scan(&legacyRows).Error; err != nil {
		return 0, fmt.Errorf("reading legacy holiday_hours: %w", err)
	}

	for _, old := range legacyRows {
		newPtoID, ok := ptoIDMap[old.PtoID]
		if !ok {
			return 0, fmt.Errorf("holiday_hours (legacy id %d) references unknown legacy pto id %d", old.ID, old.PtoID)
		}
		newRow := models.HolidayHour{
			Base:  models.Base{CreatedAt: old.CreatedAt},
			PtoID: newPtoID,
			Date:  old.Date,
			Name:  old.Name,
			Hours: old.Hours,
		}
		if err := target.Create(&newRow).Error; err != nil {
			return 0, fmt.Errorf("inserting holiday_hours (legacy id %d): %w", old.ID, err)
		}
	}
	return len(legacyRows), nil
}
