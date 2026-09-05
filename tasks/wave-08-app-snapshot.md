# STA-001 / APP-001: Snapshot, plan, apply, reset

Status: done
Recommended owner: control-plane agent
Dependencies: CFG-001, STORE-001, FIL-001
Exclusive ownership: `internal/app`, `internal/snapshot`, `internal/compiler` compile-to-snapshot, `internal/capabilities` seed

## Goal

One `app.Service` owns mutations. Reset rereads bootstrap and wipes
the store. The process never writes the bootstrap file.

## Design references

- [x] `docs/04-state-and-configuration.md`
- [x] `docs/05-control-plane-and-parity.md`
- [x] LabNTP live vs reset-only split

## Scope

- [x] Compile YAML → immutable `Snapshot` behind `atomic.Pointer`
- [x] `Validate`, `Plan`, `Apply`, `Export`, `Reset`
- [x] Optimistic concurrency: `expectedRevision`
- [x] Idempotency-Key on apply
- [x] Live vs reset-only (`docs/04` / C30; no extra operations):
      - live: `replaceStoreCaps`, `replaceFilters`, `replaceAdmission`,
        `replaceSyslogParse` (parse booleans + `maxMessageBytes`),
        `replaceBehavior`, `replaceObservability` (`logLevel` only)
      - reset-only: listeners, auth, tls, token files, HTTP limits,
        `allowedOrigins`, `allowLegacyClients`, `syslog.hostname`,
        `udpMaxDatagramBytes`, `tcpIdleTimeout`, `metrics.publicPath`,
        `audit.ring`
- [x] Reset: re-read bootstrap, recompile, Swap snapshot, Wipe store
      and audit. Rebind listeners only if effective address changed
      (bind new, then drain old)
- [x] Flags override YAML on serve and reset

## Explicit non-scope

- HTTP handlers
- MCP

## Required tests

- [x] Apply with stale revision fails
- [x] Reset wipes store and restores bootstrap filters
- [x] Service does not open the bootstrap file for write
- [x] Idempotent apply

## Acceptance criteria

- Domain operations exist with no `net/http` import in `internal/app`.
