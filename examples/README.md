# Examples

DEP-001 ships the `:1514` compose smoke. SWAP-001 ships the lab overlay,
labinfo, and MCPJungle BOM (`examples/labsyslog.yaml`,
`examples/labinfo/`, `examples/mcpjungle/`). SWAP-001 does not own
`examples/compose.smoke.yaml`. The integrator vendor pin is a follow-on
after the first `v*` tag.

## compose.smoke.yaml

Binds `127.0.0.1:1514` UDP+TCP and `127.0.0.1:18088:8088`.
YAML listeners use `:1514` (`testdata/container/config.yaml`).
`cap_drop: ALL`. No `NET_BIND_SERVICE`. `make test-container` mints
`testdata/container/token` (≥32 bytes, mode `0o644`, gitignored) and
runs `docker compose -f examples/compose.smoke.yaml up --build`
(fails closed without the compose plugin). Manual:

```bash
python3 -c 'import pathlib,secrets; pathlib.Path("testdata/container/token").write_text(secrets.token_urlsafe(48))'
chmod 644 testdata/container/token
docker compose -f examples/compose.smoke.yaml up --build
```

## labsyslog.yaml (product + lab overlay)

Lab overlay for mcp-integration-lab
(`profiles/default/labsyslog/bootstrap.yaml`). Product YAML binds
`:514`. Integrator compose maps residual host **10514** → container
**514** and adds `cap_add: [NET_BIND_SERVICE]` because the process
binds `:514` (C17 / ADR 0010). Contract:
[docs/13-integration-lab-swap.md](../docs/13-integration-lab-swap.md).

Frozen shape:

- `apiVersion: labsyslog.dev/v1alpha1`
- `kind: LabSyslog`
- `spec.listeners.udp/tcp.address: ":514"`
- `spec.listeners.tls.enabled: false`
- `spec.auth.mode: bearer`
- `spec.management.mcp.allowLegacyClients: true` in the lab overlay
- `spec.admission.allowClientCidrs` includes `10.99.42.0/24`

## labinfo/services-labsyslog.yaml

Catalog fragment. Integrator merges into
`profiles/default/labinfo/services.yaml`. Id is `labsyslog` from day
one (not `syslog`, not a `maildev`-style alias). A `connection` block
is required.

## mcpjungle/servers/labsyslog.json

MCPJungle 0.4.6 registration. Filename equals JSON `name`. URL is
`http://labsyslog:8088/mcp`. Bearer interpolates `${LABSYSLOG_TOKEN}`.
The integrator adds the server to the curated tool group so
`make smoke` can call `syslog_messages_wait`.
