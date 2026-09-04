# MCP-001: Streamable HTTP and parity

Status: not-started
Recommended owner: MCP agent
Dependencies: API-001, SEC-001
Exclusive ownership: `internal/control/mcp`, `api/mcp`, `cmd/labsyslog/mcp-stdio.go`

## Goal

`POST /mcp` protocol `2026-07-28`. Tools `syslog_*`. Resources
`labsyslog://`. Parity with REST.

## Design references

- [ ] `docs/07-mcp-api.md`
- [ ] ADR 0007 pin MCP versions
- [ ] LabMail MCP catalog pattern

## Scope

- [ ] SDK `github.com/modelcontextprotocol/go-sdk v1.7.0`
- [ ] Stateless Streamable HTTP
- [ ] Bearer only (same tokens as REST)
- [ ] Tools from frozen table
- [ ] Resources: `labsyslog://capabilities|status|schema/config|features|state|messages|messages/{id}|audit`
- [ ] `allowLegacyClients` default false; profile sets true for Jungle
- [ ] `labsyslog mcp-stdio --config --token-file`
- [ ] `make test-parity`

## Explicit non-scope

- `subscriptions/listen` beyond URI-only notify if Jungle needs it
  (keep strict like LabNTP)
- Health/metrics/session as tools (REST_ONLY_PROTOCOL)

## Required tests

- [ ] Every PARITY_REQUIRED row has a tool and a REST path
- [ ] Goldens under `testdata/mcp/`
- [ ] Protocol version recorded in buildinfo and `/v1/version`

## Acceptance criteria

- MCPJungle 0.4.6 can register with `allowLegacyClients: true`.
- Adapters do not import each other.
