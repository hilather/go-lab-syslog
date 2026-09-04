# Evaluation of mcp-integration-lab projects

**Status:** accepted for LabSyslog design  
**Last reviewed:** 2026-09-04  
**Integrator:** https://github.com/hilather/mcp-integration-lab  
**Target appliance:** https://github.com/hilather/go-lab-syslog  

This document evaluates every project that participates in the MCP Integration Lab and extracts the methodology LabSyslog must copy. Product logic stays in `go-lab-syslog`. The integrator only vendors, publishes ports, registers MCP, and catalogs the connection.

## What the lab is

`mcp-integration-lab` is a docker-compose laboratory that publishes real network protocols on the host and drives them from one Model Context Protocol gateway (MCPJungle). Configuration lives in `profiles/<name>/`. Runtime state is tmpfs. A restart wipes it.

Ground rules that bind every appliance, including LabSyslog:

1. Permanent config is YAML; runtime state is ephemeral.
2. Desired state is fail-closed (`KnownFields(true)`).
3. Data planes use IANA/native host destinations; management stays on high ports.
4. Services expose REST `/v1`, MCP `POST /mcp` (protocol `2026-07-28`), and usually an embedded UI.
5. Containers run UID `65532`, `cap_drop: ALL`, read-only rootfs, no-new-privileges.
6. Secrets are file refs. Tokens are ≥32 bytes. Never inline.
7. labinfo requires a `connection` block per service or the lab fails closed.
8. `LAB_DEV_MODE` is the single posture knob.
9. Documentation ships in the same change as code.
10. CI failures are investigated and hardened, never retried away.

Vendor pins observed 2026-09-04:

| Appliance | Repo | Pin |
|---|---|---|
| LabDNS | `hilather/go-lab-dns` | v1.3.0 |
| LabLDAP | `hilather/go-lab-ldap-mcp` | v0.5.0 |
| TacLab | `hilather/go-lab-tacacs-mcp` | v1.5.0 |
| LabMail | `hilather/go-lab-maildev` | v1.0.0-rc.4 |
| LabMITM | `hilather/go-lab-mitmproxy` | v1.6.0 |
| LabNTP | `hilather/go-lab-ntp` | v1.0.0-rc.2 |
| LabSSO | `hilather/go-lab-sso` | v1.0.0-rc.1 |
| MCPJungle | `mcpjungle/mcpjungle` | 0.4.6 |
| ratarmount-rs | signed `.deb` | 0.1.28 |

Empty sibling appliances created the same day as LabSyslog and **out of scope** for this pack: `go-lab-snmp`, `go-lab-netconf`.

---

## Member-by-member evaluation

### 1. MCPJungle (gateway)

**Role.** Single MCP gateway. Clients talk to Jungle; Jungle proxies to each appliance's `/mcp`.

**What LabSyslog copies.**

- Streamable HTTP `POST /mcp`.
- Protocol pin `2026-07-28`.
- `spec.management.mcp.allowLegacyClients: true` in the *lab* bootstrap so Jungle can register. Product default stays `false`.
- Registrar discovery: `profiles/<name>/mcpjungle/servers/labsyslog.json` whose filename matches JSON `name`.
- Bearer token injected from `secrets/labsyslog-token`.

**What LabSyslog leaves behind.**

- Gateway ACLs, client tokens, development-vs-enterprise mode. Those belong to the lab, not the appliance.

### 2. LabDNS (`go-lab-dns`) — methodology donor

**Role.** Authoritative laboratory DNS: overrides, wildcards, suffix forwarding, bounded chaos, operator console.

**Surfaces.** DNS 53 (residual host 10053), management REST/MCP/UI.

**Schema.** `labdns.dev/v1alpha1` kind `LabDNS`. MCP tools `dns_*`.

**What LabSyslog copies.**

- The `tasks/` contract format: program board, one file per wave, agent-task-template, parallelization-plan, reviewer-checklist.
- Numbered docs pack `docs/01-…`.
- ADRs as the only way to change an invariant.
- Generated OpenAPI / JSON Schema / MCP manifest / capability map with `make generate` + `verify-generated`.
- Placeholder Make targets that **exit 1**, never succeed as no-ops.
- Bounded deterministic chaos only, default off. No random fault injection in 1.0.

**What LabSyslog leaves behind.**

- Chaos effects, forwarding, cache. A sink does not recurse and does not answer queries.

### 3. LabLDAP (`go-lab-ldap-mcp`)

**Role.** Control plane around a directory engine. Default engine is now Go-native `labldapd`; 389 DS remains the oracle (`engine: 389ds`).

**Surfaces.** LDAP 3389 / LDAPS 3636 (native dest 389/636), HTTPS 8443 REST/MCP/UI.

**What LabSyslog copies.**

