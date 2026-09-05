# 08 — Security architecture

Last reviewed: 2026-09-04

LabSyslog is laboratory software. It still fail-closes on auth,
origin, secrets, and outbound reachability.

## Receive-only fence

Production packages `internal/syslogserver`, `internal/store`,
`internal/app`, `internal/syslogwire`, `internal/syslogframing` must
not contain the identifiers `Dial`, `DialTimeout`, `Dialer`,
`net.Dial`, or `http.NewRequest` (except app may construct in-process
test helpers behind `//go:build test` — do not). The import-boundary
test parses those packages and fails the build.

There is no spec field for a remote collector. Reserved keys listed
in AGENTS.md are rejected at decode.

## Auth

- Default and only 1.0 mode `bearer`. Token from `secretFile`, ≥32 bytes.
  Compare is SHA-256 digest + `crypto/subtle` (constant time).
- Roles: `administrator` → `syslog.read` `syslog.write` `syslog.admin`
  `syslog.audit.read`; `reader` → `syslog.read`.
- No HTTP Basic. No `dev-loopback-unauth`. Loopback still requires
  bearer or a session cookie.
- Auth lives at `spec.auth` (LabNTP shape). `spec.management.auth` is an unknown field and rejects.
- Cookie `labsyslog_session` is `HttpOnly`, `SameSite=Lax`,
  `Path=/`, no `Secure` required on loopback HTTP (lab). CSRF header
  `X-LabSyslog-CSRF` on cookie-authenticated mutating REST.
- Tokens are never written to `localStorage` by the SPA.
- Management bind with zero usable tokens fail-closes. Management
  REST is unusable without a valid token when `auth.mode=bearer`.

## Origin

`spec.management.allowedOrigins` is an exact list. Default `[]` =
loopback only. A non-empty list is exactly those origins; loopback
is not unioned. `originAllowlist` is an unknown field and rejects.
1.0 has no `"*"` / `"private"` sentinels. Non-allowed Origin on SPA
JS or mutating REST → `403 origin_not_allowed`. No CORS `*`
reflection. `OPTIONS` is 403. Audit ring (plan/apply/reset/delete/clear)
wipes with the store on reset.

## Container

UID 65532, read-only rootfs, `cap_drop: ALL`, `no-new-privileges`,
tmpfs `/tmp`. Integrator may add `NET_BIND_SERVICE` for host 514.
Secrets mounted `ro` at `0o644` so UID 65532 can read them (0o600
fails the bind-mount for non-root).

## Data plane

Syslog UDP/TCP is unauthenticated. That is the protocol. Admission
CIDRs are the only gate. Do not invent a syslog AUTH in 1.0.
Do not log raw message bodies at info; debug may, and must be
off by default.

## Threats we accept in 1.0

- Anyone inside `allowClientCidrs` can flood the store up to caps.
- Message content may contain credentials the SUT logged. The UI
  renders them as text, not as HTML (`textContent` only).
- Residual host port 10514 is reachable on all interfaces by family
  rule 5. Bearer protects management, not syslog ingest.
