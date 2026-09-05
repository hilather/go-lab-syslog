# Changelog

## [Unreleased]

### Added

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
