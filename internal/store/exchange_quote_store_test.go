package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestExchangeQuoteStoreCreate(t *testing.T) {
	ctx := context.Background()
	created := false
	store := NewExchangeQuoteStore(stubDB{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "INSERT INTO exchange_quotes") {
				t.Fatalf("unexpected query: %s", query)
			}
			created = true
			return stubResult{rows: 1}, nil
		},
	})
	input := ExchangeQuoteInput{
		ID:             "quote-1",
		UserID:         "user-1",
		FromAccountID:  "acc-1",
		ToAccountID:    "acc-2",
		AmountMinor:    1000,
		ConvertedMinor: 920,
		Rate:           "0.920000",
		BaseCurrency:   "USD",
		QuoteCurrency:  "EUR",
		ExpiresAt:      time.Now().UTC(),
	}
	if err := store.Create(ctx, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Fatalf("expected insert")
	}
}

func TestExchangeQuoteStoreGetByID(t *testing.T) {
	ctx := context.Background()
	store := NewExchangeQuoteStore(stubDB{
		getFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "FROM exchange_quotes") {
				t.Fatalf("unexpected query: %s", query)
			}
			*dest.(*ExchangeQuote) = ExchangeQuote{
				ID:             "quote-1",
				UserID:         "user-1",
				AmountMinor:    1000,
				ConvertedMinor: 920,
				Rate:           "0.920000",
				ExpiresAt:      time.Now().UTC(),
			}
			return nil
		},
	})
	quote, err := store.GetByID(ctx, "quote-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quote.ID != "quote-1" {
		t.Fatalf("unexpected quote: %#v", quote)
	}
}

func TestExchangeQuoteStoreConsume(t *testing.T) {
	ctx := context.Background()
	store := NewExchangeQuoteStore(stubDB{})
	rows, err := store.Consume(ctx, stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "UPDATE exchange_quotes") {
				t.Fatalf("unexpected query: %s", query)
			}
			return stubResult{rows: 1}, nil
		},
	}, "quote-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 row, got %d", rows)
	}
}
