# ADR 0004 — Shared capability registry

- Status: Accepted
- Date: 2026-09-04

## Context

LabMail ADR 0004 and LabDNS both froze REST↔MCP rows in one table so
adapters cannot drift.

## Decision

- `internal/capabilities` is the only list of public operations.
- `api/capabilities/v1.json` is generated from that table.
- REST and MCP adapters look up rows; they do not invent paths or
  tool names.
- PARITY_REQUIRED rows must have both transports. REST_ONLY_PROTOCOL
  is health, session, metrics scrape, SPA static.
- Frozen tool names are `syslog_*` as listed in AGENTS.md.
- Filters are not a parallel CRUD surface in 1.0; they mutate through
  `syslog_change_plan` / `syslog_change_apply`.

## Consequences

Renaming a tool is an ADR. `make test-parity` fails the build when a
row is missing a twin.
