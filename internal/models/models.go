package models

import "time"

type User struct {
	ID           string    `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

type Account struct {
	ID        string    `db:"id" json:"id"`
	UserID    *string   `db:"user_id" json:"user_id,omitempty"`
	Currency  string    `db:"currency" json:"currency"`
	Balance   int64     `db:"balance" json:"balance"`
	IsSystem  bool      `db:"is_system" json:"is_system"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Transaction struct {
	ID             string    `db:"id" json:"id"`
	UserID         string    `db:"user_id" json:"user_id"`
	Type           string    `db:"type" json:"type"`
	Status         string    `db:"status" json:"status"`
	Amount         int64     `db:"amount" json:"amount"`
	Currency       string    `db:"currency" json:"currency"`
	FromAccountID  *string   `db:"from_account_id" json:"from_account_id,omitempty"`
	ToAccountID    *string   `db:"to_account_id" json:"to_account_id,omitempty"`
	ExchangeRateID *string   `db:"exchange_rate_id" json:"exchange_rate_id,omitempty"`
	Metadata       string    `db:"metadata" json:"metadata"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

type LedgerEntry struct {
	ID            string    `db:"id" json:"id"`
	TransactionID string    `db:"transaction_id" json:"transaction_id"`
	AccountID     string    `db:"account_id" json:"account_id"`
	Amount        int64     `db:"amount" json:"amount"`
	Currency      string    `db:"currency" json:"currency"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	Description   string    `db:"description" json:"description"`
}

type ExchangeRate struct {
	ID            string     `db:"id" json:"id"`
	BaseCurrency  string     `db:"base_currency" json:"base_currency"`
	QuoteCurrency string     `db:"quote_currency" json:"quote_currency"`
	Rate          string     `db:"rate" json:"rate"`
	IsActive      bool       `db:"is_active" json:"is_active"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	DeletedAt     *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}
