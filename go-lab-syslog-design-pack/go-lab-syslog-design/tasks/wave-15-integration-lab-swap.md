# SWAP-001: Integration-lab BOM

Status: not-started
Recommended owner: integrator-docs agent
Dependencies: MCP-001, SEC-001, DEP-001
Exclusive ownership: `examples/`, `docs/13-integration-lab.md`

## Goal

This repo ships every file the integrator needs to copy. No product
logic is added to `mcp-integration-lab` in this wave.

## Design references

- [ ] `docs/13-integration-lab.md`
- [ ] Evaluation `docs/00-evaluation.md`

## Scope

- [ ] Examples listed in DEP-001 complete and documented
- [ ] Integrator checklist in docs/13 is accurate against current
      mcp-integration-lab layout (`vendor.go`, compose, profile.env,
      labinfo, jungle, secrets.go, Make APP list, rule 15)
- [ ] Smoke recipe using `syslog_messages_wait`
- [ ] Note that the actual integrator PR is a follow-on after the
      first `v*` tag

## Explicit non-scope

- Landing the integrator PR
- labgraph fixture packs

## Required tests

- [ ] Example YAML/JSON parse
- [ ] Docs mention every current labinfo id we must not collide with

## Acceptance criteria

- An integrator agent can land the lab PR from docs/13 alone.
