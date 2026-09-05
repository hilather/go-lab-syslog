# LabSyslog documentation pack

Normative after FND-001. Do not invent paths, types, validate rules,
or capability IDs. If an invariant must change, write an ADR first.

| Doc | Topic |
|---|---|
| [00-family-evaluation.md](00-family-evaluation.md) | Every lab member; copy vs leave |
| [01-architecture.md](01-architecture.md) | Process, packages, planes |
| [02-syslog-semantics.md](02-syslog-semantics.md) | RFC 3164 / 5424 / 5426 / 6587 |
| [03-message-store.md](03-message-store.md) | Bounded ring, wait, wipe |
| [04-state-and-configuration.md](04-state-and-configuration.md) | YAML, revisions, reset |
| [05-control-plane-and-parity.md](05-control-plane-and-parity.md) | Capability registry |
| [06-rest-api.md](06-rest-api.md) | `/v1` |
| [07-mcp-api.md](07-mcp-api.md) | `syslog_*` / `labsyslog://` |
| [08-security-architecture.md](08-security-architecture.md) | Auth, receive-only fence |
| [09-observability.md](09-observability.md) | slog, OpenMetrics, ready |
| [10-testing-strategy.md](10-testing-strategy.md) | Required locks |
| [11-deployment.md](11-deployment.md) | Image, ports, compose |
| [12-web-ui.md](12-web-ui.md) | Operator SPA |
| [reviews/mira-ui-001.md](reviews/mira-ui-001.md) | Mira UI review placeholder (required before v1.0.0) |
| [13-integration-lab-swap.md](13-integration-lab-swap.md) | mcp-integration-lab BOM |
| [implementation-design.md](implementation-design.md) | Decisions, schema, PR plan |
| [known-limitations.md](known-limitations.md) | 1.0 residual limits |
| [adr/](adr/) | Frozen decisions |

Waves and the program board live under [`../tasks/`](../tasks/).
