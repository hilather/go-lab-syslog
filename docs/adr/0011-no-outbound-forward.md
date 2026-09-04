# ADR 0011 — No outbound forward

- Status: Accepted
- Date: 2026-09-04
- Related: ADR 0002

## Decision

There is no forward, relay, or dual-sink feature in 1.0 or v1.1.
A second LabSyslog cannot be chained from the first. Reserved-key
reject and the Dial AST fence are the enforcement. A
`POST /v1/messages:forward` route does not exist; if proposed later
it is 403 `receive_only` and requires a new ADR.

## Reserved-key list (normalized)

`forward*`, `relay*`, `remote*`, `destination*`, `smarthost*`,
`outgoing*`, `output*`, `targethost*`, `remotehost*`, `omfwd*`,
`rsyslog*`, `syslogng*`.

AGENTS.md and CFG-001 tests must use this same list.
