# GA-001: GA hardening

Status: done
Recommended owner: release agent
Dependencies: waves 1–15
Exclusive ownership: `docs/releases/`, `docs/known-limitations.md`, fuzz/soak

## Goal

Tag-ready 1.0 RC. Residual limitations match docs. No silent gaps.
Do **not** tag `v1.0.0` until Mira UI review is recorded.

## Scope

- [x] Fuzz syslogwire + framing with a real corpus
- [x] Soak: 10k UDP/s for 60s on test bind, store caps hold, no leak
- [x] Release notes template `docs/releases/v1.0.0-rc.1.md`
- [x] `docs/known-limitations.md` lists TLS/RELP/persistence/HA/OAuth
- [x] Import-boundary Dial AST is required CI
- [x] Forbidden module AST is required CI
- [x] Changelog complete
- [x] Mira review artifact exists under `docs/` (placeholder, not an
      approval; do not tag v1.0.0)

## Explicit non-scope

- TLS-001
- Integrator merge
- Tagging `v1.0.0`

## Acceptance criteria

- All required CI green on the tag SHA.
- Known limitations do not claim production-collector completeness.
