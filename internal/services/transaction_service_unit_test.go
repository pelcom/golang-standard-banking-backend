package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"banking/internal/store"
	"banking/internal/websocket"

	"github.com/jmoiron/sqlx"
)

type fakeTxRunner struct {
	err error
}

func (f fakeTxRunner) WithTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	if f.err != nil {
		return f.err
	}
	return fn(nil)
}

type stubAccountStore struct {
	getByIDFn         func(ctx context.Context, accountID string) (store.Account, error)
	getForUpdateFn    func(ctx context.Context, tx store.Getter, accountID string) (store.Account, error)
	updateBalanceFn   func(ctx context.Context, tx store.Execer, accountID string, balance int64) error
	getSystemAccountFn func(ctx context.Context, currency string) (string, error)
}

func (s stubAccountStore) GetByID(ctx context.Context, accountID string) (store.Account, error) {
	if s.getByIDFn == nil {
		return store.Account{}, nil
	}
	return s.getByIDFn(ctx, accountID)
}

func (s stubAccountStore) GetForUpdate(ctx context.Context, tx store.Getter, accountID string) (store.Account, error) {
	return s.getForUpdateFn(ctx, tx, accountID)
}

func (s stubAccountStore) UpdateBalance(ctx context.Context, tx store.Execer, accountID string, balance int64) error {
	if s.updateBalanceFn == nil {
		return nil
	}
	return s.updateBalanceFn(ctx, tx, accountID, balance)
}

func (s stubAccountStore) GetSystemAccount(ctx context.Context, currency string) (string, error) {
	if s.getSystemAccountFn == nil {
		return "", nil
	}
	return s.getSystemAccountFn(ctx, currency)
}

type stubLedgerStore struct {
	insertFn func(ctx context.Context, tx store.Execer, entries []store.LedgerEntryInput) error
}

func (s stubLedgerStore) InsertEntries(ctx context.Context, tx store.Execer, entries []store.LedgerEntryInput) error {
	if s.insertFn == nil {
		return nil
	}
	return s.insertFn(ctx, tx, entries)
}

type stubTransactionStore struct {
	createFn func(ctx context.Context, tx store.Execer, input store.TransactionInput) error
}

func (s stubTransactionStore) Create(ctx context.Context, tx store.Execer, input store.TransactionInput) error {
	if s.createFn == nil {
		return nil
	}
	return s.createFn(ctx, tx, input)
}

type stubExchangeStore struct {
	getActiveFn func(ctx context.Context, baseCurrency, quoteCurrency string) (map[string]any, error)
}

func (s stubExchangeStore) GetActive(ctx context.Context, baseCurrency, quoteCurrency string) (map[string]any, error) {
	return s.getActiveFn(ctx, baseCurrency, quoteCurrency)
}

type stubQuoteStore struct {
	createFn  func(ctx context.Context, input store.ExchangeQuoteInput) error
	getByIDFn func(ctx context.Context, quoteID string) (store.ExchangeQuote, error)
	consumeFn func(ctx context.Context, tx store.Execer, quoteID string) (int64, error)
}

func (s stubQuoteStore) Create(ctx context.Context, input store.ExchangeQuoteInput) error {
	if s.createFn == nil {
		return nil
	}
	return s.createFn(ctx, input)
}

func (s stubQuoteStore) GetByID(ctx context.Context, quoteID string) (store.ExchangeQuote, error) {
	return s.getByIDFn(ctx, quoteID)
}

func (s stubQuoteStore) Consume(ctx context.Context, tx store.Execer, quoteID string) (int64, error) {
	if s.consumeFn == nil {
		return 1, nil
	}
	return s.consumeFn(ctx, tx, quoteID)
}

type stubAuditStore struct {
	logFn func(ctx context.Context, tx store.Execer, actorID, action, entityType, entityID, data string) error
}

func (s stubAuditStore) Log(ctx context.Context, tx store.Execer, actorID, action, entityType, entityID, data string) error {
	if s.logFn == nil {
		return nil
	}
	return s.logFn(ctx, tx, actorID, action, entityType, entityID, data)
}

