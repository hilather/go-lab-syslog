# 04 — State and configuration

Last reviewed: 2026-09-04

Desired state is one YAML (or JSON) document. Runtime messages are
not desired state. The process never writes the bootstrap file.

## Document

```
apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: <dns-label, required>
spec: { ... }
```

Unknown top-level keys and unknown spec keys fail validation.
`KnownFields(true)` on the YAML decoder. JSON input to
`state:validate` uses the same struct with `DisallowUnknownFields`.

`apiVersion` and `kind` are exact. `labsyslog.dev/v1`, `LabSyslogd`,
`lab-syslog` are errors.

## YAML field map (frozen)

Wire names are camelCase to match LabMail / LabNTP documents already
in the family (LabNTP mixes: `refid` is special; LabSyslog has no
such legacy). Do not accept kebab-case aliases in 1.0.

Listener addresses are Go listen strings (`:514`, `127.0.0.1:1514`).
Durations are Go duration strings (`2m`, `500ms`). Byte sizes are
binary units (`64KiB`, `256MiB`) parsed by `internal/config`.

### spec.listeners

| Field | Default | Notes |
|---|---|---|
| `udp.enabled` | true | |
| `udp.address` | `:514` | reset-only (requires process restart / reset to rebind) |
| `tcp.enabled` | true | |
| `tcp.address` | `:514` | reset-only |
| `tcp.framing` | `auto` | `auto` \| `octet-counting` \| `non-transparent` |
| `tls.enabled` | false | `true` → `tls_unsupported` in 1.0 (ADR 0012) |
| `tls.address` | `:6514` | ignored while disabled |
| `tls.certFile` | empty | v1.1 placeholder; ignored while `enabled` is false |
| `tls.keyFile` | empty | v1.1 placeholder; ignored while `enabled` is false |
| `tls.caFile` | empty | v1.1 placeholder; ignored while `enabled` is false |
| `tls.clientAuth` | false | v1.1 placeholder; ignored while `enabled` is false |
| `management.address` | empty | empty means management off unless CLI flag set |
| `management.restPath` | `/v1` | |
| `management.mcpPath` | `/mcp` | |

Listener addresses are **reset-only**. `changes:apply` that tries to
change them returns `immutable_field`.

### spec.auth

| Field | Default | Notes |
|---|---|---|
| `mode` | `bearer` | `bearer` only. No HTTP Basic. No `dev-loopback-unauth`. Management bind requires ≥1 usable token unless listen is off. `spec.management.auth` is an unknown field and rejects. |
| `tokens[].id` | required | |
| `tokens[].role` | `administrator` | `administrator` \| `reader` |
| `tokens[].secretFile` | required | path required. If the file exists at `validate`, trimmed contents must be ≥32 bytes. A missing file is allowed at `validate`. |

Inline `secret:` / `token:` keys are reserved-key rejects.

### spec.syslog

| Field | Default |
|---|---|
| `parse.rfc3164` | true |
| `parse.rfc5424` | true |
| `parse.bestEffort` | true |
| `maxMessageBytes` | `64KiB` |
| `udpMaxDatagramBytes` | `64KiB` |
| `tcpIdleTimeout` | `2m` |
| `hostname` | `labsyslog.lab` |
| `behavior.mode` | `accept` |

At least one of `rfc3164` / `rfc5424` must be true.

### spec.admission

| Field | Default |
|---|---|
| `allowClientCidrs` | `["127.0.0.0/8", "::1/128"]` if omitted at compile; lab overlay sets the compose subnet |
| `maxDatagramsPerSec` | 20000 |
| `maxDatagramsPerIP` | 2000 |
| `maxTcpConns` | 256 |
| `maxTcpConnsPerIP` | 16 |
| `sessionTimeout` | `2m` |

Empty `allowClientCidrs` is a validate error (would black-hole the sink).
IPv4-mapped IPv6 is normalized before match.

### spec.store

