package store

import (
	"context"
	"database/sql"
)

type stubDB struct {
	getFn    func(ctx context.Context, dest any, query string, args ...any) error
	selectFn func(ctx context.Context, dest any, query string, args ...any) error
	execFn   func(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s stubDB) GetContext(ctx context.Context, dest any, query string, args ...any) error {
	if s.getFn == nil {
		return nil
	}
	return s.getFn(ctx, dest, query, args...)
}

func (s stubDB) SelectContext(ctx context.Context, dest any, query string, args ...any) error {
	if s.selectFn == nil {
		return nil
	}
	return s.selectFn(ctx, dest, query, args...)
}

func (s stubDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s.execFn == nil {
		return stubResult{}, nil
	}
	return s.execFn(ctx, query, args...)
}

type stubExecer struct {
	execFn func(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s stubExecer) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s.execFn == nil {
		return stubResult{}, nil
	}
	return s.execFn(ctx, query, args...)
}

type stubGetter struct {
	getFn func(ctx context.Context, dest any, query string, args ...any) error
}

func (s stubGetter) GetContext(ctx context.Context, dest any, query string, args ...any) error {
	if s.getFn == nil {
		return nil
	}
	return s.getFn(ctx, dest, query, args...)
}

type stubTx struct {
	execFn func(ctx context.Context, query string, args ...any) (sql.Result, error)
	getFn  func(ctx context.Context, dest any, query string, args ...any) error
}

func (s stubTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s.execFn == nil {
		return stubResult{}, nil
	}
	return s.execFn(ctx, query, args...)
}

func (s stubTx) GetContext(ctx context.Context, dest any, query string, args ...any) error {
	if s.getFn == nil {
		return nil
	}
	return s.getFn(ctx, dest, query, args...)
}

type stubResult struct {
	rows int64
	err  error
}

func (r stubResult) LastInsertId() (int64, error) {
	return 0, r.err
}

func (r stubResult) RowsAffected() (int64, error) {
	return r.rows, r.err
}
