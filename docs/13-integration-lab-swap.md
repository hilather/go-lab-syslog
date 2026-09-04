# Integration with MCP Integration Lab

Status: Proposed normative contract  
Owners: Architecture, Deployment  
Last reviewed: 2026-09-04  
Related ADRs: 0001, 0002, 0010  
Depends on: DEP-001, MCP-001, SEC-001  
Integrator change is LAST. This document is the BOM the appliance ships;
`mcp-integration-lab` copies it. Product logic never moves into the
integrator.

## What the integrator owns

`hilather/mcp-integration-lab` owns orchestration, profiles, secrets
layout, gateway policy, preflight, and smoke. It vendors appliance
repos into `third_party/` at a pinned tag.

LabSyslog ships the examples this document names. The integrator
change is a follow-on PR after the first appliance tag exists.

## Naming in the lab

| Kind | Value |
|---|---|
| Compose service | `labsyslog` |
| labinfo catalog id | `labsyslog` |
| MCPJungle server name / filename | `labsyslog` / `labsyslog.json` |
| Token file | `secrets/labsyslog-token` (mode `0o644`) |
| Token env interpolation | `${LABSYSLOG_TOKEN}` |
| Config mount | `/etc/labsyslog/config.yaml` |
| Secret mount | `/run/secrets/labsyslog-token` |
| Image (local lab) | `labsyslog:local` from `./third_party/go-lab-syslog` |
| Image (released) | `ghcr.io/hilather/labsyslog` |

Do not reuse the LabMail `maildev` rename wart. The id is `labsyslog`
from day one.

## Vendor pin

Append to `internal/lab/vendor.go` `vendorRepos`:

```go
{
    URL:  "https://github.com/hilather/go-lab-syslog",
    Dest: "third_party/go-lab-syslog",
    Ref:  "v1.0.0-rc.1", // first tagged RC
},
```

`mcplab vendor` clones `--depth 1 --branch Ref`. Do not edit
`third_party/go-lab-syslog` in place. Patches live in
`patches/go-lab-syslog-*.patch` and must be sent upstream.

## Compose fragment

Copy LabNTP, not LabLDAP (no overlay project).

```yaml
labsyslog:
  image: labsyslog:local
  build: ./third_party/go-lab-syslog
  container_name: mcplab-labsyslog
  command:
    - serve
    - --config=/etc/labsyslog/config.yaml
    - --management-listen=:8088
  user: "65532:65532"
  read_only: true
  tmpfs: ["/tmp"]
  cap_drop: [ALL]
  security_opt: ["no-new-privileges:true"]
  restart: unless-stopped
  ports:
    - "${LABSYSLOG_SYSLOG_PORT:-10514}:514/udp"
    - "${LABSYSLOG_SYSLOG_TCP_PORT:-10514}:514/tcp"
    - "${LABSYSLOG_REST_PORT:-18514}:8088/tcp"
  volumes:
    - ${MCPLAB_PROFILE_DIR:-./profiles/default}/labsyslog/bootstrap.yaml:/etc/labsyslog/config.yaml:ro
    - ./secrets/labsyslog-token:/run/secrets/labsyslog-token:ro
  healthcheck:
    test: ["CMD", "/labsyslog", "healthcheck", "--url=http://127.0.0.1:8088/v1/health/ready"]
    interval: 5s
    timeout: 3s
    retries: 12
    start_period: 3s
```

`cap_add: [NET_BIND_SERVICE]` is added **only** when the profile
publishes host 514. The default residual 10514 does not need it.
The appliance's own `examples/compose.smoke.yaml` binds `:1514` and
never adds `NET_BIND_SERVICE`.

Same Docker network as the rest of `mcplab` (`mcplab-shared`,
`LAB_DOCKER_SUBNET` default `10.99.42.0/24`).

## Profile variables

Add to `profiles/default/profile.env`:

```
LABSYSLOG_SYSLOG_PORT=10514
LABSYSLOG_SYSLOG_TCP_PORT=10514
LABSYSLOG_REST_PORT=18514
```

IANA dest is 514/udp and 514/tcp. Residual 10514 is the default
profile escape because rsyslog, syslog-ng, and journald-remote
commonly hold 514. Policy is `AGENTS.md` rule 15: native dest on the
host; residual is not a second design.

Operator escape to native:

```
LABSYSLOG_SYSLOG_PORT=514
LABSYSLOG_SYSLOG_TCP_PORT=514
```

