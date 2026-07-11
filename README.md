# nepapi

A public JSON API for Nepal's public data, starting with **daily foreign
exchange rates** from Nepal Rastra Bank. One clean, documented, rate-limited
API instead of every Nepali developer scraping the same sources again.

## API

Base URL: (deployment pending)

### `GET /v1/forex/latest`

All rates for the most recent published date.

```json
{
  "date": "2026-07-11",
  "rates": [
    {"date": "2026-07-11T00:00:00Z", "iso3": "USD", "name": "U.S. Dollar", "unit": 1, "buy": 152.4, "sell": 153.0}
  ]
}
```

### `GET /v1/forex/rates?from=2026-07-01&to=2026-07-11&currency=USD`

Historical rates for a date range (max 366 days). `currency` is optional —
omit it for all currencies.

### `GET /v1/health`

Liveness check.

Notes:
- Rates are quoted in NPR per `unit` of the currency (NRB quotes INR and JPY
  per 100 / per 10).
- Responses are `application/json`; errors use `{"error": "message"}`.
- Rate limit: 2 requests/second sustained per IP, bursts up to 30.

## Architecture

```
NRB forex API ──hourly poll──▶ ingester ──▶ Postgres ──▶ HTTP API ──▶ you
                (cmd/api)                                (net/http)
```

- **Go standard library HTTP** (Go 1.22+ mux) — no framework.
- **Postgres** via pgx; embedded SQL migrations applied on startup.
- **Ingestion**: the server polls NRB hourly for the last 3 days (covers
  restarts and NRB's same-day revisions). `cmd/backfill` loads history in
  month-sized chunks.
- **Middleware**: structured request logging (slog JSON), per-IP token-bucket
  rate limiting, CORS for browser clients.
- Source of truth is NRB's official API; nepapi stores and re-serves it with
  history, filtering, and uptime NRB doesn't guarantee.

## Development

```sh
make db-up          # local Postgres 16 in Docker (port 5433)
make run            # migrate + poll + serve on :8080
make test           # unit tests (no database needed)
make test-integration  # store tests against local Postgres
make backfill FROM=2024-01-01 TO=2026-07-11
```

## Roadmap

- [ ] Deploy (VPS + Caddy + uptime monitoring, public base URL)
- [ ] Bikram Sambat ↔ AD calendar conversion endpoints
- [ ] Public holidays dataset
- [ ] Gold/silver prices (FENEGOSIDA)
- [ ] API keys for higher rate limits
- [ ] OpenAPI spec + docs site

## License

MIT
