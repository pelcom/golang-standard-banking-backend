package handlers

import (
	"context"

	"banking/internal/services"
	"banking/internal/store"
)

type UserStore interface {
	Create(ctx context.Context, tx store.Execer, id, username, email, passwordHash string) error
	GetByEmail(ctx context.Context, email string) (map[string]any, error)
	GetByUsername(ctx context.Context, username string) (map[string]any, error)
	GetByID(ctx context.Context, userID string) (map[string]any, error)
}

type AccountStore interface {
	Create(ctx context.Context, tx store.Execer, id string, userID *string, currency string, balance int64, isSystem bool) error
	GetByUser(ctx context.Context, userID string) ([]store.AccountBalanceSummary, error)
	GetByUserAndCurrency(ctx context.Context, userID, currency string) (store.Account, error)
	GetByID(ctx context.Context, accountID string) (store.Account, error)
	ListAllWithUsers(ctx context.Context) ([]store.AccountWithUser, error)
	GetSystemAccount(ctx context.Context, currency string) (string, error)
	AdjustBalance(ctx context.Context, tx store.Execer, accountID string, delta int64) (int64, error)
}

type LedgerStore interface {
	InsertEntries(ctx context.Context, tx store.Execer, entries []store.LedgerEntryInput) error
}

type TransactionStore interface {
	ListByUser(ctx context.Context, userID, txType string, limit, offset int) ([]map[string]any, error)
	ListAll(ctx context.Context, limit, offset int) ([]map[string]any, error)
}

type ExchangeStore interface {
	SetRate(ctx context.Context, tx store.Tx, baseCurrency, quoteCurrency, rate string, actorID string) (string, error)
}

type AdminStore interface {
	IsAdmin(ctx context.Context, userID string) (bool, bool, error)
	HasRole(ctx context.Context, userID, role string) (bool, error)
	CreateAdmin(ctx context.Context, tx store.Execer, userID string, isSuper bool, createdBy *string) error
	GrantRole(ctx context.Context, tx store.Execer, adminUserID, role string) error
	HasAnyAdmin(ctx context.Context) (bool, error)
}

type AuditStore interface {
	Log(ctx context.Context, tx store.Execer, actorID, action, entityType, entityID, data string) error
	List(ctx context.Context, limit, offset int) ([]map[string]any, error)
}

type TransactionService interface {
	Transfer(ctx context.Context, req services.TransferRequest) (string, error)
	Exchange(ctx context.Context, req services.ExchangeRequest) (string, error)
	QuoteExchange(ctx context.Context, req services.ExchangeQuoteRequest) (services.ExchangeQuote, error)
}
