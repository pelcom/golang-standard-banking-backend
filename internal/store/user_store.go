package store

import "context"

type UserStore struct {
	db DB
}

func NewUserStore(db DB) *UserStore {
	return &UserStore{db: db}
}

type userRow struct {
	ID           string `db:"id"`
	Username     string `db:"username"`
	Email        string `db:"email"`
	PasswordHash string `db:"password_hash"`
	CreatedAt    any    `db:"created_at"`
}

func (s *UserStore) Create(ctx context.Context, tx Execer, id, username, email, passwordHash string) error {
	query := `
		INSERT INTO users (id, username, email, password_hash)
		VALUES ($1, $2, $3, $4)
	`
	_, err := tx.ExecContext(ctx, query, id, username, email, passwordHash)
	return err
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (map[string]any, error) {
	var row userRow
	err := s.db.GetContext(ctx, &row, `SELECT id, username, email, password_hash, created_at FROM users WHERE email = $1`, email)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":            row.ID,
		"username":      row.Username,
		"email":         row.Email,
		"password_hash": row.PasswordHash,
		"created_at":    row.CreatedAt,
	}, nil
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (map[string]any, error) {
	var row userRow
	err := s.db.GetContext(ctx, &row, `SELECT id, username, email, created_at FROM users WHERE username = $1`, username)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":         row.ID,
		"username":   row.Username,
		"email":      row.Email,
		"created_at": row.CreatedAt,
	}, nil
}

func (s *UserStore) GetByID(ctx context.Context, userID string) (map[string]any, error) {
	var row userRow
	err := s.db.GetContext(ctx, &row, `SELECT id, username, email, created_at FROM users WHERE id = $1`, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":         row.ID,
		"username":   row.Username,
		"email":      row.Email,
		"created_at": row.CreatedAt,
	}, nil
}
