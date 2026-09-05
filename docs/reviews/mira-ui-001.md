# Mira review — UI-001 Operator SPA

Status: **required, not started**

Last reviewed: 2026-09-04

Mira review of the operator SPA is required before tagging **v1.0.0**
(GA-001). It is **not** a merge gate for UI-001 and is **not** required
to start or initially merge this PR.

This file is a placeholder. Do not treat it as an approval.

Review must confirm, at minimum:

- Message `message` and `raw` are rendered as text (`textContent`), never
  `innerHTML` / `srcdoc` / markdown.
- Tokens are never written to `localStorage` or `sessionStorage`.
- There is no relay or forward control in the DOM.
- Cookie session + `X-LabSyslog-CSRF` on mutations.
- `spec.ui.enabled: false` does not serve the SPA.

Write the outcome here before GA-001 tags.
