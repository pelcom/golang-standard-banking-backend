package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"banking/internal/config"
	"banking/internal/db"
	"banking/internal/handlers"
	"banking/internal/services"
	"banking/internal/store"
	"banking/internal/websocket"
)

func main() {
	cfg := config.Load()
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer database.Close()

	users := store.NewUserStore(database)
	accounts := store.NewAccountStore(database)
	ledger := store.NewLedgerStore(database)
	transactions := store.NewTransactionStore(database)
	exchange := store.NewExchangeStore(database)
	quotes := store.NewExchangeQuoteStore(database)
	admin := store.NewAdminStore(database)
	audit := store.NewAuditStore(database)
	txRunner := db.NewTxRunner(database)
	hub := websocket.NewHub()
	service := services.NewTransactionService(txRunner, accounts, ledger, transactions, exchange, quotes, audit, hub)

	handler := handlers.New(database, txRunner, cfg, users, accounts, ledger, transactions, exchange, admin, audit, service, hub)
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("banking API listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
}
