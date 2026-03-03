package service

import (
	"context"
	"errors"

	"github.com/shariski/simple-wallet/internal/entity"
	"github.com/shariski/simple-wallet/internal/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WalletService struct {
	db *gorm.DB
}

func NewWalletService(db *gorm.DB) *WalletService {
	return &WalletService{
		db: db,
	}
}

func normalizeAmount(input string) (decimal.Decimal, error) {
	amount, err := decimal.NewFromString(input)
	if err != nil {
		return decimal.Zero, errors.New("invalid amount format")
	}

	amount = amount.Round(2)

	if amount.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, errors.New("amount must be > 0")
	}

	if amount.LessThan(decimal.NewFromFloat(0.01)) {
		return decimal.Zero, errors.New("below minimum unit")
	}

	return amount, nil
}

func (s *WalletService) TopUp(ctx context.Context, req *model.TopupWalletRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		amount, err := normalizeAmount(req.Amount)
		if err != nil {
			return err
		}

		var wallet entity.Wallet

		// Concurrency-safe row lock
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&wallet, "id = ?", req.ID).Error; err != nil {
			return err
		}

		if wallet.Status != "ACTIVE" {
			return errors.New("wallet suspended")
		}

		wallet.Balance = wallet.Balance.Add(amount)

		if err := tx.Save(&wallet).Error; err != nil {
			return err
		}

		// entry := model.LedgerEntry{
		// 	ID:       uuid.New(),
		// 	WalletID: wallet.ID,
		// 	Amount:   amount,
		// 	Currency: wallet.Currency,
		// 	Type:     "TOPUP",
		// }

		// if err := tx.Create(&entry).Error; err != nil {
		// 	return err
		// }

		return nil
	})
}
