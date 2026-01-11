package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"banking/internal/auth"
	"banking/internal/config"
	"banking/internal/db"
	"banking/internal/middleware"
	"banking/internal/services"
	"banking/internal/store"
	"banking/internal/websocket"

	"github.com/jmoiron/sqlx"
)

type fakeTxRunner struct {
	withTxFn func(ctx context.Context, fn func(*sqlx.Tx) error) error
}

func (f fakeTxRunner) WithTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	if f.withTxFn != nil {
		return f.withTxFn(ctx, fn)
	}
	return fn(nil)
}

type stubReconcileDB struct {
	selectFn func(ctx context.Context, dest any, query string, args ...any) error
}

func (s stubReconcileDB) SelectContext(ctx context.Context, dest any, query string, args ...any) error {
	if s.selectFn == nil {
		return nil
	}
	return s.selectFn(ctx, dest, query, args...)
}

type stubUserStore struct {
	createFn        func(ctx context.Context, tx store.Execer, id, username, email, passwordHash string) error
	getByEmailFn    func(ctx context.Context, email string) (map[string]any, error)
	getByUsernameFn func(ctx context.Context, username string) (map[string]any, error)
	getByIDFn       func(ctx context.Context, userID string) (map[string]any, error)
}

func (s stubUserStore) Create(ctx context.Context, tx store.Execer, id, username, email, passwordHash string) error {
	if s.createFn == nil {
		return nil
	}
	return s.createFn(ctx, tx, id, username, email, passwordHash)
}

func (s stubUserStore) GetByEmail(ctx context.Context, email string) (map[string]any, error) {
	if s.getByEmailFn == nil {
		return nil, nil
	}
	return s.getByEmailFn(ctx, email)
}

func (s stubUserStore) GetByUsername(ctx context.Context, username string) (map[string]any, error) {
	if s.getByUsernameFn == nil {
		return nil, nil
	}
	return s.getByUsernameFn(ctx, username)
}

func (s stubUserStore) GetByID(ctx context.Context, userID string) (map[string]any, error) {
	if s.getByIDFn == nil {
		return nil, nil
	}
	return s.getByIDFn(ctx, userID)
}

type stubAccountStore struct {
	createFn            func(ctx context.Context, tx store.Execer, id string, userID *string, currency string, balance int64, isSystem bool) error
	getByUserFn         func(ctx context.Context, userID string) ([]store.AccountBalanceSummary, error)
	getByUserCurrencyFn func(ctx context.Context, userID, currency string) (store.Account, error)
	getByIDFn           func(ctx context.Context, accountID string) (store.Account, error)
	listAllWithUsersFn  func(ctx context.Context) ([]store.AccountWithUser, error)
	getSystemAccountFn  func(ctx context.Context, currency string) (string, error)
	adjustBalanceFn     func(ctx context.Context, tx store.Execer, accountID string, delta int64) (int64, error)
}

func (s stubAccountStore) Create(ctx context.Context, tx store.Execer, id string, userID *string, currency string, balance int64, isSystem bool) error {
	if s.createFn == nil {
		return nil
	}
	return s.createFn(ctx, tx, id, userID, currency, balance, isSystem)
}

func (s stubAccountStore) GetByUser(ctx context.Context, userID string) ([]store.AccountBalanceSummary, error) {
	if s.getByUserFn == nil {
		return nil, nil
	}
	return s.getByUserFn(ctx, userID)
}

func (s stubAccountStore) GetByUserAndCurrency(ctx context.Context, userID, currency string) (store.Account, error) {
	if s.getByUserCurrencyFn == nil {
		return store.Account{}, nil
	}
	return s.getByUserCurrencyFn(ctx, userID, currency)
}

func (s stubAccountStore) GetByID(ctx context.Context, accountID string) (store.Account, error) {
	if s.getByIDFn == nil {
		return store.Account{}, nil
	}
	return s.getByIDFn(ctx, accountID)
}

func (s stubAccountStore) ListAllWithUsers(ctx context.Context) ([]store.AccountWithUser, error) {
	if s.listAllWithUsersFn == nil {
		return nil, nil
	}
	return s.listAllWithUsersFn(ctx)
}

func (s stubAccountStore) GetSystemAccount(ctx context.Context, currency string) (string, error) {
	if s.getSystemAccountFn == nil {
		return "", nil
	}
	return s.getSystemAccountFn(ctx, currency)
}

func (s stubAccountStore) AdjustBalance(ctx context.Context, tx store.Execer, accountID string, delta int64) (int64, error) {
	if s.adjustBalanceFn == nil {
		return 1, nil
	}
	return s.adjustBalanceFn(ctx, tx, accountID, delta)
}

type stubLedgerStore struct {
	insertFn func(ctx context.Context, tx store.Execer, entries []store.LedgerEntryInput) error
}

func (s stubLedgerStore) InsertEntries(ctx context.Context, tx store.Execer, entries []store.LedgerEntryInput) error {
	if s.insertFn == nil {
		return nil
	}
	return s.insertFn(ctx, tx, entries)
}

type stubTransactionStore struct {
	listByUserFn func(ctx context.Context, userID, txType string, limit, offset int) ([]map[string]any, error)
	listAllFn    func(ctx context.Context, limit, offset int) ([]map[string]any, error)
}

