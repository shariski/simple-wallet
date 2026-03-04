package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type LedgerEntry struct {
	ID          uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	WalletID    uuid.UUID       `gorm:"type:uuid;not null;index"`
	Currency    string          `gorm:"type:varchar(3);not null"`
	Amount      decimal.Decimal `gorm:"type:numeric(20,2);not null"`
	Type        string          `gorm:"type:varchar(20);not null"`
	ReferenceID *uuid.UUID      `gorm:"type:uuid;index"`
	CreatedAt   time.Time       `gorm:"autoCreateTime"`
}

func (l *LedgerEntry) TableName() string {
	return "ledger_entries"
}
