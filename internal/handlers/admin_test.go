package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"banking/internal/auth"
	"banking/internal/middleware"
	"banking/internal/store"
)

func TestPromoteAdminForbidden(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return true, false, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{})

	body := []byte(`{"identifier":"bob"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/promote", bytes.NewReader(body))
	token, _ := auth.GenerateToken("secret", "user-1", time.Minute)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.PromoteAdmin)).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestPromoteAdminSuccess(t *testing.T) {
	created := 0
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) {
			return map[string]any{"id": "user-2"}, nil
		},
		getByEmailFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:     func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		isAdminFn: func(context.Context, string) (bool, bool, error) { return true, true, nil },
		createAdminFn: func(context.Context, store.Execer, string, bool, *string) error {
			created++
			return nil
		},
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{})

	body := []byte(`{"identifier":"bob"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/promote", bytes.NewReader(body))
	token, _ := auth.GenerateToken("secret", "admin-1", time.Minute)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.PromoteAdmin)).ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	if created != 1 {
		t.Fatalf("expected admin creation")
	}
}

func TestGrantRoleSuccess(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		isAdminFn: func(_ context.Context, userID string) (bool, bool, error) {
			if userID == "admin-1" {
				return true, true, nil
			}
			return true, false, nil
		},
		grantRoleFn:   func(context.Context, store.Execer, string, string) error { return nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{})

	body := []byte(`{"admin_user_id":"target","role":"CanViewUsers"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/roles/grant", bytes.NewReader(body))
	token, _ := auth.GenerateToken("secret", "admin-1", time.Minute)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.GrantRole)).ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestSetExchangeRate(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return true, true, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{})

	body := []byte(`{"quote_currency":"EUR","rate":"1.1"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/exchange-rate", bytes.NewReader(body))
	token, _ := auth.GenerateToken("secret", "admin-1", time.Minute)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.SetExchangeRate)).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestAdminListUsers(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{
		listAllWithUsersFn: func(context.Context) ([]store.AccountWithUser, error) {
			return []store.AccountWithUser{{ID: "acc-1"}}, nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return true, true, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{})

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rr := httptest.NewRecorder()
	handler.AdminListUsers(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAdminListTransactions(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{
		listAllFn: func(context.Context, int, int) ([]map[string]any, error) {
			return []map[string]any{{"id": "tx-1"}}, nil
		},
		listByUserFn: func(context.Context, string, string, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubExchangeStore{}, stubAdminStore{
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return true, true, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{})

	req := httptest.NewRequest(http.MethodGet, "/admin/transactions", nil)
	rr := httptest.NewRecorder()
	handler.AdminListTransactions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestListAuditLogs(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return true, true, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
	}, stubAuditStore{
		listFn: func(context.Context, int, int) ([]map[string]any, error) {
			return []map[string]any{{"id": "log-1"}}, nil
		},
		logFn: func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
	}, stubService{})

	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	rr := httptest.NewRecorder()
	handler.ListAuditLogs(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestReconcile(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{
		selectFn: func(_ context.Context, dest any, _ string, _ ...any) error {
			value := reflect.ValueOf(dest)
			if value.Kind() != reflect.Ptr || value.Elem().Kind() != reflect.Slice {
				return nil
			}
			slice := reflect.MakeSlice(value.Elem().Type(), 1, 1)
			row := slice.Index(0)
			row.FieldByName("AccountID").SetString("acc-1")
			row.FieldByName("LedgerSum").SetInt(1000)
			row.FieldByName("AccountBalance").SetInt(1000)
			row.FieldByName("Difference").SetInt(0)
			value.Elem().Set(slice)
			return nil
		},
	}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return true, true, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
	}, stubAuditStore{
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
	}, stubService{})

	req := httptest.NewRequest(http.MethodGet, "/admin/reconcile", nil)
	rr := httptest.NewRecorder()
	handler.Reconcile(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestWSBalancesMissingToken(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return true, true, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
	}, stubAuditStore{
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
	}, stubService{})

	req := httptest.NewRequest(http.MethodGet, "/ws/balances", nil)
	rr := httptest.NewRecorder()
	handler.WSBalances(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestWSBalancesInvalidToken(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return true, true, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
	}, stubAuditStore{
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
	}, stubService{})

	req := httptest.NewRequest(http.MethodGet, "/ws/balances?token=bad", nil)
	rr := httptest.NewRecorder()
	handler.WSBalances(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
