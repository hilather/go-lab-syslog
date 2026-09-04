# 09 — Observability

Last reviewed: 2026-09-04

## Logs

JSON slog to stdout. Level from `spec.observability.logLevel`
(`debug` | `info` | `warn` | `error`). Default `info`.

Do not log raw syslog bodies at info. Debug may, and is off by
default. Management mutations log actor, operation, reason, revision.

## Metrics

Hand-rolled OpenMetrics at `GET /v1/metrics` when
`spec.observability.metrics.publicPath` is true; otherwise 404.
No `github.com/prometheus/*`.

Required series:

| Series | Labels | Notes |
|---|---|---|
| `labsyslog_messages_received_total` | `transport` | after admission |
| `labsyslog_messages_stored_total` | `transport`, `protocol` | protocol=`rfc3164`\|`rfc5424`\|`raw` |
| `labsyslog_messages_dropped_total` | `reason` | `admission_cidr`, `admission_rate`, `filter`, `oversize`, `store_full`, `behavior` |
| `labsyslog_udp_oversize_total` | | |
| `labsyslog_tcp_framing_errors_total` | | |
| `labsyslog_admission_drop_total` | `reason` | |
| `labsyslog_store_messages` | | gauge |
| `labsyslog_store_bytes` | | gauge |
| `labsyslog_store_generation` | | gauge |
| `labsyslog_store_evicted_total` | | |
| `labsyslog_tcp_conns` | | gauge |
| `labsyslog_waiters` | | gauge |

No client-IP labels (cardinality).

## Ready

`GET /v1/health/ready` is 200 iff snapshot compiled, store constructed,
every enabled data-plane listener is bound, and management is bound
or was not requested.

`GET /v1/health/live` is 200 as soon as the process is running.

## Audit

Ring of management mutations only (plan, apply, reset, delete, clear).
Not every syslog line. Queried via `/v1/audit` and `syslog_audit_*`.
