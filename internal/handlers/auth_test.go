package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"banking/internal/auth"
	"banking/internal/middleware"
	"banking/internal/store"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

func TestRegisterSuccess(t *testing.T) {
	createdUsers := 0
	createdAccounts := 0
	createdAdmins := 0
	ledgerEntries := make([]store.LedgerEntryInput, 0, 4)
	systemAdjusts := 0
	runner, txExecs := newTestTxRunner(t)
	handler := newTestHandler(stubReconcileDB{}, runner, stubUserStore{
		createFn: func(_ context.Context, _ store.Execer, _, _, _, _ string) error {
			createdUsers++
			return nil
		},
		getByEmailFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
		getByUsernameFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
		getByIDFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
	}, stubAccountStore{
		createFn: func(context.Context, store.Execer, string, *string, string, int64, bool) error {
			createdAccounts++
			return nil
		},
		getSystemAccountFn: func(_ context.Context, currency string) (string, error) {
			if currency == "USD" {
				return "sys-usd", nil
			}
			return "sys-eur", nil
		},
		adjustBalanceFn: func(context.Context, store.Execer, string, int64) (int64, error) {
			systemAdjusts++
			return 1, nil
		},
	}, stubLedgerStore{
		insertFn: func(_ context.Context, _ store.Execer, entries []store.LedgerEntryInput) error {
			ledgerEntries = append(ledgerEntries, entries...)
			return nil
		},
	}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) {
			return false, nil
		},
		createAdminFn: func(context.Context, store.Execer, string, bool, *string) error {
			createdAdmins++
			return nil
		},
		isAdminFn: func(context.Context, string) (bool, bool, error) {
			return false, false, nil
		},
		hasRoleFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}, stubAuditStore{
		logFn: func(context.Context, store.Execer, string, string, string, string, string) error {
			return nil
		},
		listFn: func(context.Context, int, int) ([]map[string]any, error) {
			return nil, nil
		},
	}, stubService{})

	body := []byte(`{"username":"alice","email":"alice@example.com","password":"pass1234"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.Register(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	var payload map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload["token"] == "" {
		t.Fatalf("expected token")
	}
	if createdUsers != 1 || createdAccounts != 2 || createdAdmins != 1 {
		t.Fatalf("unexpected create counts: users=%d accounts=%d admins=%d", createdUsers, createdAccounts, createdAdmins)
	}
	if len(ledgerEntries) != 4 {
		t.Fatalf("expected 4 ledger entries, got %d", len(ledgerEntries))
	}
	if systemAdjusts != 2 {
		t.Fatalf("expected 2 system balance adjustments, got %d", systemAdjusts)
	}
	expected := map[string]int64{
		"sys-usd:-100000": 1,
		"sys-eur:-50000":  1,
	}
	userCredits := map[string]int64{
		"USD:100000": 1,
		"EUR:50000":  1,
	}
	for _, entry := range ledgerEntries {
		if entry.Amount < 0 {
			key := fmt.Sprintf("%s:%d", entry.AccountID, entry.Amount)
			expected[key]--
		} else {
			key := fmt.Sprintf("%s:%d", entry.Currency, entry.Amount)
			userCredits[key]--
		}
	}
	for key, remaining := range expected {
		if remaining != 0 {
			t.Fatalf("missing system debit entry: %s", key)
		}
	}
	for key, remaining := range userCredits {
		if remaining != 0 {
			t.Fatalf("missing user credit entry: %s", key)
		}
	}
	for _, entry := range ledgerEntries {
		if entry.Amount < 0 && entry.Description != "Opening balance debit" {
			t.Fatalf("unexpected debit description: %s", entry.Description)
		}
		if entry.Amount > 0 && entry.Description != "Opening balance credit" {
			t.Fatalf("unexpected credit description: %s", entry.Description)
		}
	}
	if len(*txExecs) != 2 {
		t.Fatalf("expected 2 transaction inserts, got %d", len(*txExecs))
	}
	for _, exec := range *txExecs {
		if !strings.Contains(exec.query, "INSERT INTO transactions") {
			t.Fatalf("unexpected query: %s", exec.query)
		}
		if len(exec.args) < 7 {
			t.Fatalf("unexpected transaction args: %#v", exec.args)
		}
		metadata, ok := exec.args[6].(string)
		if !ok || !strings.Contains(metadata, "\"opening_balance\":\"true\"") {
			t.Fatalf("missing opening_balance metadata: %#v", exec.args[6])
		}
	}
}

func TestRegisterDuplicateUser(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		createFn: func(context.Context, store.Execer, string, string, string, string) error {
			return &pq.Error{Code: "23505"}
		},
		getByEmailFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
		getByUsernameFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
		getByIDFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) {
			return true, nil
		},
		isAdminFn: func(context.Context, string) (bool, bool, error) {
			return false, false, nil
		},
		hasRoleFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}, stubAuditStore{
		logFn: func(context.Context, store.Execer, string, string, string, string, string) error {
			return nil
		},
		listFn: func(context.Context, int, int) ([]map[string]any, error) {
			return nil, nil
		},
	}, stubService{})

	body := []byte(`{"username":"alice","email":"alice@example.com","password":"pass1234"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.Register(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestLoginSuccess(t *testing.T) {
	passwordHash, err := auth.HashPassword("pass1234")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn: func(context.Context, string) (map[string]any, error) {
			return map[string]any{"id": "user-1", "password_hash": passwordHash}, nil
		},
		getByUsernameFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
		getByIDFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
		createFn: func(context.Context, store.Execer, string, string, string, string) error {
			return nil
		},
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) {
			return true, nil
		},
		isAdminFn: func(context.Context, string) (bool, bool, error) {
			return false, false, nil
		},
		hasRoleFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}, stubAuditStore{
		logFn: func(context.Context, store.Execer, string, string, string, string, string) error {
			return nil
		},
		listFn: func(context.Context, int, int) ([]map[string]any, error) {
			return nil, nil
		},
	}, stubService{})

	body := []byte(`{"email":"alice@example.com","password":"pass1234"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.Login(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByEmailFn: func(context.Context, string) (map[string]any, error) {
			return nil, sql.ErrNoRows
		},
		getByUsernameFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
		getByIDFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
		createFn: func(context.Context, store.Execer, string, string, string, string) error {
			return nil
		},
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) {
			return true, nil
		},
		isAdminFn: func(context.Context, string) (bool, bool, error) {
			return false, false, nil
		},
		hasRoleFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}, stubAuditStore{
		logFn: func(context.Context, store.Execer, string, string, string, string, string) error {
			return nil
		},
		listFn: func(context.Context, int, int) ([]map[string]any, error) {
			return nil, nil
		},
	}, stubService{})

	body := []byte(`{"email":"alice@example.com","password":"pass1234"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.Login(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestMe(t *testing.T) {
	handler := newTestHandler(stubReconcileDB{}, fakeTxRunner{}, stubUserStore{
		getByIDFn: func(context.Context, string) (map[string]any, error) {
			return map[string]any{"id": "user-1", "username": "alice", "email": "a@b.com"}, nil
		},
		getByEmailFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
		getByUsernameFn: func(context.Context, string) (map[string]any, error) {
			return nil, nil
		},
		createFn: func(context.Context, store.Execer, string, string, string, string) error {
			return nil
		},
	}, stubAccountStore{}, stubLedgerStore{}, stubTransactionStore{}, stubExchangeStore{}, stubAdminStore{
		hasAnyAdminFn: func(context.Context) (bool, error) {
			return true, nil
		},
		isAdminFn: func(context.Context, string) (bool, bool, error) {
			return false, false, nil
		},
		hasRoleFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}, stubAuditStore{
		logFn: func(context.Context, store.Execer, string, string, string, string, string) error {
			return nil
		},
		listFn: func(context.Context, int, int) ([]map[string]any, error) {
			return nil, nil
		},
	}, stubService{})

	token, err := auth.GenerateToken("secret", "user-1", time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware.Auth("secret")(http.HandlerFunc(handler.Me)).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

type noopDriver struct{}

func (d noopDriver) Open(name string) (driver.Conn, error) {
	return &noopConn{}, nil
}

type noopConn struct{}

func (c *noopConn) Prepare(query string) (driver.Stmt, error) {
	return &noopStmt{}, nil
}

func (c *noopConn) Close() error {
	return nil
}

func (c *noopConn) Begin() (driver.Tx, error) {
	return &noopTx{}, nil
}

func (c *noopConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return &noopTx{}, nil
}

func (c *noopConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	recordTxExec(query, args)
	return noopResult{}, nil
}

type noopStmt struct{}

func (s *noopStmt) Close() error {
	return nil
}

func (s *noopStmt) NumInput() int {
	return -1
}

func (s *noopStmt) Exec(args []driver.Value) (driver.Result, error) {
	return noopResult{}, nil
}

func (s *noopStmt) Query(args []driver.Value) (driver.Rows, error) {
	return nil, nil
}

type noopTx struct{}

func (t *noopTx) Commit() error {
	return nil
}

func (t *noopTx) Rollback() error {
	return nil
}

type noopResult struct{}

func (r noopResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r noopResult) RowsAffected() (int64, error) {
	return 1, nil
}

var noopDriverCounter uint64

type txExecRecord struct {
	query string
	args  []any
}

var txExecRecords []txExecRecord

func recordTxExec(query string, args []driver.NamedValue) {
	record := txExecRecord{query: query, args: make([]any, 0, len(args))}
	for _, arg := range args {
		record.args = append(record.args, arg.Value)
	}
	txExecRecords = append(txExecRecords, record)
}

func newTestTxRunner(t *testing.T) (fakeTxRunner, *[]txExecRecord) {
	t.Helper()
	txExecRecords = nil
	name := fmt.Sprintf("noop-%d", atomic.AddUint64(&noopDriverCounter, 1))
	sql.Register(name, noopDriver{})
	dbConn, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("failed to open noop db: %v", err)
	}
	xdb := sqlx.NewDb(dbConn, name)
	return fakeTxRunner{
		withTxFn: func(ctx context.Context, fn func(*sqlx.Tx) error) error {
			tx, err := xdb.BeginTxx(ctx, nil)
			if err != nil {
				return err
			}
			if err := fn(tx); err != nil {
				_ = tx.Rollback()
				return err
			}
			return tx.Commit()
		},
	}, &txExecRecords
}
