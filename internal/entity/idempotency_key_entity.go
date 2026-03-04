package entity

import "time"

type IdempotencyKey struct {
	Key       string    `gorm:"type:varchar(100);primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (i *IdempotencyKey) TableName() string {
	return "idempotency_keys"
}
