# ADR 0005 — Lab static bearer

- Status: Accepted
- Date: 2026-09-04

## Context

Family appliances authenticate management with a static bearer file.
LabNTP is bearer-only. LabMail adds Basic because of `/email` compat.
LabSyslog has no compat surface that needs Basic.

## Decision

- Auth lives at `spec.auth` (LabNTP shape). `spec.management.auth` is
  an unknown field and rejects.
- `mode` is `bearer` only in 1.0.
- Tokens are file refs, ≥32 bytes, mounted `0o644`.
- Roles: `administrator` (all scopes) and `reader` (`syslog.read`).
- MCP is bearer-only.
- Cookie `labsyslog_session` + `X-LabSyslog-CSRF` is REST-only.
- `bearer_and_basic` and `dev-loopback-unauth` are unknown fields
  in 1.0. A later ADR may add them.
- Tokens never enter `localStorage`.
- Management bind with zero usable tokens fail-closes.

## Consequences

Integrator mints `secrets/labsyslog-token` and interpolates
`${LABSYSLOG_TOKEN}` for Jungle. Short tokens fail closed at validate.
