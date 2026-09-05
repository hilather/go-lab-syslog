# Program board — LabSyslog 1.0

Status: UDP-001 and STORE-001 done; next is TCP-001  
Status: TCP-001 done; next is FIL-001 (STORE-001 is parallel after WIRE-001)  
Status: FIL-001 done; next is STA-001 / APP-001  
Status: STA-001 / APP-001 done; next is API-001  
Status: API-001 done; next is SEC-001  
Status: SEC-001 done; next is MCP-001  
Status: OBS-001 done; next is SEC-001 (M2) then DEP-001 (M3)  
Status: MCP-001 done; next is OBS-001  
Status: SEC-001, OBS-001, and DEP-001 done; next is MCP-001, UI-001, SWAP-001  
Status: UI-001 done on this branch; MCP-001 / OBS-001 / DEP-001 remain  
Last reviewed: 2026-09-04  
Source of truth: the numbered `docs/` pack and accepted ADRs (0001–0012).

Agents implement one work package per change. Do not start a package
whose dependencies are incomplete. Control-plane order is
CFG → STA → API → SEC → MCP. Data plane proceeds after WIRE.

## Work packages

| Order | Task | ID | Depends on | Primary output | Milestone | Status |
| ---: | --- | --- | --- | --- | --- | --- |
| 1 | Repository foundation | FND-001 | — | Go module, CI, Makefile, stub CLI, docs skeleton | M0 | done |
| 2 | Domain + fail-closed YAML | CFG-001 | FND-001 | `labsyslog.dev/v1alpha1`, KnownFields, reserved-key reject, revisions | M0 | done |
| 3 | First-party syslogwire codec | WIRE-001 | CFG-001 | RFC 3164 + RFC 5424 parse/serialize, PRI, SD, testdata/packets | M1 | done |
| 4 | UDP sink RFC 5426 | UDP-001 | WIRE-001 | `internal/syslogserver` UDP, one datagram = one message | M1 | done |
| 5 | TCP sink RFC 6587 | TCP-001 | WIRE-001 | Octet-counting + non-transparent NL, `framing: auto` | M1 | not-started |
| 6 | Bounded message store | STORE-001 | WIRE-001 | ULID inbox, caps, wait, wipe, generation | M1 | done |
| 5 | TCP sink RFC 6587 | TCP-001 | UDP-001 | Octet-counting + non-transparent, `framing: auto` | M1 | done |
| 6 | Bounded message store | STORE-001 | WIRE-001 | ULID inbox, caps, wait, wipe, generation | M1 | not-started |
| 7 | Admission + filters | FIL-001 | CFG-001, UDP-001 | CIDR admission, first-match classify/drop, unmatched = capture | M1 | not-started |
| 7 | Admission + filters | FIL-001 | CFG-001, UDP-001 | CIDR admission, first-match classify/drop, unmatched = capture | M1 | done |
| 8 | Snapshot plan/apply/reset | STA-001 / APP-001 | CFG-001, STORE-001 | `app.Service`, live vs reset-only, expectedRevision | M2 | done |
| 9 | REST `/v1` + OpenAPI | API-001 | STA-001 | problem+json, wait, export, OpenAPI | M2 | done |
| 10 | Auth bearer + CSRF | SEC-001 | API-001 | Token ≥32, cookie, CSRF, audit ring | M2 | done |
| 11 | MCP Streamable HTTP + parity | MCP-001 | API-001, SEC-001 | `syslog_*`, `labsyslog://`, `make test-parity` | M2 | not-started |
| 12 | Observability | OBS-001 | UDP-001, API-001 | slog JSON, hand-rolled OpenMetrics, ready semantics | M3 | done |
| 11 | MCP Streamable HTTP + parity | MCP-001 | API-001, SEC-001 | `syslog_*`, `labsyslog://`, `make test-parity` | M2 | done |
| 12 | Observability | OBS-001 | UDP-001, API-001 | slog JSON, hand-rolled OpenMetrics, ready semantics | M3 | not-started |
| 13 | CLI + scratch image | DEP-001 | UDP-001, TCP-001, API-001, OBS-001 | Hardened image, compose.smoke, healthcheck | M3 | not-started |
| 13 | CLI + scratch image | DEP-001 | UDP-001, TCP-001, API-001, OBS-001 | Hardened image, compose.smoke, healthcheck | M3 | done |
| 14 | Operator SPA | UI-001 | API-001, SEC-001 | Embedded inbox, no localStorage tokens | M4 | not-started |
| 14 | Operator SPA | UI-001 | API-001, SEC-001 | Embedded inbox, no localStorage tokens | M4 | done |
| 15 | Integration-lab BOM | SWAP-001 | MCP-001, SEC-001, DEP-001 | examples for vendor/compose/labinfo/jungle | M4 | not-started |
| 16 | GA hardening | GA-001 | 1–15 | Fuzz, soak, release notes, known limitations | M5 | not-started |
| — | RFC 5425 TLS listener | TLS-001 | DEP-001 | v1.1 only; 1.0 rejects `tls.enabled: true` | v1.1 | not-started |

