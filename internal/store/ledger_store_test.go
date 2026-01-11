package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestLedgerStoreInsertEntries(t *testing.T) {
	ctx := context.Background()
	calls := 0
	execer := stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "INSERT INTO ledger_entries") {
				t.Fatalf("unexpected query: %s", query)
			}
			calls++
			return stubResult{rows: 1}, nil
		},
	}
	store := NewLedgerStore(stubDB{})
	entries := []LedgerEntryInput{
		{ID: "1", TransactionID: "tx", AccountID: "acc1", Amount: 100, Currency: "USD", Description: "a"},
		{ID: "2", TransactionID: "tx", AccountID: "acc2", Amount: -100, Currency: "USD", Description: "b"},
	}
	if err := store.InsertEntries(ctx, execer, entries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 inserts, got %d", calls)
	}
}

func TestLedgerStoreSumByAccount(t *testing.T) {
	ctx := context.Background()
	store := NewLedgerStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "FROM ledger_entries") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 1 || args[0] != "acc1" {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*int64) = 1000
			return nil
		},
	})
	sum, err := store.SumByAccount(ctx, "acc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum != 1000 {
		t.Fatalf("unexpected sum: %d", sum)
	}
}
