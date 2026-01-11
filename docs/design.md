# Ledger design and consistency

## Double-entry ledger model
- Every financial action produces ledger entries that sum to zero per currency.
- Transfers create two entries: debit from sender, credit to receiver.
- Exchanges create four entries using system accounts for each currency:
  - User debit (from currency)
  - System credit (from currency)
  - System debit (to currency)
  - User credit (to currency)

## Balance consistency
- `accounts.balance` is the performance cache.
- `ledger_entries` is the authoritative audit trail.
- All writes happen in a serializable transaction, updating balances and ledger entries together.
- Reconciliation endpoint recomputes sums from the ledger and compares to cached balances.

## Atomicity and double-spend protection
- Each transaction is executed in a single database transaction with serializable isolation.
- Accounts are locked with `SELECT ... FOR UPDATE`.
- Balance checks prevent negative balances.
- Idempotency key (`client_request_id`) prevents duplicate processing.

## Decimal precision
- `NUMERIC(20,6)` is used for all monetary fields in PostgreSQL.
- API uses decimal-safe parsing with `shopspring/decimal`.
- USD/EUR values are rounded to 2 decimal places in responses.

## Ledger indexing strategy
- `ledger_entries(transaction_id)` for transaction drilldowns.
- `ledger_entries(account_id, created_at DESC)` for account history.
- `ledger_entries(currency, created_at DESC)` for currency audits.

## Balance verification
- `/admin/reconcile` computes `SUM(ledger_entries.amount)` per account and compares to `accounts.balance`.

## Scaling considerations
- PostgreSQL indexing supports ledger and transaction queries at scale.
- Use connection pooling and read replicas for read-heavy endpoints.
- Partition ledger table by time or account for very high throughput.
- Replace WebSocket in-memory hub with Redis pub/sub or Postgres LISTEN/NOTIFY.
