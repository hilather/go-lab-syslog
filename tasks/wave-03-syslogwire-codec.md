# WIRE-001: First-party RFC 3164 + RFC 5424 codec

Status: not-started
Recommended owner: protocol agent
Dependencies: CFG-001
Exclusive ownership: `internal/syslogwire`, `testdata/packets`

## Goal

Parse and serialize RFC 3164 and RFC 5424 messages without leaking
third-party types. Best-effort mode returns `parseWarning` rather
than failing the store path.

## Design references

- [ ] `docs/02-syslog-semantics.md`
- [ ] ADR 0002 in-tree codec

## Scope

- [ ] `PRI` = facility*8 + severity; facility 0–23, severity 0–7
- [ ] RFC 5424: `<PRI>VERSION SP TIMESTAMP SP HOSTNAME SP APP-NAME SP PROCID SP MSGID SP STRUCTURED-DATA [SP MSG]`
- [ ] RFC 3164: `<PRI>TIMESTAMP SP HOSTNAME SP TAG[PID]: SP MSG` (best-effort tag/pid)
- [ ] Structured data parse into `[]SDElement{id, params}`
- [ ] NILVALUE `-` handling
- [ ] Timestamp: RFC 3339 with optional fraction for 5424; 3164 stamp best-effort in current year
- [ ] UTF-8 BOM on MSG allowed and stripped
- [ ] Serialize for tests and export
- [ ] Fuzz corpus from `testdata/packets/{rfc3164,rfc5424,malformed}`

## Explicit non-scope

- Framing (TCP-001)
- UDP/TCP listen
- TLS

## Required tests

- [ ] Golden packets for common PRI values, SD with escapes, NILVALUE
- [ ] Malformed still returns a `Parsed` with `Warning` set
- [ ] Never import `github.com/leodido/go-syslog`, `gopkg.in/mcuadros/go-syslog`, `log/syslog`
- [ ] Fuzz-smoke on the parser

## Acceptance criteria

- Interop fixtures from rsyslog/logger/`nc` transcripts parse.
- Library-import AST test is red if a forbidden module appears.