Preflight (`internal/lab/ports.go`) must occupancy-check both UDP
bound and TCP LISTEN. `EACCES` / `EPERM` is **not** occupied
(privileged ports on GH-hosted runners). Fail closed if a non-lab
process holds the port. Error text must name the fix: stop rsyslog /
syslog-ng / journald-remote, bind an extra IP on `LAB_PUBLIC_HOST`,
or keep the 10514 escape (SUTs that hardcode dest 514 cannot follow
the escape).

Also add `LABSYSLOG_SYSLOG_PORT`, `LABSYSLOG_SYSLOG_TCP_PORT`, and
`LABSYSLOG_REST_PORT` to the labinfo compose `environment:` block so
`${VAR}` expansion sees them.

## Bootstrap YAML (profile-owned)

`profiles/default/labsyslog/bootstrap.yaml` is lab-owned desired
state. Appliance examples ship a copy under
`examples/labinfo/`-adjacent `examples/labsyslog.yaml`.

```yaml
apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab-sink
spec:
  listeners:
    udp:
      address: ":514"
      enabled: true
    tcp:
      address: ":514"
      enabled: true
      framing: auto
    tls:
      enabled: false
    management:
      address: ":8088"
      restPath: /v1
      mcpPath: /mcp
  auth:
    mode: bearer
    tokens:
      - id: admin
        role: administrator
        secretFile: /run/secrets/labsyslog-token
  ui:
    enabled: true
  management:
    allowedOrigins: []
    mcp:
      allowLegacyClients: true
    bodyLimit: 1MiB
    requestsPerSecond: 32
    burst: 64
    maxConcurrent: 256
  syslog:
    parse:
      rfc3164: true
      rfc5424: true
      bestEffort: true
    hostname: labsyslog.lab
    maxMessageBytes: 64KiB
    udpMaxDatagramBytes: 64KiB
    tcpIdleTimeout: 2m
    behavior:
      mode: accept
  store:
    maxMessages: 10000
    maxBytes: 256MiB
    fullPolicy: evict_oldest
    maxWait: 60s
    rawRetain: true
  admission:
    allowClientCidrs:
      - "10.99.42.0/24"
      - "127.0.0.0/8"
      - "::1/128"
    maxDatagramsPerSec: 20000
    maxDatagramsPerIP: 2000
    maxTcpConns: 256
    maxTcpConnsPerIP: 16
  filters: []
```

`allowLegacyClients: true` is required for MCPJungle 0.4.6.
`allowedOrigins: []` is deny-all for browser JS (loopback already
allowed by the appliance). Do not put `"*"` in appliance YAML.

`listeners.tls.enabled` must be `false`. `true` fail-closes in 1.0.

## MCPJungle registration

`profiles/default/mcpjungle/servers/labsyslog.json`:

```json
{
  "name": "labsyslog",
  "transport": "streamable_http",
  "description": "Receive-only syslog sink (LabSyslog): RFC 3164 + RFC 5424 over UDP/TCP, wait/list/export, plan/apply/reset over REST /v1 and MCP /mcp.",
  "url": "http://labsyslog:8088/mcp",
  "bearer_token": "${LABSYSLOG_TOKEN}"
}
```

Filename must equal `name`. Registrar discovers `servers/*.json`.
Add the server to the curated tool group
`profiles/default/mcpjungle/groups/` so `make smoke` sees `syslog_*`.

## labinfo catalog

New block in `profiles/default/labinfo/services.yaml`. labinfo
**fails to start** without a `connection` block.

