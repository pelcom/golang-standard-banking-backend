package store

import (
	"context"
	"database/sql"
)

type AdminStore struct {
	db DB
}

func NewAdminStore(db DB) *AdminStore {
	return &AdminStore{db: db}
}

func (s *AdminStore) IsAdmin(ctx context.Context, userID string) (bool, bool, error) {
	var isSuper bool
	err := s.db.GetContext(ctx, &isSuper, `
		SELECT is_super
		FROM admins
		WHERE user_id = $1
	`, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, false, nil
		}
		return false, false, err
	}
	return true, isSuper, nil
}

func (s *AdminStore) HasRole(ctx context.Context, userID, role string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count, `
		SELECT COUNT(1)
		FROM admin_roles
		WHERE admin_user_id = $1 AND role = $2
	`, userID, role)
	return count > 0, err
}

func (s *AdminStore) CreateAdmin(ctx context.Context, tx Execer, userID string, isSuper bool, createdBy *string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO admins (user_id, is_super, created_by)
		VALUES ($1, $2, $3)
	`, userID, isSuper, createdBy)
	return err
}

func (s *AdminStore) GrantRole(ctx context.Context, tx Execer, adminUserID, role string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO admin_roles (admin_user_id, role)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, adminUserID, role)
	return err
}

func (s *AdminStore) HasAnyAdmin(ctx context.Context) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count, `SELECT COUNT(1) FROM admins`)
	return count > 0, err
}
