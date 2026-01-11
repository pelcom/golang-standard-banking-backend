package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"banking/internal/auth"
	"banking/internal/middleware"
	"banking/internal/store"

	"github.com/go-chi/chi/v5"
)

func TestListAccounts(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{
		getByUserFn: func(context.Context, string) ([]store.AccountBalanceSummary, error) {
			return []store.AccountBalanceSummary{
				{
					ID:                "acc-1",
					UserID:            stringPtr("user-1"),
					Currency:          "USD",
					StoredBalance:     int64(1000),
					CalculatedBalance: int64(1000),
					IsSystem:          false,
				},
			}, nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
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
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.ListAccounts)).ServeHTTP(rr, req)
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

func TestGetBalanceForbidden(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByUsernameFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:       func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:        func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{
		getByIDFn: func(context.Context, string) (store.Account, error) {
			return store.Account{ID: "acc-1", UserID: stringPtr("other"), Currency: "USD", Balance: int64(1000)}, nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
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
	req := httptest.NewRequest(http.MethodGet, "/accounts/acc-1/balance", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "acc-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.GetBalance)).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestSelfCheck(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{
		selectFn: func(_ context.Context, dest any, _ string, _ ...any) error {
			value := reflect.ValueOf(dest)
			if value.Kind() != reflect.Ptr || value.Elem().Kind() != reflect.Slice {
				return nil
			}
			slice := reflect.MakeSlice(value.Elem().Type(), 1, 1)
			row := slice.Index(0)
			row.FieldByName("AccountID").SetString("acc-1")
			row.FieldByName("Currency").SetString("USD")
			row.FieldByName("AccountBalance").SetInt(1000)
			row.FieldByName("LedgerSum").SetInt(1000)
			row.FieldByName("Difference").SetInt(0)
			value.Elem().Set(slice)
			return nil
		},
	}, fakeTxRunner{}, stubUserStore{
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
	}, stubService{})

	token, err := auth.GenerateToken("secret", "user-1", time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/accounts/self-check", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.SelfCheck)).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
