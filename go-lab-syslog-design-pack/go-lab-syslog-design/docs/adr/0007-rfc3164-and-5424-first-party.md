# ADR 0007 — RFC 3164 and RFC 5424 first-party parse

- Status: Accepted
- Date: 2026-09-04

## Context

A sink must accept both traditional BSD syslog and structured 5424.
SUTs and `logger` emit both. Third-party parsers leak types (ADR 0002).

## Decision

- `internal/syslogwire` implements both codecs.
- Protocol selection: `^<\d{1,3}>1 ` plus `parse.rfc5424` → 5424;
  else 3164 if enabled; else best-effort raw or drop.
- Best-effort stores the raw bytes with `parseWarning` rather than
  dropping (LabMail malformed MIME pattern).
- RFC 5424's historical 2048-byte cap is not enforced. YAML
  `maxMessageBytes` (default 64KiB) is the product cap.
- UDP oversize is drop-and-do-not-store. TCP framing errors close
  the session.
- Serialize exists for tests and export, not for forwarding.

## Consequences

Golden files under `testdata/packets/` are the oracle. New parse
behavior requires a golden.
