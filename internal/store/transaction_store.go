package store

import (
	"context"
	"fmt"
)

type TransactionStore struct {
	db DB
}

type transactionRow struct {
	ID             string  `db:"id"`
	UserID         string  `db:"user_id"`
	Username       *string `db:"username"`
	FromUsername   *string `db:"from_username"`
	ToUsername     *string `db:"to_username"`
	Type           string  `db:"type"`
	Status         string  `db:"status"`
	Amount         int64   `db:"amount"`
	Currency       string  `db:"currency"`
	FromAccountID  *string `db:"from_account_id"`
	ToAccountID    *string `db:"to_account_id"`
	ExchangeRateID *string `db:"exchange_rate_id"`
	Metadata       string  `db:"metadata"`
	CreatedAt      any     `db:"created_at"`
}

func NewTransactionStore(db DB) *TransactionStore {
	return &TransactionStore{db: db}
}

func (s *TransactionStore) Create(ctx context.Context, tx Execer, input TransactionInput) error {
	query := `
		INSERT INTO transactions (id, user_id, type, status, amount, currency, from_account_id, to_account_id, exchange_rate_id, metadata, client_request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := tx.ExecContext(ctx, query,
		input.ID, input.UserID, input.Type, input.Status, input.Amount, input.Currency,
		input.FromAccountID, input.ToAccountID, input.ExchangeRateID, input.Metadata, input.ClientRequestID,
	)
	return err
}

func (s *TransactionStore) UpdateStatus(ctx context.Context, tx Execer, transactionID, status string) error {
	_, err := tx.ExecContext(ctx, `UPDATE transactions SET status = $1 WHERE id = $2`, status, transactionID)
	return err
}

func (s *TransactionStore) ListByUser(ctx context.Context, userID, txType string, limit, offset int) ([]map[string]any, error) {
	var rows []transactionRow
	query := `
		SELECT DISTINCT t.id, t.user_id, u.username, fu.username AS from_username, tu.username AS to_username,
		       t.type, t.status, t.amount, t.currency, t.from_account_id, t.to_account_id, t.exchange_rate_id,
		       t.metadata, t.created_at
		FROM transactions t
		LEFT JOIN users u ON u.id = t.user_id
		LEFT JOIN accounts fa ON fa.id = t.from_account_id
		LEFT JOIN users fu ON fu.id = fa.user_id
		LEFT JOIN accounts ta ON ta.id = t.to_account_id
		LEFT JOIN users tu ON tu.id = ta.user_id
		WHERE (t.user_id = $1 OR t.to_account_id IN (SELECT id FROM accounts WHERE user_id = $1))
	`
	args := []any{userID}
	param := 2
	if txType != "" {
		query += " AND type = $2"
		args = append(args, txType)
		param = 3
	}
	query += " ORDER BY created_at DESC LIMIT $" + itoa(param) + " OFFSET $" + itoa(param+1)
	args = append(args, limit, offset)
	err := s.db.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, err
	}
	return transactionRowsToMaps(rows), nil
}

func (s *TransactionStore) ListAll(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	var rows []transactionRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT t.id, t.user_id, u.username, fu.username AS from_username, tu.username AS to_username,
		       t.type, t.status, t.amount, t.currency, t.from_account_id, t.to_account_id, t.exchange_rate_id,
		       t.metadata, t.created_at
		FROM transactions t
		LEFT JOIN users u ON u.id = t.user_id
		LEFT JOIN accounts fa ON fa.id = t.from_account_id
		LEFT JOIN users fu ON fu.id = fa.user_id
		LEFT JOIN accounts ta ON ta.id = t.to_account_id
		LEFT JOIN users tu ON tu.id = ta.user_id
		ORDER BY t.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	return transactionRowsToMaps(rows), nil
}

type TransactionInput struct {
	ID              string
	UserID          string
	Type            string
	Status          string
	Amount          int64
	Currency        string
	FromAccountID   *string
	ToAccountID     *string
	ExchangeRateID  *string
	Metadata        string
	ClientRequestID *string
}

func itoa(value int) string {
	return fmt.Sprintf("%d", value)
}

func transactionRowsToMaps(rows []transactionRow) []map[string]any {
	maps := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		maps = append(maps, map[string]any{
			"id":               row.ID,
			"user_id":          row.UserID,
			"username":         derefStringPtr(row.Username),
			"from_username":    derefStringPtr(row.FromUsername),
			"to_username":      derefStringPtr(row.ToUsername),
			"type":             row.Type,
			"status":           row.Status,
			"amount":           row.Amount,
			"currency":         row.Currency,
			"from_account_id":  derefStringPtr(row.FromAccountID),
			"to_account_id":    derefStringPtr(row.ToAccountID),
			"exchange_rate_id": derefStringPtr(row.ExchangeRateID),
			"metadata":         row.Metadata,
			"created_at":       row.CreatedAt,
		})
	}
	return maps
}
