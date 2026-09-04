# Architecture decision records

| ADR | Title |
|---|---|
| [0001](0001-use-go.md) | Use Go 1.26 |
| [0002](0002-in-tree-syslog-receive-only.md) | In-tree codec, structural receive-only |
| [0003](0003-ephemeral-state-and-gitops.md) | Ephemeral store, GitOps YAML |
| [0004](0004-shared-capability-registry.md) | Shared REST↔MCP registry |
| [0005](0005-lab-static-bearer.md) | Lab static bearer |
| [0006](0006-pin-mcp-protocol-versions.md) | MCP `2026-07-28` |
| [0007](0007-rfc3164-and-5424-first-party.md) | First-party 3164 + 5424 |
| [0008](0008-host-publish-514-residual.md) | Native 514, residual 10514 |
| [0009](0009-unmatched-filter-is-capture.md) | Unmatched filter = capture |
| [0010](0010-container-514-net-bind-service.md) | Container `:514` + NET_BIND_SERVICE |
| [0011](0011-no-outbound-forward.md) | No outbound forward (Accepted) |
| [0012](0012-tls-is-1-1.md) | RFC 5425 TLS is v1.1 |

ADR 0011 is Accepted. Do not invent a second 0011.
