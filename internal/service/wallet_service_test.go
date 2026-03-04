package service_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shariski/simple-wallet/internal/entity"
	"github.com/shariski/simple-wallet/internal/model"
	"github.com/shariski/simple-wallet/internal/service"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, *service.WalletService) {
	t.Helper()

	dsn := "host=localhost user=admin password=admin dbname=wallet_test port=5432 sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect db: %v", err)
	}

	db.Exec(`
		TRUNCATE wallets,
		ledger_entries,
		idempotency_keys
		RESTART IDENTITY CASCADE
	`)

	err = db.AutoMigrate(
		&entity.Wallet{},
		&entity.LedgerEntry{},
		&entity.IdempotencyKey{},
	)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	return db, service.NewWalletService(db)
}

func TestCreateWallet_Success(t *testing.T) {
	db, svc := setupTestDB(t)

	req := &model.CreateWalletRequest{
		OwnerID:  "user1",
		Currency: "USD",
	}

	res, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Currency != "USD" {
		t.Fatalf("expected USD got %s", res.Currency)
	}

	var count int64
	db.Model(&entity.Wallet{}).Count(&count)

	if count != 1 {
		t.Fatalf("expected 1 wallet got %d", count)
	}
}

func TestCreateWallet_DuplicateCurrency(t *testing.T) {
	_, svc := setupTestDB(t)

	req := &model.CreateWalletRequest{
		OwnerID:  "user1",
		Currency: "USD",
	}

	_, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.Create(context.Background(), req)
	if err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestTopUp_Success(t *testing.T) {
	db, svc := setupTestDB(t)

	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	db.Create(&wallet)

	req := &model.TopupWalletRequest{
		ID:             wallet.ID,
		OwnerID:        wallet.OwnerID,
		Currency:       wallet.Currency,
		IdempotencyKey: "key1",
		Amount:         "10.00",
	}

	res, err := svc.TopUp(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated entity.Wallet
	db.First(&updated, wallet.ID)

	if updated.Balance.String() != "10" && updated.Balance.String() != "10.00" {
		t.Fatalf("unexpected balance %s", updated.Balance.String())
	}

	if res.Balance.String() != "10" && res.Balance.String() != "10.00" {
		t.Fatalf("unexpected balance %s", updated.Balance.String())
	}
}

func TestTopUp_Idempotency(t *testing.T) {
	db, svc := setupTestDB(t)

	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	db.Create(&wallet)

	req := &model.TopupWalletRequest{
		ID:             wallet.ID,
		OwnerID:        wallet.OwnerID,
		Currency:       wallet.Currency,
		IdempotencyKey: "same-key",
		Amount:         "10.00",
	}

	res, err := svc.TopUp(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.TopUp(context.Background(), req)
	if err == nil {
		t.Fatalf("expected idempotency duplicate error")
	}

	var updated entity.Wallet
	db.First(&updated, wallet.ID)

	if updated.Balance.String() != "10" && updated.Balance.String() != "10.00" {
		t.Fatalf("balance changed unexpectedly: %s", updated.Balance.String())
	}

	if res.Balance.String() != "10" && res.Balance.String() != "10.00" {
		t.Fatalf("balance changed unexpectedly: %s", updated.Balance.String())
	}
}

func TestPayment_InsufficientBalance(t *testing.T) {
	db, svc := setupTestDB(t)

	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(5),
	}

	db.Create(&wallet)

	req := &model.PaymentWalletRequest{
		ID:             wallet.ID,
		OwnerID:        "user1",
		Currency:       "USD",
		IdempotencyKey: "pay1",
		Amount:         "10.00",
	}

	_, err := svc.Payment(context.Background(), req)

	if err == nil {
		t.Fatalf("expected insufficient balance error")
	}
}

func TestTransfer_Success(t *testing.T) {
	db, svc := setupTestDB(t)

	sender := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(100),
	}

	receiver := entity.Wallet{
		OwnerID:  "user2",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	db.Create(&sender)
	db.Create(&receiver)

	req := &model.TransferWalletRequest{
		SenderID:         "user1",
		SenderCurrency:   "USD",
		ReceiverID:       "user2",
		ReceiverCurrency: "USD",
		IdempotencyKey:   "transfer1",
		Amount:           "50.00",
	}

	_, _, err := svc.Transfer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	db.First(&sender, sender.ID)
	db.First(&receiver, receiver.ID)

	if sender.Balance.String() != "50" && sender.Balance.String() != "50.00" {
		t.Fatalf("unexpected sender balance: %s", sender.Balance.String())
	}

	if receiver.Balance.String() != "50" && receiver.Balance.String() != "50.00" {
		t.Fatalf("unexpected receiver balance: %s", receiver.Balance.String())
	}
}

func TestTransfer_CrossCurrency(t *testing.T) {
	db, svc := setupTestDB(t)

	sender := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(100),
	}

	receiver := entity.Wallet{
		OwnerID:  "user2",
		Currency: "EUR",
		Status:   "ACTIVE",
	}

	db.Create(&sender)
	db.Create(&receiver)

	req := &model.TransferWalletRequest{
		SenderID:         "user1",
		SenderCurrency:   "USD",
		ReceiverID:       "user2",
		ReceiverCurrency: "EUR",
		IdempotencyKey:   "transfer2",
		Amount:           "10.00",
	}

	_, _, err := svc.Transfer(context.Background(), req)

	if err == nil {
		t.Fatalf("expected cross currency error")
	}
}

func TestLedgerInvariant(t *testing.T) {
	db, svc := setupTestDB(t)

	ctx := context.Background()

	// create wallets
	sender := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.Zero,
	}

	receiver := entity.Wallet{
		OwnerID:  "user2",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.Zero,
	}

	if err := db.Create(&sender).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&receiver).Error; err != nil {
		t.Fatal(err)
	}

	// topup sender
	_, err := svc.TopUp(ctx, &model.TopupWalletRequest{
		ID:             sender.ID,
		OwnerID:        sender.OwnerID,
		Currency:       sender.Currency,
		IdempotencyKey: "topup1",
		Amount:         "100.00",
	})
	if err != nil {
		t.Fatalf("topup failed: %v", err)
	}

	// payment
	_, err = svc.Payment(ctx, &model.PaymentWalletRequest{
		ID:             sender.ID,
		OwnerID:        "user1",
		Currency:       "USD",
		IdempotencyKey: "payment1",
		Amount:         "20.00",
	})
	if err != nil {
		t.Fatalf("payment failed: %v", err)
	}

	// transfer
	_, _, err = svc.Transfer(ctx, &model.TransferWalletRequest{
		SenderID:         "user1",
		SenderCurrency:   "USD",
		ReceiverID:       "user2",
		ReceiverCurrency: "USD",
		IdempotencyKey:   "transfer1",
		Amount:           "30.00",
	})
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}

	// reload wallets
	var wallets []entity.Wallet
	if err := db.Find(&wallets).Error; err != nil {
		t.Fatal(err)
	}

	// check invariant
	for _, wallet := range wallets {

		var sum decimal.Decimal

		err := db.Model(&entity.LedgerEntry{}).
			Where("wallet_id = ?", wallet.ID).
			Select("COALESCE(SUM(amount),0)").
			Scan(&sum).Error

		if err != nil {
			t.Fatal(err)
		}

		if !wallet.Balance.Equal(sum) {
			t.Fatalf(
				"ledger invariant violated for wallet %s: balance=%s ledger_sum=%s",
				wallet.ID,
				wallet.Balance.String(),
				sum.String(),
			)
		}
	}
}

