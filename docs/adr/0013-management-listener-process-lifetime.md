# ADR 0013 — Management listener is process-lifetime

- Status: Accepted
- Date: 2026-10-07

## Context

`cmd/labsyslog` calls `http.Server.Serve` once on the management
listener from startup. `internal/app` must not import `net/http`, so
it cannot serve a replacement socket. Reset that binds a new
management address and closes the previous one leaves the management
plane dark: the old socket is closed, and nothing accepts HTTP on the
new one.

UDP and TCP are owned by `syslogserver.Server` and can rebind.

## Decision

The management `net.Listener` is process-lifetime. After `Start`,
reset compares the effective management address (YAML after
`--management-listen`, including `off`) and refuses a change with
`validation_failed`, including turning management on or off. The
refusal returns before any listen. It does not close the HTTP
listener, swap the snapshot, or wipe. UDP and TCP are not closed or
listened again. Moving management requires a process restart. When
that management address is unchanged, reset still swaps and wipes. A
UDP or TCP socket whose effective address is unchanged stays bound.
That plane listens again when its effective address changes or it is
turned on, and it is closed when it is turned off. A stable
`--management-listen`, including `off`, still wins over the YAML
address.

## Consequences

Operators who edit `listeners.management.address` and reset get
`validation_failed`, a still-running old socket, and `drifted: true`
until they restart or revert the file. Data-plane listeners and auth
files stay reset-only (ADR 0003).
