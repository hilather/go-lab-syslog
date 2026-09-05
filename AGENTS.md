# Agent guide — LabSyslog

This repo is a receive-only syslog lab appliance. It is a sibling of
LabMail and LabNTP and is destined to be vendored by
`mcp-integration-lab`. These are the rules we work by.

Read before modifying code:

1. `docs/00-family-evaluation.md`
2. `docs/01-architecture.md`
3. `docs/02-syslog-semantics.md`
4. `docs/03-message-store.md`
5. `docs/04-state-and-configuration.md`
6. `docs/05-control-plane-and-parity.md`
7. `docs/08-security-architecture.md`
8. `docs/10-testing-strategy.md`
9. Every ADR relevant to the changed area.

The numbered docs are the source of truth after FND-001. Do not invent
paths, types, regexes, validation rules, or capability IDs. Changing
invariants requires an ADR first.

## Frozen product identity

| Field | Value |
|---|---|
| Product | LabSyslog |
| Binary | `labsyslog` |
| Module | `github.com/hilather/go-lab-syslog` |
| Image | `ghcr.io/hilather/labsyslog` |
| Schema | `labsyslog.dev/v1alpha1` |
| Kind | `LabSyslog` |
| Cookie | `labsyslog_session` |
| CSRF header | `X-LabSyslog-CSRF` |
| Container user | `65532:65532` |
| Config mount | `/etc/labsyslog/config.yaml` |
| Token mount | `/run/secrets/labsyslog-token` |
| labinfo id | `labsyslog` |
| MCP tools | `syslog_*` |
| MCP resources | `labsyslog://…` |
| MCP protocol | `2026-07-28` |
| Go | 1.26 |
| MCP SDK | `github.com/modelcontextprotocol/go-sdk v1.7.0` |
| License | Apache-2.0 |

## Ground rules

1. **Two planes, one process.** Data plane (UDP/TCP syslog) keeps
   accepting if management is unbound or slow. `internal/syslogwire`,
   `internal/syslogframing`, `internal/syslogserver`, and `internal/store`
   MUST NOT import `internal/control`, `internal/web`, or `net/http`.
2. **Receive-only is structural.** Production packages
   `internal/syslogserver`, `internal/store`, `internal/app`,
   `internal/syslogwire`, `internal/syslogframing` MUST NOT call
   `net.Dial`, `net.DialTimeout`, or `net.Dialer.Dial`. Listen/Accept
   only. No config type for a remote destination. Import-boundary tests
   fail the build on a Dial identifier.
3. **Never rewrite bootstrap YAML.** Desired state is the mounted file.
   Reset rereads bootstrap and wipes the store. The process does not
   write the config file.
4. **KnownFields(true).** Unknown fields reject. YAML wire names are
   camelCase (LabMail dialect). kebab-case aliases (`max-message-bytes`)
   reject. Durations as strings (`"2m"`). Byte sizes as binary units
   (`64KiB`). See docs/04 for the frozen map.
5. **Secrets are file refs only.** Never inline tokens. Tokens ≥32 bytes
   (`auth.MinTokenBytes`). Fail-closed if the file is missing or short.
6. **REST and MCP are adapters.** They must not contain independent
   business logic and must not call each other. One `internal/app.Service`.
   Shared capability registry. Control-plane implementation order is
   CFG → STA → API → SEC → MCP.
7. **First-party codec.** Do not import `log/syslog`,
   `gopkg.in/mcuadros/go-syslog`, `github.com/leodido/go-syslog`, or
   exec rsyslog / syslog-ng / journalctl. Types from those libraries
   must not leak into `internal/model`.
8. **TLS listener is v1.1.** Schema key `spec.listeners.tls` exists so
   documents do not have to change later. `enabled: true` is rejected in
   1.0 (same pattern as LabNTP `nts.enabled` and LabMail implicit SMTPS).
9. **No Prometheus client.** Metrics are hand-rolled OpenMetrics.
   `github.com/prometheus/*` is forbidden.
10. **Documentation ships with the change.** Stale docs are a defect.
    Update numbered docs, ADRs if invariants move, examples, schemas,
    program board status, and CHANGELOG `[Unreleased]` in the same change.
11. **CI failures get investigated and hardened, never retried away.**
12. **Unmatched filter = capture.** `admission.allowClientCidrs` is the
    ignore-outside gate. Filters classify or drop after parse. No
    matching filter means store the message. This is the opposite of
    LabNTP unmatched-drop because a sink's job is capture (ADR 0009).

## Reserved configuration keys

The loader rejects any key whose normalized name (strip `-` and `_`,
lower-case) matches:

`forward*`, `relay*`, `remote*`, `destination*`, `smarthost*`,
`outgoing*`, `output*`, `targethost*`, `remotehost*`, `omfwd*`, `rsyslog*`, `syslogng*`.

Keep that guard tested. Do not add a type for an outbound host.

## Package layout

