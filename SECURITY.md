# Security

LabSyslog is laboratory software. Still:

- Report issues against https://github.com/hilather/go-lab-syslog.
- Do not open inbound 514 on an untrusted network without
  understanding that the data plane is unauthenticated.
- Management tokens are ≥32 bytes file refs. Never commit them.
- The appliance never forwards syslog. A PR that adds Dial in
  `internal/syslogserver`, `internal/store`, or `internal/app` is
  a security defect.

No production SLA. No bounty program in 1.0.
