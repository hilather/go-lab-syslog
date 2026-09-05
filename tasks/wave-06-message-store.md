# STORE-001: Bounded message store

Status: done
Recommended owner: store agent
Dependencies: WIRE-001
Exclusive ownership: `internal/store`, `internal/syslogtest` wait helpers

## Goal

Ephemeral inbox with ULID ids, caps, wait, wipe, generation.
Restart/reset wipes. No database.

## Design references

- [x] `docs/03-message-store.md`
- [x] LabMail store caps / wait / epoch

## Scope

- [x] Record fields: id ULID, receivedAt, transport `udp|tcp`,
      remoteAddr, raw, parsed, parseWarning, truncated
- [x] `maxMessages`, `maxBytes`, `fullPolicy` `evict_oldest|reject`
- [x] UDP reject-equivalent is drop (no NACK). TCP reject discards the
      frame and leaves the connection up (C18); the store only returns
      `store_full`.
- [x] `Wait(ctx, filter, timeout)` unblocks on match or `maxWait`
- [x] `Wipe()` increments `storeGeneration`
- [x] Indexes optional: host, app, facility, severity (1.0 is a linear scan)
- [x] `rawRetain` default true; if false, raw is dropped after parse
      (still keep parseWarning)

## Explicit non-scope

- Persistence
- Disk spool
- Full-text search engine
- Wiring `store.Store` as syslogserver Handler (FIL-001)

## Required tests

- [x] Cap eviction order
- [x] Wait wakes on insert and does not leak goroutines
- [x] Wipe empties and bumps generation
- [x] Concurrent insert + wait + wipe race test

## Acceptance criteria

- 10k inserts under cap stay under `maxBytes`.
- Wait + wipe does not deadlock (`test-race`).