## Parallelization

- CFG-001 before WIRE-001.
- UDP-001, TCP-001, STORE-001 can proceed in parallel after WIRE-001.
- FIL-001 needs UDP-001 (source IP) and CFG-001 (schema).
- STA-001 needs CFG-001 + STORE-001.
- API-001 after STA-001.
- SEC-001 after API-001; MCP-001 after API-001 + SEC-001
  (family control-plane order REST → Auth → MCP).
- OBS-001 can start as soon as UDP + REST emit hooks.
- UI-001 must not block the ingest/API swap gate. 1.0 GA is
  incomplete without it (LabMail precedent).
- SWAP-001 is docs + examples in **this** repo. The integrator PR
  is out of band and must not land product logic.
- TLS-001 does not reopen GA-001.

## Milestones

### M0 — Contracts compile

- FND-001 and CFG-001 complete.
- ADRs 0001–0012 accepted.
- Schema and semantic fixtures exist under `testdata/config/`.
- CI runs format, lint, unit, docs.

### M1 — Ingest usable without control plane

- WIRE-001, UDP-001, TCP-001, STORE-001, FIL-001 complete.
- `logger` / `nc` against localhost UDP and TCP stores a message.
- Store is bounded and wipeable.
- `--management-listen=off` still accepts syslog.

### M2 — Agent-controllable

- STA-001, API-001, SEC-001, MCP-001 complete.
- Plan/apply/export/reset and `syslog_messages_wait` work on both
  transports.
- Parity tests green.

### M3 — Deployable RC

- OBS-001 and DEP-001 complete.
- Hardened image, HTTP ready healthcheck, compose.smoke.

### M4 — Release candidate with UI + BOM

- UI-001 and SWAP-001 complete.
- Documentation current.
- Integrator examples compile as fixtures even if the integrator
  PR has not landed.

### M5 — GA

- GA-001 acceptance review passes.
- All required CI green on the tag commit.
- Residual limitations match `docs/known-limitations.md`.

## Frozen product decisions

- Q1: labinfo id and compose service name are `labsyslog` from day one.
- Q2: 1.0 includes the operator UI; GA is not done without UI-001.
- Q3: TLS RFC 5425 is v1.1. Schema key exists; `enabled: true` rejected.
- Q4: Unmatched filter after admission = capture.
- Q5: TCP `framing: auto` tries octet-counting when the first bytes
  are `DIGIT+` then SP; otherwise non-transparent NL.
- Q6: UDP oversize > `udpMaxDatagramBytes` / `maxMessageBytes` is
  dropped with a metric; nothing stored. TCP may split frames.

## Cross-cutting blockers

Halt dependent work if any of these are unstable:

- Canonical IDs and names (binary, schema, MCP prefix, cookie, CSRF).
- Configuration schema source.
- Capability registry API and frozen tool names.
- Domain error shape (`urn:labsyslog:error:`).
- Store generation / epoch contract.
- Supported MCP protocol version (`2026-07-28`).
- Receive-only import boundary.
- Unmatched-filter policy (Q4).
- Framing auto algorithm (Q5).
