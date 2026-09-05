# 10 — Testing strategy

Last reviewed: 2026-09-05

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
| Import fence | `internal/testutil/fence_test.go` + CI job `import-fence` | Dial AST; forbidden-module AST; no control import from data plane; required since FND-001 |
| REST contract | `internal/control/rest` | problem+json codes, wait, pagination, missing bearer 401, CSRF 403, origin_not_allowed |
| Observability | `internal/observability` | OpenMetrics parse; no prometheus import AST; ready false if UDP bind failed when enabled; `publicPath` false → `/v1/metrics` 404 even with auth |
| MCP + parity | `make test-parity` | every PARITY_REQUIRED row |
| Container | `make test-container` | bind `:1514`, cap_drop ALL, no NET_BIND_SERVICE, exec healthcheck, Bearer wait/reset |
| Security scan | `make security-scan` | `go vet` + `govulncheck` (tool, not a product module) |
| Docs | `make test-docs` | links; phrases `NAT collision`, `userland-proxy`, and `cap_add: [NET_BIND_SERVICE]` present in docs |
| Operator SPA | `web/` Vitest + `internal/web` | no relay/forward control; no localStorage tokens; `ui.enabled: false` does not serve HTML; `textContent` for message/raw; facility keywords match syslogwire; `make verify-web-dist` locks the committed embed |
| Fuzz-smoke | PRI, 5424 SD, RFC 6587 splitter (`make test-fuzz-smoke`) seeded from `testdata/corpus/{syslogwire,syslogframing}` | corpus non-empty |
| Soak | `internal/syslogserver` TestSoakUDPCapsHold | 10k UDP/s for 60s on test bind; store caps hold; `truncated` false; no goroutine leak (skipped `-short` and `-race`) |
| Race | wait + insert + wipe (`internal/store` TestRaceInsertWaitWipe) | |

## Make targets

Listed in AGENTS.md. Missing targets `exit 1`. CI jobs: format,
lint, unit, race, fuzz-smoke, docs, changelog, generated-file,
parity, container-test, security-scan, web, import-fence. `make test-fuzz-smoke` is a 5s
`FuzzParse` of `internal/syslogwire` plus a short `FuzzNext` of
`internal/syslogframing`, both seeded from `testdata/corpus/`. `make test-container` is
`scripts/test-container.sh`. `make security-scan` is `go vet` plus
`govulncheck` via `go run` (not a `go.mod` require). The `import-fence`
job runs Dial AST and forbidden-module AST tests in
`internal/testutil/fence_test.go` (required since PR 1).

## Transcripts

New syslog behavior requires a packet or framing golden plus a
`internal/syslogtest` session transcript. Do not assert against
live rsyslog.
