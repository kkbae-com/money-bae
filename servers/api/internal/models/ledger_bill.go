package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// LedgerBill is a bill linked into one cycle. BillID/Bill are nullable: a
// "generic expense" is a LedgerBill scoped to just this ledger with no
// catalog Bill behind it, using its own Name instead of Bill.Name.
type LedgerBill struct {
	Base
	LedgerID uuid.UUID       `gorm:"not null;index"`
	Ledger   Ledger          `gorm:"foreignKey:LedgerID"`
	BillID   *uuid.UUID      `gorm:"index"`
	Bill     *Bill           `gorm:"foreignKey:BillID"`
	Amount   decimal.Decimal `gorm:"not null;type:numeric"`
	DueDay   *time.Time
	IsPayed  bool `gorm:"not null"`
	Notes    *string
	Name     *string
}
