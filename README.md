# Banking Backend

Mini banking platform backend built in Go with PostgreSQL, focusing on financial correctness, auditability, and safe concurrent transactions. The API powers user registration/login, USD/EUR accounts, transfers, currency exchange with a fixed rate, and admin reconciliation.

Live deployment: https://golang-standard-banking-backend.onrender.com/

## Highlights
- Double-entry ledger with balanced entries per transaction and per currency.
- Accounts keep cached balances for performance, with reconciliation against the ledger.
- Serializable transactions with retry logic and row-level locks to prevent double spending.
- Monetary values stored as minor units (cents) using BIGINT to avoid floating-point drift.
- Idempotency support via `client_request_id` to prevent duplicate transfers/exchanges.
- WebSocket balance updates for real-time UI refresh.

## Tech stack
- Go 1.22, Chi router, JWT auth
- PostgreSQL with SQL migrations
- WebSocket via Gorilla
- Render deployment

## Features mapped to requirements
### Database design
- `ledger_entries` is the authoritative audit trail. Every transfer/exchange creates balanced entries.
- `accounts.balance` is a cached balance used for fast reads, with reconciliation endpoints.
- `transactions` provide user-facing history and metadata.
- `users`, `accounts`, `ledger_entries`, and `transactions` are implemented per requirements.
- Additional tables: `exchange_rates`, `exchange_quotes`, `admins`, `admin_roles`, `audit_logs`.

### User management
- Registration is implemented (`POST /auth/register`).
- Each new user automatically gets:
  - USD account with $1000.00 initial balance
  - EUR account with EUR 500.00 initial balance
- Opening balances are recorded through the ledger using system accounts.
- First registered user is promoted to super admin automatically.

### Authentication
- JWT bearer tokens on protected routes.
- `/auth/login` and `/auth/me` endpoints are provided.

### Transaction operations
- Transfers between users in the same currency.
- Currency exchange between a user's USD and EUR accounts.
- Fixed exchange rate of 1 USD = 0.92 EUR (inverse rate for EUR -> USD).
- Optional exchange quotes to lock rate for 2 minutes (`/transactions/exchange/quote`).

### Transaction history
- `GET /transactions` with filters (`type=transfer|exchange`) and pagination (`page`, `limit`).

### Business rules enforced
- No negative balances (insufficient funds checks).
- Atomic writes: ledger, balances, and transaction records are updated together.
- Concurrency-safe: serializable transactions + `SELECT ... FOR UPDATE`.
- Precise amounts: stored as minor units and formatted with two decimals in responses.

## API overview
Base URL (local): `http://localhost:8080`

Auth
- `POST /auth/register`
- `POST /auth/login`
- `GET /auth/me`

Accounts
- `GET /accounts` (includes stored balance, ledger-calculated balance, and difference)
- `GET /accounts/{id}/balance`
- `GET /accounts/self-check` (user-level reconciliation against ledger)

Transactions
- `POST /transactions/transfer`
- `POST /transactions/exchange/quote`
- `POST /transactions/exchange`
- `GET /transactions` (filters and pagination)

Users lookup
- `GET /users/username/{username}`
- `GET /users/email/{email}`

Admin (JWT + admin role required)
- `GET /admin/users`
- `GET /admin/transactions`
- `POST /admin/promote` (super admin only)
- `POST /admin/roles/grant` (super admin only)
- `GET /admin/audit`
- `GET /admin/reconcile`

WebSocket
- `GET /ws/balances?token=JWT` for live balance updates.

Docs
- OpenAPI: `docs/openapi.yaml`
- Ledger design: `docs/design.md`

## Data model (summary)
- `users`: account owners.
- `accounts`: one USD + one EUR per user, plus system accounts for exchange/seeded balances.
- `ledger_entries`: immutable double-entry records (balanced per transaction).
- `transactions`: user-facing record of transfers/exchanges with metadata.
- `exchange_rates`: active USD/EUR rate (fixed in code to 0.92).
- `exchange_quotes`: short-lived quotes used to lock rate at execution time.
- `admins`, `admin_roles`, `audit_logs`: admin permissions and audit trail.

## Financial integrity details
- All writes happen inside a serializable transaction with retry on serialization conflicts.
- Accounts are locked in a consistent order to avoid deadlocks.
- Transfers create two ledger entries (debit/credit).
- Exchanges create four ledger entries using system accounts (debit/credit per currency).
- Reconciliation endpoints compute ledger sums and compare to cached balances.

## Precision and money handling
- Database stores amounts in minor units (cents) as BIGINT.
- API accepts amounts as strings with up to 2 decimal places.
- Formatting is consistently returned as `0.00` strings.

## Setup
1. Copy `.env.example` to `.env` and adjust values.
2. Run migrations:
   ```bash
   go run ./cmd/migrate
   ```
3. Start the API:
   ```bash
   go run ./cmd/server
   ```

Environment variables
- `APP_ENV`
- `PORT`
- `DATABASE_URL`
- `JWT_SECRET`
- `TOKEN_TTL_MINUTES`
- `ALLOWED_ORIGINS`

## Running tests
```bash
go test ./...
```

## Docker
```bash
docker build -t banking-backend .
docker run -p 8080:8080 --env-file .env banking-backend
```
Note: run migrations before starting the container or as a one-off job.

## Design choices and trade-offs
- Fixed exchange rate in code to match the requirement; admin rate updates are intentionally disabled.
- Minor-unit storage chosen for precision and audit consistency.
- Admin and audit features added to support reconciliation, visibility, and operational controls.

## Known limitations
- Rate updates are fixed to 0.92 and not configurable via API.
