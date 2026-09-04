# CFG-001: Domain model and fail-closed YAML

Status: done
Recommended owner: config agent
Dependencies: FND-001
Exclusive ownership: `internal/model`, `internal/config`, `internal/compiler` stub, `api/jsonschema`, `testdata/config`, `cmd/labsyslog` `validate` / `canonicalize`

## Goal

`labsyslog validate --config` and `canonicalize` work. Unknown
fields, reserved keys, and `tls.enabled: true` fail closed.
Revision hash is `sha256:` + lowercase hex of canonical JSON of the
spec with secret *paths*, not secret values.

## Design references

- [ ] `docs/04-state-and-configuration.md`
- [ ] ADR 0003 fail-closed YAML, ADR 0010 no-forward keys

## Scope

- [ ] Types: `Document`, `Spec`, `Listeners`, `Syslog`, `Store`, `Admission`, `Filter`, `Auth`, `Management`, `UI`
- [ ] YAML wire names frozen in `docs/04-state-and-configuration.md`
- [ ] Decode: YAML `KnownFields(true)`, JSON `DisallowUnknownFields`
- [ ] Reject multi-doc, empty, non-UTF-8, oversize (>1 MiB)
- [ ] Normalize defaults (see architecture)
- [ ] Validate: CIDRs, token secretFile present when token declared (existence not required at validate), `tls.enabled` must be false, `maxMessageBytes` bounds, `fullPolicy` enum, `framing` enum `auto|octet|newline`
- [ ] Reserved-key reject after normalizing (strip `-`, `_`, case). Prefixes: `forward`, `relay`, `remote`, `destination`, `smarthost`, `outgoing`, `output`, `targethost`, `omfwd`, `rsyslog`, `syslogng`
- [ ] `cmd/labsyslog validate` and `canonicalize`
- [ ] JSON Schema `api/jsonschema/labsyslog.dev.v1alpha1.json`

## Explicit non-scope

- Compile to live snapshot (STA-001)
- Listeners binding
- Secret file existence at serve-time (compiler, not validate)

## Required tests

- [ ] valid fixtures under `testdata/config/valid/`
- [ ] invalid: unknown field, kebab-case alias, `tls.enabled: true`, reserved keys, missing catch-all-not-required (admission CIDR ok empty-means-deny-all or explicit)
- [ ] revision stable across canonicalize
- [ ] secret values never enter the hash (paths only)

## Acceptance criteria

- A full valid document round-trips through canonicalize.
- Every reserved key fixture fails with a stable domain error code.
- `tls.enabled: true` fails with a documented code (`tls_not_in_1_0`).