func (s stubTransactionStore) ListByUser(ctx context.Context, userID, txType string, limit, offset int) ([]map[string]any, error) {
	if s.listByUserFn == nil {
		return nil, nil
	}
	return s.listByUserFn(ctx, userID, txType, limit, offset)
}

func (s stubTransactionStore) ListAll(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	if s.listAllFn == nil {
		return nil, nil
	}
	return s.listAllFn(ctx, limit, offset)
}

type stubExchangeStore struct {
	setRateFn func(ctx context.Context, tx store.Tx, baseCurrency, quoteCurrency, rate string, actorID string) (string, error)
}

func (s stubExchangeStore) SetRate(ctx context.Context, tx store.Tx, baseCurrency, quoteCurrency, rate string, actorID string) (string, error) {
	if s.setRateFn == nil {
		return "", nil
	}
	return s.setRateFn(ctx, tx, baseCurrency, quoteCurrency, rate, actorID)
}

type stubAdminStore struct {
	isAdminFn     func(ctx context.Context, userID string) (bool, bool, error)
	hasRoleFn     func(ctx context.Context, userID, role string) (bool, error)
	createAdminFn func(ctx context.Context, tx store.Execer, userID string, isSuper bool, createdBy *string) error
	grantRoleFn   func(ctx context.Context, tx store.Execer, adminUserID, role string) error
	hasAnyAdminFn func(ctx context.Context) (bool, error)
}

func (s stubAdminStore) IsAdmin(ctx context.Context, userID string) (bool, bool, error) {
	if s.isAdminFn == nil {
		return false, false, nil
	}
	return s.isAdminFn(ctx, userID)
}

func (s stubAdminStore) HasRole(ctx context.Context, userID, role string) (bool, error) {
	if s.hasRoleFn == nil {
		return false, nil
	}
	return s.hasRoleFn(ctx, userID, role)
}

func (s stubAdminStore) CreateAdmin(ctx context.Context, tx store.Execer, userID string, isSuper bool, createdBy *string) error {
	if s.createAdminFn == nil {
		return nil
	}
	return s.createAdminFn(ctx, tx, userID, isSuper, createdBy)
}

func (s stubAdminStore) GrantRole(ctx context.Context, tx store.Execer, adminUserID, role string) error {
	if s.grantRoleFn == nil {
		return nil
	}
	return s.grantRoleFn(ctx, tx, adminUserID, role)
}

func (s stubAdminStore) HasAnyAdmin(ctx context.Context) (bool, error) {
	if s.hasAnyAdminFn == nil {
		return false, nil
	}
	return s.hasAnyAdminFn(ctx)
}

type stubAuditStore struct {
	logFn  func(ctx context.Context, tx store.Execer, actorID, action, entityType, entityID, data string) error
	listFn func(ctx context.Context, limit, offset int) ([]map[string]any, error)
}

func (s stubAuditStore) Log(ctx context.Context, tx store.Execer, actorID, action, entityType, entityID, data string) error {
	if s.logFn == nil {
		return nil
	}
	return s.logFn(ctx, tx, actorID, action, entityType, entityID, data)
}

func (s stubAuditStore) List(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	if s.listFn == nil {
		return nil, nil
	}
	return s.listFn(ctx, limit, offset)
}

type stubService struct {
	transferFn func(ctx context.Context, req services.TransferRequest) (string, error)
	exchangeFn func(ctx context.Context, req services.ExchangeRequest) (string, error)
	quoteFn    func(ctx context.Context, req services.ExchangeQuoteRequest) (services.ExchangeQuote, error)
}

func (s stubService) Transfer(ctx context.Context, req services.TransferRequest) (string, error) {
	if s.transferFn == nil {
		return "", nil
	}
	return s.transferFn(ctx, req)
}

func (s stubService) Exchange(ctx context.Context, req services.ExchangeRequest) (string, error) {
	if s.exchangeFn == nil {
		return "", nil
	}
	return s.exchangeFn(ctx, req)
}

func (s stubService) QuoteExchange(ctx context.Context, req services.ExchangeQuoteRequest) (services.ExchangeQuote, error) {
	if s.quoteFn == nil {
		return services.ExchangeQuote{}, nil
	}
	return s.quoteFn(ctx, req)
}

func newTestHandler(reconcileDB store.Selecter, txRunner db.TxRunner, users UserStore, accounts AccountStore, ledger LedgerStore, transactions TransactionStore, exchange ExchangeStore, admin AdminStore, audit AuditStore, service TransactionService) *Handler {
	cfg := config.Config{
		AppEnv:         "test",
		Port:           "0",
		DatabaseURL:    "",
		JWTSecret:      "secret",
		TokenTTL:       time.Minute,
		AllowedOrigins: "*",
	}
	return New(reconcileDB, txRunner, cfg, users, accounts, ledger, transactions, exchange, admin, audit, service, websocket.NewHub())
}

func serveWithAuth(t *testing.T, handler http.HandlerFunc, userID string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := auth.GenerateToken("secret", userID, time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	middleware.Auth("secret")(handler).ServeHTTP(rr, req)
	return rr
}

func stringPtr(value string) *string {
	return &value
}
