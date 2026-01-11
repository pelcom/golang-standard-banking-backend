package store

import "context"

type ExchangeStore struct {
	db DB
}

type exchangeRateRow struct {
	ID            string  `db:"id"`
	BaseCurrency  string  `db:"base_currency"`
	QuoteCurrency string  `db:"quote_currency"`
	Rate          string  `db:"rate"`
	CreatedAt     any     `db:"created_at"`
}

func NewExchangeStore(db DB) *ExchangeStore {
	return &ExchangeStore{db: db}
}

func (s *ExchangeStore) GetActive(ctx context.Context, baseCurrency, quoteCurrency string) (map[string]any, error) {
	var row exchangeRateRow
	err := s.db.GetContext(ctx, &row, `
		SELECT id, base_currency, quote_currency, rate, created_at
		FROM exchange_rates
		WHERE base_currency = $1 AND quote_currency = $2 AND is_active = TRUE
	`, baseCurrency, quoteCurrency)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":             row.ID,
		"base_currency":  row.BaseCurrency,
		"quote_currency": row.QuoteCurrency,
		"rate":           row.Rate,
		"created_at":     row.CreatedAt,
	}, nil
}

func (s *ExchangeStore) SetRate(ctx context.Context, tx Tx, baseCurrency, quoteCurrency, rate string, actorID string) (string, error) {
	var id string
	err := tx.GetContext(ctx, &id, `
		INSERT INTO exchange_rates (id, base_currency, quote_currency, rate, is_active, created_by)
		VALUES (gen_random_uuid()::text, $1, $2, $3, TRUE, $4)
		RETURNING id
	`, baseCurrency, quoteCurrency, rate, actorID)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE exchange_rates
		SET is_active = FALSE, deleted_at = NOW()
		WHERE base_currency = $1 AND quote_currency = $2 AND id <> $3 AND is_active = TRUE
	`, baseCurrency, quoteCurrency, id)
	if err != nil {
		return "", err
	}
	return id, nil
}
