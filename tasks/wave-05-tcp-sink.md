# TCP-001: TCP sink (RFC 6587)

Status: done
Recommended owner: data-plane agent
Dependencies: UDP-001 (package `internal/syslogserver`)
Exclusive ownership: `internal/syslogframing`, TCP path in `internal/syslogserver`

## Goal

Accept TCP sessions. Frame messages with
`framing: auto|octet-counting|non-transparent`.
Idle timeout. Session and per-IP caps (stub constants until FIL-001).

## Design references

- [x] `docs/02-syslog-semantics.md` framing
- [x] RFC 6587

## Scope

- [x] `octet-counting`: `MSG-LEN SP SYSLOG-MSG` where MSG-LEN is
      non-zero-leading decimal. Cap `maxMessageBytes`.
- [x] `non-transparent`: split on LF; NUL accepted and stripped;
      optional trailing CR before LF stripped (C19). Trailer is not
      part of `raw`.
- [x] `auto`: if the first bytes of the stream are `DIGIT+` then SP,
      use octet-counting for that session; else non-transparent. Decision is
      per-session at first non-empty read (D11). Later disagreement is a
      framing error (close, no partial store).
- [x] Half-close / idle timeout `tcpIdleTimeout` (default 2m)
- [x] Max concurrent sessions / per-IP (constants or admission hook)
- [x] Do not treat a TCP connection as a syslog client we dial
- [x] Testdata under `testdata/framing/`

## Explicit non-scope

- TLS (TLS-001)
- RELP
- Octet-counting pipelining bugs beyond documented limits

## Required tests

- [x] Octet-counting two messages back-to-back
- [x] NL two messages
- [x] `auto` picks octet when `DIGIT SP`, NL when `<`
- [x] Over-length frame: close session, metric, no store of partial
      beyond cap
- [x] Idle timeout
- [x] Import fence still holds

## Acceptance criteria

- RFC 6587 octet-counting from a Python/`nc` client lands two messages.
- `auto` fixtures cannot be flipped by a leading `<` (PRI).
