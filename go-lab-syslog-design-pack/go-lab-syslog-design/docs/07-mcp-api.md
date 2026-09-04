# 07 — MCP API

Last reviewed: 2026-09-04

Transport: Streamable HTTP `POST /mcp`. Protocol version
`2026-07-28`. Developer adapter: `labsyslog mcp-stdio --config … --token-file …`.

Lab overlay sets `spec.management.mcp.allowLegacyClients: true` so
MCPJungle 0.4.6 can register. Default in product YAML is false.

## Tools

| Tool | REST twin | Scope |
|---|---|---|
| `syslog_version_get` | `GET /v1/version` | `syslog.read` |
| `syslog_capabilities_get` | `GET /v1/capabilities` | `syslog.read` |
| `syslog_status_get` | `GET /v1/status` | `syslog.read` |
| `syslog_schema_get` | `GET /v1/schema/config` | `syslog.read` |
| `syslog_features_list` | `GET /v1/features` | `syslog.read` |
| `syslog_state_get` | `GET /v1/state` | `syslog.read` |
| `syslog_state_validate` | `POST /v1/state:validate` | `syslog.admin` |
| `syslog_state_export` | `GET /v1/state:export` | `syslog.admin` |
| `syslog_state_reset` | `POST /v1/state:reset` | `syslog.admin` |
| `syslog_change_plan` | `POST /v1/changes:plan` | `syslog.admin` |
| `syslog_change_apply` | `POST /v1/changes:apply` | `syslog.admin` |
| `syslog_messages_list` | `GET /v1/messages` | `syslog.read` |
| `syslog_message_get` | `GET /v1/messages/{id}` | `syslog.read` |
| `syslog_message_raw_get` | `GET /v1/messages/{id}/raw` | `syslog.read` |
| `syslog_message_delete` | `DELETE /v1/messages/{id}` | `syslog.write` |
| `syslog_messages_clear` | `POST /v1/messages:clear` | `syslog.write` |
| `syslog_messages_wait` | `POST /v1/messages:wait` | `syslog.read` |
| `syslog_stats_get` | `GET /v1/stats` | `syslog.read` |
| `syslog_audit_query` | `GET /v1/audit` | `syslog.audit.read` |
| `syslog_audit_get` | `GET /v1/audit/{id}` | `syslog.audit.read` |

Do not ship a second name for any tool. Input schemas live in
`api/mcp/v1.json` and are generated.

`syslog_change_apply` takes `idempotencyKey` as a tool argument
(MCP has no HTTP header).

## Resources

- `labsyslog://capabilities`
- `labsyslog://status`
- `labsyslog://schema/config`
- `labsyslog://features`
- `labsyslog://state`
- `labsyslog://messages`
- `labsyslog://messages/{id}`
- `labsyslog://stats`
- `labsyslog://audit`

Resources are read-only GET twins. Mutations are tools.

## Auth

Bearer only. A missing or short token is MCP error `unauthorized`.
Cookie sessions are not accepted on `/mcp`.

## Agent recipe (smoke)

1. `syslog_status_get` — confirm ready.
2. Send a probe from the SUT (or `logger`).
3. `syslog_messages_wait` with `messageContains: "lab-probe"` and
   `timeout: "10s"`.
4. `syslog_state_reset` between scenarios.

Do not poll `syslog_messages_list` in a tight loop; that is what
wait is for.
