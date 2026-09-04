# ADR 0006 — Pin MCP protocol versions

- Status: Accepted
- Date: 2026-09-04

## Context

The family pins MCP protocol `2026-07-28`. MCPJungle 0.4.6 needs
`allowLegacyClients: true` to register. Product default should stay
strict; the lab overlay opens the hatch.

## Decision

- Serve Streamable HTTP `POST /mcp` at protocol `2026-07-28`.
- Official SDK `github.com/modelcontextprotocol/go-sdk v1.7.0`.
- `spec.management.mcp.allowLegacyClients` default false.
- Lab overlay and `examples/labsyslog.yaml` set it true.
- `labsyslog mcp-stdio` is a developer adapter, not the lab path.
- Do not patch the appliance for Jungle. The profile sets the flag.

## Consequences

Bumping the SDK or protocol is a task plus a docs sweep. Parity
tests lock the protocol string.
