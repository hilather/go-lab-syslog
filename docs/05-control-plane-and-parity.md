# 05 — Control plane and parity

Last reviewed: 2026-09-04

REST and MCP are two adapters over one `internal/app.Service` and one
capability registry. They must not call each other. They must make the
same authorization decision and return the same domain types.

## Implementation order

CFG-001 → STA-001 → API-001 → SEC-001 → MCP-001.

Do not start MCP until REST is contract-tested. Do not start SEC
until the session and bearer hooks exist on REST.

## Capability registry

Package `internal/capabilities`. Source of truth is
`api/capabilities/v1.json` (generated from a Go table in
`internal/capabilities/table.go`). `make generate` writes the JSON
and a golden used by `make test-parity`.

Each row:

```
ID, REST method+path, MCP tool name, MCP resource URI or empty,
Scope, Flags (REST_ONLY_PROTOCOL | PARITY_REQUIRED),
Mutating bool, Idempotent bool
```

Frozen IDs: [AGENTS.md](../AGENTS.md) capability table. Adding a row
is a schema change and needs a task; renaming a row is an ADR.

## Mutation contract

Every mutating capability must support:

- validation of the candidate
- dry-run plan (`changes:plan`)
- optimistic concurrency (`expectedRevision`)
- idempotency key
- actor identity (token id or session id)
- reason string (optional, audited)
- deterministic domain errors
- audit emission
- atomic commit (swap snapshot + optional store side effect together)

Reset is mutating. Wait is not.

## Errors

`internal/domainerr`. REST wraps as `application/problem+json`:

```json
{
  "type": "https://labsyslog.dev/errors/validation_failed",
  "title": "validation failed",
  "status": 400,
  "code": "validation_failed",
  "detail": "unknown field spec.listeners.foo"
}
```

MCP returns the same `code` and `detail` without the HTTP envelope.

Frozen codes (1.0): `validation_failed`, `unknown_field`,
`reserved_key`, `immutable_field`, `revision_mismatch`,
`idempotency_conflict`, `not_found`, `store_wiped`, `store_full`,
`wait_timeout`, `unauthorized`, `forbidden`, `origin_not_allowed`,
`bootstrap_invalid`, `payload_too_large`, `rate_limited`,
`cursor_stale`, `tls_unsupported`, `unparseable` (never on insert
when bestEffort; used when a raw GET cannot decode).

## Auth decision is shared

`internal/auth` answers: anonymous / bearer token / cookie session.
Scopes come from the token role:

| Role | Scopes |
|---|---|
| administrator | `syslog.read` `syslog.write` `syslog.admin` `syslog.audit.read` |
| reader | `syslog.read` |

1.0 has no unauthenticated management mode. Loopback still requires bearer or session cookie.
Non-loopback in that mode is `forbidden`.

MCP is bearer-only. Cookie sessions are REST-only. CSRF header
`X-LabSyslog-CSRF` is required when the caller presents the cookie
and not when the caller presents Authorization.

## Parity tests

`make test-parity` drives every PARITY_REQUIRED row through REST and
MCP against the same in-process server and diffs:

- input domain type
- output domain type
- status/code on success and on each frozen error
- revision / generation side effects

A new REST handler without an MCP twin fails parity.
A new MCP tool without a REST twin fails parity.
REST_ONLY_PROTOCOL rows are listed so the test can ignore them.
