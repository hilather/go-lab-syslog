# Changelog

## [Unreleased]

### Fixed

- Health probes `GET /v1/health/live` and `GET /v1/health/ready` are
  exempt from management RPS and maxConcurrent so a parked wait or SSE
  client cannot 429 the DEP-001 healthcheck. Handler 404s keep their
  problem `detail` (a missing message is `"message not found"`, not
  `"no such route"`).

### Added

- REST `/v1` adapter and OpenAPI (API-001): `internal/control/rest` mounts
  on the serve process management listener. Frozen table routes plus
  REST_ONLY health, session stubs, metrics placeholder, and SSE
  `GET /v1/events/stream`. Errors are `application/problem+json` with
  `type: https://labsyslog.dev/errors/{code}`. Wait timeout is `504
  wait_timeout`; wipe during wait is `409 store_wiped`. Snapshot-backed
  `bodyLimit` is `413 payload_too_large`; RPS/burst/maxConcurrent is
  `429 rate_limited`. List cursors are process-local HMAC.
  `make generate` writes `api/openapi/v1.json`, `api/errors/v1.json`,
  and `api/capabilities/v1.json`. No auth middleware yet (SEC-001).
- Snapshot, plan, apply, and reset (STA-001 / APP-001): `app.Service`
  compiles an immutable `Snapshot` behind `atomic.Pointer`, with
  `Validate` / `Plan` / `Apply` / `Export` / `Reset` and no `net/http`.
  Live operations are exactly the docs/04 closed set
  (`replaceStoreCaps`, `replaceFilters`, `replaceAdmission`,
  `replaceSyslogParse` including `maxMessageBytes`, `replaceBehavior`,
  `replaceObservability` `logLevel` only). Unspecified fields are
  reset-only (`immutable_field`). Apply uses `expectedRevision` and
  Idempotency-Key. Reset rereads bootstrap, wipes store and audit, and
  never writes the file; listen flags overlay on serve and reset.
  `labsyslog serve` constructs `app.Service`, binds from the snapshot,
  and pushes live fields into FIL's Admission/Classifier/Behavior and
  store caps without replacing the insert Handler. Missing token files
  fail closed only when management is bound. Capability table seeded.
- Admission, filters, and M1 serve glue (FIL-001): CIDR `allowClientCidrs`
  (IPv4-mapped unmapped) and `maxDatagramsPerSec` / `maxDatagramsPerIP`
  rate caps are the first policy gate after size/framing. Miss is silent
  ignore (UDP) or close (TCP); nothing is stored. First-match
  `spec.filters[]` (`name` + `action.mode`/`tag`) run in
  `internal/syslogserver`; unmatched = capture (ADR 0009). `tag` writes
  `Message.Tags`. `spec.syslog.behavior.mode` is applied after admission
  and before parse. `labsyslog serve` installs `store.Store` as the
  ingest Handler so a localhost UDP 3164 and TCP 5424 datagram are
  stored with `--management-listen=off`. Spec-direct load is replaced
  by `app.Service` in STA-001.
- TCP sink (TCP-001): `internal/syslogframing` RFC 6587 splitter and
  `internal/syslogserver` Listen/Accept path. Frozen framing names are
  `auto`, `octet-counting`, and `non-transparent` (not `octet`/`newline`).
  `auto` is sticky per session (DIGIT+ SP → octet-counting, else
  non-transparent; later disagreement closes with no partial store).
  Complete SYSLOG-MSG frames (LF/NUL trailers stripped, optional CR before
  LF) enter the same ingest order as UDP. Octet MSG-LEN > `maxMessageBytes`
  closes the session; non-transparent oversize drops the frame.
  `tcpIdleTimeout` default `2m`. Session/per-IP caps are constants until
  FIL-001. `labsyslog serve --syslog-tcp-listen ADDR`. No TLS, no RELP,
  no Dial. Store-as-Handler wiring is FIL-001.
- UDP sink (UDP-001): `internal/syslogserver` RFC 5426 `ListenPacket`
  path and ingest pipeline (size/framing → admission → behavior → parse →
  classify → `Handler.Insert`) with stub allow-all admission, `accept`
  behavior, and capture-all classifier. Parse runs inside the pipeline via
  `syslogwire`. Oversize, empty, and (when `bestEffort` is false) unparseable
  datagrams are dropped with a metric and are not stored; `truncated` stays
  false. Drop reasons on `labsyslog_messages_dropped_total` include
  `empty` and `unparseable`. `labsyslog serve --config FILE
  --syslog-udp-listen ADDR --management-listen=off` binds UDP without a
  management plane. TCP listen and store-as-Handler wiring are later waves.
- Bounded ephemeral message store (STORE-001): `internal/store` ULID
  inbox with caps (`evict_oldest` / `reject`), list/wait AND filters,
  wait (existing/inserted/timeout/wipe), wipe, generation, and
  `rawRetain`. `github.com/oklog/ulid/v2` is the frozen ID generator
  (time-sortable; the Go standard library has no ULID). Insert reject
  does not close TCP (C18); the store returns `store_full` and the
  caller discards the frame. Wait helpers live in `internal/syslogtest`.
  management plane.
- First-party RFC 3164 and RFC 5424 codec (WIRE-001): `internal/syslogwire`
  parse/serialize into `model.Parsed` with `parseWarning` tokens
  (`missing_pri`, `utf8_bom`, `no_parser`, `rfc5424_disabled`,
  `unknown_facility`). Golden packets live under `testdata/packets/`.
  `make test-fuzz-smoke` runs a short `FuzzParse` of the parser.
- Domain model and fail-closed YAML (CFG-001): `labsyslog.dev/v1alpha1`
  types (`Document`/`Spec`/`Message`/`Parsed`), KnownFields decode,
  reserved-key reject, canonical YAML revision (`sha256:`), and
  `labsyslog validate` / `canonicalize`. JSON Schema is generated at
  `api/jsonschema/labsyslog.dev.v1alpha1.json`. `gopkg.in/yaml.v3` is
  the YAML decoder (the Go standard library has no `encoding/yaml`;
  this is the LabMail/LabNTP family dialect).
- Repository foundation (FND-001): Go module `github.com/hilather/go-lab-syslog`
  (Go 1.26), Apache-2.0 LICENSE, stub CLI (`labsyslog version` / `help`),
  Makefile, GitHub Actions CI, import-fence tests, and test helpers.
- Numbered design pack, ADRs, program board, and wave files promoted to
  repository root as the living source of truth.
- ADRs 0001–0012 accepted. ADR 0011 (no outbound forward) is Accepted;
  it is not reserved-unused.
- Family evaluation of every mcp-integration-lab member.
- Program board and wave implementation notes FND-001 through TLS-001.

`make generate`, `make verify-generated`, `make test-config-compat`, and
`make test-fuzz-smoke` are implemented. Remaining placeholder Make
targets fail closed (`exit 1`). Default CI runs format, lint, unit,
race, fuzz-smoke, docs, changelog, generated, and config-compat.
REST, MCP, UI, and the container image land in later waves.
