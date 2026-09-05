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

Exchange bearer for cookie `labsyslog_session`. LabSyslog does not
accept HTTP Basic.
Tokens never in `localStorage`. Mutations send `X-LabSyslog-CSRF`.
SSE `/v1/events/stream` for live tail.

## Security

Message `message` and `raw` are rendered as text (`textContent`).
No `innerHTML`, no `srcdoc`, no markdown. Origin policy from
docs/08.

## Build

`make web-test` runs Vitest in `web/`. `make web-build` emits
`web/dist` and copies it to `internal/web/dist` for `go:embed`.
`make verify-web-dist` fails if that embed is stale versus the
just-built tree (the scratch image has no Node stage). Node
**22.14.0**. `spec.ui.enabled: false` does not serve the SPA
(`GET /` is `404` problem+json). There is no relay or forward
control. Facility chips and the list filter use the docs/02
keywords (`console` / `cron2` at 14 / 15, not `alert` / `clock`).

## GA rule

UI is required for 1.0 GA (LabMail PR 12 precedent). rc.1 may ship
API-complete without UI; 1.0 does not.

Mira review after first UI implementation is required before
**v1.0.0** (GA-001). It is not a merge gate for UI-001. Track the
outcome in [reviews/mira-ui-001.md](reviews/mira-ui-001.md); that
file is a placeholder until the review lands.
