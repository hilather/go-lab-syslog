# ADR 0002 — In-tree syslog, structural receive-only

- Status: Accepted
- Date: 2026-09-04
- Deciders: Architecture

## Context

Wrapping rsyslog or syslog-ng would make "never forward" a config
flag. LabMail ADR 0002 rejected that class of design: outbound must
be unrepresentable.

Third-party parsers (`leodido/go-syslog`, `mcuadros/go-syslog`)
leak types and make the import fence harder to prove.

## Decision

- First-party `internal/syslogwire` and `internal/syslogframing`.
- Production data-plane packages Listen/Accept only.
- Reserved YAML keys rejected: `forward*`, `relay*`, `remote*`,
  `destination*`, `smarthost*`, `outgoing*`, `output*`,
  `targethost*`, `omfwd*`, `rsyslog*`, `syslogng*` (normalized).
- No `logger` / `rsyslogd` / `syslog-ng` exec in production.
- Test client isolated in `internal/syslogtest`.

## Consequences

- Wire semantics live in `docs/02-syslog-semantics.md`.
- AST tests in CI prove the fence.
- TLS RFC 5425 is a later listener on the same in-tree server,
  not a sidecar.

## Alternatives rejected

- rsyslog/syslog-ng wrapper images.
- Forward-with-flag for "lab chaining".
- Depending on `leodido/go-syslog` types in `internal/model`.
