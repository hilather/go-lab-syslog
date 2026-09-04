# 12 — Web UI

Last reviewed: 2026-09-04

Embedded operator SPA at `/` when `spec.ui.enabled`. Source under
`web/` (Vite + React + TypeScript, Node **22.14.0**). Embedded via
`go:embed` in `internal/web`. `cmd/labsyslog` wires the handler.
Production `internal/control/rest` must not import `internal/web`.

## Required 1.0 pages

- Live tail / message list: facility, severity, app, host,
  transport, contains filters; generation badge
- Message detail: parsed fields + raw (monospace); no HTML render
- Status: revision, listeners, store caps, stats
- Filters table: view current snapshot filters (replace is admin
  via plan/apply, not a raw form posting YAML)
- Audit recent
- Gated reset and clear (type the resource name)

## Auth

Exchange bearer (or Basic if enabled) for cookie `labsyslog_session`.
Tokens never in `localStorage`. Mutations send `X-LabSyslog-CSRF`.
SSE `/v1/events/stream` for live tail.

## Security

Message `message` and `raw` are rendered as text (`textContent`).
No `innerHTML`, no `srcdoc`, no markdown. Origin policy from
docs/08.

## GA rule

UI is required for 1.0 GA (LabMail PR 12 precedent). rc.1 may ship
API-complete without UI; 1.0 does not.
