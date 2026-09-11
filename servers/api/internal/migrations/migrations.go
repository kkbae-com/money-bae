package migrations

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/mrcunninghamz/money-bae/servers/api/internal/models"
)

// numericColumns lists every decimal.Decimal-backed column, as
// table/column pairs. None of them had an explicit GORM `type:` tag until
// now, so AutoMigrate's initial CREATE TABLE picked its own fallback type
// for a Go struct that only implements sql.Scanner/driver.Valuer (as
// shopspring/decimal.Decimal does, with no GormDataType hint) — text, which
// Postgres can store money/hours values in well enough for simple
// read/write, but which breaks the moment SQL needs to treat them as
// numbers: SUM(amount) fails outright ("function sum(text) does not
// exist"), and sorting/comparison would be lexicographic, not numeric.
var numericColumns = [][2]string{
	{"bills", "amount"},
	{"incomes", "amount"},
	{"ledger_bills", "amount"},
	{"ledgers", "bank_balance"},
	{"ledgers", "income"},
	{"ledgers", "expenses"},
	{"ledgers", "net"},
	{"ledgers", "total"},
	{"ptos", "prev_year_hours"},
	{"ptos", "available_hours"},
	{"ptos", "hours_planned"},
	{"ptos", "hours_used"},
	{"ptos", "hours_remaining"},
	{"pto_plans", "hours"},
	{"holiday_hours", "hours"},
}

// All holds every versioned migration added after the initial schema below.
// New schema changes get appended here, each with its own ID/Migrate/Rollback
// — never edit an already-applied entry.
var All = []*gormigrate.Migration{
	{
		ID: "20260905000000_decimal_columns_to_numeric",
		Migrate: func(tx *gorm.DB) error {
			for _, tc := range numericColumns {
				table, column := tc[0], tc[1]
				sql := fmt.Sprintf(
					"ALTER TABLE %s ALTER COLUMN %s TYPE numeric USING %s::numeric",
					table, column, column,
				)
				if err := tx.Exec(sql).Error; err != nil {
					return fmt.Errorf("altering %s.%s to numeric: %w", table, column, err)
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			for _, tc := range numericColumns {
				table, column := tc[0], tc[1]
				sql := fmt.Sprintf(
					"ALTER TABLE %s ALTER COLUMN %s TYPE text USING %s::text",
					table, column, column,
				)
				if err := tx.Exec(sql).Error; err != nil {
					return fmt.Errorf("reverting %s.%s to text: %w", table, column, err)
				}
			}
			return nil
		},
	},
	{
		// Lets a ledger_bill be a "generic expense": a one-off cost scoped
		// to just one ledger cycle, with no catalog Bill behind it. bill_id
		// becomes optional, and name carries the display label in that case
		// (models.LedgerBill.Name — ignored when bill_id is set, since the
		// catalog Bill's own name is used instead).
		ID: "20260910000000_ledger_bills_generic_expense",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Exec("ALTER TABLE ledger_bills ALTER COLUMN bill_id DROP NOT NULL").Error; err != nil {
				return fmt.Errorf("dropping ledger_bills.bill_id not-null constraint: %w", err)
			}
			if err := tx.Exec("ALTER TABLE ledger_bills ADD COLUMN name text").Error; err != nil {
				return fmt.Errorf("adding ledger_bills.name column: %w", err)
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Exec("ALTER TABLE ledger_bills DROP COLUMN name").Error; err != nil {
				return fmt.Errorf("dropping ledger_bills.name column: %w", err)
			}
			if err := tx.Exec("ALTER TABLE ledger_bills ALTER COLUMN bill_id SET NOT NULL").Error; err != nil {
				return fmt.Errorf("restoring ledger_bills.bill_id not-null constraint: %w", err)
			}
			return nil
		},
	},
}

// Run applies the initial schema (via InitSchema, GORM's AutoMigrate against
// the current models — safe even if some of these tables already exist from
// an earlier plain AutoMigrate call, since AutoMigrate only adds what's
// missing) plus any migrations in All, tracked in a "migrations" table so
// this is safe to call on every startup.
func Run(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, All)
	m.InitSchema(func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			&models.User{},
			&models.Bill{},
			&models.Income{},
			&models.Ledger{},
			&models.Pto{},
			&models.LedgerBill{},
			&models.PtoPlan{},
			&models.HolidayHour{},
		)
	})
	return m.Migrate()
}
