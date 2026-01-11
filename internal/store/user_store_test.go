package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestUserStoreCreate(t *testing.T) {
	ctx := context.Background()
	execer := stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "INSERT INTO users") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 4 || args[0] != "user-1" || args[1] != "name" {
				t.Fatalf("unexpected args: %#v", args)
			}
			return stubResult{rows: 1}, nil
		},
	}
	store := NewUserStore(stubDB{})
	if err := store.Create(ctx, execer, "user-1", "name", "email@example.com", "hash"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserStoreGetByEmail(t *testing.T) {
	ctx := context.Background()
	store := NewUserStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "FROM users") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 1 || args[0] != "email@example.com" {
				t.Fatalf("unexpected args: %#v", args)
			}
			row := dest.(*userRow)
			*row = userRow{ID: "user-1"}
			return nil
		},
	})
	row, err := store.GetByEmail(ctx, "email@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row["id"] != "user-1" {
		t.Fatalf("unexpected row: %#v", row)
	}
}

func TestUserStoreGetByUsername(t *testing.T) {
	ctx := context.Background()
	store := NewUserStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "WHERE username = $1") {
				t.Fatalf("unexpected query: %s", query)
			}
			row := dest.(*userRow)
			*row = userRow{ID: "user-1"}
			return nil
		},
	})
	row, err := store.GetByUsername(ctx, "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row["id"] != "user-1" {
		t.Fatalf("unexpected row: %#v", row)
	}
}

func TestUserStoreGetByID(t *testing.T) {
	ctx := context.Background()
	store := NewUserStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "WHERE id = $1") {
				t.Fatalf("unexpected query: %s", query)
			}
			row := dest.(*userRow)
			*row = userRow{ID: "user-1"}
			return nil
		},
	})
	row, err := store.GetByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row["id"] != "user-1" {
		t.Fatalf("unexpected row: %#v", row)
	}
}
