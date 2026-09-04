# ADR 0009 — Unmatched filter is capture

- Status: Accepted
- Date: 2026-09-04
- Deciders: Architecture

## Context

LabNTP drops unmatched packets because it is a time *source* and
must not leak virtual time to an unfiltered client. LabSyslog is a
*sink*. Dropping unmatched after a required catch-all would either
force every profile to include a dummy catch-all or silently lose
the traffic the appliance exists to capture.

## Decision

- `admission.allowClientCidrs` is the ignore-outside list.
  Empty list denies all (fail-closed).
- Filters are optional. First enabled match wins (list order, not
  longest-prefix — that part is copied from LabNTP ADR 0009).
- No matching filter ⇒ **capture**.
- Operators who want deny-by-default add an explicit `drop-silent`
  filter. There is no required catch-all.

## Consequences

- Agents must not copy NTP unmatched-drop into the syslog data plane.
- Tests lock: allow-list hit + empty filters = stored.
- Allow-list miss = not stored, even if a filter would have matched.

## Alternatives rejected

- Required catch-all + unmatched drop (NTP-identical). Wrong product.
- Longest-prefix CIDR on filters. Inconsistent with the family.
