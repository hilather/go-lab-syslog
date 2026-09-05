# LabSyslog

**One sink. Every syslog datagram.**

Receive RFC 3164 and RFC 5424 from systems under test over UDP and TCP.
Index them. Expose them over REST, MCP, and an operator console.
Never forward. Never relay. Restart wipes the store.

This is laboratory software. It is not rsyslog, syslog-ng, or a production
log collector.

| | |
|---|---|
| Product | LabSyslog |
| Binary | `labsyslog` |
| Module | [`github.com/hilather/go-lab-syslog`](https://github.com/hilather/go-lab-syslog) |
| Image | `ghcr.io/hilather/labsyslog` |
| Config | `labsyslog.dev/v1alpha1` · kind `LabSyslog` |
| Data plane | UDP + TCP syslog · container `:514` · host publish **10514** (residual; IANA dest is **514**) |
| Control plane | REST `/v1` · MCP `/mcp` · operator UI `/` · host publish **18514** |
| License | Apache-2.0 |
| User | `65532:65532` |

**Implementation status.** FND-001, CFG-001, WIRE-001, UDP-001, and
STORE-001 are in tree: `labsyslog version`, `help`, `validate`,
`canonicalize`, and `serve --syslog-udp-listen ADDR --management-listen=off`
work. `internal/syslogwire` parses RFC 3164 / RFC 5424; `internal/syslogserver`
binds RFC 5426 UDP through a stub Handler (store wiring is FIL-001);
`internal/store` is the bounded ULID inbox (wait, wipe, generation).
Later waves land TCP, REST, MCP, UI, and the container image.
TCP-001 are in tree: `labsyslog version`, `help`, `validate`,
`canonicalize`, and `serve --syslog-udp-listen ADDR --syslog-tcp-listen
ADDR --management-listen=off` work. `internal/syslogwire` parses RFC 3164
/ RFC 5424; `internal/syslogframing` splits RFC 6587; `internal/syslogserver`
binds UDP and TCP through a stub Handler (store wiring is FIL-001).
Later waves land the store, REST, MCP, UI, and the container image.
**Implementation status.** FND-001, CFG-001, WIRE-001, UDP-001, TCP-001,
STORE-001, and FIL-001 are in tree: `labsyslog version`, `help`,
`validate`, `canonicalize`, and `serve --syslog-udp-listen ADDR
--syslog-tcp-listen ADDR --management-listen=off` work.
`internal/syslogwire` parses RFC 3164 / RFC 5424; `internal/syslogframing`
splits RFC 6587; `internal/syslogserver` admits, classifies, and inserts
into `internal/store`. Later waves land plan/apply, REST, MCP, UI, and
the container image.
Unimplemented CLI subcommands fail closed. Remaining placeholder Make
targets exit 1.

Documentation
· [Start here](START-HERE.md)
· [Architecture](docs/01-architecture.md)
· [Syslog semantics](docs/02-syslog-semantics.md)
· [Message store](docs/03-message-store.md)
· [State and configuration](docs/04-state-and-configuration.md)
· [REST](docs/06-rest-api.md)
· [MCP](docs/07-mcp-api.md)
· [Deploy](docs/11-deployment.md)
· [Integration-lab swap](docs/13-integration-lab-swap.md)
· [Program board](tasks/00-program-board.md)
· [Agent guide](AGENTS.md)

## Why this exists

Lab compose graphs already emit logs. Systems under test emit auth, kernel,
application, and appliance syslog. QA still needs a sink they can:

- Point a SUT at without standing up rsyslog or syslog-ng.
- Query from an agent (`syslog_messages_wait`) the same way LabMail waits for mail.
- Reset between scenarios without leftover messages from the last run.
- Drive from YAML, REST, or MCP.

LabSyslog belongs with LabMail (receive-only sink) and LabNTP (first-party
wire protocol). It is **not** a production collector, **not** a rsyslog
wrapper, and it never opens an outbound syslog session.

## Quick start

You need Go 1.26+. From a clone:

```bash
git clone https://github.com/hilather/go-lab-syslog.git
cd go-lab-syslog
go build -o bin/labsyslog ./cmd/labsyslog
./bin/labsyslog version
./bin/labsyslog validate --config testdata/config/valid/defaults.yaml
./bin/labsyslog serve --config testdata/config/valid/defaults.yaml \
  --syslog-udp-listen 127.0.0.1:10514 \
  --syslog-tcp-listen 127.0.0.1:10514 \
  --management-listen 127.0.0.1:8088
```

Send a probe:

```bash
logger -n 127.0.0.1 -P 10514 -T -p user.notice "lab-probe hello"
curl -sS -H "Authorization: Bearer $(cat /run/secrets/labsyslog-token)" \
  http://127.0.0.1:8088/v1/messages
```

`validate` and `canonicalize` load one fail-closed YAML document.
`serve` binds UDP/TCP syslog and management HTTP (`/v1`, `POST /mcp`, UI `/`).

Read the loaded snapshot with `GET /v1/state`.
Validate a candidate with `POST /v1/state:validate`.
Dry-run and commit mutations with `POST /v1/changes:plan` and `POST /v1/changes:apply`.
Reload the bootstrap file and wipe the store with `POST /v1/state:reset`.
Block until a matching message arrives with `POST /v1/messages:wait`.

## What you get

| Surface | Where |
|---|---|
| UDP syslog | `udp://127.0.0.1:10514` (residual; native dest 514) |
| TCP syslog | `tcp://127.0.0.1:10514` (RFC 6587 octet-counting + non-transparent) |
| UI | `http://127.0.0.1:18514/` |
| REST | `http://127.0.0.1:18514/v1` |
| MCP (HTTP) | `POST http://127.0.0.1:18514/mcp` |
| Health | `GET /v1/health/live` · ready: `GET /v1/health/ready` |

TLS syslog (RFC 5425, dest 6514) is **v1.1**. A 1.0 document with
`spec.listeners.tls.enabled: true` fail-closes at validate.

## Family

LabSyslog is a sibling of the appliances vendored by
[`mcp-integration-lab`](https://github.com/hilather/mcp-integration-lab):

| Appliance | What LabSyslog copies |
|---|---|
| LabMail | Receive-only invariant, bounded ephemeral store, wait API, reserved-key reject, inbox-style UI |
| LabNTP | First-party wire codec, data-plane independence, first-match filters, `NET_BIND_SERVICE`, residual high host port |
| LabDNS | Numbered docs pack, ADR discipline, program board, fail-closed KnownFields YAML |
| LabMITM / LabSSO | Bearer token files, `allowLegacyClients`, origin allowlist, operator SPA |

The evaluation of every lab member and the mapping onto this design lives in
[docs/00-family-evaluation.md](docs/00-family-evaluation.md).

## In / out of scope (1.0)

**In scope**

- RFC 3164 and RFC 5424 parse and store
- UDP (RFC 5426) and TCP (RFC 6587 octet-counting and non-transparent)
- Bounded ephemeral store with wait, list, get, raw, delete, clear
- YAML desired state with revisions, plan/apply/reset
- REST `/v1` and MCP `syslog_*` parity
- Operator console
- Hardened scratch image, compose contract, integration-lab swap docs

**Out of scope (1.0)**

- RFC 5425 TLS / dest 6514 (v1.1)
- RFC 3195 BEEP, RFC 5848 signed syslog, RELP, journald native
- Forwarding, relaying, remote destinations, output plugins
- Persistent store across restart
- Wrapping rsyslog, syslog-ng, or Vector
- Random/probabilistic drop engines

## Documentation

The numbered pack under `docs/` is the source of truth after FND-001.
Do not invent paths, types, capability IDs, or validation rules from a
task summary when a design document exists.

See [docs/README.md](docs/README.md) for the catalog.
See [tasks/00-program-board.md](tasks/00-program-board.md) for waves.
See [AGENTS.md](AGENTS.md) before changing code.
