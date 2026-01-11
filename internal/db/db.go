package db

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"time"

	"github.com/lib/pq"
	"github.com/jmoiron/sqlx"
)

type TxRunner interface {
	WithTx(ctx context.Context, fn func(*sqlx.Tx) error) error
}

type SQLXTxRunner struct {
	db *sqlx.DB
}

func NewTxRunner(db *sqlx.DB) SQLXTxRunner {
	return SQLXTxRunner{db: db}
}

func (r SQLXTxRunner) WithTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	return WithTx(ctx, r.db, fn)
}

func Connect(databaseURL string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(30)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

func WithTx(ctx context.Context, db *sqlx.DB, fn func(*sqlx.Tx) error) error {
	const maxAttempts = 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		tx, err := db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			return err
		}
		if err := fn(tx); err != nil {
			_ = tx.Rollback()
			if isRetryablePGError(err) && attempt < maxAttempts {
				sleepWithBackoff(attempt)
				continue
			}
			return err
		}
		if err := tx.Commit(); err != nil {
			if isRetryablePGError(err) && attempt < maxAttempts {
				sleepWithBackoff(attempt)
				continue
			}
			return err
		}
		return nil
	}
	return errors.New("transaction retry limit exceeded")
}

func isRetryablePGError(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return pqErr.Code == "40001" || pqErr.Code == "40P01"
}

func sleepWithBackoff(attempt int) {
	base := 20 * time.Millisecond
	backoff := time.Duration(attempt*attempt) * base
	jitter := time.Duration(rand.Int63n(int64(10 * time.Millisecond)))
	time.Sleep(backoff + jitter)
}
