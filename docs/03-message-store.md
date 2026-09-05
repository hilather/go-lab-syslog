# 03 — Message store

Last reviewed: 2026-09-04

The store is ephemeral, memory-first, and bounded. It is not desired
state. Restart and `state:reset` wipe it.

## Layout

Package `internal/store`.

```
Store
  mu            sync.Mutex
  items         []Message        # oldest at 0
  byID          map[string]int
  generation    uint64           # bumps on insert, delete, clear, wipe
  bytes         int
  waiters       []Waiter
  maxMessages   int
  maxBytes      int
  fullPolicy    evict_oldest | reject
  rawRetain     bool
```

`id` is a ULID (`oklog/ulid/v2`) generated at insert **before** the
mutex is acquired (entropy can block). Sortable by time.
`generation` is monotonic in-process and is returned on every list,
wait, and state payload so agents can detect races.

Go API (`internal/store`). Stores `model.Message`; there is no second
message type.

```
New(Config) *Store
Insert(msg model.Message) (generation uint64, err error)
List(f ListFilter, cursor string, limit int) (ListResult, error)
Get(id string) (model.Message, error)
Delete(id string) error
Clear()
Wipe()
Wait(ctx, f ListFilter, timeout) (WaitResult, error)
Stats() Stats
```

List is newest-first. `cursor` is the last-returned id; a missing id is
`cursor_stale` (REST wraps this as opaque HMAC). `Insert` returns
`store_full` when the candidate is rejected. The store does not close
sockets: UDP callers drop silently and TCP callers discard the frame
and keep the connection (C18).

## Caps

| Field | Default | Meaning |
|---|---|---|
| `maxMessages` | 10000 | hard count including the candidate |
| `maxBytes` | 256MiB | sum of `len(raw)` plus a fixed per-message overhead of 256 bytes |
| `fullPolicy` | `evict_oldest` | see below |
| `maxWait` | 60s | cap on `messages:wait` timeout |
| `rawRetain` | true | keep `raw`; if false, `raw` is discarded after parse and raw GET returns 404 |

`fullPolicy`:

- `evict_oldest` — drop the oldest messages until the candidate fits.
  Each eviction increments `labsyslog_store_evicted_total`. A successful
  insert (including any eviction batch) bumps generation once, not once
  per evicted message.
- `reject` — do not insert; return `store_full`. UDP: silent. TCP: the
  framed message is discarded, connection stays up. Metric
  `labsyslog_store_rejected_total`. The store itself never closes a
  connection.

A single message larger than `maxBytes` is rejected even under
`evict_oldest`. It cannot fit.

## Indexes

In 1.0 the store is a linear ring. List filters scan. That is
intentional: 10k messages, lab scale. Do not add SQLite or a inverted
index without an ADR.

List/wait filter fields (all AND):

- `facility` (keyword or number)
- `severity` (exact) / `severityAtLeast` (numeric ≤, because 0 is emerg)
- `appName`, `hostname`, `msgID`, `procID`
- `messageContains` (substring, case-sensitive)
- `protocol` (`rfc3164` / `rfc5424`)
- `transport` (`udp` / `tcp`)
- `sourceCidr`
- `after` / `before` (RFC3339, compares `receivedAt`)
- `truncated` (bool; always false in 1.0 — oversize is dropped, not stored)
- `parseWarning` (bool: any warning present)

## Wait

`POST /v1/messages:wait` registers a waiter.

1. Scan existing items newest-first for a match. If found, return it
   immediately (`matched: existing`).
2. Else park until a matching insert, timeout, wipe, or context cancel.
3. Timeout default 10s, cap `store.maxWait`.
4. Wipe / reset unblocks waiters with domain error `store_wiped`.
5. Wait does not delete and does not mark anything.

Subscribe-before-flush: inserts that race with waiter registration
must still be visible (LabMail event-stream rule). Implementation:
hold `mu` while checking-then-parking.

## Wipe

`Wipe()` empties items, resets bytes, increments generation, unblocks
waiters with `store_wiped`. Called from reset and from process
shutdown. Does not change the snapshot.

## Delete / clear

- `Delete(id)` removes one message, bumps generation, 404 if missing.
- `Clear()` is Wipe without the waiter error distinction — waiters
  still see `store_wiped`. Audit records the actor.

## Concurrency

UDP reads, TCP session goroutines, and HTTP handlers all call
`Insert` / `List` / `Wait`. The mutex is coarse. Do not shard in 1.0.
`Insert` must not hold the lock while allocating ULIDs from entropy
if entropy can block; generate the ULID before locking.

## What the store is not

- Not a disk log.
- Not journald.
- Not a ring file under `/var`.
- Not shared across processes.
- Not preserved across `serve` restarts.
A tmpfs spill for oversized raw blobs needs an ADR. 1.0 stays in heap.