| Field | Default |
|---|---|
| `maxMessages` | 10000 |
| `maxBytes` | `256MiB` |
| `fullPolicy` | `evict_oldest` |
| `maxWait` | `60s` |
| `rawRetain` | true |

`maxMessages` minimum 1, maximum 1_000_000. `maxBytes` minimum `64KiB`.

### spec.filters[]

```yaml
filters:
  - name: drop-debug
    enabled: true
    match:
      sourceCidrs: ["10.99.42.0/24"]
      facilities: [user]
      severities: [debug]
      severityAtLeast: ""        # alternative to severities[]
      appNames: ["noisy"]
      hostnames: []
      transports: [udp, tcp]
    action:
      mode: drop-silent          # capture | drop-silent | tag
      tag: ""                    # required when mode=tag
```

First enabled match wins. No match → capture (ADR 0009). Duplicate
`name` values are a validate error. `name` is a DNS label.

### spec.ui / spec.management / spec.observability

Mirror LabNTP / LabMail:

- `ui.enabled` default true
- `management.allowedOrigins` default `[]` (loopback only)
- `management.allowedOrigins` only in 1.0. Empty list is loopback
  http(s) only; a non-empty list is exactly the listed origins
  (loopback is not unioned). No `"*"` / `"private"` sentinels
  (those are a later SEC-002 if copied from LabMail).
  `originAllowlist` is an unknown field and rejects.
- `management.mcp.allowLegacyClients` default false; lab overlay sets true
- `management.bodyLimit` default `1MiB`
- `management.requestsPerSecond` 32, `burst` 64, `maxConcurrent` 256
- `observability.logLevel` `info`
- `observability.metrics.publicPath` false

## Revision

Revision is the lower-case hex SHA-256 of the canonical YAML produced
by `canonicalize`. Canonical YAML:

- fields in struct order
- secret **paths** included, secret **bytes** never
- defaulted values materialized
- zero/empty optional slices omitted except `allowClientCidrs` and
  `filters` (always present)

`labsyslog canonicalize --config FILE` prints that document plus
`revision: sha256:…` on stderr. `validate` exits 0 and prints the
revision. Unknown fields exit 2.

## Live vs reset-only

| Live via plan/apply | Reset-only |
|---|---|
| store caps, fullPolicy, maxWait, rawRetain | listener addresses, enabled flags, `tcp.framing` |
| filters replace | auth.mode, token files |
| admission rate caps and CIDRs | tls block |
| syslog.parse, maxMessageBytes, behavior.mode | ui.enabled, management.address, restPath, mcpPath |
| observability.logLevel | `management.bodyLimit`, `requestsPerSecond`, `burst`, `maxConcurrent`, `allowedOrigins`, `mcp.allowLegacyClients` |
| | `syslog.hostname`, `udpMaxDatagramBytes`, `tcpIdleTimeout` |
| | `observability.metrics.publicPath`, `observability.audit.ring` |

Unspecified fields are reset-only. Reset-only fields in a plan produce
operation `replaceListeners` which is rejected with `immutable_field`
unless the plan is a reset. Do not add live operations for HTTP limits
or hostname (C30).

## Plan / apply

Closed operation set (1.0):

- `replaceStoreCaps`
- `replaceFilters`
- `replaceAdmission`
- `replaceSyslogParse`
- `replaceBehavior`
- `replaceObservability`

Request:

```json
{
  "expectedRevision": "sha256:…",
  "operations": [ { "type": "replaceFilters", "filters": [ … ] } ],
  "reason": "scenario-12"
}
```

Apply requires the same `expectedRevision` and an `Idempotency-Key`
header (REST) or `idempotencyKey` argument (MCP). Duplicate key +
identical body returns the original result.

## Reset

`POST /v1/state:reset`:

1. Re-read bootstrap path.
2. Compile. On failure, keep the live snapshot and return
   `bootstrap_invalid` (do not bind-break a running server).
3. Swap snapshot.
4. Wipe store.
5. Audit `state.reset`.

CLI `--config` path is the bootstrap path. There is no second file.
