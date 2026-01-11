package store

import "context"

type AccountStore struct {
	db DB
}

type Account struct {
	ID        string  `db:"id"`
	UserID    *string `db:"user_id"`
	Currency  string  `db:"currency"`
	Balance   int64   `db:"balance"`
	IsSystem  bool    `db:"is_system"`
	CreatedAt any     `db:"created_at"`
}

type AccountBalanceSummary struct {
	ID                string  `db:"id"`
	UserID            *string `db:"user_id"`
	Currency          string  `db:"currency"`
	StoredBalance     int64   `db:"stored_balance"`
	CalculatedBalance int64   `db:"calculated_balance"`
	Difference        int64   `db:"difference"`
	IsSystem          bool    `db:"is_system"`
	CreatedAt         any     `db:"created_at"`
}

type AccountWithUser struct {
	ID        string  `db:"id"`
	Currency  string  `db:"currency"`
	Balance   int64   `db:"balance"`
	IsSystem  bool    `db:"is_system"`
	CreatedAt any     `db:"created_at"`
	Username  *string `db:"username"`
	Email     *string `db:"email"`
}

func NewAccountStore(db DB) *AccountStore {
	return &AccountStore{db: db}
}

func (s *AccountStore) Create(ctx context.Context, tx Execer, id string, userID *string, currency string, balance int64, isSystem bool) error {
	query := `
		INSERT INTO accounts (id, user_id, currency, balance, is_system)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := tx.ExecContext(ctx, query, id, userID, currency, balance, isSystem)
	return err
}

func (s *AccountStore) GetByUser(ctx context.Context, userID string) ([]AccountBalanceSummary, error) {
	var rows []AccountBalanceSummary
	err := s.db.SelectContext(ctx, &rows, `
		SELECT a.id,
		       a.user_id,
		       a.currency,
		       a.balance AS stored_balance,
		       COALESCE(SUM(l.amount), 0) AS calculated_balance,
		       (a.balance - COALESCE(SUM(l.amount), 0)) AS difference,
		       a.is_system,
		       a.created_at
		FROM accounts a
		LEFT JOIN ledger_entries l ON l.account_id = a.id
		WHERE a.user_id = $1
		GROUP BY a.id, a.user_id, a.currency, a.balance, a.is_system, a.created_at
		ORDER BY a.currency
	`, userID)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *AccountStore) GetByUserAndCurrency(ctx context.Context, userID, currency string) (Account, error) {
	var row Account
	err := s.db.GetContext(ctx, &row, `
		SELECT id, user_id, currency, balance, is_system, created_at
		FROM accounts
		WHERE user_id = $1 AND currency = $2
	`, userID, currency)
	if err != nil {
		return Account{}, err
	}
	return row, nil
}

func (s *AccountStore) GetByID(ctx context.Context, accountID string) (Account, error) {
	var row Account
	err := s.db.GetContext(ctx, &row, `
		SELECT id, user_id, currency, balance, is_system, created_at
		FROM accounts
		WHERE id = $1
	`, accountID)
	if err != nil {
		return Account{}, err
	}
	return row, nil
}

func (s *AccountStore) GetForUpdate(ctx context.Context, tx Getter, accountID string) (Account, error) {
	var row Account
	err := tx.GetContext(ctx, &row, `
		SELECT id, user_id, currency, balance, is_system
		FROM accounts
		WHERE id = $1
		FOR UPDATE
	`, accountID)
	if err != nil {
		return Account{}, err
	}
	return row, nil
}

func (s *AccountStore) UpdateBalance(ctx context.Context, tx Execer, accountID string, balance int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE accounts
		SET balance = $1, updated_at = NOW()
		WHERE id = $2
	`, balance, accountID)
	return err
}

func (s *AccountStore) AdjustBalance(ctx context.Context, tx Execer, accountID string, delta int64) (int64, error) {
	res, err := tx.ExecContext(ctx, `
		UPDATE accounts
		SET balance = balance + $1, updated_at = NOW()
		WHERE id = $2
	`, delta, accountID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *AccountStore) GetSystemAccount(ctx context.Context, currency string) (string, error) {
	var id string
	err := s.db.GetContext(ctx, &id, `
		SELECT id
		FROM accounts
		WHERE is_system = TRUE AND currency = $1
	`, currency)
	return id, err
}

func (s *AccountStore) ListAllWithUsers(ctx context.Context) ([]AccountWithUser, error) {
	var rows []AccountWithUser
	err := s.db.SelectContext(ctx, &rows, `
		SELECT a.id, a.currency, a.balance, a.is_system, a.created_at,
		       u.username, u.email
		FROM accounts a
		LEFT JOIN users u ON u.id = a.user_id
		ORDER BY a.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
