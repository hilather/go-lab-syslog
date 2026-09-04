# UDP-001: UDP sink (RFC 5426)

Status: not-started
Recommended owner: data-plane agent
Dependencies: WIRE-001
Exclusive ownership: `internal/syslogserver` UDP path, `cmd/labsyslog/serve.go` UDP bind

## Goal

Bind UDP, accept one datagram as one message, hand to a
`Handler` interface. Management may be off. Dual-stack
`ListenPacket("udp", addr)`. IPv4-mapped IPv6 unmapped before
CIDR match.

## Design references

- [ ] `docs/01-architecture.md` import fence
- [ ] `docs/02-syslog-semantics.md` UDP section
- [ ] LabNTP `internal/ntpserver` listen pattern

## Scope

- [ ] `net.ListenPacket("udp", addr)`
- [ ] One datagram = one message. No framing.
- [ ] Datagram larger than `udpMaxDatagramBytes` / `maxMessageBytes`:
      drop, increment metric, store nothing
- [ ] Empty datagram: drop, metric
- [ ] Admission hook (FIL-001 may stub allow-all until that wave)
- [ ] `--syslog-udp-listen` flag overrides YAML
- [ ] Serve with `--management-listen=off` still accepts
- [ ] Import fence test: package does not import `internal/control`,
      `internal/web`, `net/http`

## Explicit non-scope

- TCP
- Store implementation (inject a fake Handler)
- Filters beyond a callback

## Required tests

- [ ] Dual-stack send from 127.0.0.1 and ::1
- [ ] Oversize drop
- [ ] Management unbound does not block UDP
- [ ] Import fence

## Acceptance criteria

- `printf '<14>hello\n' | nc -u` against the test bind stores through
  the fake handler.
- No `Dial` identifier in the package.
