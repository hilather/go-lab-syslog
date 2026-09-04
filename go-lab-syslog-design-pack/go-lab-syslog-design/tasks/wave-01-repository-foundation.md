# FND-001: Repository foundation

Status: not-started
Recommended owner: platform agent
Dependencies: none
Exclusive ownership: repo root, `.github/`, `cmd/labsyslog` stub, `Makefile`, `docs/` skeleton

## Goal

A clean checkout builds, tests, and fails closed on unimplemented
later targets. Architectural boundaries exist as packages with
comments and no feature code.

## Design references

- [ ] `AGENTS.md` (create in this wave)
- [ ] `docs/01-architecture.md`
- [ ] ADR 0001 (use Go), ADR 0002 (in-tree receive-only)

## Scope

- [ ] `go.mod` module `github.com/hilather/go-lab-syslog`, Go 1.26
- [ ] Apache-2.0 `LICENSE`
- [ ] `cmd/labsyslog` with `version` only; context-based shutdown wiring
- [ ] Package dirs from `docs/01-architecture.md` with package comments
- [ ] `internal/buildinfo` version / commit / build time
- [ ] Makefile targets from `AGENTS.md`. Unimplemented targets `exit 1`
- [ ] CI: format, lint, unit, race, fuzz-smoke, docs, changelog
- [ ] `CHANGELOG.md` with `[Unreleased]`
- [ ] `START-HERE.md`, `CONTRIBUTING.md`, `SECURITY.md`, `README.md`
- [ ] Test helpers: fake clock, cleanup conventions (`internal/testutil`)
- [ ] Script or test that required root documents exist

## Explicit non-scope

- Syslog codec
- Listeners
- REST / MCP / UI
- Dockerfile finalize (stub allowed; container test may be placeholder-fail)

## Required tests

- [ ] `go test ./...` passes
- [ ] No required Make target is a no-op success
- [ ] Docs-link check runs
- [ ] At least one concurrency test so `test-race` is real

## Acceptance criteria

- Repository builds on linux/amd64.
- Package import graph has no cycles.
- `labsyslog version` prints buildinfo.
- Feature code is not coupled to a syslog or MCP library.
