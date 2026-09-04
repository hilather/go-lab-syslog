# UI-001: Operator SPA

Status: not-started
Recommended owner: UI agent
Dependencies: API-001, SEC-001
Exclusive ownership: `web/`, `internal/web`

## Goal

Embedded inbox SPA. Calls REST only. No localStorage tokens.

## Design references

- [ ] `docs/12-web-ui.md`
- [ ] LabMail inbox + LabNTP operator console

## Scope

- [ ] Vite + TS
- [ ] Message list with facility/severity chips, transport, host, app
- [ ] Message detail: raw + parsed + SD
- [ ] Wait/live tail via REST poll or SSE if API-001 added SSE
      (poll is enough for 1.0)
- [ ] Filter table enable/disable
- [ ] Reset button gated
- [ ] Session login against REST
- [ ] `go:embed` dist
- [ ] `make web-build web-test`
- [ ] Storage test forbids localStorage token keys

## Explicit non-scope

- Editing bootstrap YAML on disk
- TLS settings
- Forward/relay button (must not exist)

## Required tests

- [ ] No relay control in the DOM
- [ ] Token not written to localStorage
- [ ] `ui.enabled: false` does not serve the SPA

## Acceptance criteria

- GA / 1.0 is incomplete without this wave.
