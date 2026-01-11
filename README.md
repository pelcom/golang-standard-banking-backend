# Banking Backend

## Setup
1. Copy `.env.example` to `.env` and update values.
2. Run migrations:
   - `go run ./cmd/migrate`
3. Start the API:
   - `go run ./cmd/server`

## Key endpoints
- `POST /auth/register`
- `POST /auth/login`
- `GET /auth/me`
- `GET /accounts`
- `GET /accounts/{id}/balance`
- `POST /transactions/transfer`
- `POST /transactions/exchange`
- `GET /transactions`
- Admin:
  - `POST /admin/exchange-rate`
  - `GET /admin/users`
  - `GET /admin/transactions`
  - `POST /admin/promote`
  - `POST /admin/roles/grant`
  - `GET /admin/reconcile`

## WebSockets
- `GET /ws/balances?token=JWT` for live balance updates.

## Docs
- OpenAPI: `docs/openapi.yaml`
- Ledger design: `docs/design.md`