func TestTransfer_Concurrent(t *testing.T) {
	db, svc := setupTestDB(t)
	ctx := context.Background()

	sender := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(100),
	}

	receiver := entity.Wallet{
		OwnerID:  "user2",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.Zero,
	}

	if err := db.Create(&sender).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&receiver).Error; err != nil {
		t.Fatal(err)
	}

	const goroutines = 10
	const transferAmount = "5.00"

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()

			_, _, err := svc.Transfer(ctx, &model.TransferWalletRequest{
				SenderID:         "user1",
				SenderCurrency:   "USD",
				ReceiverID:       "user2",
				ReceiverCurrency: "USD",
				IdempotencyKey:   fmt.Sprintf("concurrent-%d", i),
				Amount:           transferAmount,
			})

			if err != nil {
				t.Errorf("transfer failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// reload wallets
	var updatedSender entity.Wallet
	var updatedReceiver entity.Wallet

	db.First(&updatedSender, sender.ID)
	db.First(&updatedReceiver, receiver.ID)

	t.Logf("sender balance: %s", updatedSender.Balance.String())
	t.Logf("receiver balance: %s", updatedReceiver.Balance.String())

	expectedSender := decimal.NewFromInt(100).Sub(decimal.NewFromInt(5 * goroutines))
	expectedReceiver := decimal.NewFromInt(5 * goroutines)

	if !updatedSender.Balance.Equal(expectedSender) {
		t.Fatalf(
			"sender balance incorrect: expected %s got %s",
			expectedSender.String(),
			updatedSender.Balance.String(),
		)
	}

	if !updatedReceiver.Balance.Equal(expectedReceiver) {
		t.Fatalf(
			"receiver balance incorrect: expected %s got %s",
			expectedReceiver.String(),
			updatedReceiver.Balance.String(),
		)
	}

	// check ledger entries
	var count int64
	db.Model(&entity.LedgerEntry{}).Count(&count)

	expectedLedger := goroutines * 2

	if count != int64(expectedLedger) {
		t.Fatalf(
			"ledger entries mismatch: expected %d got %d",
			expectedLedger,
			count,
		)
	}
}

func TestNormalizeAmount_InvalidFormat(t *testing.T) {
	_, err := service.NormalizeAmount("abc")

	if err == nil {
		t.Fatal("expected error for invalid amount")
	}
}

func TestNormalizeAmount_Zero(t *testing.T) {
	_, err := service.NormalizeAmount("0")

	if err == nil {
		t.Fatal("expected error for zero amount")
	}
}

func TestNormalizeAmount_Negative(t *testing.T) {
	_, err := service.NormalizeAmount("-10")

	if err == nil {
		t.Fatal("expected error for negative amount")
	}
}

func TestNormalizeAmount_BelowMinimum(t *testing.T) {
	_, err := service.NormalizeAmount("0.001")

	if err == nil {
		t.Fatal("expected minimum unit error")
	}
}

func TestTopUp_SuspendedWallet(t *testing.T) {
	db, svc := setupTestDB(t)

	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "SUSPENDED",
	}

	db.Create(&wallet)

	_, err := svc.TopUp(context.Background(), &model.TopupWalletRequest{
		ID:             wallet.ID,
		OwnerID:        wallet.OwnerID,
		Currency:       wallet.Currency,
		IdempotencyKey: "key1",
		Amount:         "10.00",
	})

	if err == nil {
		t.Fatal("expected suspended wallet error")
	}
}

