# GA-001: GA hardening

Status: not-started
Recommended owner: release agent
Dependencies: waves 1–15
Exclusive ownership: `docs/releases/`, `docs/known-limitations.md`, fuzz/soak

## Goal

Tag-ready 1.0 RC. Residual limitations match docs. No silent gaps.

## Scope

- [ ] Fuzz syslogwire + framing with a real corpus
- [ ] Soak: 10k UDP/s for 60s on test bind, store caps hold, no leak
- [ ] Release notes template `docs/releases/v1.0.0-rc.1.md`
- [ ] `docs/known-limitations.md` lists TLS/RELP/persistence/HA/OAuth
- [ ] Import-boundary Dial AST is required CI
- [ ] Forbidden module AST is required CI
- [ ] Changelog complete

## Explicit non-scope

- TLS-001
- Integrator merge

## Acceptance criteria

- All required CI green on the tag SHA.
- Known limitations do not claim production-collector completeness.
