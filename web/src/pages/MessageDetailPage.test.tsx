import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, resetClientState, sessionView } from "../test/render";
import { MessageDetailPage } from "./MessageDetailPage";
import { Route, Routes } from "react-router-dom";
import { MemoryRouter } from "react-router-dom";
import { AuthProvider } from "../auth/AuthProvider";
import { render } from "@testing-library/react";

describe("MessageDetailPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("renders parsed fields and raw as text", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.includes("/raw")) {
          return new Response(`<script>alert(1)</script>raw-bytes`, {
            status: 200,
            headers: { "Content-Type": "application/octet-stream" },
          });
        }
        if (url.includes("/v1/messages/")) {
          return json(200, {
            id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
            receivedAt: "2026-09-04T00:00:00Z",
            transport: "udp",
            truncated: false,
            parsed: {
              pri: 14,
              facility: 1,
              severity: 6,
              version: 1,
              hostname: "sut-1",
              appName: "sshd",
              message: `<b>not-html</b>`,
              structured: [{ id: "sshd@0", params: [{ name: "user", value: "alice" }] }],
            },
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found" });
      }),
    );
    const { container } = render(
      <MemoryRouter initialEntries={["/messages/01ARZ3NDEKTSV4RRFFQ69G5FAV"]}>
        <AuthProvider>
          <Routes>
            <Route path="/messages/:id" element={<MessageDetailPage />} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    );
    expect(await screen.findByText("sut-1")).toBeInTheDocument();
    expect(screen.getByText("<b>not-html</b>")).toBeInTheDocument();
    expect(await screen.findByText(/script/)).toBeInTheDocument();
    expect(container.querySelector("b")).toBeNull();
    expect(container.querySelector("script")).toBeNull();
  });
});
