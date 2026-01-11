package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestTransactionStoreCreate(t *testing.T) {
	ctx := context.Background()
	execer := stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "INSERT INTO transactions") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 11 || args[0] != "tx-1" {
				t.Fatalf("unexpected args: %#v", args)
			}
			return stubResult{rows: 1}, nil
		},
	}
	store := NewTransactionStore(stubDB{})
	err := store.Create(ctx, execer, TransactionInput{ID: "tx-1", Amount: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransactionStoreUpdateStatus(t *testing.T) {
	ctx := context.Background()
	execer := stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "UPDATE transactions") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 2 || args[0] != "done" || args[1] != "tx-1" {
				t.Fatalf("unexpected args: %#v", args)
			}
			return stubResult{rows: 1}, nil
		},
	}
	store := NewTransactionStore(stubDB{})
	if err := store.UpdateStatus(ctx, execer, "tx-1", "done"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransactionStoreListByUser(t *testing.T) {
	ctx := context.Background()
	store := NewTransactionStore(stubDB{
		selectFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "LEFT JOIN users") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "LEFT JOIN accounts fa") || !strings.Contains(query, "LEFT JOIN accounts ta") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "from_username") || !strings.Contains(query, "to_username") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "WHERE (t.user_id = $1 OR t.to_account_id IN (SELECT id FROM accounts WHERE user_id = $1))") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "LIMIT $2 OFFSET $3") {
				t.Fatalf("unexpected limit/offset in query: %s", query)
			}
			if len(args) != 3 || args[0] != "user-1" || args[1] != 10 || args[2] != 0 {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*[]transactionRow) = []transactionRow{{ID: "tx-1"}}
			return nil
		},
	})
	rows, err := store.ListByUser(ctx, "user-1", "", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0]["id"] != "tx-1" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestTransactionStoreListByUserWithType(t *testing.T) {
	ctx := context.Background()
	store := NewTransactionStore(stubDB{
		selectFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "LEFT JOIN users") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "LEFT JOIN accounts fa") || !strings.Contains(query, "LEFT JOIN accounts ta") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "from_username") || !strings.Contains(query, "to_username") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "AND type = $2") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "LIMIT $3 OFFSET $4") {
				t.Fatalf("unexpected limit/offset in query: %s", query)
			}
			if len(args) != 4 || args[0] != "user-1" || args[1] != "transfer" {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*[]transactionRow) = []transactionRow{{ID: "tx-1"}}
			return nil
		},
	})
	rows, err := store.ListByUser(ctx, "user-1", "transfer", 5, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestTransactionStoreListAll(t *testing.T) {
	ctx := context.Background()
	store := NewTransactionStore(stubDB{
		selectFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "LEFT JOIN users") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "LEFT JOIN accounts fa") || !strings.Contains(query, "LEFT JOIN accounts ta") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "from_username") || !strings.Contains(query, "to_username") {
				t.Fatalf("unexpected query: %s", query)
			}
			if !strings.Contains(query, "FROM transactions t") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 2 || args[0] != 10 || args[1] != 10 {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*[]transactionRow) = []transactionRow{{ID: "tx-1"}}
			return nil
		},
	})
	rows, err := store.ListAll(ctx, 10, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}
