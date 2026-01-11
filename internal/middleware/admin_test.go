package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubAdminStore struct {
	isAdminFn func(ctx context.Context, userID string) (bool, bool, error)
	hasRoleFn func(ctx context.Context, userID, role string) (bool, error)
}

func (s stubAdminStore) IsAdmin(ctx context.Context, userID string) (bool, bool, error) {
	return s.isAdminFn(ctx, userID)
}

func (s stubAdminStore) HasRole(ctx context.Context, userID, role string) (bool, error) {
	return s.hasRoleFn(ctx, userID, role)
}

func TestRequireAdminMissingUser(t *testing.T) {
	handler := RequireAdmin(stubAdminStore{
		isAdminFn: func(context.Context, string) (bool, bool, error) {
			t.Fatalf("unexpected call")
			return false, false, nil
		},
		hasRoleFn: func(context.Context, string, string) (bool, error) {
			t.Fatalf("unexpected call")
			return false, nil
		},
	}, "role")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called")
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAdminNotAdmin(t *testing.T) {
	handler := RequireAdmin(stubAdminStore{
		isAdminFn: func(context.Context, string) (bool, bool, error) {
			return false, false, nil
		},
		hasRoleFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}, "role")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called")
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user-1"))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestRequireAdminSuperUser(t *testing.T) {
	handler := RequireAdmin(stubAdminStore{
		isAdminFn: func(context.Context, string) (bool, bool, error) {
			return true, true, nil
		},
		hasRoleFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}, "role")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user-1"))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestRequireAdminMissingRole(t *testing.T) {
	handler := RequireAdmin(stubAdminStore{
		isAdminFn: func(context.Context, string) (bool, bool, error) {
			return true, false, nil
		},
		hasRoleFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}, "role")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called")
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user-1"))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestRequireAdminWithRole(t *testing.T) {
	handler := RequireAdmin(stubAdminStore{
		isAdminFn: func(context.Context, string) (bool, bool, error) {
			return true, false, nil
		},
		hasRoleFn: func(context.Context, string, string) (bool, error) {
			return true, nil
		},
	}, "role")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user-1"))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func contextWithUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
