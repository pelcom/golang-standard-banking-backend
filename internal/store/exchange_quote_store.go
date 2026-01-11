package store

import (
	"context"
	"time"
)

type ExchangeQuoteStore struct {
	db DB
}

func NewExchangeQuoteStore(db DB) *ExchangeQuoteStore {
	return &ExchangeQuoteStore{db: db}
}

type ExchangeQuote struct {
	ID             string     `db:"id"`
	UserID         string     `db:"user_id"`
	FromAccountID  string     `db:"from_account_id"`
	ToAccountID    string     `db:"to_account_id"`
	AmountMinor    int64      `db:"amount_minor"`
	ConvertedMinor int64      `db:"converted_minor"`
	Rate           string     `db:"rate"`
	BaseCurrency   string     `db:"base_currency"`
	QuoteCurrency  string     `db:"quote_currency"`
	ExpiresAt      time.Time  `db:"expires_at"`
	ConsumedAt     *time.Time `db:"consumed_at"`
}

type ExchangeQuoteInput struct {
	ID             string
	UserID         string
	FromAccountID  string
	ToAccountID    string
	AmountMinor    int64
	ConvertedMinor int64
	Rate           string
	BaseCurrency   string
	QuoteCurrency  string
	ExpiresAt      time.Time
}

func (s *ExchangeQuoteStore) Create(ctx context.Context, input ExchangeQuoteInput) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO exchange_quotes (id, user_id, from_account_id, to_account_id, amount_minor, converted_minor, rate, base_currency, quote_currency, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, input.ID, input.UserID, input.FromAccountID, input.ToAccountID, input.AmountMinor, input.ConvertedMinor, input.Rate, input.BaseCurrency, input.QuoteCurrency, input.ExpiresAt)
	return err
}

func (s *ExchangeQuoteStore) GetByID(ctx context.Context, quoteID string) (ExchangeQuote, error) {
	var quote ExchangeQuote
	err := s.db.GetContext(ctx, &quote, `
		SELECT id, user_id, from_account_id, to_account_id, amount_minor, converted_minor, rate, base_currency, quote_currency, expires_at, consumed_at
		FROM exchange_quotes
		WHERE id = $1
	`, quoteID)
	return quote, err
}

func (s *ExchangeQuoteStore) Consume(ctx context.Context, tx Execer, quoteID string) (int64, error) {
	result, err := tx.ExecContext(ctx, `
		UPDATE exchange_quotes
		SET consumed_at = NOW()
		WHERE id = $1 AND consumed_at IS NULL AND expires_at > NOW()
	`, quoteID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
