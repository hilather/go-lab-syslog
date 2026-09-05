import { describe, expect, it } from "vitest";
import { navItems } from "./nav";
import { canSubmitClear, canSubmitReset } from "./reset";

describe("operator nav", () => {
  it("omits Reset and Audit without scopes", () => {
    expect(navItems({ canAudit: false, canReset: false }).map((i) => i.label)).toEqual([
      "Messages",
      "Status",
      "Filters",
    ]);
    expect(navItems({ canAudit: true, canReset: true }).map((i) => i.label)).toEqual([
      "Messages",
      "Status",
      "Filters",
      "Audit",
      "Reset",
    ]);
    expect(navItems({ canAudit: true, canReset: true }).some((i) => /relay|forward/i.test(i.label))).toBe(false);
  });

  it("gates reset on the resource name and confirmation", () => {
    expect(canSubmitReset("lab-sink", "lab-sink", true, true)).toBe(true);
    expect(canSubmitReset("lab-sink", "other", true, true)).toBe(false);
    expect(canSubmitReset("lab-sink", "lab-sink", false, true)).toBe(false);
    expect(canSubmitReset("lab-sink", "lab-sink", true, false)).toBe(false);
  });

  it("gates clear on the messages resource name", () => {
    expect(canSubmitClear("messages", true, true)).toBe(true);
    expect(canSubmitClear("MESSAGES", true, true)).toBe(false);
    expect(canSubmitClear("messages", false, true)).toBe(false);
    expect(canSubmitClear("messages", true, false)).toBe(false);
  });
});
