package handlers

import (
	"net/http"

	"banking/internal/config"
	"banking/internal/db"
	"banking/internal/middleware"
	"banking/internal/store"
	"banking/internal/websocket"

	"github.com/go-chi/cors"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	reconcileDB  store.Selecter
	txRunner     db.TxRunner
	cfg          config.Config
	users        UserStore
	accounts     AccountStore
	ledger       LedgerStore
	transactions TransactionStore
	exchange     ExchangeStore
	admin        AdminStore
	audit        AuditStore
	service      TransactionService
	hub          *websocket.Hub
}

func New(reconcileDB store.Selecter, txRunner db.TxRunner, cfg config.Config, users UserStore, accounts AccountStore, ledger LedgerStore, transactions TransactionStore, exchange ExchangeStore, admin AdminStore, audit AuditStore, service TransactionService, hub *websocket.Hub) *Handler {
	return &Handler{
		reconcileDB:  reconcileDB,
		txRunner:     txRunner,
		cfg:          cfg,
		users:        users,
		accounts:     accounts,
		ledger:       ledger,
		transactions: transactions,
		exchange:     exchange,
		admin:        admin,
		audit:        audit,
		service:      service,
		hub:          hub,
	}
}

func (h *Handler) Routes() http.Handler {
	router := chi.NewRouter()
	router.Use(chimiddleware.Logger)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{h.cfg.AllowedOrigins},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	router.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
		r.With(middleware.Auth(h.cfg.JWTSecret)).Get("/me", h.Me)
	})
	router.With(middleware.Auth(h.cfg.JWTSecret)).Get("/accounts", h.ListAccounts)
	router.With(middleware.Auth(h.cfg.JWTSecret)).Get("/accounts/{id}/balance", h.GetBalance)
	router.With(middleware.Auth(h.cfg.JWTSecret)).Get("/accounts/self-check", h.SelfCheck)
	router.With(middleware.Auth(h.cfg.JWTSecret)).Post("/transactions/transfer", h.Transfer)
	router.With(middleware.Auth(h.cfg.JWTSecret)).Post("/transactions/exchange/quote", h.ExchangeQuote)
	router.With(middleware.Auth(h.cfg.JWTSecret)).Post("/transactions/exchange", h.Exchange)
	router.With(middleware.Auth(h.cfg.JWTSecret)).Get("/transactions", h.ListTransactions)
	router.With(middleware.Auth(h.cfg.JWTSecret)).Get("/users/username/{username}", h.GetUserByUsername)
	router.With(middleware.Auth(h.cfg.JWTSecret)).Get("/users/email/{email}", h.GetUserByEmail)
	router.Get("/ws/balances", h.WSBalances)

	router.Route("/admin", func(r chi.Router) {
		r.Use(middleware.Auth(h.cfg.JWTSecret))
		r.With(middleware.RequireAdmin(h.admin, "CanViewUsers")).Get("/users", h.AdminListUsers)
		r.With(middleware.RequireAdmin(h.admin, "CanViewTransactions")).Get("/transactions", h.AdminListTransactions)
		r.With(middleware.RequireAdmin(h.admin, "")).Post("/roles/grant", h.GrantRole)
		r.With(middleware.RequireAdmin(h.admin, "")).Post("/promote", h.PromoteAdmin)
		r.With(middleware.RequireAdmin(h.admin, "CanViewTransactions")).Get("/audit", h.ListAuditLogs)
		r.With(middleware.RequireAdmin(h.admin, "CanViewTransactions")).Get("/reconcile", h.Reconcile)
	})

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return router
}
