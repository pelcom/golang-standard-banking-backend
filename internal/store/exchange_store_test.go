package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestExchangeStoreGetActive(t *testing.T) {
	ctx := context.Background()
	store := NewExchangeStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "FROM exchange_rates") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 2 || args[0] != "USD" || args[1] != "EUR" {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*exchangeRateRow) = exchangeRateRow{ID: "rate-1"}
			return nil
		},
	})
	row, err := store.GetActive(ctx, "USD", "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row["id"] != "rate-1" {
		t.Fatalf("unexpected row: %#v", row)
	}
}

func TestExchangeStoreSetRate(t *testing.T) {
	ctx := context.Background()
	calls := 0
	tx := stubTx{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "INSERT INTO exchange_rates") {
				t.Fatalf("unexpected query: %s", query)
			}
			*dest.(*string) = "rate-1"
			return nil
		},
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "UPDATE exchange_rates") {
				t.Fatalf("unexpected query: %s", query)
			}
			calls++
			return stubResult{rows: 1}, nil
		},
	}
	store := NewExchangeStore(stubDB{})
	id, err := store.SetRate(ctx, tx, "USD", "EUR", "1.23", "actor-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "rate-1" {
		t.Fatalf("unexpected id: %s", id)
	}
	if calls != 1 {
		t.Fatalf("expected 1 update, got %d", calls)
	}
}
