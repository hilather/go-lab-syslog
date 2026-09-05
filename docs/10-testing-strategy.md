# 10 — Testing strategy

Last reviewed: 2026-09-04

Every area, protocol behavior, capability, configuration semantic,
and bug fix must have automated coverage. Bug fixes start with a
test that fails before the fix.

## Required classes

| Class | Where | Locks |
|---|---|---|
| Config valid | `testdata/config/valid/` | defaults, lab overlay, maxed caps |
| Config invalid | `testdata/config/invalid/` | unknown field, reserved key, tls.enabled true, empty CIDRs, short token, both parsers false |
| Revision | `internal/compiler` | canonicalize stable, secret bytes absent |
| Wire | `testdata/packets/` | 3164, 5424, SD escapes, NILVALUE, missing PRI, BOM |
| Framing | `testdata/framing/` | octet-counting, NL, NUL trailer, auto heuristic, oversize, idle |
| Store | `internal/store` | evict order, oversized reject under evict_oldest, wait existing/inserted/timeout/wipe, generation, 10k inserts under maxBytes, insert+wait+wipe race |
| Admission / filters | `internal/syslogserver` | CIDR miss silent, first-match, unmatched capture |
| UDP sink | `internal/syslogserver` | dual-stack 127.0.0.1 and ::1; oversize/empty drop; `truncated` false; serve `--management-listen=off` |
| Import fence | `internal/testutil/fence_test.go` | no Dial in server/store/app/wire; no syslog libs; no control import from data plane |
| REST contract | `internal/control/rest` | problem+json codes, wait, pagination, missing bearer 401, CSRF 403, origin_not_allowed |
| MCP + parity | `make test-parity` | every PARITY_REQUIRED row |
| Container | `make test-container` | bind `:1514`, cap_drop ALL, healthcheck |
| Docs | `make test-docs` | links; phrases `NAT collision` and `userland-proxy` present in deploy docs |
| Fuzz-smoke | PRI, 5424 SD, RFC 6587 splitter (`make test-fuzz-smoke`) | |
| Race | wait + insert + wipe (`internal/store` TestRaceInsertWaitWipe) | |

## Make targets

Listed in AGENTS.md. Missing targets `exit 1`. CI jobs: format,
lint, unit, race, fuzz-smoke, docs, changelog, generated-file,
parity, container-test, web. `make test-fuzz-smoke` is a 5s
`FuzzParse` of `internal/syslogwire` plus a short `FuzzNext` of
`internal/syslogframing`.

## Transcripts

New syslog behavior requires a packet or framing golden plus a
`internal/syslogtest` session transcript. Do not assert against
live rsyslog.
