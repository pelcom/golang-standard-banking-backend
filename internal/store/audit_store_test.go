package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestAuditStoreLog(t *testing.T) {
	ctx := context.Background()
	execer := stubExecer{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			if !strings.Contains(query, "INSERT INTO audit_logs") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 5 || args[0] != "actor-1" || args[1] != "action" {
				t.Fatalf("unexpected args: %#v", args)
			}
			return stubResult{rows: 1}, nil
		},
	}
	store := NewAuditStore(stubDB{})
	if err := store.Log(ctx, execer, "actor-1", "action", "entity", "entity-1", "{}"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditStoreList(t *testing.T) {
	ctx := context.Background()
	store := NewAuditStore(stubDB{
		selectFn: func(_ context.Context, dest any, query string, args ...any) error {
			if !strings.Contains(query, "FROM audit_logs") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 2 || args[0] != 10 || args[1] != 5 {
				t.Fatalf("unexpected args: %#v", args)
			}
			*dest.(*[]auditRow) = []auditRow{{ID: "log-1"}}
			return nil
		},
	})
	rows, err := store.List(ctx, 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0]["id"] != "log-1" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}
