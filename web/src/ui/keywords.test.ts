import { describe, expect, it } from "vitest";
import { FACILITIES, SEVERITIES, facilityName, severityName } from "./keywords";

// Frozen docs/02 / internal/syslogwire facilityKeywords. LookupFacility
// accepts these strings on GET /v1/messages?facility=.
const SYSLOGWIRE_FACILITIES = [
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
  "console",
  "cron2",
  "local0",
  "local1",
  "local2",
  "local3",
  "local4",
  "local5",
  "local6",
  "local7",
] as const;

describe("facility keywords", () => {
  it("matches syslogwire 0–23 including console/cron2", () => {
    expect([...FACILITIES]).toEqual([...SYSLOGWIRE_FACILITIES]);
    expect(FACILITIES).toHaveLength(24);
    expect(facilityName(14)).toBe("console");
    expect(facilityName(15)).toBe("cron2");
    expect(FACILITIES).not.toContain("alert");
    expect(FACILITIES).not.toContain("clock");
  });

  it("keeps severity 1 as alert (not a facility)", () => {
    expect([...SEVERITIES]).toEqual(["emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"]);
    expect(severityName(1)).toBe("alert");
  });
});
