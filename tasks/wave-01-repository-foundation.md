# FND-001: Repository foundation

Status: done
Recommended owner: platform agent
Dependencies: none
Exclusive ownership: repo root, `.github/`, `cmd/labsyslog` stub, `Makefile`, `docs/` skeleton

## Goal

A clean checkout builds, tests, and fails closed on unimplemented
later targets. Architectural boundaries exist as packages with
comments and no feature code.

## Design references

- [x] `AGENTS.md` (create in this wave)
- [x] `docs/01-architecture.md`
- [x] ADR 0001 (use Go), ADR 0002 (in-tree receive-only)

## Scope

- [x] `go.mod` module `github.com/hilather/go-lab-syslog`, Go 1.26
- [x] Apache-2.0 `LICENSE`
- [x] `cmd/labsyslog` with `version` only; context-based shutdown wiring
- [x] Package dirs from `docs/01-architecture.md` with package comments
- [x] `internal/buildinfo` version / commit / build time
- [x] Makefile targets from `AGENTS.md`. Unimplemented targets `exit 1`
- [x] CI: format, lint, unit, race, fuzz-smoke, docs, changelog
- [x] `CHANGELOG.md` with `[Unreleased]`
- [x] `START-HERE.md`, `CONTRIBUTING.md`, `SECURITY.md`, `README.md`
- [x] Test helpers: fake clock, cleanup conventions (`internal/testutil`)
- [x] Script or test that required root documents exist

## Explicit non-scope

- Syslog codec
- Listeners
- REST / MCP / UI
- Dockerfile finalize (stub allowed; container test may be placeholder-fail)

## Required tests

- [x] `go test ./...` passes
- [x] No required Make target is a no-op success
- [x] Docs-link check runs
- [x] At least one concurrency test so `test-race` is real

## Acceptance criteria

- Repository builds on linux/amd64.
- Package import graph has no cycles.
- `labsyslog version` prints buildinfo.
- Feature code is not coupled to a syslog or MCP library.

Default GitHub Actions CI runs format, lint, unit, race, docs, and
changelog so the PR stays green. `make test-fuzz-smoke` (and the other
unimplemented AGENTS.md targets) still `exit 1` locally until the owning
wave flips them.
