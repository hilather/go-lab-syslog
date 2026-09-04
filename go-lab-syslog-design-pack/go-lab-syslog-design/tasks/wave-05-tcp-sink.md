# TCP-001: TCP sink (RFC 6587)

Status: not-started
Recommended owner: data-plane agent
Dependencies: WIRE-001
Exclusive ownership: `internal/syslogframing`, TCP path in `internal/syslogserver`

## Goal

Accept TCP sessions. Frame messages with `framing: auto|octet|newline`.
Idle timeout. Session and per-IP caps (stub constants until FIL-001).

## Design references

- [ ] `docs/02-syslog-semantics.md` framing
- [ ] RFC 6587

## Scope

- [ ] `octet-counting`: `MSG-LEN SP SYSLOG-MSG` where MSG-LEN is
      non-zero-leading decimal. Cap `maxMessageBytes`.
- [ ] `non-transparent` / `newline`: split on LF; optional trailing CR
      stripped. Trailer other than NL is 1.1.
- [ ] `auto`: if the first bytes of the stream are `DIGIT+` then SP,
      use octet-counting for that session; else NL. Decision is
      per-session at first non-empty read. Documented, not guessed.
- [ ] Half-close / idle timeout `tcpIdleTimeout` (default 2m)
- [ ] Max concurrent sessions / per-IP (constants or admission hook)
- [ ] Do not treat a TCP connection as a syslog client we dial
- [ ] Testdata under `testdata/framing/`

## Explicit non-scope

- TLS (TLS-001)
- RELP
- Octet-counting pipelining bugs beyond documented limits

## Required tests

- [ ] Octet-counting two messages back-to-back
- [ ] NL two messages
- [ ] `auto` picks octet when `DIGIT SP`, NL when `<`
- [ ] Over-length frame: close session, metric, no store of partial
      beyond cap
- [ ] Idle timeout
- [ ] Import fence still holds

## Acceptance criteria

- RFC 6587 octet-counting from a Python/`nc` client lands two messages.
- `auto` fixtures cannot be flipped by a leading `<` (PRI).