func TestPayment_SuspendedWallet(t *testing.T) {
	db, svc := setupTestDB(t)

	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "SUSPENDED",
		Balance:  decimal.NewFromInt(100),
	}

	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}

	req := &model.PaymentWalletRequest{
		ID:             wallet.ID,
		OwnerID:        "user1",
		Currency:       "USD",
		IdempotencyKey: "payment-suspended",
		Amount:         "10.00",
	}

	_, err := svc.Payment(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for suspended wallet payment")
	}
}

func TestTransfer_SenderSuspended(t *testing.T) {
	db, svc := setupTestDB(t)

	sender := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "SUSPENDED",
		Balance:  decimal.NewFromInt(100),
	}

	receiver := entity.Wallet{
		OwnerID:  "user2",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	if err := db.Create(&sender).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&receiver).Error; err != nil {
		t.Fatal(err)
	}

	req := &model.TransferWalletRequest{
		SenderID:         "user1",
		SenderCurrency:   "USD",
		ReceiverID:       "user2",
		ReceiverCurrency: "USD",
		IdempotencyKey:   "transfer-suspended-sender",
		Amount:           "10.00",
	}

	_, _, err := svc.Transfer(context.Background(), req)

	if err == nil {
		t.Fatal("expected error when sender wallet is suspended")
	}
}

func TestTransfer_ReceiverSuspended(t *testing.T) {
	db, svc := setupTestDB(t)

	sender := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(100),
	}

	receiver := entity.Wallet{
		OwnerID:  "user2",
		Currency: "USD",
		Status:   "SUSPENDED",
	}

	if err := db.Create(&sender).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&receiver).Error; err != nil {
		t.Fatal(err)
	}

	req := &model.TransferWalletRequest{
		SenderID:         "user1",
		SenderCurrency:   "USD",
		ReceiverID:       "user2",
		ReceiverCurrency: "USD",
		IdempotencyKey:   "transfer-suspended-receiver",
		Amount:           "10.00",
	}

	_, _, err := svc.Transfer(context.Background(), req)

	if err == nil {
		t.Fatal("expected error when receiver wallet is suspended")
	}
}

