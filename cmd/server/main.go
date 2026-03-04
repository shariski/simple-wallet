package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/shariski/simple-wallet/internal/db"
	"github.com/shariski/simple-wallet/internal/entity"
	"github.com/shariski/simple-wallet/internal/handler"
	"github.com/shariski/simple-wallet/internal/service"
	"github.com/shariski/simple-wallet/internal/util"
)

func main() {
	dsn := "host=localhost user=admin password=admin dbname=wallet port=5432 sslmode=disable"

	database, err := db.NewPostgres(dsn)
	if err != nil {
		log.Fatal(err)
	}

	err = database.AutoMigrate(
		&entity.Wallet{},
		&entity.LedgerEntry{},
		&entity.IdempotencyKey{},
	)

	validator := util.NewValidator()

	walletService := service.NewWalletService(database)
	walletHandler := handler.NewWalletHandler(walletService, validator)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /wallets", walletHandler.Create)
	mux.HandleFunc("POST /wallets/{id}/topup", walletHandler.TopUp)
	mux.HandleFunc("POST /wallets/{id}/pay", walletHandler.Payment)
	mux.HandleFunc("POST /wallets/transfer", walletHandler.Transfer)
	mux.HandleFunc("POST /wallets/{id}/suspend", walletHandler.Suspend)
	mux.HandleFunc("GET /wallets/{id}", walletHandler.Status)

	port := ":8080"
	fmt.Println("Server running on port" + port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatal(err)
	}
}
