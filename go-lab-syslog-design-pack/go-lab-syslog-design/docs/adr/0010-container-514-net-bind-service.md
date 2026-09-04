# ADR 0010 — Container :514 and NET_BIND_SERVICE

- Status: Accepted
- Date: 2026-09-04

## Context

LabNTP container listens `:123` and the integrator compose adds
`CAP_NET_BIND_SERVICE` when publishing privileged dests. Product
tests should not require that cap.

## Decision

- Container listen address in product YAML is `:514`.
- Integrator compose adds `cap_add: [NET_BIND_SERVICE]` **only**
  when the profile publishes host 514. Residual 10514→514 still
  needs the cap inside the container because the process binds
  `:514`. Correction: mapping host 10514 to container 514 still
  requires the container process to bind 514, so the integrator
  compose **does** add `NET_BIND_SERVICE` whenever the container
  address is `:514`.
- This repo's `make test-container` and `examples/compose.smoke.yaml`
  bind `:1514` and `cap_drop: ALL` with no cap_add.
- Image user remains `65532:65532`. The cap is a bind exception,
  not a root shell.

## Consequences

Agents must not change the image user to 0 to bind 514.
