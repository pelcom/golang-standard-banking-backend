package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestAdminStoreIsAdminNoRows(t *testing.T) {
	ctx := context.Background()
	store := NewAdminStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			return sql.ErrNoRows
		},
	})
	isAdmin, isSuper, err := store.IsAdmin(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isAdmin || isSuper {
		t.Fatalf("expected non-admin result")
	}
}

func TestAdminStoreIsAdmin(t *testing.T) {
	ctx := context.Background()
	store := NewAdminStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "FROM admins") {
				t.Fatalf("unexpected query: %s", query)
			}
			*dest.(*bool) = true
			return nil
		},
	})
	isAdmin, isSuper, err := store.IsAdmin(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isAdmin || !isSuper {
		t.Fatalf("expected admin/super true")
	}
}

func TestAdminStoreHasRole(t *testing.T) {
	ctx := context.Background()
	store := NewAdminStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "FROM admin_roles") {
				t.Fatalf("unexpected query: %s", query)
			}
			*dest.(*int) = 1
			return nil
		},
	})
	hasRole, err := store.HasRole(ctx, "user-1", "role")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRole {
		t.Fatalf("expected role to be granted")
	}
}

func TestAdminStoreCreateAdmin(t *testing.T) {
	ctx := context.Background()
	execer := stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "INSERT INTO admins") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 3 || args[0] != "user-1" || args[1] != true {
				t.Fatalf("unexpected args: %#v", args)
			}
			return stubResult{rows: 1}, nil
		},
	}
	store := NewAdminStore(stubDB{})
	if err := store.CreateAdmin(ctx, execer, "user-1", true, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdminStoreGrantRole(t *testing.T) {
	ctx := context.Background()
	execer := stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "INSERT INTO admin_roles") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 2 || args[0] != "user-1" || args[1] != "role" {
				t.Fatalf("unexpected args: %#v", args)
			}
			return stubResult{rows: 1}, nil
		},
	}
	store := NewAdminStore(stubDB{})
	if err := store.GrantRole(ctx, execer, "user-1", "role"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdminStoreHasAnyAdmin(t *testing.T) {
	ctx := context.Background()
	store := NewAdminStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "SELECT COUNT(1)") {
				t.Fatalf("unexpected query: %s", query)
			}
			*dest.(*int) = 2
			return nil
		},
	})
	hasAny, err := store.HasAnyAdmin(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAny {
		t.Fatalf("expected admins to exist")
	}
}