type stubHub struct {
	calls []websocket.BalanceUpdate
}

func (s *stubHub) BroadcastBalance(_ string, update websocket.BalanceUpdate) {
	s.calls = append(s.calls, update)
}

func TestTransferInvalidAmount(t *testing.T) {
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getForUpdateFn: func(context.Context, store.Getter, string) (store.Account, error) {
			t.Fatalf("unexpected store call")
			return store.Account{}, nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubQuoteStore{}, stubAuditStore{}, &stubHub{})
	_, err := service.Transfer(context.Background(), TransferRequest{
		UserID: "user-1", FromAccountID: "a1", ToAccountID: "a2", AmountMinor: 0,
	})
	if err != ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount, got %v", err)
	}
}

func TestTransferUnauthorized(t *testing.T) {
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getForUpdateFn: func(_ context.Context, _ store.Getter, accountID string) (store.Account, error) {
			if accountID == "from" {
				return store.Account{UserID: stringPtr("other"), Currency: "USD", Balance: int64(10000)}, nil
			}
			return store.Account{UserID: stringPtr("user-2"), Currency: "USD", Balance: int64(5000)}, nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubQuoteStore{}, stubAuditStore{}, &stubHub{})
	_, err := service.Transfer(context.Background(), TransferRequest{
		UserID: "user-1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 1000,
	})
	if err != ErrUnauthorizedAccount {
		t.Fatalf("expected ErrUnauthorizedAccount, got %v", err)
	}
}

func TestTransferCurrencyMismatch(t *testing.T) {
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getForUpdateFn: func(_ context.Context, _ store.Getter, accountID string) (store.Account, error) {
			if accountID == "from" {
				return store.Account{UserID: stringPtr("user-1"), Currency: "USD", Balance: int64(10000)}, nil
			}
			return store.Account{UserID: stringPtr("user-2"), Currency: "EUR", Balance: int64(5000)}, nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubQuoteStore{}, stubAuditStore{}, &stubHub{})
	_, err := service.Transfer(context.Background(), TransferRequest{
		UserID: "user-1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 1000,
	})
	if err != ErrCurrencyMismatch {
		t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
	}
}

func TestTransferInsufficientFunds(t *testing.T) {
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getForUpdateFn: func(_ context.Context, _ store.Getter, accountID string) (store.Account, error) {
			if accountID == "from" {
				return store.Account{UserID: stringPtr("user-1"), Currency: "USD", Balance: int64(500)}, nil
			}
			return store.Account{UserID: stringPtr("user-2"), Currency: "USD", Balance: int64(5000)}, nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubQuoteStore{}, stubAuditStore{}, &stubHub{})
	_, err := service.Transfer(context.Background(), TransferRequest{
		UserID: "user-1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 1000,
	})
	if err != ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
}

