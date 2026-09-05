import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CSRF_HEADER } from "../api/client";
import { json, renderApp, resetClientState, seedCSRF, sessionView } from "../test/render";
import { FiltersPage } from "./FiltersPage";

describe("FiltersPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("toggles enabled via replaceFilters with CSRF", async () => {
    const user = userEvent.setup();
    seedCSRF();
    const applies: Array<RequestInit | undefined> = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = (init?.method ?? "GET").toUpperCase();
        if (url.endsWith("/v1/session") && method === "GET") {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/state") && method === "GET") {
          return json(200, {
            apiVersion: "labsyslog.dev/v1alpha1",
            kind: "LabSyslog",
            metadata: { name: "lab-sink" },
            spec: {
              filters: [
                {
                  name: "drop-debug",
                  enabled: true,
                  match: { severities: ["debug"] },
                  action: { mode: "drop-silent" },
                },
              ],
            },
            revision: "sha256:runtime",
            generation: 1,
            drifted: false,
          });
        }
        if (url.endsWith("/v1/changes:apply") && method === "POST") {
          applies.push(init);
          return json(200, { revision: "sha256:next", plan: { expectedRevision: "sha256:runtime", operations: [] } });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found" });
      }),
    );

    renderApp(<FiltersPage />, { route: "/filters" });
    const box = await screen.findByRole("checkbox", { name: /Enable drop-debug/i });
    expect(box).toBeEnabled();
    await user.click(box);
    await waitFor(() => {
      expect(applies).toHaveLength(1);
    });
    const headers = new Headers(applies[0]?.headers);
    expect(headers.get(CSRF_HEADER)).toBe("csrf-test");
    const body = JSON.parse(String(applies[0]?.body)) as { operations: Array<{ type: string }> };
    expect(body.operations[0]?.type).toBe("replaceFilters");
  });

  it("disables toggle without syslog.admin", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView(["syslog.read"]));
        }
        if (url.endsWith("/v1/state")) {
          return json(200, {
            apiVersion: "labsyslog.dev/v1alpha1",
            kind: "LabSyslog",
            metadata: { name: "lab-sink" },
            spec: {
              filters: [{ name: "drop-debug", enabled: true, match: {}, action: { mode: "drop-silent" } }],
            },
            revision: "sha256:runtime",
            generation: 1,
            drifted: false,
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found" });
      }),
    );
    renderApp(<FiltersPage />, { route: "/filters" });
    const box = await screen.findByRole("checkbox", { name: /Enable drop-debug/i });
    expect(box).toBeDisabled();
  });
});
