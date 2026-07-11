# nepapi

[![CI](https://github.com/prashantkoirala465/nepapi/actions/workflows/ci.yml/badge.svg)](https://github.com/prashantkoirala465/nepapi/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A public JSON API for Nepal's public data, starting with **daily foreign
exchange rates** from Nepal Rastra Bank. One documented, rate-limited API
instead of every Nepali developer scraping the same sources again.

## Quickstart

```sh
git clone https://github.com/prashantkoirala465/nepapi && cd nepapi
make db-up && make run            # Postgres in Docker, API on :8080
curl 'localhost:8080/v1/forex/latest'
```

A hosted instance is coming; until then, run your own
([#1](https://github.com/prashantkoirala465/nepapi/issues/1)).

## API

### `GET /v1/forex/latest`

All rates for the most recent published date.

```json
{
  "date": "2026-07-11",
  "rates": [
    {
      "date": "2026-07-11T00:00:00Z",
      "iso3": "USD",
      "name": "U.S. Dollar",
      "unit": 1,
      "buy": "152.23",
      "sell": "152.83"
    }
  ]
}
```

### `GET /v1/forex/rates?from=2026-07-09&to=2026-07-11&currency=USD`

Historical rates for a date range (max 366 days). `currency` (ISO 4217
alpha-3) is optional — omit it for all currencies.

```json
{
  "from": "2026-07-09",
  "to": "2026-07-11",
  "count": 3,
  "rates": [
    {"date": "2026-07-09T00:00:00Z", "iso3": "USD", "name": "U.S. Dollar", "unit": 1, "buy": "152.6", "sell": "153.2"},
    {"date": "2026-07-10T00:00:00Z", "iso3": "USD", "name": "U.S. Dollar", "unit": 1, "buy": "152.33", "sell": "152.93"},
    {"date": "2026-07-11T00:00:00Z", "iso3": "USD", "name": "U.S. Dollar", "unit": 1, "buy": "152.23", "sell": "152.83"}
  ]
}
```

### `GET /v1/health` · `GET /v1/ready`

`health` is liveness (process up, checks nothing else). `ready` pings the
database and returns `503` if it can't answer within 2 seconds.

### Reading rates

- Rates are NPR per `unit` of the currency. **Check `unit`**: NRB quotes
  INR per 100 and JPY per 10.
- `buy`/`sell` are decimal **strings** with the exact digits NRB published —
  parse with a decimal type, not a float.
- NRB publishes once daily shortly after midnight NPT (UTC+5:45) and
  occasionally revises same-day values; nepapi picks revisions up within an
  hour. Dates NRB didn't publish are absent, not interpolated.

### Errors

Errors are `{"error": "<message>"}` with an appropriate status:

| Status | When | Example message |
|---|---|---|
| 400 | bad or missing parameters | `invalid or missing 'from' (want YYYY-MM-DD)` · `'to' is before 'from'` · `range too large (max 366 days)` |
| 404 | no data yet (empty database) | `no rates available yet` |
| 429 | rate limit exceeded; honor `Retry-After` | `rate limit exceeded` |
| 500 | our bug — details are in server logs, not the response | `internal error` |
| 503 | `/v1/ready` with the database unreachable | `database unreachable` |

Rate limit: 2 requests/second sustained per IP, bursts up to 30. If you
need more, open an issue.

### Stability

- Everything under `/v1/` is a contract: fields may be **added**, never
  renamed, removed, or retyped. Breaking changes mean `/v2/` with `/v1/`
  kept alive through a deprecation window announced in releases.
- This is a free public service run on a best-effort basis — no SLA.
  Treat the data as informational; **verify against NRB before using it
  for anything financial**.

## Data source & attribution

All forex data originates from [Nepal Rastra Bank's public
API](https://www.nrb.org.np/forex/). nepapi stores and re-serves it with
history, filtering, and uptime the origin doesn't offer; it is not
affiliated with or endorsed by NRB. When you republish the data,
attribute NRB as the source.

## Architecture

```
NRB forex API ──hourly poll──▶ internal/ingest ──▶ Postgres ──▶ HTTP API ──▶ you
               retry w/ backoff                                 (net/http)
```

- **Go standard library HTTP** (Go 1.22+ mux) — no framework.
- **Postgres** via pgx; embedded SQL migrations applied on startup; each
  day's rates upserted as one batched round-trip.
- **Ingestion** (`internal/ingest`): polls NRB hourly for the last 3 days
  (covers restarts and same-day revisions) with exponential backoff.
  `cmd/backfill` loads history in month-sized chunks.
- **Middleware**: structured request logging (slog JSON), per-IP
  token-bucket rate limiting, CORS. `X-Forwarded-For` is only trusted when
  `TRUST_PROXY=true` (i.e. behind your own reverse proxy).
- **Observability**: expvar metrics (poll success/failure counters, last
  success time, memstats) on a localhost-only admin port at `/debug/vars`.

### Configuration

| Env | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | (required) | Postgres connection string |
| `PORT` | `8080` | public API port |
| `ADMIN_PORT` | `8081` | localhost-only metrics port |
| `TRUST_PROXY` | unset | set `true` behind a reverse proxy to honor `X-Forwarded-For` |

## Development

```sh
make db-up             # local Postgres 16 in Docker (port 5433)
make run               # migrate + poll + serve on :8080
make test              # unit tests (no database needed)
make test-integration  # store tests against local Postgres
make backfill FROM=2024-01-01 TO=2026-07-11
```

Container image (static binary on `scratch`, non-root):

```sh
docker build -t nepapi .
docker run -e DATABASE_URL=... -p 8080:8080 nepapi
```

CI runs gofmt, go vet, staticcheck, and the full test suite against
Postgres 16 on every push and PR.

## Scope

Planned datasets and features are tracked in
[issues](https://github.com/prashantkoirala465/nepapi/issues): hosted
deployment, Bikram Sambat ↔ AD calendar conversion, public holidays,
gold/silver prices, OpenAPI spec.

Non-goals: crypto prices, commercial-bank retail rates (NRB reference
rates only), anything requiring scraping sources without stable terms.

## License

[MIT](LICENSE). The license covers this software, not the underlying NRB
data — see [Data source & attribution](#data-source--attribution).
