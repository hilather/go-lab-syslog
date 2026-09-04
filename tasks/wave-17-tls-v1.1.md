# TLS-001: RFC 5425 TLS listener (v1.1)

Status: deferred
Recommended owner: protocol agent
Dependencies: DEP-001
Exclusive ownership: TLS path in `internal/syslogserver`

## Goal

Not in 1.0. Schema key `spec.listeners.tls` exists so 1.0 does not
have to break the document later. `enabled: true` is rejected until
this wave.

## Scope (when opened)

- [ ] Container `:6514`, host residual `16514`, IANA 6514
- [ ] File-ref cert/key/CA
- [ ] Client-auth optional
- [ ] Same framing as TCP
- [ ] labinfo connection endpoint added in a later integrator pin

## Explicit non-scope for 1.0

- Everything in this file

## Acceptance criteria for 1.0

- CFG-001 fixtures prove `enabled: true` fails closed.