func TestPayment_Idempotency(t *testing.T) {
	db, svc := setupTestDB(t)

	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(100),
	}

	db.Create(&wallet)

	req := &model.PaymentWalletRequest{
		ID:             wallet.ID,
		OwnerID:        "user1",
		Currency:       "USD",
		IdempotencyKey: "dup-key",
		Amount:         "10.00",
	}

	svc.Payment(context.Background(), req)
	_, err := svc.Payment(context.Background(), req)

	if err == nil {
		t.Fatal("expected duplicate request error")
	}
}

func TestTransfer_ToSelfWallet(t *testing.T) {
	db, svc := setupTestDB(t)

	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(100),
	}

	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}

	req := &model.TransferWalletRequest{
		SenderID:         "user1",
		SenderCurrency:   "USD",
		ReceiverID:       "user1",
		ReceiverCurrency: "USD",
		IdempotencyKey:   "self-transfer",
		Amount:           "10.00",
	}

	_, _, err := svc.Transfer(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for self transfer")
	}

	// balance should not change
	var updated entity.Wallet
	if err := db.First(&updated, wallet.ID).Error; err != nil {
		t.Fatal(err)
	}

	if !updated.Balance.Equal(decimal.NewFromInt(100)) {
		t.Fatalf(
			"balance changed unexpectedly: expected 100 got %s",
			updated.Balance.String(),
		)
	}

	// ledger should not be created
	var ledgerCount int64
	db.Model(&entity.LedgerEntry{}).Count(&ledgerCount)

	if ledgerCount != 0 {
		t.Fatalf("ledger should not be created for self transfer")
	}
}

