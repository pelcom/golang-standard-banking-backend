package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestAccountStoreCreate(t *testing.T) {
	ctx := context.Background()
	execer := stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "INSERT INTO accounts") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 5 {
				t.Fatalf("expected 5 args, got %d", len(args))
			}
			if args[0] != "acc-1" || args[2] != "USD" || args[3] != int64(1000) || args[4] != true {
				t.Fatalf("unexpected args: %#v", args)
			}
			if args[1] != nil {
				ptr, ok := args[1].(*string)
				if !ok || ptr != nil {
					t.Fatalf("unexpected user id arg: %#v", args[1])
				}
			}
			return stubResult{rows: 1}, nil
		},
	}
	store := NewAccountStore(stubDB{})
	if err := store.Create(ctx, execer, "acc-1", nil, "USD", 1000, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAccountStoreGetByUser(t *testing.T) {
	ctx := context.Background()
	store := NewAccountStore(stubDB{
		selectFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "FROM accounts a") || !strings.Contains(query, "ledger_entries") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 1 || args[0] != "user-1" {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*[]AccountBalanceSummary) = []AccountBalanceSummary{{ID: "acc-1"}}
			return nil
		},
	})
	rows, err := store.GetByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "acc-1" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestAccountStoreGetByUserAndCurrency(t *testing.T) {
	ctx := context.Background()
	store := NewAccountStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "WHERE user_id = $1 AND currency = $2") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 2 || args[0] != "user-1" || args[1] != "USD" {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*Account) = Account{ID: "acc-1"}
			return nil
		},
	})
	row, err := store.GetByUserAndCurrency(ctx, "user-1", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.ID != "acc-1" {
		t.Fatalf("unexpected row: %#v", row)
	}
}

func TestAccountStoreGetByID(t *testing.T) {
	ctx := context.Background()
	store := NewAccountStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "WHERE id = $1") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 1 || args[0] != "acc-1" {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*Account) = Account{ID: "acc-1"}
			return nil
		},
	})
	row, err := store.GetByID(ctx, "acc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.ID != "acc-1" {
		t.Fatalf("unexpected row: %#v", row)
	}
}

func TestAccountStoreGetForUpdate(t *testing.T) {
	ctx := context.Background()
	getter := stubGetter{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "FOR UPDATE") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 1 || args[0] != "acc-1" {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*Account) = Account{ID: "acc-1"}
			return nil
		},
	}
	store := NewAccountStore(stubDB{})
	row, err := store.GetForUpdate(ctx, getter, "acc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.ID != "acc-1" {
		t.Fatalf("unexpected row: %#v", row)
	}
}

func TestAccountStoreUpdateBalance(t *testing.T) {
	ctx := context.Background()
	execer := stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "UPDATE accounts") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 2 || args[0] != int64(9900) || args[1] != "acc-1" {
				t.Fatalf("unexpected args: %#v", args)
			}
			return stubResult{rows: 1}, nil
		},
	}
	store := NewAccountStore(stubDB{})
	if err := store.UpdateBalance(ctx, execer, "acc-1", 9900); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAccountStoreAdjustBalance(t *testing.T) {
	ctx := context.Background()
	execer := stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "SET balance = balance + $1") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 2 || args[0] != int64(500) || args[1] != "acc-1" {
				t.Fatalf("unexpected args: %#v", args)
			}
			return stubResult{rows: 2}, nil
		},
	}
	store := NewAccountStore(stubDB{})
	rows, err := store.AdjustBalance(ctx, execer, "acc-1", 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows != 2 {
		t.Fatalf("expected 2 rows affected, got %d", rows)
	}
}

func TestAccountStoreGetSystemAccount(t *testing.T) {
	ctx := context.Background()
	store := NewAccountStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "WHERE is_system = TRUE") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 1 || args[0] != "USD" {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*string) = "sys-1"
			return nil
		},
	})
	id, err := store.GetSystemAccount(ctx, "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "sys-1" {
		t.Fatalf("unexpected id: %s", id)
	}
}

func TestAccountStoreListAllWithUsers(t *testing.T) {
	ctx := context.Background()
	store := NewAccountStore(stubDB{
		selectFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "LEFT JOIN users") {
				t.Fatalf("unexpected query: %s", query)
			}
			*dest.(*[]AccountWithUser) = []AccountWithUser{{ID: "acc-1"}}
			return nil
		},
	})
	rows, err := store.ListAllWithUsers(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "acc-1" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}
