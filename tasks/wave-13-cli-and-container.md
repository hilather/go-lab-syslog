# DEP-001: CLI, container, examples BOM

Status: done
Recommended owner: deploy agent
Dependencies: UDP-001, TCP-001, API-001, OBS-001
Exclusive ownership: `Dockerfile`, `examples/compose.smoke.yaml`,
`:1514` smoke overlay, `scripts/test-container.sh`

SWAP-001 owns `examples/labsyslog.yaml`, `examples/labinfo/`, and
`examples/mcpjungle/`.

## Goal

Scratch image UID 65532. Smoke compose. Examples the integrator
will copy.

## Design references

- [x] `docs/11-deployment.md`
- [x] `docs/13-integration-lab-swap.md` (SWAP-001)

## Scope

- [x] Dockerfile scratch, `USER 65532`, CMD serve
      `--config=/etc/labsyslog/config.yaml --management-listen=:8088`
- [x] `scripts/test-container.sh` binds `:1514` udp+tcp, `cap_drop ALL`,
      no `NET_BIND_SERVICE`
- [x] `examples/compose.smoke.yaml`
- [x] `examples/labsyslog.yaml` (SWAP-001)
- [x] `examples/labinfo/services-labsyslog.yaml` (SWAP-001)
- [x] `examples/mcpjungle/servers/labsyslog.json` (SWAP-001)
- [x] CLI: serve, validate, canonicalize, healthcheck, version;
      mcp-stdio remains MCP-001
- [x] Optional `labsyslog send` is **forbidden** in production packages
      (receive-only). A test-only client lives in `internal/syslogtest`.

## Explicit non-scope

- Integrator vendor.go change (out of repo)
- Native 514 in the appliance smoke compose
- Lab overlay / labinfo / mcpjungle (SWAP-001)

## Required tests

- [x] Container test: send UDP+TCP, GET ready, Bearer wait, reset
- [x] Smoke YAML parses; `:1514` overlay validates
- [x] Image has no shell if scratch; healthcheck is the static binary
- [x] `make security-scan` runs `go vet` and `govulncheck`

## Acceptance criteria

- `examples/compose.smoke.yaml` comes up on a clean Docker and
  captures one UDP message.
