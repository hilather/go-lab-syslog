# STORE-001: Bounded message store

Status: not-started
Recommended owner: store agent
Dependencies: WIRE-001
Exclusive ownership: `internal/store`, `internal/syslogtest` wait helpers

## Goal

Ephemeral inbox with ULID ids, caps, wait, wipe, generation.
Restart/reset wipes. No database.

## Design references

- [ ] `docs/03-message-store.md`
- [ ] LabMail store caps / wait / epoch

## Scope

- [ ] Record fields: id ULID, receivedAt, transport `udp|tcp`,
      remoteAddr, raw, parsed, parseWarning, truncated
- [ ] `maxMessages`, `maxBytes`, `fullPolicy` `evict_oldest|reject`
- [ ] UDP reject-equivalent is drop (no NACK). TCP reject closes
      after the current frame policy (document).
- [ ] `Wait(ctx, filter, timeout)` unblocks on match or `maxWait`
- [ ] `Wipe()` increments `storeGeneration`
- [ ] Indexes optional: host, app, facility, severity
- [ ] `rawRetain` default true; if false, raw is dropped after parse
      (still keep parseWarning)

## Explicit non-scope

- Persistence
- Disk spool
- Full-text search engine

## Required tests

- [ ] Cap eviction order
- [ ] Wait wakes on insert and does not leak goroutines
- [ ] Wipe empties and bumps generation
- [ ] Concurrent insert + wait + wipe race test

## Acceptance criteria

- 10k inserts under cap stay under `maxBytes`.
- Wait + wipe does not deadlock (`test-race`).
