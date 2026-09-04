# Reviewer checklist

Use on every LabSyslog PR.

## Always

- [ ] Docs updated in the same change
- [ ] Tests fail before the fix / pass after for bugfix PRs
- [ ] No unimplemented Make target now exits 0
- [ ] Changelog `[Unreleased]` if user-visible
- [ ] No third-party syslog types leaked
- [ ] No `Dial` in production data-plane packages
- [ ] Capability IDs not invented
- [ ] YAML unknown fields still fail closed

## Data plane

- [ ] UDP still one datagram = one message
- [ ] TCP framing is only `auto|octet|newline`
- [ ] Oversize UDP stores nothing
- [ ] Unmatched filter still captures
- [ ] Allow-list miss still stores nothing

## Control plane

- [ ] Handler calls `app.Service` only
- [ ] REST and MCP not calling each other
- [ ] Scopes enforced
- [ ] Reset does not write bootstrap

## Container

- [ ] UID 65532
- [ ] `cap_drop: ALL` in appliance smoke
- [ ] Token file ref, ≥32 bytes
