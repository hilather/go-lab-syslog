# Examples

DEP-001 ships the `:1514` compose smoke. SWAP-001 ships the lab overlay,
labinfo, and MCPJungle BOM (`examples/labsyslog.yaml`,
`examples/labinfo/`, `examples/mcpjungle/`).

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

Product YAML binds `:514` and is SWAP-001.

## labsyslog.yaml (product + lab overlay)

See the document in [docs/01-architecture.md](../docs/01-architecture.md)
and the integrator copy in
[docs/13-integration-lab-swap.md](../docs/13-integration-lab-swap.md).
Shipped as a real file in SWAP-001.

Frozen shape:

- `apiVersion: labsyslog.dev/v1alpha1`
- `kind: LabSyslog`
- `spec.listeners.udp/tcp.address: ":514"`
- `spec.listeners.tls.enabled: false`
- `spec.auth.mode: bearer`
- `spec.management.mcp.allowLegacyClients: true` in the lab overlay
- `spec.admission.allowClientCidrs` includes `10.99.42.0/24`