- Distinct data-plane protocol vs control-plane HTTPS.
- Bootstrap is the only supported configuration path.
- Soft reset is gated.
- Read tools on by default; mutations register only when asked — *except* LabSyslog follows LabMail more closely: capture tools are on, wipe/apply stay scoped `syslog.write` / `syslog.admin`.

**What LabSyslog leaves behind.**

- Wrapping an external daemon as the data plane. LabSyslog is first-party Go, like LabMail and LabNTP, not like historic 389 DS.

### 4. TacLab (`go-lab-tacacs-mcp`) + RADIUS

**Role.** All-in-one TACACS+ lab appliance. RFC 8907 + RFC 9887 TLS 1.3. RADIUS 1812/1813 on the same compose project.

**Surfaces.** TACACS+ 49 / 300, RADIUS 1812/1813, management 18049.

**What LabSyslog copies.**

- Native IANA data-plane ports as the design (rule 15).
- Privileged port preflight: `EACCES` is **not** occupied; dockerd can still publish.
- `allow_legacy_clients` knob so Jungle registers without an appliance patch.
- labgen-style secret minting is *not* copied; LabSyslog tokens are file-backed static bearers like LabMail/LabNTP.

**What LabSyslog leaves behind.**

- AAA protocol state machines, Argon2id password files, RADIUS.

### 5. LabMail (`go-lab-maildev`) — primary product analog

**Role.** Receive-only SMTP lab appliance. Capture mail, inspect over REST, MCP, and an inbox UI. Never relays.

**Surfaces.** SMTP 1025 (native dest 25), management 1080.

**Schema.** `labmail.dev/v1alpha1` kind `LabMail`. MCP tools `mail_*`. Resources `labmail://…`.

**This is the sink pattern LabSyslog must follow.**

Copied invariants:

- Receive-only is **structural**. Production packages do not Dial. Reserved YAML keys (`forward*`, `relay*`, `remote*`, `destination*`, `smarthost*`, `outgoing*`) reject after normalization.
- Compat or future "forward this message" endpoints return **403** `receive_only`. There is no implementation behind them.
- Bounded store with `maxMessages`, `maxBytes`, `fullPolicy`, `maxWait`.
- `messages.wait` so agents can block until a SUT emits.
- Best-effort parse: malformed input is stored with a warning, not dropped on the floor (LabMail MIME).
- Reset wipes the store and rereads bootstrap. The process never writes the bootstrap file.
- Inbox / message viewer UI is required for 1.0 GA.
- Compose service may keep a stable catalog id if a swap rename would break the lab; LabSyslog has no predecessor, so labinfo id is `labsyslog` from day one.
- `allowLegacyClients: true` in the lab overlay only.

Not copied:

- maildev `/email` compat shim. LabSyslog has no incumbent HTTP API to emulate.
- MIME / attachments. Syslog payload is text + optional RFC 5424 structured data.

### 6. LabMITM (`go-lab-mitmproxy`)

**Role.** Laboratory HTTP(S) intercepting proxy. Capture, decrypt, inspect.

**Surfaces.** Forward proxy (not IANA dest 443). Management 18090-class.

**What LabSyslog copies.**

- Capture appliance with a flow/message list, raw view, and wipe.
- Embedded inspector UI pattern.
- YAML desired state for listen/auth/UI.

**What LabSyslog leaves behind.**

- TLS interception, CONNECT handling, mitmproxy inheritance. Syslog TLS (RFC 5425) is v1.1.

### 7. LabNTP (`go-lab-ntp`) — primary protocol-appliance analog

**Role.** Laboratory NTPv3/v4 server with per-IP virtual clocks.

**Surfaces.** UDP NTP container `:123` host residual `10123`, management container `:8088` host `18123`.

**Schema.** `labntp.dev/v1alpha1` kind `LabNTP`. MCP tools `ntp_*`.

Copied invariants:

- First-party wire codec. No third-party protocol library types leak.
- Two planes, one process. Data plane keeps answering if management is off or slow.
- `--management-listen` defaults off; image CMD binds `:8088`.
- Immutable snapshot swapped atomically. In-flight packets keep the snapshot they loaded.
- First-match filters by list order (admission/classify), not longest-prefix.
- UID 65532 + `CAP_NET_BIND_SERVICE` only when binding `:514` (LabNTP `:123`).
- Default *host* publish is a high residual (NTP `10123`, syslog `10514`) because the IANA port is often held (`timesyncd` / `rsyslog` / `journald`). Native dest remains the documented design; profile.env is the operator escape.
- Hand-rolled OpenMetrics. No Prometheus client.
- Official MCP SDK only on the adapter side.
- Host-clock-style import fence becomes a **no-Dial / no-forward** fence.

Not copied:

- Virtual clocks, MAC keys, KoD packets.
- NTP unmatched-packet-drop. A sink's unmatched filter action is **capture**.

### 8. LabSSO (`go-lab-sso`)

