# SEC-001: Auth, CSRF, audit

Status: not-started
Recommended owner: security agent
Dependencies: API-001
Exclusive ownership: `internal/auth`, `internal/audit`, REST middleware

## Goal

Bearer tokens ≥32 bytes from files. SPA session cookie + CSRF.
Audit ring for mutations and (sampled) ingest.

## Design references

- [ ] `docs/08-security-architecture.md`
- [ ] ADR 0006 lab static bearer

## Scope

- [ ] `spec.auth.mode: bearer` (required in 1.0)
- [ ] Token SHA-256 compare constant time
- [ ] Scopes: `syslog.read`, `syslog.write`, `syslog.admin`, `syslog.audit.read`
- [ ] Cookie `labsyslog_session` HttpOnly Secure SameSite
- [ ] Header `X-LabSyslog-CSRF`
- [ ] `allowedOrigins` exact match; no `"*"` sentinel in appliance
- [ ] No `localStorage` tokens (UI-001 will lock this)
- [ ] Audit ring: apply/reset/delete/clear + optional ingest

## Explicit non-scope

- OAuth PRM
- HTTP Basic (LabMail compat only; not here)

## Required tests

- [ ] Short token rejected at compile
- [ ] Missing bearer on `/v1/state` is 401
- [ ] CSRF missing on cookie POST is 403
- [ ] Audit entries survive apply and are wiped on reset? **No** —
      reset policy: audit ring wipes with store (document)

## Acceptance criteria

- Management is unusable without a valid token when auth.mode=bearer.
