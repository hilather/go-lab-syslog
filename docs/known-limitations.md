# Known limitations (1.0)

Last reviewed: 2026-09-05

LabSyslog 1.0 is laboratory software. It is not a production collector
and does not claim production-collector completeness.

- No persistence: heap-only ephemeral store; restart and reset wipe.
  No disk spool, SQLite, inverted index, or tmpfs spill.
- No HA, clustering, or multi-process store.
- No forwarding, relaying, remote destinations, or outbound syslog
  (ADR 0002 / ADR 0011).
- RFC 5425 TLS / dest 6514 is not implemented. `tls.enabled: true`
  fail-closes with `tls_unsupported`. TLS-001 is v1.1 and must not
  reopen GA-001.
- No RELP, RFC 3195 BEEP, RFC 5848 signed syslog, journald native,
  `/dev/log`.
- No OAuth, OIDC, HTTP Basic, or `dev-loopback-unauth`. Bearer file
  tokens and cookie sessions only.
- No rsyslog/syslog-ng compatibility surface beyond the wire
  formats.
- UDP oversize is dropped, not truncated into the store.
  `Message.Truncated` is **always false** in 1.0; no 1.0 path
  rewrites raw.
- Per-IP admission on host-published UDP is best-effort when
  Docker `userland-proxy` SNATs sources (NAT collision). Reliable
  path is the compose network.
- Store is in-process memory. 10k / 256MiB default caps.
- No structured-data query beyond list/wait AND filters.
- IANA dest 514 is not the shipped default profile port (10514).
- Operator UI is English-only in 1.0.
- Mira review of the operator SPA is required before tagging
  **v1.0.0**. [reviews/mira-ui-001.md](reviews/mira-ui-001.md) is a
  placeholder, not an approval.
- Integrator vendor pin is a follow-on after the first `v*` tag.
  This repo does not change `mcp-integration-lab` or `vendor.go`.
