export const FACILITIES = [
  "kern",
  "user",
  "mail",
  "daemon",
  "auth",
  "syslog",
  "lpr",
  "news",
  "uucp",
  "cron",
  "authpriv",
  "ftp",
  "ntp",
  "audit",
  "alert",
  "clock",
  "local0",
  "local1",
  "local2",
  "local3",
  "local4",
  "local5",
  "local6",
  "local7",
] as const;

export const SEVERITIES = ["emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"] as const;

export function facilityName(n: number): string {
  return FACILITIES[n] ?? String(n);
}

export function severityName(n: number): string {
  return SEVERITIES[n] ?? String(n);
}
