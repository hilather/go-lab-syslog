# 02 — Syslog semantics

Last reviewed: 2026-09-04

Normative receive behavior for LabSyslog 1.0. Agents implement
`internal/syslogwire` and `internal/syslogframing` from this document
and the golden files under `testdata/packets/` and `testdata/framing/`.

## Transports

| Transport | RFC | Unit | 1.0 |
|---|---|---|---|
| UDP | 5426 (5424 over UDP), 3164 traditional | one datagram = one message | yes |
| TCP | 6587 | framed stream of messages | yes |
| TLS | 5425 | TLS then 6587 | no — v1.1 |
| Unix socket | — | — | no |

UDP has no framing. `net.ListenPacket("udp", addr)` is dual-stack.
One datagram is one message. IPv4-mapped IPv6 remotes are unmapped
before admission. A datagram larger than `udpMaxDatagramBytes` or
`maxMessageBytes` is dropped, metrics `labsyslog_udp_oversize_total`
and `labsyslog_messages_dropped_total{reason="oversize"}` increment,
and nothing is stored (`truncated` is not set). An empty datagram is
dropped the same way (metric `reason="empty"`). LabSyslog does **not**
split UDP datagrams.

TCP is a byte stream. Framing is configured by
`spec.listeners.tcp.framing`:

| Value | Behavior |
|---|---|
| `octet-counting` | `MSG-LEN SP SYSLOG-MSG` as RFC 6587 §3.4.1. MSG-LEN is ASCII digits. |
| `non-transparent` | trailer is `LF` (`\n`). `NUL` is accepted as an alternate trailer and stripped. Trailer is not part of `raw`. |
| `auto` | **default.** If the first non-empty bytes of a new message are `DIGIT+ SP`, parse as octet-counting. Otherwise parse as non-transparent. |

Do not invent a third 1.0 mode. `auto` is sticky per TCP session: the
first message decides the mode for that connection. A later message
that disagrees is a framing error; the connection is closed; the
partial message is not stored.

`tcpIdleTimeout` closes an idle session. `maxTcpConns` /
`maxTcpConnsPerIP` reject new accepts.

## PRI

Every well-formed message begins with `<PRI>` where PRI is 1–3 decimal
digits, 0–191 inclusive. `facility = PRI / 8`, `severity = PRI % 8`.

| Facility | Keyword |
|---|---|
| 0 kern | 1 user | 2 mail | 3 daemon | 4 auth | 5 syslog | 6 lpr | 7 news |
| 8 uucp | 9 cron | 10 authpriv | 11 ftp | 12 ntp | 13 audit | 14 console | 15 cron2 |
| 16–23 local0–local7 |

| Severity | Keyword |
|---|---|
| 0 emerg | 1 alert | 2 crit | 3 err | 4 warning | 5 notice | 6 info | 7 debug |

Unknown facility numbers outside 0–23 are a parse warning
(`parseWarning=unknown_facility`); the numeric PRI and facility are
still stored. This is not gated on `bestEffort`. PRI missing: if
`bestEffort` is true, treat as `<13>` (user.notice) and set
`parseWarning=missing_pri`. If `bestEffort` is false, drop.

## RFC 5424

```
<PRI>VERSION SP TIMESTAMP SP HOSTNAME SP APP-NAME SP PROCID SP MSGID SP STRUCTURED-DATA [SP MSG]
```

- VERSION is the single digit `1`.
- TIMESTAMP is RFC 3339 with optional fractional seconds and timezone.
  `-` NILVALUE is allowed and yields zero time.
- HOSTNAME, APP-NAME, PROCID, MSGID: 1–255 / 1–48 / 1–128 / 1–32
  printable ASCII except SP. `-` is NILVALUE → empty string.
- STRUCTURED-DATA is `-` or one or more `[SD-ID PARAM="VALUE" …]`.
  Escapes: `\"`, `\\`, `\]`.
