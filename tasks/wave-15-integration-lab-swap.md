# SWAP-001: Integration-lab BOM

Status: done
Recommended owner: integrator-docs agent
Dependencies: MCP-001, SEC-001, DEP-001
Exclusive ownership: `examples/labsyslog.yaml`, `examples/labinfo/`,
`examples/mcpjungle/`, `docs/13-integration-lab-swap.md`

## Goal

This repo ships every file the integrator needs to copy. No product
logic is added to `mcp-integration-lab` in this wave.

## Design references

- [x] `docs/13-integration-lab-swap.md`
- [x] Evaluation `docs/00-family-evaluation.md`

## Scope

- [x] Examples listed in DEP-001 complete and documented
- [x] Integrator checklist in docs/13 is accurate against current
      mcp-integration-lab layout (`vendor.go`, compose, profile.env,
      labinfo, jungle, secrets.go, Make APP list, rule 15)
- [x] Smoke recipe using `syslog_messages_wait`
- [x] Note that the actual integrator PR is a follow-on after the
      first `v*` tag
- [x] Compose fragment `cap_add: [NET_BIND_SERVICE]` whenever the
      container process binds `:514`, including host 10514→514 (C17)
- [x] Keep `LABSYSLOG_REST_PORT` (C24)

## Explicit non-scope

- Landing the integrator PR
- labgraph fixture packs
- `examples/compose.smoke.yaml` (DEP-001)
- TLS / GA soak

## Required tests

- [x] Example YAML/JSON parse
- [x] Docs mention every current labinfo id we must not collide with
- [x] `docs/13` compose includes `NET_BIND_SERVICE` for `:514` and
      does not claim residual 10514 needs no cap

## Acceptance criteria

- An integrator agent can land the lab PR from docs/13 alone.
