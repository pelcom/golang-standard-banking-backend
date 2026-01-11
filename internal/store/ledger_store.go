package store

import "context"

type LedgerStore struct {
	db DB
}

func NewLedgerStore(db DB) *LedgerStore {
	return &LedgerStore{db: db}
}

func (s *LedgerStore) InsertEntries(ctx context.Context, tx Execer, entries []LedgerEntryInput) error {
	query := `
		INSERT INTO ledger_entries (id, transaction_id, account_id, amount, currency, description)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	for _, entry := range entries {
		if _, err := tx.ExecContext(ctx, query, entry.ID, entry.TransactionID, entry.AccountID, entry.Amount, entry.Currency, entry.Description); err != nil {
			return err
		}
	}
	return nil
}

func (s *LedgerStore) SumByAccount(ctx context.Context, accountID string) (int64, error) {
	var sum int64
	err := s.db.GetContext(ctx, &sum, `
		SELECT COALESCE(SUM(amount), 0)
		FROM ledger_entries
		WHERE account_id = $1
	`, accountID)
	return sum, err
}

type LedgerEntryInput struct {
	ID            string
	TransactionID string
	AccountID     string
	Amount        int64
	Currency      string
	Description   string
}