```yaml
- id: labsyslog
  name: Syslog sink (LabSyslog, receive-only)
  description: Receive-only syslog sink. RFC 3164 and RFC 5424 over UDP and TCP. REST /v1, MCP /mcp, operator UI. Never forwards.
  urls:
    - name: Web UI
      url: http://${LAB_PUBLIC_HOST}:${LABSYSLOG_REST_PORT}/
    - name: REST API (native /v1)
      url: http://${LAB_PUBLIC_HOST}:${LABSYSLOG_REST_PORT}/v1
    - name: MCP endpoint
      url: http://${LAB_PUBLIC_HOST}:${LABSYSLOG_REST_PORT}/mcp
  note: "Syslog ingest (no auth, no TLS in 1.0): UDP and TCP ${LAB_PUBLIC_HOST}:${LABSYSLOG_SYSLOG_PORT}. Point systems under test at it as their remote syslog destination. Nothing is forwarded."
  credential:
    file: /run/lab-secrets/labsyslog-token
    usage: "HTTP header 'Authorization: Bearer <token>' for native /v1 and MCP; on the lab host: secrets/labsyslog-token"
  connection:
    endpoints:
      - name: Syslog ingest (UDP)
        protocol: syslog-udp
        address: ${LAB_PUBLIC_HOST}:${LABSYSLOG_SYSLOG_PORT}
        note: RFC 3164 or RFC 5424; one datagram is one message; no framing
      - name: Syslog ingest (TCP)
        protocol: syslog-tcp
        address: ${LAB_PUBLIC_HOST}:${LABSYSLOG_SYSLOG_TCP_PORT}
        note: RFC 6587 framing auto (octet-counting if the first bytes are DIGIT+SP, else non-transparent NL)
      - name: MCP (streamable HTTP)
        protocol: mcp-streamable-http
        address: http://${LAB_PUBLIC_HOST}:${LABSYSLOG_REST_PORT}/mcp
        note: bearer only; gateway interpolates LABSYSLOG_TOKEN
    parameters:
      auth: "none on the data plane"
      tls: "not available in 1.0; spec.listeners.tls.enabled must be false"
      formats: "rfc3164, rfc5424, best-effort raw store on parse failure"
    credentials:
      - name: labsyslog-token
        file: /run/lab-secrets/labsyslog-token
        usage: "HTTP header 'Authorization: Bearer <token>' for native /v1 and MCP"
```

## Secrets

`internal/lab/secrets.go`:

- Mint `secrets/labsyslog-token` if missing. Length ≥32 bytes
  (`auth.MinTokenBytes`). Mode `0o644` so UID 65532 can read the
  bind-mount.
- Add the token to `stageLabinfoCreds`.
- `LAB_DEV_MODE=true` reconciles the token from
  `profiles/<name>/dev-credentials.yaml` (no merge with `default`;
  fail-closed if the catalog key is missing).
- `make creds` prints the token in dev mode only.
- Leaving dev mode remints orchestrator tokens.

## Make / CLI touchpoints

- `APP` list: add `labsyslog`. `make reload APP=labsyslog` rebuilds
  and recreates only that container.
- `make smoke`: send one RFC 5424 UDP datagram and one RFC 6587
  octet-counted TCP frame to `$LAB_PUBLIC_HOST:$LABSYSLOG_SYSLOG_PORT`,
  then call `syslog_messages_wait` through the gateway.
- `make register` already picks up `servers/*.json`.
- Port preflight already iterates published host ports; add the new
  ones to the inventory.

Suggested smoke senders (integrator test container, not product):

```bash
# UDP RFC 5424
printf '<14>1 2026-09-04T20:00:00.000Z sut.lab testproc 1 MSGID - hello-udp\n' \
  | nc -u -w1 "$LAB_PUBLIC_HOST" "$LABSYSLOG_SYSLOG_PORT"

# TCP RFC 6587 octet-counting
msg='<14>1 2026-09-04T20:00:00.000Z sut.lab testproc 2 MSGID - hello-tcp'
printf '%d %s' "${#msg}" "$msg" | nc -w1 "$LAB_PUBLIC_HOST" "$LABSYSLOG_SYSLOG_TCP_PORT"
```

## Documentation sweep in the integrator (same change)

Per integrator `AGENTS.md` rule 12:

- `README.md` service table + mermaid + project list
- `docs/architecture.md`
- `docs/guides/configuration.md`
- `docs/guides/quickstart.md`
- `AGENTS.md` rule 15 (syslog 514 native / 10514 residual; 6514 is 1.1)
- `CHANGELOG.md` `[Unreleased]`
- Pages: `docs/services.html`, `docs/architecture.html`, `docs/configure.html`
- Known quirks: rsyslog/journald occupancy; Docker userland-proxy
  collapses UDP source IPs so per-IP admission is best-effort on
  host-publish (same class as LabNTP ADR 0014 — document, do not add
  a Go `userland-proxy` probe)

## What must not happen

- No codec, store, or MCP product logic in `cmd/mcplab` / `internal/lab`.
- No compose service name `syslog` or labinfo id `syslog`.
- No host port `514` as the shipped default profile value.
- No `MAILDEV_*` style leftover names.
- No TLS listener enabled in the default bootstrap.
- No patch against go-lab-syslog for Jungle compatibility — the
  profile sets `allowLegacyClients: true`.
