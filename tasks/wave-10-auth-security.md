# SEC-001: Auth, CSRF, audit

Status: done
Recommended owner: security agent
Dependencies: API-001
Exclusive ownership: `internal/auth`, `internal/audit`, REST middleware

## Goal

Bearer tokens ≥32 bytes from files. SPA session cookie + CSRF.
Audit ring for management mutations only (no ingest).

## Design references

- [x] `docs/08-security-architecture.md`
- [x] ADR 0005 lab static bearer

## Scope

- [x] `spec.auth.mode: bearer` (required in 1.0)
- [x] Token SHA-256 compare constant time
- [x] Scopes: `syslog.read`, `syslog.write`, `syslog.admin`, `syslog.audit.read`
- [x] Cookie `labsyslog_session` HttpOnly SameSite=Lax Path=/ (Secure not required on loopback HTTP)
- [x] Header `X-LabSyslog-CSRF`
- [x] `allowedOrigins` exact match; no `"*"` sentinel in appliance
- [x] No `localStorage` tokens (UI-001 will lock this)
- [x] Audit ring: plan/apply/reset/delete/clear; wipe with store on reset

## Explicit non-scope

- OAuth PRM
- HTTP Basic (LabMail compat only; not here)
- MCP (bearer-only in MCP-001; `internal/auth.Verifier` is shared)

## Required tests

- [x] Short token rejected at validate when the file exists (CFG-001)
- [x] Missing bearer on `/v1/state` is 401
- [x] CSRF missing on cookie POST is 403
- [x] Audit entries exist after apply; reset wipes the audit ring with the store
- [x] Origin not on allowedOrigins → 403 `origin_not_allowed`

## Acceptance criteria

- Management is unusable without a valid token when auth.mode=bearer.
- Health live/ready stay unauthenticated. Metrics `publicPath` false is 404.
