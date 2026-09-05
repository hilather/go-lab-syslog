# OBS-001: Observability

Status: done
Recommended owner: observability agent
Dependencies: UDP-001, API-001
Exclusive ownership: `internal/observability`, `api/metrics`, `cmd/labsyslog/healthcheck.go`

## Goal

slog JSON. Hand-rolled OpenMetrics. Live/ready contract.

## Design references

- [x] `docs/09-observability.md`
- [x] LabNTP/LabMail metrics style

## Scope

C11/C12 override the listen-port and series-name sketches below.
Scrape is `GET /v1/metrics` on management (`publicPath`); there is no
`metrics.listen` field. Implement every `docs/09` series; extras from
this wave (`wait_timeouts`, `apply`, `http_requests`) are in addition.

- [x] No `github.com/prometheus/*`
- [x] No `metrics.listen` (C11); scrape is `GET /v1/metrics`
- [x] Every `docs/09` series (not `datagrams_total` / `frames_total`)
- [x] `labsyslog_store_messages`, `labsyslog_store_bytes`
- [x] `labsyslog_wait_timeouts_total`
- [x] `labsyslog_apply_total{result}`
- [x] `labsyslog_http_requests_total{code,route}`
- [x] No client-IP labels
- [x] Ready: UDP and/or TCP bound per spec + snapshot loaded +
      management bound or explicitly off (D27)
- [x] `labsyslog healthcheck --url=`

## Explicit non-scope

- Tracing exporters
- Log shipping (we are the sink)
- Auth (SEC-001) and MCP (MCP-001)

## Required tests

- [x] Ready false if UDP bind failed
- [x] Metrics parse as OpenMetrics
- [x] No prometheus import AST
- [x] `publicPath: false` → `/v1/metrics` is 404 even with auth; `true` → unauthenticated scrape

## Acceptance criteria

- Compose healthcheck uses `healthcheck --url=http://127.0.0.1:8088/v1/health/ready`.
