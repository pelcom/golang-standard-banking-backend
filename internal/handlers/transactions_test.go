package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"banking/internal/auth"
	"banking/internal/middleware"
	"banking/internal/services"
	"banking/internal/store"

	"github.com/lib/pq"
)

func TestTransferSuccess(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return false, false, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{
		transferFn: func(context.Context, services.TransferRequest) (string, error) {
			return "tx-1", nil
		},
		exchangeFn: func(context.Context, services.ExchangeRequest) (string, error) { return "", nil },
	})

	body := []byte(`{"from_account_id":"a1","to_account_id":"a2","amount":"10.00","confirm":true}`)
	req := httptest.NewRequest(http.MethodPost, "/transactions/transfer", bytes.NewReader(body))
	token, err := auth.GenerateToken("secret", "user-1", time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.Transfer)).ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestTransferInsufficientFunds(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return false, false, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{
		transferFn: func(context.Context, services.TransferRequest) (string, error) {
			return "", services.ErrInsufficientFunds
		},
		exchangeFn: func(context.Context, services.ExchangeRequest) (string, error) { return "", nil },
	})

	body := []byte(`{"from_account_id":"a1","to_account_id":"a2","amount":"10.00","confirm":true}`)
	req := httptest.NewRequest(http.MethodPost, "/transactions/transfer", bytes.NewReader(body))
	token, err := auth.GenerateToken("secret", "user-1", time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.Transfer)).ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestTransferDuplicateRequest(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return false, false, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{
		transferFn: func(context.Context, services.TransferRequest) (string, error) {
			return "", &pq.Error{Code: "23505"}
		},
	})

	body := []byte(`{"from_account_id":"a1","to_account_id":"a2","amount":"10.00","confirm":true}`)
	req := httptest.NewRequest(http.MethodPost, "/transactions/transfer", bytes.NewReader(body))
	token, err := auth.GenerateToken("secret", "user-1", time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.Transfer)).ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestExchangeSuccess(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return false, false, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{
		transferFn: func(context.Context, services.TransferRequest) (string, error) { return "", nil },
		exchangeFn: func(context.Context, services.ExchangeRequest) (string, error) { return "tx-2", nil },
	})

	body := []byte(`{"from_account_id":"a1","to_account_id":"a2","amount":"10.00","confirm":true,"quoted_rate":"0.920000"}`)
	req := httptest.NewRequest(http.MethodPost, "/transactions/exchange", bytes.NewReader(body))
	token, err := auth.GenerateToken("secret", "user-1", time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.Exchange)).ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestExchangeQuote(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return false, false, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{
		quoteFn: func(context.Context, services.ExchangeQuoteRequest) (services.ExchangeQuote, error) {
			return services.ExchangeQuote{
				ID:             "quote-1",
				Rate:           "0.920000",
				ConvertedMinor: 920,
				ExpiresAt:      time.Now().Add(1 * time.Minute),
			}, nil
		},
	})

	body := []byte(`{"from_account_id":"a1","to_account_id":"a2","amount":"10.00"}`)
	req := httptest.NewRequest(http.MethodPost, "/transactions/exchange/quote", bytes.NewReader(body))
	token, err := auth.GenerateToken("secret", "user-1", time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.ExchangeQuote)).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestListTransactions(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{
		listByUserFn: func(context.Context, string, string, int, int) ([]map[string]any, error) {
			return []map[string]any{{"id": "tx-1"}}, nil
		},
		listAllFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return false, false, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{})

	token, err := auth.GenerateToken("secret", "user-1", time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.ListTransactions)).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var payload []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestExchangeDuplicateRequest(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return false, false, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{
		exchangeFn: func(context.Context, services.ExchangeRequest) (string, error) {
			return "", &pq.Error{Code: "23505"}
		},
	})

	body := []byte(`{"from_account_id":"a1","to_account_id":"a2","amount":"10.00","confirm":true,"quoted_rate":"0.920000"}`)
	req := httptest.NewRequest(http.MethodPost, "/transactions/exchange", bytes.NewReader(body))
	token, err := auth.GenerateToken("secret", "user-1", time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.Exchange)).ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}