- MSG is the remainder. BOM `EF BB BF` is stripped and noted in
  `parseWarning` if present (`utf8_bom`).

A leading `<PRI>1 ` selects the 5424 parser even if `parse.rfc5424` is
the only enabled parser. If `parse.rfc5424` is false and the message
looks like 5424, store with `parseWarning=rfc5424_disabled` only when
`bestEffort` is true; otherwise drop.

## RFC 3164

```
<PRI>TIMESTAMP SP HOSTNAME SP TAG[PROCID]: SP CONTENT
```

- TIMESTAMP is `Mmm dd hh:mm:ss` (local, no year, no zone). Year is
  inferred as the process clock's year, rolled back one year if the
  stamp is more than 24h in the future.
- HOSTNAME is the next token.
- TAG is alphanumeric plus `-` `_` `.`. Optional `[PROCID]` before `:`.
- CONTENT is the remainder.

A message that does not start with `<PRI>1 ` and has `parse.rfc3164`
true is parsed as 3164. `version` is stored as 0.

## Protocol selection

`auto` (the only 1.0 selector; there is no YAML `defaultProtocol` field
beyond the two parse booleans):

1. If bytes match `^<\d{1,3}>1 ` and `rfc5424` is enabled → 5424.
2. Else if `rfc3164` is enabled → 3164.
3. Else if `bestEffort` → store raw with `parseWarning=no_parser`.
4. Else drop.

`parseWarning` is empty when the parse is clean. Frozen tokens (comma-separated
when more than one applies):

| Token | When |
|---|---|
| `missing_pri` | PRI absent; `bestEffort` treated the bytes as `<13>` |
| `utf8_bom` | RFC 5424 MSG started with UTF-8 BOM `EF BB BF` (stripped) |
| `no_parser` | neither parser selected for this byte pattern; `bestEffort` |
| `rfc5424_disabled` | bytes match `^<\d{1,3}>1 ` but `parse.rfc5424` is false; `bestEffort` |
| `unknown_facility` | PRI yielded facility > 23; numeric PRI/facility still stored (not a drop) |

Serialize exists for tests and export. It is not a forward path.

## Behavior knob

`spec.syslog.behavior.mode` is process-wide and deterministic.

| Mode | UDP | TCP |
|---|---|---|
| `accept` | store after filters | store after filters |
| `drop-silent` | discard after admission, no store | discard after admission, no store, leave conn up |
| `close` | same as drop-silent | discard, close connection |

No probability field in 1.0. A random drop engine needs an ADR.

## Size caps

- `udpMaxDatagramBytes` default `64KiB`. Oversize datagram: drop, metric
  `labsyslog_udp_oversize_total`.
- `maxMessageBytes` default `64KiB` applies to the SYSLOG-MSG after
  framing. TCP octet-counting MSG-LEN greater than this is a framing
  error (close). Non-transparent frames that exceed it are dropped and
  the session continues.
- RFC 5424's historical 2048-byte cap is **not** enforced. Lab traffic
  is larger. The YAML cap is the product cap.

## Time

`receivedAt` is always the process clock. Parsed timestamps are stored
as received, not rewritten. LabNTP can skew the SUT; LabSyslog does
not invent a second clock.

## Examples (normative)

RFC 5424:

```
<165>1 2026-09-04T20:52:35.000Z sut-1 sshd 1234 ID47 [sshd@0 user="alice"] Failed password
```

RFC 3164:

```
<34>Sep  4 20:52:35 sut-1 sshd[1234]: Failed password
```

TCP octet-counting (length includes the SYSLOG-MSG only, not the
length digits or the SP):

```
57 <165>1 2026-09-04T20:52:35.000Z sut-1 sshd 1234 - - hello
```

TCP non-transparent:

```
<165>1 2026-09-04T20:52:35.000Z sut-1 sshd 1234 - - hello\n
```

Golden files in `testdata/packets/` and `testdata/framing/` are the
oracle. New parse behavior requires a golden.
