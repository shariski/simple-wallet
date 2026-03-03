package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
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

func (s *WalletService) Create(ctx context.Context, req *model.CreateWalletRequest) (*model.WalletResponse, error) {
	wallet := &entity.Wallet{
		OwnerID:  req.OwnerID,
		Currency: req.Currency,
		Status:   "ACTIVE",
		Balance:  decimal.Zero,
	}

	err := s.db.WithContext(ctx).Create(wallet).Error
	if err != nil {
		// unique violation
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return nil, errors.New("wallet already exists for this currency")
		}

		return nil, err
	}

	return model.WalletToResponse(wallet), nil
}

func (s *WalletService) TopUp(ctx context.Context, req *model.TopupWalletRequest) error {
	amount, err := normalizeAmount(req.Amount)
	if err != nil {
		return err
	}

	tx := s.db.WithContext(ctx).Begin()
	defer tx.Rollback()

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

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

func (s *WalletService) Payment(ctx context.Context, req *model.PaymentWalletRequest) (*model.WalletResponse, error) {
	amount, err := normalizeAmount(req.Amount)
	if err != nil {
		return nil, err
	}

	tx := s.db.WithContext(ctx).Begin()
	defer tx.Rollback()

	var wallet entity.Wallet

	if err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND owner_id = ? AND currency = ?",
			req.ID, req.OwnerID, req.Currency).
		First(&wallet).Error; err != nil {
		return nil, err
	}

	if wallet.Status != "ACTIVE" {
		return nil, errors.New("wallet suspended")
	}

	if wallet.Balance.LessThan(amount) {
		return nil, errors.New("balance insufficient")
	}

	wallet.Balance = wallet.Balance.Sub(amount)

	if err := tx.Save(&wallet).Error; err != nil {
		return nil, err
	}
	//
	// ledger := entity.LedgerEntry{
	// 	ID:       uuid.New(),
	// 	WalletID: wallet.ID,
	// 	Amount:   amount.Neg(),
	// 	Currency: wallet.Currency,
	// 	Type:     "PAYMENT",
	// }
	//
	// if err := tx.Create(&ledger).Error; err != nil {
	// 	return err
	// }

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return model.WalletToResponse(&wallet), nil
}

// TODO: fixed lock ordering not by identifier (wallet id) can cause deadlock
func (s *WalletService) Transfer(ctx context.Context, req *model.TransferWalletRequest) (*model.WalletResponse, *model.WalletResponse, error) {
	amount, err := normalizeAmount(req.Amount)
	if err != nil {
		return nil, nil, err
	}

	var sender entity.Wallet
	var receiver entity.Wallet

	tx := s.db.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_id = ? AND currency = ?",
			req.SenderID, req.SenderCurrency).
		First(&sender).Error; err != nil {
		return nil, nil, err
	}

	if sender.Status != "ACTIVE" {
		return nil, nil, errors.New("status not active")
	}

	if sender.Balance.LessThan(amount) {
		return nil, nil, errors.New("balance insufficient")
	}

	if err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_id = ? AND currency = ?",
			req.ReceiverID, req.ReceiverCurrency).
		First(&receiver).Error; err != nil {
		return nil, nil, err
	}

	if receiver.Status != "ACTIVE" {
		return nil, nil, errors.New("status not active")
	}

	if sender.Currency != receiver.Currency {
		return nil, nil, errors.New("cannot transfer cross-currency")
	}

	sender.Balance = sender.Balance.Sub(amount)
	receiver.Balance = receiver.Balance.Add(amount)

	if err := tx.
		Save(&sender).Error; err != nil {
		return nil, nil, err
	}

	if err := tx.
		Save(&receiver).Error; err != nil {
		return nil, nil, err
	}

	// TODO: add to ledger entry

	if err := tx.Commit().Error; err != nil {
		return nil, nil, err
	}

	return model.WalletToResponse(&sender), model.WalletToResponse(&receiver), nil
}

func (s *WalletService) Suspend(ctx context.Context, req *model.SuspendWalletRequest) (*model.WalletResponse, error) {
	result := s.db.WithContext(ctx).
		Model(&entity.Wallet{}).
		Where("id = ? AND status = ?", req.ID, "ACTIVE").
		Update("status", "SUSPENDED")

	if result.Error != nil {
		return nil, result.Error
	}

	if result.RowsAffected == 0 {
		return nil, errors.New("wallet not found or already suspended")
	}

	var wallet entity.Wallet
	if err := s.db.WithContext(ctx).
		First(&wallet, "id = ?", req.ID).Error; err != nil {
		return nil, err
	}

	return model.WalletToResponse(&wallet), nil
}

func (s *WalletService) Status(ctx context.Context, req *model.StatusWalletRequest) (*model.WalletResponse, error) {
	var wallet entity.Wallet

	if err := s.db.WithContext(ctx).
		First(&wallet, "id = ?", req.ID).Error; err != nil {
		return nil, err
	}

	return model.WalletToResponse(&wallet), nil
}
