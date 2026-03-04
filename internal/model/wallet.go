package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shariski/simple-wallet/internal/entity"
	"github.com/shopspring/decimal"
)

type WalletResponse struct {
	ID        uuid.UUID       `json:"id"`
	OwnerID   string          `json:"owner_id"`
	Currency  string          `json:"currency"`
	Balance   decimal.Decimal `json:"balance"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type CreateWalletRequest struct {
	OwnerID  string `json:"owner_id" validate:"required,min=3,max=20"`
	Currency string `json:"currency" validate:"required,len=3"`
}

type TopupWalletRequest struct {
	IdempotencyKey string    `validate:"min=3,max=100"`
	ID             uuid.UUID `validate:"uuid,required"`
	OwnerID        string    `json:"owner_id" validate:"required,min=3,max=20"`
	Currency       string    `json:"currency" validate:"required,len=3"`
	Amount         string    `json:"amount" validate:"required"`
}

type PaymentWalletRequest struct {
	IdempotencyKey string    `validate:"min=3,max=100"`
	ID             uuid.UUID `json:"id" validate:"uuid,required"`
	OwnerID        string    `json:"owner_id" validate:"required,min=3,max=20"`
	Currency       string    `json:"currency" validate:"required,len=3"`
	Amount         string    `json:"amount" validate:"required"`
}

type TransferWalletRequest struct {
	IdempotencyKey   string `validate:"min=3,max=100"`
	SenderID         string `json:"sender_id" validate:"required,min=3,max=20"`
	ReceiverID       string `json:"receiver_id" validate:"required,min=3,max=20"`
	SenderCurrency   string `json:"sender_currency" validate:"required,len=3"`
	ReceiverCurrency string `json:"receiver_currency" validate:"required,len=3"`
	Amount           string `json:"amount" validate:"required"`
}

type SuspendWalletRequest struct {
	ID uuid.UUID `json:"id" validate:"uuid,required"`
}

type StatusWalletRequest struct {
	ID uuid.UUID `json:"id" validate:"uuid,required"`
}

func WalletToResponse(w *entity.Wallet) *WalletResponse {
	return &WalletResponse{
		ID:        w.ID,
		OwnerID:   w.OwnerID,
		Currency:  w.Currency,
		Balance:   w.Balance,
		Status:    w.Status,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	}
}
