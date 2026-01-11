package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/lib/pq"
	"github.com/jmoiron/sqlx"
)

type txState struct {
	commits   int64
	rollbacks int64
}

type trackingDriver struct {
	state *txState
}

func (d *trackingDriver) Open(name string) (driver.Conn, error) {
	return &trackingConn{state: d.state}, nil
}

type trackingConn struct {
	state *txState
}

func (c *trackingConn) Prepare(query string) (driver.Stmt, error) {
	return &trackingStmt{}, nil
}

func (c *trackingConn) Close() error {
	return nil
}

func (c *trackingConn) Begin() (driver.Tx, error) {
	return &trackingTx{state: c.state}, nil
}

func (c *trackingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return &trackingTx{state: c.state}, nil
}

type trackingTx struct {
	state *txState
}

func (t *trackingTx) Commit() error {
	atomic.AddInt64(&t.state.commits, 1)
	return nil
}

func (t *trackingTx) Rollback() error {
	atomic.AddInt64(&t.state.rollbacks, 1)
	return nil
}

type trackingStmt struct{}

func (s *trackingStmt) Close() error {
	return nil
}

func (s *trackingStmt) NumInput() int {
	return -1
}

func (s *trackingStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, nil
}

func (s *trackingStmt) Query(args []driver.Value) (driver.Rows, error) {
	return nil, nil
}

var driverCounter uint64

func registerTrackingDriver(state *txState) string {
	name := fmt.Sprintf("tracking-%d", atomic.AddUint64(&driverCounter, 1))
	sql.Register(name, &trackingDriver{state: state})
	return name
}

type retryState struct {
	commitCalls int64
	failCommits int64
	failCode    string
}

type retryDriver struct {
	state *retryState
}

func (d *retryDriver) Open(name string) (driver.Conn, error) {
	return &retryConn{state: d.state}, nil
}

type retryConn struct {
	state *retryState
}

func (c *retryConn) Prepare(query string) (driver.Stmt, error) {
	return &trackingStmt{}, nil
}

func (c *retryConn) Close() error {
	return nil
}

func (c *retryConn) Begin() (driver.Tx, error) {
	return &retryTx{state: c.state}, nil
}

func (c *retryConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return &retryTx{state: c.state}, nil
}

type retryTx struct {
	state *retryState
}

func (t *retryTx) Commit() error {
	call := atomic.AddInt64(&t.state.commitCalls, 1)
	if call <= t.state.failCommits {
		code := t.state.failCode
		if code == "" {
			code = "40001"
		}
		return &pq.Error{Code: pq.ErrorCode(code)}
	}
	return nil
}

func (t *retryTx) Rollback() error {
	return nil
}

func registerRetryDriver(state *retryState) string {
	name := fmt.Sprintf("retry-%d", atomic.AddUint64(&driverCounter, 1))
	sql.Register(name, &retryDriver{state: state})
	return name
}

func TestWithTxCommits(t *testing.T) {
	state := &txState{}
	driverName := registerTrackingDriver(state)
	sqlDB, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer sqlDB.Close()

	xdb := sqlx.NewDb(sqlDB, driverName)
	if err := WithTx(context.Background(), xdb, func(*sqlx.Tx) error { return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("expected commit=1 rollback=0, got %d/%d", state.commits, state.rollbacks)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	state := &txState{}
	driverName := registerTrackingDriver(state)
	sqlDB, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer sqlDB.Close()

	xdb := sqlx.NewDb(sqlDB, driverName)
	if err := WithTx(context.Background(), xdb, func(*sqlx.Tx) error { return errors.New("boom") }); err == nil {
		t.Fatalf("expected error")
	}
	if state.rollbacks != 1 {
		t.Fatalf("expected rollback=1, got %d", state.rollbacks)
	}
}

func TestWithTxRetriesOnSerializableConflict(t *testing.T) {
	state := &retryState{failCommits: 1}
	driverName := registerRetryDriver(state)
	sqlDB, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer sqlDB.Close()

	xdb := sqlx.NewDb(sqlDB, driverName)
	if err := WithTx(context.Background(), xdb, func(*sqlx.Tx) error { return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.commitCalls != 2 {
		t.Fatalf("expected 2 commits, got %d", state.commitCalls)
	}
}

func TestWithTxRetryCapExceeded(t *testing.T) {
	state := &retryState{failCommits: 10, failCode: "40P01"}
	driverName := registerRetryDriver(state)
	sqlDB, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer sqlDB.Close()

	xdb := sqlx.NewDb(sqlDB, driverName)
	err = WithTx(context.Background(), xdb, func(*sqlx.Tx) error { return nil })
	if err == nil {
		t.Fatalf("expected retry limit error")
	}
	if state.commitCalls != 5 {
		t.Fatalf("expected 5 commits, got %d", state.commitCalls)
	}
}
