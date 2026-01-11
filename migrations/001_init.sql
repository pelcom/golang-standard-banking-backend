-- +migrate Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS accounts (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id),
    currency CHAR(3) NOT NULL CHECK (currency IN ('USD', 'EUR')),
    balance NUMERIC(20, 6) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS exchange_rates (
    id TEXT PRIMARY KEY,
    base_currency CHAR(3) NOT NULL CHECK (base_currency IN ('USD')),
    quote_currency CHAR(3) NOT NULL CHECK (quote_currency IN ('EUR')),
    rate NUMERIC(20, 6) NOT NULL CHECK (rate > 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by TEXT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS exchange_rates_active_idx
    ON exchange_rates (base_currency, quote_currency)
    WHERE is_active = TRUE;

CREATE TABLE IF NOT EXISTS transactions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    type TEXT NOT NULL CHECK (type IN ('transfer', 'exchange')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'completed', 'failed')),
    amount NUMERIC(20, 6) NOT NULL CHECK (amount > 0),
    currency CHAR(3) NOT NULL CHECK (currency IN ('USD', 'EUR')),
    from_account_id TEXT REFERENCES accounts(id),
    to_account_id TEXT REFERENCES accounts(id),
    exchange_rate_id TEXT REFERENCES exchange_rates(id),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    client_request_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS transactions_idempotency_idx
    ON transactions (user_id, client_request_id)
    WHERE client_request_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS transactions_user_idx
    ON transactions (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id TEXT PRIMARY KEY,
    transaction_id TEXT NOT NULL REFERENCES transactions(id),
    account_id TEXT NOT NULL REFERENCES accounts(id),
    amount NUMERIC(20, 6) NOT NULL CHECK (amount <> 0),
    currency CHAR(3) NOT NULL CHECK (currency IN ('USD', 'EUR')),
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS ledger_transaction_idx
    ON ledger_entries (transaction_id);

CREATE INDEX IF NOT EXISTS ledger_account_idx
    ON ledger_entries (account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ledger_currency_idx
    ON ledger_entries (currency, created_at DESC);

CREATE TABLE IF NOT EXISTS admins (
    user_id TEXT PRIMARY KEY REFERENCES users(id),
    is_super BOOLEAN NOT NULL DEFAULT FALSE,
    created_by TEXT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_roles (
    admin_user_id TEXT NOT NULL REFERENCES admins(user_id),
    role TEXT NOT NULL,
    PRIMARY KEY (admin_user_id, role)
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    actor_user_id TEXT REFERENCES users(id),
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS audit_logs_actor_idx
    ON audit_logs (actor_user_id, created_at DESC);

INSERT INTO accounts (id, user_id, currency, balance, is_system)
SELECT gen_random_uuid()::text, NULL, 'USD', 1000000, TRUE
WHERE NOT EXISTS (SELECT 1 FROM accounts WHERE is_system = TRUE AND currency = 'USD');

INSERT INTO accounts (id, user_id, currency, balance, is_system)
SELECT gen_random_uuid()::text, NULL, 'EUR', 1000000, TRUE
WHERE NOT EXISTS (SELECT 1 FROM accounts WHERE is_system = TRUE AND currency = 'EUR');

INSERT INTO exchange_rates (id, base_currency, quote_currency, rate, is_active, created_by)
SELECT gen_random_uuid()::text, 'USD', 'EUR', 0.92, TRUE, NULL
WHERE NOT EXISTS (
    SELECT 1
    FROM exchange_rates
    WHERE base_currency = 'USD' AND quote_currency = 'EUR' AND is_active = TRUE
);

-- +migrate Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS admin_roles;
DROP TABLE IF EXISTS admins;
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS users;
