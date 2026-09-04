# DEP-001: CLI, container, examples BOM

Status: not-started
Recommended owner: deploy agent
Dependencies: UDP-001, TCP-001, API-001, OBS-001
Exclusive ownership: `Dockerfile`, `examples/`, `scripts/test-container.sh`

## Goal

Scratch image UID 65532. Smoke compose. Examples the integrator
will copy.

## Design references

- [ ] `docs/11-deployment.md`
- [ ] `docs/13-integration-lab.md`

## Scope

- [ ] Dockerfile scratch, `USER 65532`, CMD serve
      `--config=/etc/labsyslog/config.yaml --management-listen=:8088`
- [ ] `scripts/test-container.sh` binds `:1514` udp+tcp, `cap_drop ALL`,
      no `NET_BIND_SERVICE`
- [ ] `examples/compose.smoke.yaml`
- [ ] `examples/labsyslog.yaml`
- [ ] `examples/labinfo/services-labsyslog.yaml`
- [ ] `examples/mcpjungle/servers/labsyslog.json`
- [ ] CLI: serve, validate, canonicalize, healthcheck, version, mcp-stdio
- [ ] Optional `labsyslog send` is **forbidden** in production packages
      (receive-only). A test-only client lives in `internal/syslogtest`.

## Explicit non-scope

- Integrator vendor.go change (out of repo)
- Native 514 in the appliance smoke compose

## Required tests

- [ ] Container test: send UDP+TCP, GET ready, GET messages
- [ ] BOM files exist and JSON/YAML parse
- [ ] Image has no shell if scratch; healthcheck is the static binary

## Acceptance criteria

- `examples/compose.smoke.yaml` comes up on a clean Docker and
  captures one UDP message.
