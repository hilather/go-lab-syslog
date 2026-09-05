import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { json, resetClientState, sessionView } from "./test/render";

describe("App nav", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("hides Reset without syslog.admin and has no relay control", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes("/v1/session")) {
          return json(200, sessionView(["syslog.read"]));
        }
        if (url.includes("/v1/messages")) {
          return json(200, { revision: "sha256:x", storeGeneration: 1, items: [] });
        }
        return json(404, {
          status: 404,
          title: "not found",
          detail: "not found",
          code: "not_found",
          type: "https://labsyslog.dev/errors/not_found",
        });
      }),
    );
    render(<App />);
    expect(await screen.findByRole("link", { name: "Messages" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Reset" })).toBeNull();
    expect(screen.queryByRole("button", { name: /relay|forward/i })).toBeNull();
    expect(screen.queryByRole("checkbox", { name: /relay|forward/i })).toBeNull();
  });
});
