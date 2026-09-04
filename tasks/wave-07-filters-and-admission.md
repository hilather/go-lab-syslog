# FIL-001: Admission and filters

Status: not-started
Recommended owner: data-plane agent
Dependencies: CFG-001, UDP-001
Exclusive ownership: admission + filter evaluation in `internal/syslogserver` / `internal/compiler`

## Goal

CIDR admission first. Filters first-match classify or drop.
Unmatched filter after allow-list = **capture**.

## Design references

- [ ] ADR 0009 unmatched-capture
- [ ] LabNTP first-match list order (not longest-prefix)
- [ ] `docs/03-message-store.md` filter section

## Scope

- [ ] `allowClientCidrs`: no match → silent ignore, no store, metric
      `labsyslog_datagrams_total{decision="deny_cidr"}`
- [ ] Unmap IPv4-mapped IPv6 before match
- [ ] Filters match: `sourceCidrs`, `facilities[]`, `severities[]`,
      `appNames[]`, `hostnames[]`, `transports[]`
- [ ] Action: `capture` | `drop-silent` | `tag`
- [ ] List order, first enabled wins
- [ ] No required catch-all
- [ ] Operator who wants deny-by-default adds an explicit drop filter
- [ ] Docker userland-proxy source-IP warning documented

## Explicit non-scope

- Parallel filter CRUD REST (1.0 mutates filters only via changes:plan/apply)
- Chaos random drop

## Required tests

- [ ] First-match wins over a later broader CIDR
- [ ] Outside allow-list never stored
- [ ] No filter + allow-list hit = capture
- [ ] Tag action sets `parsed.tags` or record tag field

## Acceptance criteria

- Policy is documented in ADR 0009 so agents do not invent NTP drop.