func TestTransferSuccess(t *testing.T) {
	var balances []int64
	var ledgerEntries []store.LedgerEntryInput
	var createdTx store.TransactionInput
	hub := &stubHub{}
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getForUpdateFn: func(_ context.Context, _ store.Getter, accountID string) (store.Account, error) {
			if accountID == "from" {
				return store.Account{UserID: stringPtr("user-1"), Currency: "USD", Balance: int64(10000)}, nil
			}
			return store.Account{UserID: stringPtr("user-2"), Currency: "USD", Balance: int64(5000)}, nil
		},
		updateBalanceFn: func(_ context.Context, _ store.Execer, _ string, balance int64) error {
			balances = append(balances, balance)
			return nil
		},
	}, stubLedgerStore{
		insertFn: func(_ context.Context, _ store.Execer, entries []store.LedgerEntryInput) error {
			ledgerEntries = entries
			return nil
		},
	}, stubTransactionStore{
		createFn: func(_ context.Context, _ store.Execer, input store.TransactionInput) error {
			createdTx = input
			return nil
		},
	}, stubExchangeStore{}, stubQuoteStore{}, stubAuditStore{}, hub)

	id, err := service.Transfer(context.Background(), TransferRequest{
		UserID: "user-1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" || createdTx.Type != "transfer" {
		t.Fatalf("unexpected transaction: %#v", createdTx)
	}
	if len(balances) != 2 || balances[0] != 9000 || balances[1] != 6000 {
		t.Fatalf("unexpected balances: %#v", balances)
	}
	if len(ledgerEntries) != 2 {
		t.Fatalf("unexpected ledger entries: %#v", ledgerEntries)
	}
	if len(hub.calls) != 2 {
		t.Fatalf("expected 2 balance broadcasts, got %d", len(hub.calls))
	}
}

func TestExchangeQuoteSuccess(t *testing.T) {
	created := false
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getByIDFn: func(_ context.Context, accountID string) (store.Account, error) {
			if accountID == "from" {
				return store.Account{UserID: stringPtr("user-1"), Currency: "USD"}, nil
			}
			return store.Account{Currency: "EUR"}, nil
		},
		getForUpdateFn: func(context.Context, store.Getter, string) (store.Account, error) {
			return store.Account{}, nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{
		getActiveFn: func(context.Context, string, string) (map[string]any, error) {
			return map[string]any{"rate": "0.92"}, nil
		},
	}, stubQuoteStore{
		createFn: func(_ context.Context, input store.ExchangeQuoteInput) error {
			created = true
			if input.Rate != "0.920000" {
				t.Fatalf("unexpected rate: %s", input.Rate)
			}
			return nil
		},
	}, stubAuditStore{}, &stubHub{})

	quote, err := service.QuoteExchange(context.Background(), ExchangeQuoteRequest{
		UserID: "user-1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quote.ID == "" || !created {
		t.Fatalf("expected quote to be created")
	}
	if quote.ConvertedMinor != 920 {
		t.Fatalf("unexpected converted minor: %d", quote.ConvertedMinor)
	}
}

func TestExchangeRateMismatch(t *testing.T) {
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getForUpdateFn: func(_ context.Context, _ store.Getter, accountID string) (store.Account, error) {
			if accountID == "from" {
				return store.Account{UserID: stringPtr("user-1"), Currency: "USD", Balance: int64(10000)}, nil
			}
			return store.Account{UserID: stringPtr("user-1"), Currency: "EUR", Balance: int64(1000)}, nil
		},
		getSystemAccountFn: func(_ context.Context, currency string) (string, error) {
			if currency == "USD" {
				return "sys-usd", nil
			}
			return "sys-eur", nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{
		getActiveFn: func(context.Context, string, string) (map[string]any, error) {
			return map[string]any{"id": "rate-1", "rate": "0.92"}, nil
		},
	}, stubQuoteStore{}, stubAuditStore{}, &stubHub{})

	badRate := "0.910000"
	_, err := service.Exchange(context.Background(), ExchangeRequest{
		UserID: "user-1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 1000, QuotedRate: &badRate,
	})
	if err != ErrRateMismatch {
		t.Fatalf("expected ErrRateMismatch, got %v", err)
	}
}

func TestExchangeSuccessWithQuote(t *testing.T) {
	hub := &stubHub{}
	consumed := false
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getForUpdateFn: func(_ context.Context, _ store.Getter, accountID string) (store.Account, error) {
			switch accountID {
			case "from":
				return store.Account{UserID: stringPtr("user-1"), Currency: "USD", Balance: int64(10000)}, nil
			case "to":
				return store.Account{UserID: stringPtr("user-1"), Currency: "EUR", Balance: int64(1000)}, nil
			case "sys-usd":
				return store.Account{Currency: "USD", Balance: int64(100000)}, nil
			case "sys-eur":
				return store.Account{Currency: "EUR", Balance: int64(100000)}, nil
			default:
				return store.Account{}, nil
			}
		},
		getSystemAccountFn: func(_ context.Context, currency string) (string, error) {
			if currency == "USD" {
				return "sys-usd", nil
			}
			return "sys-eur", nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{
		getActiveFn: func(context.Context, string, string) (map[string]any, error) {
			return map[string]any{"id": "rate-1", "rate": "0.92"}, nil
		},
	}, stubQuoteStore{
		getByIDFn: func(context.Context, string) (store.ExchangeQuote, error) {
			return store.ExchangeQuote{
				ID:             "quote-1",
				UserID:         "user-1",
				FromAccountID:  "from",
				ToAccountID:    "to",
				AmountMinor:    1000,
				ConvertedMinor: 920,
				Rate:           "0.920000",
				BaseCurrency:   "USD",
				QuoteCurrency:  "EUR",
				ExpiresAt:      time.Now().Add(5 * time.Minute),
			}, nil
		},
		consumeFn: func(context.Context, store.Execer, string) (int64, error) {
			consumed = true
			return 1, nil
		},
	}, stubAuditStore{}, hub)

	quoteID := "quote-1"
	id, err := service.Exchange(context.Background(), ExchangeRequest{
		UserID: "user-1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 1000, QuoteID: &quoteID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" || !consumed {
		t.Fatalf("expected exchange with consumed quote")
	}
	if len(hub.calls) != 2 {
		t.Fatalf("expected 2 hub broadcasts, got %d", len(hub.calls))
	}
}

func TestExchangeRoundingEdge(t *testing.T) {
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getForUpdateFn: func(_ context.Context, _ store.Getter, accountID string) (store.Account, error) {
			switch accountID {
			case "from":
				return store.Account{UserID: stringPtr("user-1"), Currency: "USD", Balance: int64(100)}, nil
			case "to":
				return store.Account{UserID: stringPtr("user-1"), Currency: "EUR", Balance: int64(0)}, nil
			case "sys-usd":
				return store.Account{Currency: "USD", Balance: int64(1000)}, nil
			case "sys-eur":
				return store.Account{Currency: "EUR", Balance: int64(1000)}, nil
			default:
				return store.Account{}, nil
			}
		},
		getSystemAccountFn: func(_ context.Context, currency string) (string, error) {
			if currency == "USD" {
				return "sys-usd", nil
			}
			return "sys-eur", nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{
		getActiveFn: func(context.Context, string, string) (map[string]any, error) { return nil, nil },
	}, stubQuoteStore{}, stubAuditStore{}, &stubHub{})

	rate := "0.920000"
	_, err := service.Exchange(context.Background(), ExchangeRequest{
		UserID: "user-1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 1, QuotedRate: &rate,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransferConcurrent(t *testing.T) {
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getForUpdateFn: func(_ context.Context, _ store.Getter, accountID string) (store.Account, error) {
			return store.Account{UserID: stringPtr("user-1"), Currency: "USD", Balance: int64(10000)}, nil
		},
		updateBalanceFn: func(context.Context, store.Execer, string, int64) error {
			return nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubQuoteStore{}, stubAuditStore{}, &stubHub{})

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Transfer(context.Background(), TransferRequest{
				UserID: "user-1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 100,
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestExchangeQuoteNotFound(t *testing.T) {
	service := NewTransactionService(fakeTxRunner{}, stubAccountStore{
		getForUpdateFn: func(context.Context, store.Getter, string) (store.Account, error) {
			return store.Account{}, nil
		},
	}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubQuoteStore{
		getByIDFn: func(context.Context, string) (store.ExchangeQuote, error) {
			return store.ExchangeQuote{}, errors.New("missing")
		},
	}, stubAuditStore{}, &stubHub{})

	quoteID := "missing"
	_, err := service.Exchange(context.Background(), ExchangeRequest{
		UserID: "user-1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 100, QuoteID: &quoteID,
	})
	if err != ErrQuoteNotFound {
		t.Fatalf("expected ErrQuoteNotFound, got %v", err)
	}
}
