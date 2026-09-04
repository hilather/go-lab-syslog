# OBS-001: Observability

Status: not-started
Recommended owner: observability agent
Dependencies: UDP-001, API-001
Exclusive ownership: `internal/observability`, `api/metrics`, `cmd/labsyslog/healthcheck.go`

## Goal

slog JSON. Hand-rolled OpenMetrics. Live/ready contract.

## Design references

- [ ] `docs/09-observability.md`
- [ ] LabNTP/LabMail metrics style

## Scope

- [ ] No `github.com/prometheus/*`
- [ ] Metrics listen default `127.0.0.1:9090`; empty disables
- [ ] `labsyslog_datagrams_total{transport,decision}`
- [ ] `labsyslog_frames_total{framing,decision}`
- [ ] `labsyslog_store_messages`, `labsyslog_store_bytes`
- [ ] `labsyslog_wait_timeouts_total`
- [ ] `labsyslog_apply_total{result}`
- [ ] `labsyslog_http_requests_total{code,route}`
- [ ] No client-IP labels
- [ ] Ready: UDP and/or TCP bound per spec + snapshot loaded +
      management bound or explicitly off
- [ ] `labsyslog healthcheck --url=`

## Explicit non-scope

- Tracing exporters
- Log shipping (we are the sink)

## Required tests

- [ ] Ready false if UDP bind failed
- [ ] Metrics parse as OpenMetrics
- [ ] No prometheus import AST

## Acceptance criteria

- Compose healthcheck uses `healthcheck --url=http://127.0.0.1:8088/v1/health/ready`.
