package store

import "context"

type AuditStore struct {
	db DB
}

type auditRow struct {
	ID          string  `db:"id"`
	ActorUserID *string `db:"actor_user_id"`
	Action      string  `db:"action"`
	EntityType  string  `db:"entity_type"`
	EntityID    string  `db:"entity_id"`
	Data        string  `db:"data"`
	CreatedAt   any     `db:"created_at"`
}

func NewAuditStore(db DB) *AuditStore {
	return &AuditStore{db: db}
}

func (s *AuditStore) Log(ctx context.Context, tx Execer, actorID, action, entityType, entityID, data string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (id, actor_user_id, action, entity_type, entity_id, data)
		VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5)
	`, actorID, action, entityType, entityID, data)
	return err
}

func (s *AuditStore) List(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	var rows []auditRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT id, actor_user_id, action, entity_type, entity_id, data, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	logs := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		logs = append(logs, map[string]any{
			"id":            row.ID,
			"actor_user_id": derefStringPtr(row.ActorUserID),
			"action":        row.Action,
			"entity_type":   row.EntityType,
			"entity_id":     row.EntityID,
			"data":          row.Data,
			"created_at":    row.CreatedAt,
		})
	}
	return logs, nil
}