func TestRandomOperations_LedgerInvariant(t *testing.T) {
	db, svc := setupTestDB(t)

	ctx := context.Background()

	// create wallets
	wallet1 := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	wallet2 := entity.Wallet{
		OwnerID:  "user2",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	if err := db.Create(&wallet1).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&wallet2).Error; err != nil {
		t.Fatal(err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	const operations = 100

	for i := 0; i < operations; i++ {

		switch rng.Intn(3) {

		case 0:
			// topup wallet1
			_, err := svc.TopUp(ctx, &model.TopupWalletRequest{
				ID:             wallet1.ID,
				OwnerID:        wallet1.OwnerID,
				Currency:       wallet1.Currency,
				IdempotencyKey: fmt.Sprintf("topup-%d", i),
				Amount:         "10.00",
			})

			if err != nil {
				t.Log("topup error:", err)
			}

		case 1:
			// payment wallet1
			_, err := svc.Payment(ctx, &model.PaymentWalletRequest{
				ID:             wallet1.ID,
				OwnerID:        "user1",
				Currency:       "USD",
				IdempotencyKey: fmt.Sprintf("payment-%d", i),
				Amount:         "5.00",
			})

			if err != nil {
				t.Log("payment error:", err)
			}

		case 2:
			// transfer wallet1 -> wallet2
			_, _, err := svc.Transfer(ctx, &model.TransferWalletRequest{
				SenderID:         "user1",
				SenderCurrency:   "USD",
				ReceiverID:       "user2",
				ReceiverCurrency: "USD",
				IdempotencyKey:   fmt.Sprintf("transfer-%d", i),
				Amount:           "3.00",
			})

			if err != nil {
				t.Log("transfer error:", err)
			}

		}
	}

	// reload wallets
	var wallets []entity.Wallet
	if err := db.Find(&wallets).Error; err != nil {
		t.Fatal(err)
	}

	// verify ledger invariant
	for _, wallet := range wallets {

		var sum decimal.Decimal

		err := db.Model(&entity.LedgerEntry{}).
			Where("wallet_id = ?", wallet.ID).
			Select("COALESCE(SUM(amount),0)").
			Scan(&sum).Error

		if err != nil {
			t.Fatal(err)
		}

		if !wallet.Balance.Equal(sum) {
			t.Fatalf(
				"ledger invariant broken for wallet %s: balance=%s ledger_sum=%s",
				wallet.ID,
				wallet.Balance.String(),
				sum.String(),
			)
		}
	}
}

func TestTopUp_WalletNotFound(t *testing.T) {
	_, svc := setupTestDB(t)

	req := &model.TopupWalletRequest{
		ID:             uuid.New(),
		IdempotencyKey: "notfound",
		OwnerID:        "user1",
		Currency:       "USD",
		Amount:         "10.00",
	}

	_, err := svc.TopUp(context.Background(), req)

	if err == nil {
		t.Fatal("expected wallet not found error")
	}
}

func TestSuspend_Success(t *testing.T) {
	db, svc := setupTestDB(t)

	wallet := entity.Wallet{
		ID:       uuid.New(),
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	db.Create(&wallet)

	res, err := svc.Suspend(context.Background(), &model.SuspendWalletRequest{
		ID: wallet.ID,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "SUSPENDED" {
		t.Fatalf("expected SUSPENDED got %s", res.Status)
	}
}

func TestSuspend_AlreadySuspended(t *testing.T) {
	db, svc := setupTestDB(t)

	wallet := entity.Wallet{
		ID:       uuid.New(),
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "SUSPENDED",
	}

	db.Create(&wallet)

	_, err := svc.Suspend(context.Background(), &model.SuspendWalletRequest{
		ID: wallet.ID,
	})

	if err == nil {
		t.Fatal("expected error when suspending already suspended wallet")
	}
}

func TestStatus(t *testing.T) {
	db, svc := setupTestDB(t)

	wallet := entity.Wallet{
		ID:       uuid.New(),
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	db.Create(&wallet)

	res, err := svc.Status(context.Background(), &model.StatusWalletRequest{
		ID: wallet.ID,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ID != wallet.ID {
		t.Fatal("wallet mismatch")
	}
}

func TestLargeBalanceOperations(t *testing.T) {
	db, svc := setupTestDB(t)

	ctx := context.Background()

	wallet := entity.Wallet{
		OwnerID:  "biguser",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}

	largeAmount := "1000000000.00"

	// large topup
	_, err := svc.TopUp(ctx, &model.TopupWalletRequest{
		ID:             wallet.ID,
		OwnerID:        wallet.OwnerID,
		Currency:       wallet.Currency,
		IdempotencyKey: "large-topup",
		Amount:         largeAmount,
	})

	if err != nil {
		t.Fatalf("large topup failed: %v", err)
	}

	// payment
	_, err = svc.Payment(ctx, &model.PaymentWalletRequest{
		ID:             wallet.ID,
		OwnerID:        "biguser",
		Currency:       "USD",
		IdempotencyKey: "large-payment",
		Amount:         "250000000.00",
	})

	if err != nil {
		t.Fatalf("large payment failed: %v", err)
	}

	// reload wallet
	var updated entity.Wallet
	if err := db.First(&updated, wallet.ID).Error; err != nil {
		t.Fatal(err)
	}

	expected := decimal.NewFromInt(1000000000).Sub(decimal.NewFromInt(250000000))

	if !updated.Balance.Equal(expected) {
		t.Fatalf(
			"unexpected balance: expected %s got %s",
			expected.String(),
			updated.Balance.String(),
		)
	}

	// verify ledger invariant
	var sum decimal.Decimal

	err = db.Model(&entity.LedgerEntry{}).
		Where("wallet_id = ?", wallet.ID).
		Select("COALESCE(SUM(amount),0)").
		Scan(&sum).Error

	if err != nil {
		t.Fatal(err)
	}

	if !sum.Equal(updated.Balance) {
		t.Fatalf(
			"ledger invariant broken: balance=%s ledger_sum=%s",
			updated.Balance.String(),
			sum.String(),
		)
	}
}

func TestOutOfOrderRequests(t *testing.T) {
	db, svc := setupTestDB(t)

	ctx := context.Background()

	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}

	// payment first (should fail)
	_, err := svc.Payment(ctx, &model.PaymentWalletRequest{
		ID:             wallet.ID,
		OwnerID:        "user1",
		Currency:       "USD",
		IdempotencyKey: "payment-first",
		Amount:         "50.00",
	})

	if err == nil {
		t.Fatal("expected insufficient balance error")
	}

	// topup later
	_, err = svc.TopUp(ctx, &model.TopupWalletRequest{
		ID:             wallet.ID,
		OwnerID:        wallet.OwnerID,
		Currency:       wallet.Currency,
		IdempotencyKey: "topup-later",
		Amount:         "100.00",
	})

	if err != nil {
		t.Fatalf("topup failed: %v", err)
	}

	// payment retry
	_, err = svc.Payment(ctx, &model.PaymentWalletRequest{
		ID:             wallet.ID,
		OwnerID:        "user1",
		Currency:       "USD",
		IdempotencyKey: "payment-retry",
		Amount:         "50.00",
	})

	if err != nil {
		t.Fatalf("payment retry failed: %v", err)
	}

	var updated entity.Wallet
	if err := db.First(&updated, wallet.ID).Error; err != nil {
		t.Fatal(err)
	}

	expected := decimal.NewFromInt(50)

	if !updated.Balance.Equal(expected) {
		t.Fatalf(
			"unexpected balance: expected %s got %s",
			expected.String(),
			updated.Balance.String(),
		)
	}
}

func TestTopUp_OwnerMismatch(t *testing.T) {
	db, svc := setupTestDB(t)

	ctx := context.Background()

	// wallet belongs to user1
	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
	}

	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}

	// attacker tries to use user2
	req := &model.TopupWalletRequest{
		ID:             wallet.ID,
		OwnerID:        "user2",
		Currency:       "USD",
		IdempotencyKey: "attack-topup",
		Amount:         "100.00",
	}

	_, err := svc.TopUp(ctx, req)

	if err == nil {
		t.Fatal("expected owner mismatch error")
	}

	// ensure balance unchanged
	var updated entity.Wallet
	db.First(&updated, wallet.ID)

	if !updated.Balance.Equal(decimal.Zero) {
		t.Fatalf("wallet balance changed unexpectedly: %s", updated.Balance)
	}
}

func TestPayment_OwnerMismatch(t *testing.T) {
	db, svc := setupTestDB(t)

	ctx := context.Background()

	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(100),
	}

	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}

	req := &model.PaymentWalletRequest{
		ID:             wallet.ID,
		OwnerID:        "user2", // malicious user
		Currency:       "USD",
		IdempotencyKey: "attack-payment",
		Amount:         "10.00",
	}

	_, err := svc.Payment(ctx, req)

	if err == nil {
		t.Fatal("expected owner mismatch error")
	}

	// balance must stay same
	var updated entity.Wallet
	db.First(&updated, wallet.ID)

	if !updated.Balance.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("wallet balance changed unexpectedly: %s", updated.Balance)
	}
}

func TestTransfer_CrossConcurrent(t *testing.T) {
	db, svc := setupTestDB(t)
	ctx := context.Background()

	a := entity.Wallet{
		OwnerID:  "userA",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(100),
	}

	b := entity.Wallet{
		OwnerID:  "userB",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(100),
	}

	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		_, _, err := svc.Transfer(ctx, &model.TransferWalletRequest{
			SenderID:         "userA",
			SenderCurrency:   "USD",
			ReceiverID:       "userB",
			ReceiverCurrency: "USD",
			IdempotencyKey:   "cross-1",
			Amount:           "10.00",
		})

		if err != nil {
			t.Logf("transfer A->B error: %v", err)
		}
	}()

	go func() {
		defer wg.Done()

		_, _, err := svc.Transfer(ctx, &model.TransferWalletRequest{
			SenderID:         "userB",
			SenderCurrency:   "USD",
			ReceiverID:       "userA",
			ReceiverCurrency: "USD",
			IdempotencyKey:   "cross-2",
			Amount:           "20.00",
		})

		if err != nil {
			t.Logf("transfer B->A error: %v", err)
		}
	}()

	wg.Wait()

	db.First(&a, a.ID)
	db.First(&b, b.ID)

	total := a.Balance.Add(b.Balance)

	expected := decimal.NewFromInt(200)

	if !total.Equal(expected) {
		t.Fatalf(
			"system money invariant broken: expected %s got %s",
			expected.String(),
			total.String(),
		)
	}
}

func TestPayment_Concurrent(t *testing.T) {
	db, svc := setupTestDB(t)
	ctx := context.Background()

	wallet := entity.Wallet{
		OwnerID:  "user1",
		Currency: "USD",
		Status:   "ACTIVE",
		Balance:  decimal.NewFromInt(50),
	}

	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}

	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {

		go func(i int) {
			defer wg.Done()

			_, err := svc.Payment(ctx, &model.PaymentWalletRequest{
				ID:             wallet.ID,
				OwnerID:        "user1",
				Currency:       "USD",
				IdempotencyKey: fmt.Sprintf("pay-race-%d", i),
				Amount:         "10.00",
			})

			if err != nil {
				t.Logf("payment failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	var updated entity.Wallet
	db.First(&updated, wallet.ID)

	if updated.Balance.LessThan(decimal.Zero) {
		t.Fatalf("wallet balance became negative: %s", updated.Balance.String())
	}
}
