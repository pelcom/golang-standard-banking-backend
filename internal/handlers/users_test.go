package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"banking/internal/store"

	"github.com/go-chi/chi/v5"
)

func TestGetUserByUsername(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByUsernameFn: func(context.Context, string) (map[string]any, error) {
			return map[string]any{"id": "user-1", "username": "alice", "email": "a@b.com"}, nil
		},
		getByEmailFn: func(context.Context, string) (map[string]any, error) { return nil, nil },
		getByIDFn:    func(context.Context, string) (map[string]any, error) { return nil, nil },
		createFn:     func(context.Context, store.Execer, string, string, string, string) error { return nil },
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) { return true, nil },
		isAdminFn:     func(context.Context, string) (bool, bool, error) { return false, false, nil },
		hasRoleFn:     func(context.Context, string, string) (bool, error) { return false, nil },
	}, stubAuditStore{
		logFn:  func(context.Context, store.Execer, string, string, string, string, string) error { return nil },
		listFn: func(context.Context, int, int) ([]map[string]any, error) { return nil, nil },
	}, stubService{})

	req := httptest.NewRequest(http.MethodGet, "/users/username/alice", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("username", "alice")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()
	handler.GetUserByUsername(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestGetUserByEmail(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn: func(context.Context, string) (map[string]any, error) {
			return map[string]any{"id": "user-1", "username": "alice", "email": "a@b.com"}, nil
		},
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

	req := httptest.NewRequest(http.MethodGet, "/users/email/a@b.com", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("email", "a@b.com")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()
	handler.GetUserByEmail(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