**Role.** Laboratory IdP. OIDC, SAML, WS-Fed from one YAML file.

**Surfaces.** HTTPS dest 443 (container 10443), management 18443.

**What LabSyslog copies.**

- Fail-closed config.
- GitOps apply/reset.
- File-referenced signing material (pattern for a future RFC 5425 cert pair).

**What LabSyslog leaves behind.**

- Identity protocols. Syslog data plane has no client authentication in 1.0.

### 9. NFS / ratarmount-rs

**Role.** Userspace NFS export of archives for fixture packs.

**What LabSyslog copies.** Nothing in-process. Syslog may later appear as a labgraph fixture source (burst-flood, oversize-drop) but does not speak NFS.

### 10. labinfo (first-party, `cmd/labinfo`)

**Role.** Service directory MCP: `endpoints_list` and `connections_list`.

**LabSyslog contract.**

- labinfo catalog id: `labsyslog`.
- URL templates over profile env, host from `LAB_PUBLIC_HOST`.
- `connection` block is mandatory:
  - endpoints: `syslog-udp` host:port, `syslog-tcp` host:port, `management` host:port
  - params: RFC 3164 + RFC 5424, framing `auto`, no TLS in 1.0, no data-plane credential
  - management token referenced from `secrets/labinfo-creds/` via `stageLabinfoCreds`
- Missing connection block must fail labinfo start (rule 9).

### 11. labgraph (first-party, `cmd/labgraph`)

**Role.** LabScenario orchestrator (REST + MCP + SPA).

**LabSyslog contract.**

- No product logic in labgraph.
- Later fixture pack may drive `logger` / `nc` bursts against LabSyslog and assert via `syslog_messages_wait`.
- Out of 1.0 appliance scope. Documented only as a follow-on in SWAP-001.

### 12. mcplab CLI + profiles

**Role.** Typed Go orchestrator. Make targets are thin wrappers.

**LabSyslog contract.**

- Profile vars: `LABSYSLOG_SYSLOG_PORT`, `LABSYSLOG_SYSLOG_TCP_PORT`, `LABSYSLOG_MGMT_PORT`.
- `make reload APP=labsyslog` recreates only that container.
- Preflight treats 514 UDP/TCP occupancy via `/proc/net`; `EACCES` is not occupied.
- Do not add a Go `userland-proxy` probe. Document NAT only if UDP source address matters for filters (it does: `remoteAddr` is stored and may match `allowClientCidrs`). Same class of note as LabNTP ADR 0014, but syslog is a sink so source preservation is observability-quality, not correctness-of-time.

---

## Methodology synthesis — what "match our other projects" means

| Concern | Family rule | LabSyslog application |
|---|---|---|
| Language | Go 1.26, Apache-2.0 | same |
| Shape | one binary, one container, two planes | `labsyslog` |
| Config | YAML kind + `*.dev/v1alpha1` | `labsyslog.dev/v1alpha1` / `LabSyslog` |
| Decode | `KnownFields(true)`, kebab-case wire names | reject `maxMessageBytes` aliases that are unknown; freeze kebab |
| State | bootstrap read-only; snapshot atomic; reset rereads | store wipe on reset |
| Control | REST `/v1` + MCP `/mcp` + UI `/` | capability registry, parity tests |
| Auth | file-backed bearer ≥32 bytes, cookie+CSRF for UI | `labsyslog_session`, `X-LabSyslog-CSRF` |
| MCP | official SDK, `2026-07-28`, prefix per product | `syslog_*`, `labsyslog://` |
| Data plane | independent of management | UDP/TCP keep accepting if `--management-listen=off` |
| Wire | first-party codec | `internal/syslogwire` + `internal/syslogframing` |
| Sink | receive-only structural | no Dial, reserved-key reject, 403 forward |
| Image | scratch, UID 65532, HEALTHCHECK exec | `ghcr.io/hilather/labsyslog` |
| Docs | numbered pack + ADRs + tasks waves | this pack |
| Integrator | pin last, examples BOM in the appliance repo | SWAP-001 |

## Gaps the lab has today

The lab can emit logs (every appliance logs, SUTs can be pointed at a host syslog) but **cannot capture or query syslog** the way it captures SMTP (LabMail) or HTTP (LabMITM). That is the product hole LabSyslog fills.

Typical lab uses after SWAP-001:

- Assert a SUT wrote `authpriv.warning` after a failed bind to LabLDAP.
- Capture TacLab accounting records sent as syslog.
- Wait for a DHCP/DNS client's logger line during a labgraph scenario.
- Prove a device under test honors `*.*.*` vs `local0.info` filtering.

## Non-goals confirmed by the family

- Not a production log shipper.
- Not a SIEM and not an Elasticsearch API.
- Not RELP, not RFC 3195, not journald native.
- Not an outbound forwarder "for convenience." That would destroy the LabMail invariant and make the lab an amplifier.
