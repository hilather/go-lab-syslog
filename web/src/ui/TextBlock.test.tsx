import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TextBlock } from "./TextBlock";

describe("TextBlock", () => {
  it("renders markup as text, not HTML", () => {
    const payload = `<img src=x onerror=alert(1)>script`;
    const { container } = render(<TextBlock text={payload} />);
    expect(screen.getByTestId("text-block").textContent).toBe(payload);
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("script")).toBeNull();
  });
});
