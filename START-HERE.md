# Start here

LabSyslog is a receive-only syslog lab appliance in the LabMail / LabNTP /
LabDNS family. Systems under test send RFC 3164 or RFC 5424 over UDP or
TCP. LabSyslog captures, indexes, and exposes every accepted message over
REST, MCP, and an embedded operator console. It never opens an outbound
syslog session and never forwards.

- **Run what exists today** — stay on this page, then follow the [README](README.md).
- **Change it** — read [AGENTS.md](AGENTS.md) before touching code.
- **Understand the family** — [docs/00-family-evaluation.md](docs/00-family-evaluation.md).

## Five-minute path

1. Install **Go 1.26** and clone this repository.
2. `go build -o bin/labsyslog ./cmd/labsyslog`
3. `./bin/labsyslog version`
4. `./bin/labsyslog help`
5. Write a `labsyslog.dev/v1alpha1` file (or use `testdata/config/valid/defaults.yaml`).
6. `./bin/labsyslog validate --config testdata/config/valid/defaults.yaml`
7. `./bin/labsyslog serve --config testdata/config/valid/defaults.yaml --syslog-udp-listen 127.0.0.1:10514 --syslog-tcp-listen 127.0.0.1:10514 --management-listen 127.0.0.1:8088`

`validate` and `canonicalize` load one fail-closed YAML document. `serve`
binds UDP and TCP syslog plus native `/v1` REST, the operator SPA at `/`,
and `POST /mcp`. Default lab auth is bearer. For a local browser session
without tokens is **not** supported in 1.0. Management bind requires a usable bearer file unless `--management-listen=off`.

Read the loaded snapshot with `GET /v1/state`. Validate a candidate with
`POST /v1/state:validate`. Dry-run and commit mutations with
`POST /v1/changes:plan` and `POST /v1/changes:apply`. Reload the bootstrap
file and wipe the store with `POST /v1/state:reset`. Block until a matching
message arrives with `POST /v1/messages:wait`.

`healthcheck` probes `GET /v1/health/ready`. `mcp-stdio` is the developer
stdio adapter (`--token-file` is verified). Session cookie
`labsyslog_session` + `X-LabSyslog-CSRF` is REST-only.

## What to read next

| If you are… | Read |
|---|---|
| Running a lab | [README.md](README.md), [docs/11-deployment.md](docs/11-deployment.md), [docs/13-integration-lab-swap.md](docs/13-integration-lab-swap.md) |
| Writing YAML or calling the state APIs | [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md) |
| Implementing the codec or listeners | [docs/02-syslog-semantics.md](docs/02-syslog-semantics.md), [docs/adr/0002-in-tree-syslog-receive-only.md](docs/adr/0002-in-tree-syslog-receive-only.md), [docs/adr/0007-rfc3164-and-5424-first-party.md](docs/adr/0007-rfc3164-and-5424-first-party.md) |
| Wiring an agent | [docs/05-control-plane-and-parity.md](docs/05-control-plane-and-parity.md), [docs/07-mcp-api.md](docs/07-mcp-api.md) |
| Integrating with mcp-integration-lab | [docs/13-integration-lab-swap.md](docs/13-integration-lab-swap.md) |
| Changing behavior | [AGENTS.md](AGENTS.md), then the design doc for that area |
| Taking a work package | [tasks/00-program-board.md](tasks/00-program-board.md) and the matching `tasks/wave-*.md` |

## For contributors and agents

Before changing code:

1. Read [AGENTS.md](AGENTS.md) completely.
2. Read architecture, syslog semantics, store, state, control-plane parity,
   security, and testing: docs `01`–`05`, `08`, `10`.
3. Read every ADR that affects the task (`docs/adr/`).
4. Take one work package from [tasks/00-program-board.md](tasks/00-program-board.md)
   whose dependencies are complete. Implement from the matching
   `tasks/wave-*.md` **and** the numbered design doc — never from the
   one-line board summary alone.
5. Add or update tests before declaring the task done.
6. Update every affected document in the same change.
7. Run every required local verification target listed in AGENTS.md.

Do not implement REST, MCP, syslog, configuration, or the store from a
task summary when a design document exists. The numbered pack is the
source of truth. If an invariant must change, write an ADR and update
the design documentation first.

### Definition of done

A task is not done until:

- The code matches the numbered design document for that area.
- Tests cover the new behavior and fail before the fix for bug work.
- Affected docs, examples, schemas, and the program board status are updated.
- Import-boundary, reserved-key, and KnownFields tests still pass.
- `make format lint test test-docs` is green locally.
