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
	)

	validator := util.NewValidator()

	walletService := service.NewWalletService(database)
	walletHandler := handler.NewWalletHandler(walletService, validator)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /wallets", homeHandler)
	mux.HandleFunc("POST /wallets/{id}/topup", walletHandler.TopUp)
	mux.HandleFunc("POST /wallets/{id}/pay", healthHandler)
	mux.HandleFunc("POST /wallets/transfer", healthHandler)
	mux.HandleFunc("POST /wallets/{id}/suspend", healthHandler)
	mux.HandleFunc("GET /wallets/{id}", healthHandler)

	port := ":8080"
	fmt.Println("Server running on port" + port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatal(err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"message":"Hello World!"}`)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "ok")
}
