import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { ResetPage } from "./ResetPage";

describe("ResetPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("keeps reset disabled until the resource name is typed and confirmed", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/state")) {
          return json(200, {
            apiVersion: "labsyslog.dev/v1alpha1",
            kind: "LabSyslog",
            metadata: { name: "lab-sink" },
            spec: {},
            revision: "sha256:x",
            generation: 1,
            drifted: false,
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found" });
      }),
    );
    renderApp(<ResetPage />, { route: "/reset" });
    const submit = await screen.findByRole("button", { name: /Reset LabSyslog/i });
    expect(submit).toBeDisabled();
    await screen.findByText("lab-sink");
    await user.type(screen.getByLabelText(/Resource name/i), "lab-sink");
    expect(submit).toBeDisabled();
    await user.click(screen.getByLabelText(/Wipe the store/i));
    expect(submit).toBeEnabled();
  });

  it("keeps reset disabled without syslog.admin", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView(["syslog.read", "syslog.write"]));
        }
        if (url.endsWith("/v1/state")) {
          return json(200, {
            apiVersion: "labsyslog.dev/v1alpha1",
            kind: "LabSyslog",
            metadata: { name: "lab-sink" },
            spec: {},
            revision: "sha256:x",
            generation: 1,
            drifted: false,
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found" });
      }),
    );
    renderApp(<ResetPage />, { route: "/reset" });
    const submit = await screen.findByRole("button", { name: /Reset LabSyslog/i });
    expect(submit).toBeDisabled();
  });
});
