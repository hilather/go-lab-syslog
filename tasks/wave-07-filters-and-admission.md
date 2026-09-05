# FIL-001: Admission and filters

Status: done
Recommended owner: data-plane agent
Dependencies: CFG-001, UDP-001, TCP-001, STORE-001
Exclusive ownership: admission + filter evaluation in `internal/syslogserver`.
Compile-to-snapshot remains STA-001 (`internal/compiler`).

## Goal

CIDR admission first. Filters first-match classify or drop.
Unmatched filter after allow-list = **capture**.

## Design references

- [x] ADR 0009 unmatched-capture
- [x] LabNTP first-match list order (not longest-prefix)
- [x] `docs/03-message-store.md` filter section

## Scope

- [x] `allowClientCidrs`: no match → silent ignore, no store, metric
      `labsyslog_messages_dropped_total{reason="admission_cidr"}` /
      `labsyslog_admission_drop_total`
- [x] Unmap IPv4-mapped IPv6 before match
- [x] Filters match: `sourceCidrs`, `facilities[]`, `severities[]`,
      `severityAtLeast`, `appNames[]`, `hostnames[]`, `transports[]`
- [x] Action: `capture` | `drop-silent` | `tag`
- [x] List order, first enabled wins
- [x] No required catch-all
- [x] Operator who wants deny-by-default adds an explicit drop filter
- [x] Docker userland-proxy source-IP warning documented
- [x] `spec.syslog.behavior.mode` after admission, before parse
- [x] M1 serve loads spec directly and wires `store.Store` as Handler

## Explicit non-scope

- Parallel filter CRUD REST (1.0 mutates filters only via changes:plan/apply)
- Chaos random drop
- `app.Service` / compile-to-snapshot (STA-001)

## Required tests

- [x] First-match wins over a later broader CIDR
- [x] Outside allow-list never stored (UDP silent; TCP connection closed)
- [x] No filter + allow-list hit = capture
- [x] Tag action sets `Message.Tags`
- [x] `behavior.mode=drop-silent` discards after admission; `close` closes TCP
- [x] Serve with store Handler: one UDP 3164 and one TCP 5424 stored

## Acceptance criteria

- Policy is documented in ADR 0009 so agents do not invent NTP drop.
