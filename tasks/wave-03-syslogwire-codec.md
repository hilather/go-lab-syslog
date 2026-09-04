# WIRE-001: First-party RFC 3164 + RFC 5424 codec

Status: done
Recommended owner: protocol agent
Dependencies: CFG-001
Exclusive ownership: `internal/syslogwire`, `testdata/packets`

## Goal

Parse and serialize RFC 3164 and RFC 5424 messages without leaking
third-party types. Best-effort mode returns `parseWarning` rather
than failing the store path.

## Design references

- [x] `docs/02-syslog-semantics.md`
- [x] ADR 0002 in-tree codec

## Scope

- [x] `PRI` = facility*8 + severity; facility 0–23, severity 0–7
- [x] RFC 5424: `<PRI>VERSION SP TIMESTAMP SP HOSTNAME SP APP-NAME SP PROCID SP MSGID SP STRUCTURED-DATA [SP MSG]`
- [x] RFC 3164: `<PRI>TIMESTAMP SP HOSTNAME SP TAG[PID]: SP MSG` (best-effort tag/pid)
- [x] Structured data parse into `[]SDElement{id, params}`
- [x] NILVALUE `-` handling
- [x] Timestamp: RFC 3339 with optional fraction for 5424; 3164 stamp best-effort in current year
- [x] UTF-8 BOM on MSG allowed and stripped
- [x] Serialize for tests and export
- [x] Fuzz corpus from `testdata/packets/{rfc3164,rfc5424,malformed}`

## Explicit non-scope

- Framing (TCP-001)
- UDP/TCP listen
- TLS

## Required tests

- [x] Golden packets for common PRI values, SD with escapes, NILVALUE
- [x] Malformed still returns a `Parsed` with `Warning` set
- [x] Never import `github.com/leodido/go-syslog`, `gopkg.in/mcuadros/go-syslog`, `log/syslog`
- [x] Fuzz-smoke on the parser

## Acceptance criteria

- Interop fixtures from rsyslog/logger/`nc` transcripts parse.
- Library-import AST test is red if a forbidden module appears.
