# Changelog

All notable changes to nepapi are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[SemVer](https://semver.org/) — the API contract itself is versioned by
URL (`/v1/`), documented in the README's Stability section.

## [0.1.0] - 2026-07-11

First working release: NRB forex rates, stored and re-served.

### Added

- `GET /v1/forex/latest` — all rates for the most recent published date.
- `GET /v1/forex/rates?from&to&currency` — historical range queries
  (max 366 days), rates as exact decimal strings.
- `GET /v1/health` (liveness) and `GET /v1/ready` (database-checking
  readiness).
- Hourly NRB ingestion with retry backoff, plus `cmd/backfill` for
  history; each day's rates upserted in one batched round-trip.
- Per-IP token-bucket rate limiting (2 rps, burst 30); `X-Forwarded-For`
  honored only behind a trusted proxy (`TRUST_PROXY=true`).
- expvar operational metrics on a localhost-only admin port.
- Postgres migrations embedded and applied on startup.
- CI: gofmt, go vet, staticcheck, unit + integration tests against
  Postgres 16.
- Production container image: static binary on `scratch`, non-root.

[0.1.0]: https://github.com/prashantkoirala465/nepapi/releases/tag/v0.1.0
