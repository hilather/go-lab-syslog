# UI-001: Operator SPA

Status: done
Recommended owner: UI agent
Dependencies: API-001, SEC-001
Exclusive ownership: `web/`, `internal/web`

## Goal

Embedded inbox SPA. Calls REST only. No localStorage tokens.

## Design references

- [x] `docs/12-web-ui.md`
- [x] LabMail inbox + LabNTP operator console

## Scope

- [x] Vite + TS
- [x] Message list with facility/severity chips, transport, host, app
- [x] Message detail: raw + parsed + SD
- [x] Wait/live tail via REST poll or SSE if API-001 added SSE
      (poll is enough for 1.0)
- [x] Filter table enable/disable
- [x] Reset button gated
- [x] Session login against REST
- [x] `go:embed` dist
- [x] `make web-build web-test`
- [x] Storage test forbids localStorage token keys

## Explicit non-scope

- Editing bootstrap YAML on disk
- TLS settings
- Forward/relay button (must not exist)

## Required tests

- [x] No relay control in the DOM
- [x] Token not written to localStorage
- [x] `ui.enabled: false` does not serve the SPA

## Acceptance criteria

- GA / 1.0 is incomplete without this wave.
- Mira review is required before v1.0.0; tracked in
  `docs/reviews/mira-ui-001.md` (placeholder, not an approval).
