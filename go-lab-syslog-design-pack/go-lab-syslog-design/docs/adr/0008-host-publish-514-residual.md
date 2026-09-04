# ADR 0008 — Native dest 514, residual host 10514

- Status: Accepted
- Date: 2026-09-04

## Context

Family rule 15: protocol data planes use the IANA dest on the host.
Syslog is 514/udp and 514/tcp. rsyslog, syslog-ng, and journald
commonly hold 514. LabNTP solved the same class with host 10123 as
the shipped default and 123 as the operator escape.

## Decision

- IANA dest is 514. That is the design.
- Default profile residual is `LABSYSLOG_SYSLOG_PORT=10514` and
  `LABSYSLOG_SYSLOG_TCP_PORT=10514`. Management is
  `LABSYSLOG_REST_PORT=18514`.
- Container listens `:514`. Integrator compose maps residual→514.
- Operator escape to native 514 is allowed after preflight.
- Preflight occupancy is `/proc/net` UDP bound + TCP LISTEN.
  `EACCES` is not occupied.
- Error copy names the fix: stop the occupant, extra IP for
  `LAB_PUBLIC_HOST`, or keep 10514.
- TLS dest 6514 is v1.1 (ADR 0012).
- No Go `userland-proxy` probe. Document NAT collision (LabNTP ADR
  0014 class): host-publish UDP source IPs may be SNAT'd, so
  per-IP admission is best-effort on that path. Compose-network
  clients are the reliable path.

## Consequences

Do not invent 10514-as-design in docs. Residual is an escape that
the default profile happens to ship, like DNS 10053.