```
cmd/labsyslog/                 CLI (version, help, validate, canonicalize, serve, healthcheck, mcp-stdio)
internal/model/                State, Spec, Message, Filter — no wire types
internal/config/               KnownFields decode, duration, bytesize, reserved-key reject
internal/compiler/             Normalize + Validate + compile Snapshot
internal/snapshot/             immutable Snapshot + atomic.Pointer store
internal/store/                bounded ring, wait, wipe, generation
internal/syslogwire/           RFC 3164 + RFC 5424 codec
internal/syslogframing/        RFC 6587 octet-counting + non-transparent
internal/syslogserver/         UDP + TCP listen, admission
internal/app/                  plan / apply / reset / wait
internal/capabilities/         frozen REST↔MCP table
internal/control/rest/         /v1 adapter (must not import internal/web)
internal/control/mcp/          /mcp adapter
internal/auth/                 bearer + cookie CSRF
internal/audit/                mutation ring
internal/domainerr/            catalog codes
internal/observability/        slog JSON, OpenMetrics
internal/buildinfo/            version
internal/web/                  go:embed operator SPA
internal/testutil/             clocks, temp dirs
internal/syslogtest/           test client (may Dial; production must not import it)
api/jsonschema/                labsyslog.dev.v1alpha1.json
api/openapi/                   v1.json
api/mcp/                       v1.json
api/capabilities/              v1.json
api/metrics/                   v1alpha1.json
api/errors/                    v1.json
web/                           Vite + React operator UI
testdata/config/{valid,invalid}/
testdata/packets/              RFC 3164 / 5424 fixtures
testdata/framing/              RFC 6587 fixtures
testdata/container/            compose smoke
examples/                      lab overlay, compose.smoke, labinfo, mcpjungle
docs/                          numbered pack + ADRs
tasks/                         program board + wave files
scripts/                       generate, tag-gate
```

## Allowed 1.0 direct dependencies

- `gopkg.in/yaml.v3`
- `github.com/modelcontextprotocol/go-sdk v1.7.0`
- `github.com/oklog/ulid/v2`

Prefer Go stdlib. New deps need PR justification and an Apache-2.0
license check. No HTTP frameworks, no syslog libraries, no Prometheus.

## Capability IDs (frozen)

REST_ONLY_PROTOCOL: `GET /v1/health/live`, `GET /v1/health/ready`,
session login/logout, metrics scrape, SPA static,
`GET /v1/events/stream`.

PARITY_REQUIRED (REST ↔ MCP):

| REST | MCP tool | Resource | Scope |
|---|---|---|---|
| `GET /v1/version` | `syslog_version_get` | | `syslog.read` |
| `GET /v1/capabilities` | `syslog_capabilities_get` | `labsyslog://capabilities` | `syslog.read` |
| `GET /v1/status` | `syslog_status_get` | `labsyslog://status` | `syslog.read` |
| `GET /v1/schema/config` | `syslog_schema_get` | `labsyslog://schema/config` | `syslog.read` |
| `GET /v1/features` | `syslog_features_list` | `labsyslog://features` | `syslog.read` |
| `GET /v1/state` | `syslog_state_get` | `labsyslog://state` | `syslog.read` |
| `POST /v1/state:validate` | `syslog_state_validate` | | `syslog.admin` |
| `GET /v1/state:export` | `syslog_state_export` | | `syslog.admin` |
| `POST /v1/state:reset` | `syslog_state_reset` | | `syslog.admin` |
| `POST /v1/changes:plan` | `syslog_change_plan` | | `syslog.admin` |
| `POST /v1/changes:apply` | `syslog_change_apply` | | `syslog.admin` |
| `GET /v1/messages` | `syslog_messages_list` | `labsyslog://messages` | `syslog.read` |
| `GET /v1/messages/{id}` | `syslog_message_get` | `labsyslog://messages/{id}` | `syslog.read` |
| `GET /v1/messages/{id}/raw` | `syslog_message_raw_get` | | `syslog.read` |
| `DELETE /v1/messages/{id}` | `syslog_message_delete` | | `syslog.write` |
| `POST /v1/messages:clear` | `syslog_messages_clear` | | `syslog.write` |
| `POST /v1/messages:wait` | `syslog_messages_wait` | | `syslog.read` |
| `GET /v1/stats` | `syslog_stats_get` | `labsyslog://stats` | `syslog.read` |
| `GET /v1/audit` | `syslog_audit_query` | `labsyslog://audit` | `syslog.audit.read` |
| `GET /v1/audit/{id}` | `syslog_audit_get` | | `syslog.audit.read` |

Do not invent a second name for any of these. Filters are mutated only
through `changes:plan` / `changes:apply` (coarse replace), not a
parallel CRUD surface in 1.0.

Scopes: `syslog.read`, `syslog.write`, `syslog.admin`, `syslog.audit.read`.

## Required completion commands

```
make format
make lint
make generate
make verify-generated
make test
make test-race
make test-fuzz-smoke
make test-parity
make test-config-compat
make test-docs
make test-container
make security-scan
make test-changelog
make web-test
make web-build
```

Missing targets must fail closed (`exit 1`), not act as no-ops.

## Tests that must stay locked

- KnownFields unknown-field reject
- reserved-key reject (forward/relay/remote/destination/…)
- `listeners.tls.enabled: true` reject in 1.0
- token file missing / short (<32 bytes) reject
- RFC 3164 and RFC 5424 golden packets
- RFC 6587 octet-counting and non-transparent framing
- oversize drop / truncate flag
- best-effort unparseable stored with `parseWarning`
- UDP independence: serve with management unbound still accepts
- no Dial identifiers in production packages
- first-match filter order
- unmatched filter = capture
- CIDR outside `allowClientCidrs` = silent ignore
- reset wipes store and reloads bootstrap
- REST/MCP parity for every PARITY_REQUIRED row
- ready is false until UDP (if enabled) is bound

## Waves

Take work from `tasks/00-program-board.md`. Each wave has a detailed
implementation note in `tasks/wave-*.md`. Parallel work is safe only
when package ownership and schema ownership do not overlap.
Integration changes to shared domain types, generated schemas, or the
capability registry must be serialized.

## Integrator

Product logic stays in this repo. `mcp-integration-lab` vendors a tag.
SWAP-001 ships examples and a contract document; it does not implement
the lab's `vendor.go` in this tree.
