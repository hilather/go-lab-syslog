# UDP-001: UDP sink (RFC 5426)

Status: done
Recommended owner: data-plane agent
Dependencies: WIRE-001
Exclusive ownership: `internal/syslogserver` UDP path, `cmd/labsyslog/serve.go` UDP bind

## Goal

Bind UDP, accept one datagram as one message, hand to a
`Handler` interface. Management may be off. Dual-stack
`ListenPacket("udp", addr)`. IPv4-mapped IPv6 unmapped before
CIDR match.

## Design references

- [x] `docs/01-architecture.md` import fence
- [x] `docs/02-syslog-semantics.md` UDP section
- [x] LabNTP `internal/ntpserver` listen pattern

## Scope

- [x] `net.ListenPacket("udp", addr)`
- [x] One datagram = one message. No framing.
- [x] Datagram larger than `udpMaxDatagramBytes` / `maxMessageBytes`:
      drop, increment metric, store nothing
- [x] Empty datagram: drop, metric
- [x] Admission hook (FIL-001 may stub allow-all until that wave)
- [x] `--syslog-udp-listen` flag overrides YAML
- [x] Serve with `--management-listen=off` still accepts
- [x] Import fence test: package does not import `internal/control`,
      `internal/web`, `net/http`

## Explicit non-scope

- TCP
- Store implementation (inject a fake Handler)
- Filters beyond a callback

## Required tests

- [x] Dual-stack send from 127.0.0.1 and ::1
- [x] Oversize drop
- [x] Management unbound does not block UDP
- [x] Import fence

## Acceptance criteria

- `printf '<14>hello\n' | nc -u` against the test bind stores through
  the fake handler.
- No `Dial` identifier in the package.
