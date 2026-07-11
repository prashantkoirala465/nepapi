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

### `GET /v1/health` and `GET /v1/ready`

`health` is liveness (process up). `ready` is readiness — it pings the
database and returns 503 if it can't answer within 2s.

Notes:
- Rates are quoted in NPR per `unit` of the currency (NRB quotes INR and JPY
  per 100 / per 10). Values are decimal **strings**, exactly as the bank
  published them — no float rounding.
- Responses are `application/json`; errors use `{"error": "message"}`.
- Rate limit: 2 requests/second sustained per IP, bursts up to 30.

## Architecture

```
NRB forex API ──hourly poll──▶ ingester ──▶ Postgres ──▶ HTTP API ──▶ you
                (cmd/api)                                (net/http)
```

- **Go standard library HTTP** (Go 1.22+ mux) — no framework.
- **Postgres** via pgx; embedded SQL migrations applied on startup; each
  day's rates upserted as one batched round-trip.
- **Ingestion** (`internal/ingest`): polls NRB hourly for the last 3 days
  (covers restarts and NRB's same-day revisions) with exponential backoff on
  failures. `cmd/backfill` loads history in month-sized chunks.
- **Middleware**: structured request logging (slog JSON), per-IP token-bucket
  rate limiting, CORS for browser clients. `X-Forwarded-For` is only trusted
  when `TRUST_PROXY=true` (i.e. behind your own reverse proxy).
- **Observability**: expvar metrics (poll success/failure counters, last
  success time, memstats) on a localhost-only admin port (`ADMIN_PORT`,
  default 8081) at `/debug/vars`.
- Source of truth is NRB's official API; nepapi stores and re-serves it with
  history, filtering, and uptime NRB doesn't guarantee.

### Configuration

| Env | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | (required) | Postgres connection string |
| `PORT` | `8080` | public API port |
| `ADMIN_PORT` | `8081` | localhost-only metrics port |
| `TRUST_PROXY` | unset | set `true` behind a reverse proxy to honor `X-Forwarded-For` |

## Development

```sh
make db-up          # local Postgres 16 in Docker (port 5433)
make run            # migrate + poll + serve on :8080
make test           # unit tests (no database needed)
make test-integration  # store tests against local Postgres
make backfill FROM=2024-01-01 TO=2026-07-11
```

Container image (static binary, distroless-style `scratch` base):

```sh
docker build -t nepapi .
docker run -e DATABASE_URL=... -p 8080:8080 nepapi
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
