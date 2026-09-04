# ADR 0003 — Ephemeral store and GitOps desired state

- Status: Accepted
- Date: 2026-09-04

## Context

LabMail ADR 0003 and LabNTP snapshot design separate desired state
(YAML in git) from runtime artifacts (inbox, query log). Operators
reset between scenarios. Persistence across restart would leak the
previous scenario into the next.

## Decision

- Desired state is one `labsyslog.dev/v1alpha1` document.
- The message store is not desired state. Restart and `state:reset`
  wipe it and reread bootstrap.
- The process never writes the bootstrap file.
- Revision is SHA-256 of the canonical spec (secret paths, not bytes).
- Live mutations go through `changes:plan` / `changes:apply` with
  `expectedRevision`. Listener binds and auth files are reset-only.

## Consequences

No database, no `/var/log`, no hidden volume without a new ADR.
Tests must prove wipe-on-reset and revision stability.
