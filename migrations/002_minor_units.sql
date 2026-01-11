-- +migrate Up
ALTER TABLE accounts
    ALTER COLUMN balance TYPE BIGINT USING ROUND(balance * 100);

ALTER TABLE transactions
    ALTER COLUMN amount TYPE BIGINT USING ROUND(amount * 100);

ALTER TABLE ledger_entries
    ALTER COLUMN amount TYPE BIGINT USING ROUND(amount * 100);

CREATE TABLE IF NOT EXISTS exchange_quotes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    from_account_id TEXT NOT NULL REFERENCES accounts(id),
    to_account_id TEXT NOT NULL REFERENCES accounts(id),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    converted_minor BIGINT NOT NULL CHECK (converted_minor > 0),
    rate NUMERIC(20, 6) NOT NULL CHECK (rate > 0),
    base_currency CHAR(3) NOT NULL CHECK (base_currency IN ('USD', 'EUR')),
    quote_currency CHAR(3) NOT NULL CHECK (quote_currency IN ('USD', 'EUR')),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS exchange_quotes_user_idx
    ON exchange_quotes (user_id, expires_at DESC);

INSERT INTO exchange_rates (id, base_currency, quote_currency, rate, is_active, created_by)
SELECT gen_random_uuid()::text, 'USD', 'EUR', 0.92, TRUE, NULL
WHERE NOT EXISTS (
    SELECT 1
    FROM exchange_rates
    WHERE base_currency = 'USD' AND quote_currency = 'EUR' AND is_active = TRUE
);

-- +migrate Down
DROP TABLE IF EXISTS exchange_quotes;

ALTER TABLE ledger_entries
    ALTER COLUMN amount TYPE NUMERIC(20, 6) USING (amount / 100.0);

ALTER TABLE transactions
    ALTER COLUMN amount TYPE NUMERIC(20, 6) USING (amount / 100.0);

ALTER TABLE accounts
    ALTER COLUMN balance TYPE NUMERIC(20, 6) USING (balance / 100.0);
