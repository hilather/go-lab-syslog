# Family evaluation — mcp-integration-lab and sibling appliances

Last reviewed: 2026-09-04

This document evaluates every project that participates in
[`mcp-integration-lab`](https://github.com/hilather/mcp-integration-lab)
and states what LabSyslog copies, what it leaves, and why. Agents must
not invent a methodology that contradicts this table.

The integration lab is **not** the product. It is the compose gateway
that vendors appliances, publishes protocol ports on the host, and
registers MCP servers into MCPJungle. Product logic lives in each
`go-lab-*` repo.

## Lab members

| Project | Pin (lab today) | Role | Wire | MCP prefix | Default profile ports | What LabSyslog copies | What it leaves |
|---|---|---|---|---|---|---|---|
| MCPJungle | 0.4.6 | MCP gateway, tool groups, optional ACLs | HTTP `/mcp` | n/a | 8080 | registration JSON, `allowLegacyClients`, tool group membership, filename==`name` | no product logic |
| LabDNS `go-lab-dns` | v1.3.0 | Authoritative lab DNS | UDP/TCP 53 | `dns_*` | 10053 residual, REST 18080 | numbered docs + ADR pack, `atomic.Pointer` snapshot, KnownFields YAML, two planes, task/program-board format | chaos engine, forwarding, zone overlays |
| LabLDAP `go-lab-ldap-mcp` | v0.5.0 | Directory + control plane | LDAP 389 / LDAPS 636 | `ldap_*` | 3389/3636 residual, HTTPS 8443 | REST+MCP+UI triad, bearer on control plane | 389ds oracle path, HTTPS control plane, lab CA as product |
| TacLab `go-lab-tacacs-mcp` | v1.5.0 | TACACS+ and RADIUS appliance | 49/300 TCP, 1812/1813 UDP | `tac_*` | native 49/300/1812/1813, HTTP 18049 | native IANA dests as policy, labgen-style secret files if needed later | AAA state machines, RadSec/DAS |
| LabMail `go-lab-maildev` | v1.0.0-rc.4 | Receive-only SMTP sink | SMTP 25 | `mail_*` | 1025 residual, web 1080 | **primary analog**: receive-only structural invariant, reserved-key reject, bounded ephemeral store, wait API, wipe-on-reset, no-Dial AST, inbox-style UI, `messages:wait` | MIME parse, `/email` compat, Basic auth as default |
| LabMITM `go-lab-mitmproxy` | v1.6.0 | HTTP(S) intercepting forward proxy | proxy listen, not dest-443 | `mitm_*` | 18888 + 18088 | `--management-listen` may default off, SPA cookie+CSRF, live vs reset-only split | CONNECT intercept, generate-mode CA |
| LabNTP `go-lab-ntp` | v1.0.0-rc.2 | Per-IP virtual clocks | NTP 123 UDP | `ntp_*` | 10123 (ADR 0014 default), REST 18123 | **protocol analog**: first-party `*wire` codec, UDP independence, `NET_BIND_SERVICE`, first-match filter docs, residual high host port, management `:8088` | clock math, never-set-host-clock fence, NTS |
| LabSSO `go-lab-sso` | v1.0.0-rc.1 | Laboratory IdP | HTTPS 443 | `sso_*` | dest-443 + 18443 | dest-vs-escape port documentation, exact identity discipline | OIDC/SAML/WS-Fed |
| ratarmount-rs | v0.1.28 | Archive-backed NFSv3 | NFS 2049 | none yet | 20490 residual | ephemeral overlay idea only | Rust, no MCP in 1.0 lab |
| labinfo (first-party) | in-tree | Service directory | HTTP | `endpoints_list`, `connections_list` | 18090 | every new service **must** have urls + a `connection` block or labinfo fails to start | lives in the integrator |
| labgraph (first-party) | in-tree | LabScenario orchestrator | HTTP | plan/apply/reset | 18091 | later fixture packs (burst flood, PRI mismatch, oversize drop) | not a 1.0 LabSyslog deliverable |
| go-lab-snmp | empty (created 2026-09-04) | future sibling | — | — | — | same pack methodology when designed | not this project |
| go-lab-netconf | empty (created 2026-09-04) | future sibling | — | — | — | same pack methodology when designed | not this project |
| go-jenkins-mcp | separate | Jenkins MCP, not a lab appliance | stdio | jenkins_* | n/a | read-only-by-default MCP posture only | keyring, enterprise Jenkins |

† Residual dests in the default profile are not a second policy. Family rule 15: protocol data planes SUTs already speak use the IANA dest on the **host**. Operator escape is a non-native port in `profile.env` when preflight cannot free the IANA port.

## Methodologies LabSyslog must follow

Copied from the family and treated as invariants:

1. **YAML desired state.** One document, `apiVersion` + `kind`, fail-closed
   `KnownFields(true)`, revision is SHA-256 of the canonical spec
   (secret paths, not secret bytes). Reset rereads the file.
2. **Ephemeral runtime.** Store is not desired state. Restart or
   `state:reset` wipes it.
3. **Two planes, one process.** Data plane must not import control/web.
   Management may be unbound; data plane still accepts.
4. **REST `/v1` + MCP `POST /mcp` + operator UI `/`.** Shared capability
   registry. Adapters do not call each other. Protocol `2026-07-28` with
   `allowLegacyClients` for MCPJungle.
5. **Plan / apply / export / reset.** Optimistic concurrency via
   `expectedRevision`. Idempotency-Key on apply.
6. **Secrets as file refs.** Token ≥32 bytes. Image UID 65532, read-only
   rootfs, `cap_drop: ALL`, `no-new-privileges`, tmpfs `/tmp`.
7. **Native host ports + residual escape.** Syslog IANA dest is 514/udp
   and 514/tcp. Default profile residual is 10514. TLS 6514 is v1.1.
8. **AI-first.** MCP tools are how agents drive the lab. Wait APIs exist
   so agents do not poll.
9. **Docs ship with the change.** Numbered pack, ADRs, program board,
   CHANGELOG `[Unreleased]`, labinfo connection block.
10. **No wrapping existing daemons.** LabMail does not wrap MailDev
    anymore; LabNTP does not wrap ntpd; LabSyslog does not wrap
    rsyslog or syslog-ng.
11. **No Prometheus client.** Hand-rolled OpenMetrics.
12. **No `localStorage` tokens.** Cookie session + CSRF header.
13. **CI failures are defects.** Harden; do not retry away.

## Why LabMail is the primary analog

LabSyslog is a **sink**, not a **source**. LabMail already solved:

- Receive-only as a structural property (import fence + reserved keys +
  no outbound type).
- Bounded store with `fullPolicy`, generation, wipe, wait.
- Best-effort parse: malformed input is stored with a warning, not
  dropped on the floor (unless admission rejects it).
- Agent wait: `POST /v1/messages:wait` / `mail_messages_wait`.
- Inbox UI over the captured stream.

LabNTP is the **protocol analog** for the data plane (first-party wire
codec, UDP listener that must not import HTTP, residual high host port
because the IANA dest is often occupied, `NET_BIND_SERVICE` only when
binding 514).

LabDNS is the **process analog** for how work is packaged (FND/CFG/…,
ADRs before invariant changes, reviewer checklists, generated contracts).

## Port policy applied to syslog

| Plane | IANA | Container | Default profile host | Escape |
|---|---|---|---|---|
| Syslog UDP | 514/udp | `:514/udp` | 10514/udp | `LABSYSLOG_SYSLOG_PORT=514` when preflight allows |
| Syslog TCP | 514/tcp | `:514/tcp` | 10514/tcp | `LABSYSLOG_SYSLOG_TCP_PORT=514` |
| Syslog TLS | 6514/tcp | `:6514/tcp` | not published in 1.0 | v1.1 |
| Management | n/a | `:8088/tcp` | 18514/tcp | `LABSYSLOG_MGMT_PORT` |

Preflight occupancy is `/proc/net` UDP bound + TCP LISTEN. `EACCES` /
`EPERM` is **not** occupied (UID cannot bind 514; dockerd can publish).
Typical occupants of 514: rsyslog, syslog-ng, systemd-journald.socket
forward. Error copy must name the fix: stop the occupant, extra IP for
`LAB_PUBLIC_HOST`, or keep the residual 10514 (SUTs that hardcode dest
514 cannot follow the residual).

## What the integrator will do later (SWAP-001)

Listed so product agents do not implement lab orchestration in this
repo. Full checklist: [13-integration-lab-swap.md](13-integration-lab-swap.md).

- Pin `third_party/go-lab-syslog` in `internal/lab/vendor.go`
- Compose service `labsyslog` copied from the LabNTP block
- `profiles/default/labsyslog/bootstrap.yaml`
- labinfo catalog id `labsyslog` with a connection block
- `mcpjungle/servers/labsyslog.json` + group membership
- `secrets/labsyslog-token` 0o644, `stageLabinfoCreds`
- `profile.env` `LABSYSLOG_*`
- AGENTS.md rule 15 + architecture table + CHANGELOG + Pages
- smoke: `logger` UDP + TCP, then `syslog_messages_wait` through the gateway
