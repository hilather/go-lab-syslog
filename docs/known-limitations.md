# Known limitations (1.0)

Last reviewed: 2026-09-04

- Not a production collector. No disk spool, no forwarding, no
  HA, no SIEM query language.
- RFC 5425 TLS / dest 6514 is not implemented. `tls.enabled: true`
  fail-closes.
- No RELP, RFC 3195 BEEP, RFC 5848 signed syslog, journald native,
  `/dev/log`.
- No rsyslog/syslog-ng compatibility surface beyond the wire
  formats.
- UDP oversize is dropped, not truncated into the store.
- Per-IP admission on host-published UDP is best-effort when
  Docker `userland-proxy` SNATs sources.
- Store is in-process memory. 10k / 256MiB default caps.
- No structured-data query beyond list/wait AND filters.
- IANA dest 514 is not the shipped default profile port (10514).
- Operator UI is English-only in 1.0.
