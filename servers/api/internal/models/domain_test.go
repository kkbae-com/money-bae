package models_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mrcunninghamz/money-bae/servers/api/internal/models"
	"github.com/mrcunninghamz/money-bae/servers/api/internal/testdb"
)

func TestDomainModels_AutoMigrateSucceeds(t *testing.T) {
	testdb.New(t,
		&models.User{},
		&models.Bill{},
		&models.Income{},
		&models.Ledger{},
		&models.Pto{},
		&models.LedgerBill{},
		&models.PtoPlan{},
		&models.HolidayHour{},
	)
}

func TestLedgerBill_PreloadsLedgerAndBill(t *testing.T) {
	db := testdb.New(t, &models.User{}, &models.Bill{}, &models.Ledger{}, &models.LedgerBill{})

	user := models.User{Sub: "auth0|domain-test", Email: "domain-test@example.com"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	bill := models.Bill{UserID: user.ID, Name: "Rent", Amount: decimal.NewFromInt(1500), IsAutoPay: true}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("failed to create bill: %v", err)
	}

	ledger := models.Ledger{
		UserID:      user.ID,
		BankBalance: decimal.NewFromInt(5000),
		Income:      decimal.NewFromInt(3000),
		Expenses:    decimal.NewFromInt(2000),
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	ledgerBill := models.LedgerBill{
		LedgerID: ledger.ID,
		BillID:   &bill.ID,
		Amount:   decimal.NewFromInt(1500),
		IsPayed:  false,
	}
	if err := db.Create(&ledgerBill).Error; err != nil {
		t.Fatalf("failed to create ledger bill: %v", err)
	}

	var reloaded models.LedgerBill
	err := db.Preload("Ledger").Preload("Bill").First(&reloaded, "id = ?", ledgerBill.ID).Error
	if err != nil {
		t.Fatalf("failed to reload ledger bill: %v", err)
	}

	if reloaded.Ledger.ID != ledger.ID {
		t.Fatalf("expected preloaded Ledger.ID %s, got %s", ledger.ID, reloaded.Ledger.ID)
	}
	if reloaded.Bill.ID != bill.ID {
		t.Fatalf("expected preloaded Bill.ID %s, got %s", bill.ID, reloaded.Bill.ID)
	}
	if reloaded.Bill.Name != "Rent" {
		t.Fatalf("expected preloaded Bill.Name %q, got %q", "Rent", reloaded.Bill.Name)
	}
}
