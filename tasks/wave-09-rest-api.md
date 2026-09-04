# API-001: REST `/v1` and OpenAPI

Status: not-started
Recommended owner: control-plane agent
Dependencies: STA-001
Exclusive ownership: `internal/control/rest`, `api/openapi/v1.json`

## Goal

Native REST over the capability table. problem+json errors.
No business logic in handlers.

## Design references

- [ ] `docs/06-rest-api.md`
- [ ] Frozen capability table in `docs/05-control-plane-and-parity.md`

## Scope

- [ ] Routes from the frozen table (health, version, capabilities,
      status, schema, features, state get/validate/export/reset,
      changes plan/apply, messages list/get/raw/delete/clear/wait,
      stats, audit; filters only via changes:plan/apply)
- [ ] Tool names match AGENTS.md freeze (`syslog_message_get`, not `syslog_messages_get`)
- [ ] `application/problem+json` with `urn:labsyslog:error:` codes
- [ ] Body limit from snapshot
- [ ] Wait: `POST /v1/messages:wait` with filter + timeout ≤ maxWait
- [ ] Must not import `internal/web` in production rest files
- [ ] Must not call MCP

## Explicit non-scope

- Auth middleware (SEC-001)
- SPA

## Required tests

- [ ] Contract tests per capability
- [ ] Wait timeout returns 200 with `matched: false` (not 404)
- [ ] Unknown route problem+json

## Acceptance criteria

- OpenAPI generated/checked in CI.
- Handlers only call `app.Service`.
