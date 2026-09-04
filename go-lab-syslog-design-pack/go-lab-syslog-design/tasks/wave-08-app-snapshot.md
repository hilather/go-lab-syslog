# STA-001 / APP-001: Snapshot, plan, apply, reset

Status: not-started
Recommended owner: control-plane agent
Dependencies: CFG-001, STORE-001
Exclusive ownership: `internal/app`, `internal/snapshot`, `internal/compiler`, `internal/capabilities` seed

## Goal

One `app.Service` owns mutations. Reset rereads bootstrap and wipes
the store. The process never writes the bootstrap file.

## Design references

- [ ] `docs/04-state-and-configuration.md`
- [ ] `docs/05-control-plane-and-parity.md`
- [ ] LabNTP live vs reset-only split

## Scope

- [ ] Compile YAML → immutable `Snapshot` behind `atomic.Pointer`
- [ ] `Validate`, `Plan`, `Apply`, `Export`, `Reset`
- [ ] Optimistic concurrency: `expectedRevision`
- [ ] Idempotency-Key on apply
- [ ] Live vs reset-only:
      - reset-only: listeners, auth, tls (always false), token files
      - live: filters, admission numeric caps, store caps, behavior.mode,
        management HTTP limits
- [ ] Reset: re-read bootstrap, recompile, Swap snapshot, Wipe store,
      bump generation. Rebind listeners only if effective address
      changed (bind new, then drain old)
- [ ] Flags override YAML on serve and reset

## Explicit non-scope

- HTTP handlers
- MCP

## Required tests

- [ ] Apply with stale revision fails
- [ ] Reset wipes store and restores bootstrap filters
- [ ] Service does not open the bootstrap file for write
- [ ] Idempotent apply

## Acceptance criteria

- Domain operations exist with no `net/http` import in `internal/app`.
