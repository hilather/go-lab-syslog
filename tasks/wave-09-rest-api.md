# API-001: REST `/v1` and OpenAPI

Status: done
Recommended owner: control-plane agent
Dependencies: STA-001
Exclusive ownership: `internal/control/rest`, `api/openapi/v1.json`

## Goal

Native REST over the capability table. problem+json errors.
No business logic in handlers.

## Design references

- [x] `docs/06-rest-api.md`
- [x] Frozen capability table in `docs/05-control-plane-and-parity.md`

## Scope

- [x] Routes from the frozen table (health, version, capabilities,
      status, schema, features, state get/validate/export/reset,
      changes plan/apply, messages list/get/raw/delete/clear/wait,
      stats, audit; filters only via changes:plan/apply)
- [x] Tool names match AGENTS.md freeze (`syslog_message_get`, not `syslog_messages_get`)
- [x] `application/problem+json` with REST `type: https://labsyslog.dev/errors/{code}` (catalog URN is internal)
- [x] Body limit from snapshot
- [x] Wait: `POST /v1/messages:wait` with filter + timeout ≤ maxWait
- [x] Must not import `internal/web` in production rest files
- [x] Must not call MCP

## Explicit non-scope

- Auth middleware (SEC-001)
- SPA

## Required tests

- [x] Contract tests per capability
- [x] Wait timeout returns 504 `wait_timeout` (not 200 `matched: false`, not 404)
- [x] Unknown route problem+json

## Acceptance criteria

- OpenAPI generated/checked in CI.
- Handlers only call `app.Service`.
