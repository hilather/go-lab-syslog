import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { MessagesPage } from "./MessagesPage";

describe("MessagesPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("renders generation, text-only payload, and no relay control", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.includes("/v1/messages")) {
          return json(200, {
            revision: "sha256:x",
            storeGeneration: 7,
            items: [
              {
                id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
                receivedAt: "2026-09-04T00:00:00Z",
                transport: "udp",
                truncated: false,
                parsed: {
                  pri: 14,
                  facility: 1,
                  severity: 6,
                  version: 0,
                  hostname: "sut-1",
                  appName: "sshd",
                  message: `<img src=x onerror=alert(1)>hello`,
                },
              },
            ],
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found" });
      }),
    );
    const { container } = renderApp(<MessagesPage />, { route: "/" });
    expect(await screen.findByText("7")).toBeInTheDocument();
    expect(screen.getByText(/img src=x onerror/)).toBeInTheDocument();
    expect(container.querySelector("img")).toBeNull();
    expect(screen.queryByRole("button", { name: /relay|forward/i })).toBeNull();
    const clear = screen.getByRole("button", { name: /Clear messages/i });
    expect(clear).toBeDisabled();
  });

  it("enables clear after typing the resource name", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.includes("/v1/messages")) {
          return json(200, { revision: "sha256:x", storeGeneration: 1, items: [] });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found" });
      }),
    );
    renderApp(<MessagesPage />, { route: "/" });
    const submit = await screen.findByRole("button", { name: /Clear messages/i });
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText(/Resource name/i), "messages");
    expect(submit).toBeDisabled();
    await user.click(screen.getByLabelText(/Wipe stored messages/i));
    expect(submit).toBeEnabled();
  });
});
