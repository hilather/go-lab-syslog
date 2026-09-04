# 06 — REST API

Last reviewed: 2026-09-04

Base path `/v1`. JSON request/response. Errors
`application/problem+json`. OpenAPI at `api/openapi/v1.json`.

Unauthenticated: `GET /v1/health/live`, `GET /v1/health/ready`.
Everything else requires bearer, Basic (when mode allows), or a
valid session cookie.

## Endpoints

| Method | Path | Scope | Notes |
|---|---|---|---|
| GET | `/v1/health/live` | — | process up |
| GET | `/v1/health/ready` | — | listeners bound per snapshot + store init |
| GET | `/v1/version` | `syslog.read` | binary / module / revision |
| GET | `/v1/capabilities` | `syslog.read` | registry dump |
| GET | `/v1/status` | `syslog.read` | ready + listener bind state + store stats |
| GET | `/v1/schema/config` | `syslog.read` | JSON Schema URL/body |
| GET | `/v1/features` | `syslog.read` | `{tls:false, rfc3164:true, rfc5424:true, framing:["auto","octet-counting","non-transparent"]}` |
| GET | `/v1/state` | `syslog.read` | redacted spec, revision, generation, drifted |
| POST | `/v1/state:validate` | `syslog.admin` | body = candidate document |
| GET | `/v1/state:export` | `syslog.admin` | canonical YAML; `?format=json` |
| POST | `/v1/state:reset` | `syslog.admin` | reread bootstrap, wipe store |
| POST | `/v1/changes:plan` | `syslog.admin` | body operations + expectedRevision |
| POST | `/v1/changes:apply` | `syslog.admin` | `Idempotency-Key` required |
| GET | `/v1/messages` | `syslog.read` | query filters + `cursor` + `limit` (default 50, max 500) |
| GET | `/v1/messages/{id}` | `syslog.read` | parsed + metadata; raw omitted unless `?raw=true` |
| GET | `/v1/messages/{id}/raw` | `syslog.read` | `Content-Type: application/octet-stream` |
| DELETE | `/v1/messages/{id}` | `syslog.write` | 204 |
| POST | `/v1/messages:clear` | `syslog.write` | 204, bumps generation |
| POST | `/v1/messages:wait` | `syslog.read` | body `{timeout, filter}` |
| GET | `/v1/stats` | `syslog.read` | counters snapshot |
| GET | `/v1/audit` | `syslog.audit.read` | ring, newest first |
| GET | `/v1/audit/{id}` | `syslog.audit.read` | |
| GET | `/v1/events/stream` | `syslog.read` | SSE: `syslog.received`, `syslog.deleted`, `store.wiped`; heartbeat 15s |
| POST | `/v1/session` | bearer/basic | sets `labsyslog_session` |
| GET | `/v1/session` | cookie/bearer | |
| DELETE | `/v1/session` | cookie/bearer | |
| GET | `/v1/metrics` | `syslog.read` when publicPath | OpenMetrics text |

## List response

```json
{
  "revision": "sha256:…",
  "storeGeneration": 42,
  "items": [ { "id": "01…", "receivedAt": "…", "transport": "udp", "parsed": { … }, "truncated": false } ],
  "nextCursor": "…"
}
```

Cursor is opaque base64url + HMAC. Stale cursor → `400 cursor_stale`.

## Wait response

```json
{
  "matched": "existing" ,
  "message": { … }
}
```

`matched` is `existing` | `inserted`. Timeout → `504 wait_timeout`.
Wipe during wait → `409 store_wiped`.

## Ready semantics

`GET /v1/health/ready` is 200 only when:

- snapshot compiled
- store constructed
- every *enabled* data-plane listener is bound
- management is bound **or** was not requested

Ready does not require messages. Ready does not require UI assets
if `ui.enabled` is false.
