# LabSyslog implementation design

**Status:** accepted for implementation  
**Date:** 2026-09-04  
**Target:** https://github.com/hilather/go-lab-syslog  
**API group:** `labsyslog.dev/v1alpha1`  
**License:** Apache-2.0  
**Source of truth:** this file until an ADR supersedes a row  

## Overview

LabSyslog is a single-process Go lab appliance. SUTs speak RFC 3164 / RFC 5424 over UDP (RFC 5426) and TCP (RFC 6587). LabSyslog stores a bounded ephemeral window and exposes it over REST `/v1`, MCP `POST /mcp`, and an embedded operator UI. It never forwards.

Closest siblings: LabMail (sink invariants) and LabNTP (first-party wire + UDP independence). Task format: LabDNS.

## Goals (1.0)

- Receive-only UDP + TCP syslog
- Parse RFC 3164 and RFC 5424 with best-effort warnings
- RFC 6587 `auto` / `octet-counting` / `non-transparent`
- Bounded store, wait, list, get, raw, clear
- YAML GitOps, plan/apply/reset, revision hashes
- REST + MCP parity, bearer auth, operator UI
- Scratch image UID 65532
- Examples BOM for mcp-integration-lab

## Non-goals (1.0)

- RFC 5425 TLS, RELP, RFC 3195, `/dev/log`
- Wrapping rsyslog / syslog-ng / journald
- Forwarding
- Persistent spool
- SIEM query language
- Product logic in mcp-integration-lab

## Decisions

| ID | Decision |
|---|---|
| D1 | New first-party module `github.com/hilather/go-lab-syslog` |
| D2 | First-party `syslogwire` / `syslogframing`; no syslog libraries |
| D3 | Two planes, one process |
| D4 | Receive-only is structural (ADR 0007) |
| D5 | YAML KnownFields fail-closed; camelCase tags; kebab aliases reject |
| D6 | Secrets are file refs; token ≥32 bytes |
| D7 | MCP protocol `2026-07-28`; tools `syslog_*`; resources `labsyslog://` |
| D8 | Control-plane order REST → Auth → MCP |
| D9 | Unmatched filter = capture (ADR 0009) |
| D10 | UDP oversize = drop and do not store |
| D11 | TCP framing default `auto` |
| D12 | TLS enable rejected in 1.0 (ADR 0012) |
| D13 | Host residual 10514; native dest 514 (ADR 0008) |
| D14 | Container `:514` needs NET_BIND_SERVICE on integrator compose (ADR 0010) |
| D15 | labinfo id `labsyslog` from day one (no maildev-style alias) |
| D16 | UI required for 1.0 GA; rc.1 may be API-complete |
| D17 | `allowLegacyClients` default false; lab overlay true |
| D18 | Hand-rolled OpenMetrics; no Prometheus client |
| D19 | Official MCP SDK only on the adapter |
| D20 | Store evict_oldest default |
| D21 | Best-effort parse stores raw + parseWarning |
| D22 | No userland-proxy probe; document NAT collision |
| D23 | Placeholder Make targets fail closed |
| D24 | Integrator pin LAST |
| D25 | Go 1.26, Apache-2.0, image `ghcr.io/hilather/labsyslog` |
| D26 | Session cookie `labsyslog_session`; CSRF `X-LabSyslog-CSRF` |
| D27 | Ready = enabled listeners bound + snapshot + (mgmt bound or off) |
| D28 | Deterministic behavior modes only; no random chaos |

## Schema sketch

```yaml
apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab
spec:
  listeners:
    udp:
      enabled: true
      address: ":514"
    tcp:
      enabled: true
      address: ":514"
      framing: auto          # auto | octet-counting | non-transparent
    tls:
      enabled: false         # true rejected in 1.0
      address: ":6514"
      certFile: ""
      keyFile: ""
    management:
      address: ":8088"
      restPath: /v1
      mcpPath: /mcp
  syslog:
    hostname: labsyslog.lab
    parse:
      rfc3164: true
      rfc5424: true
      bestEffort: true
    maxMessageBytes: 64KiB
    udpMaxDatagramBytes: 64KiB
    tcpIdleTimeout: 2m
    behavior:
      mode: accept           # accept | drop-silent | close
  admission:
    allowClientCidrs: ["0.0.0.0/0", "::/0"]
    maxDatagramsPerSec: 20000
    maxDatagramsPerIP: 2000
    maxTcpConns: 256
    maxTcpConnsPerIP: 16
  store:
    maxMessages: 10000
    maxBytes: 256MiB
    fullPolicy: evict_oldest # evict_oldest | reject
    maxWait: 60s
    rawRetain: true
    spillDirectory: ""       # non-empty rejected in 1.0
  filters: []                # first-match; unmatched = capture
  ui:
    enabled: true
  management:
    auth:
      mode: bearer
      tokens:
        - id: operator
          secretFile: /run/secrets/labsyslog-token
          role: administrator
          scopes: [syslog.read, syslog.write, syslog.admin, syslog.audit.read]
    mcp:
      allowLegacyClients: false
    originAllowlist: []
    bodyLimit: 1MiB
    requestsPerSecond: 50
    burst: 100
    maxConcurrent: 32
  observability:
    logLevel: info
    metrics:
      listen: ""
      publicPath: false
    audit:
      ring: 128
```

Filter item:

```yaml
- id: auth-noise
  enabled: true
  match:
    facilities: [auth, authpriv]
    severityAtLeast: info
    transports: [udp, tcp]
  action: capture          # capture | drop-silent | tag
  tags: [aaa]
```

## CLI

```text
labsyslog version
labsyslog validate --config FILE
labsyslog canonicalize --config FILE
labsyslog serve --config FILE
            [--syslog-udp-listen ADDR]
            [--syslog-tcp-listen ADDR]
            [--management-listen ADDR|off]
labsyslog healthcheck --url URL
labsyslog mcp-stdio --config FILE
```

## PR / wave plan

See `tasks/00-program-board.md`. Summary:

1. FND-001 foundation
2. CFG-001 domain + YAML
3. WIRE-001 codec
4. UDP-001 UDP sink
5. TCP-001 TCP sink
6. STORE-001 ring
7. FIL-001 filters
8. APP-001 / STA-001 service
9. API-001 REST
10. SEC-001 auth
11. MCP-001 MCP
12. OBS-001 observability
13. DEP-001 image + examples
14. UI-001 SPA
15. SWAP-001 integrator BOM
16. GA-001 hardening
— TLS-001 v1.1

## Testing

See `docs/10-testing-strategy.md`. Tag-gate requires format, lint, unit, race, fuzz-smoke, config-compat, docs, changelog, parity, container, web.

## Risks

| Risk | Mitigation |
|---|---|
| Port 514 occupied | residual 10514 + preflight copy |
| Docker NAT collapses UDP source | document userland-proxy; compose network |
| RFC 3164 ambiguity | best-effort + raw retain |
| RFC 6587 auto misdetect | fixtures; explicit framing override |
| Agents invent forward | ADR 0007 + reserved keys + AST |
| Agents wrap rsyslog | ADR 0002 + import tests |
