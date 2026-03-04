package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Wallet struct {
	ID        uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	OwnerID   string          `gorm:"size:20;not null;uniqueIndex:idx_owner_currency"`
	Currency  string          `gorm:"size:3;not null;uniqueIndex:idx_owner_currency"`
	Balance   decimal.Decimal `gorm:"type:numeric(20,2);not null"`
	Status    string          `gorm:"size:20;not null"`
	CreatedAt time.Time       `gorm:"column:created_at;autoCreateTime;->;<-:create"`
	UpdatedAt time.Time       `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
}

func (w *Wallet) TableName() string {
	return "wallets"
}
