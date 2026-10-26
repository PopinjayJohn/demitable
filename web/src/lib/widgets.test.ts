import { describe, it, expect } from "vitest";
import { WidgetRegistry } from "./widgets";

describe("widget registry", () => {
  it("registers a third-party widget without mutating source", () => {
    const reg = new WidgetRegistry();
    reg.register({
      id: "third-party",
      match: /^widget:/,
      render: (src) => {
        const el = document.createElement("div");
        el.textContent = src;
        return el;
      },
    });
    expect(reg.all().length).toBe(1);
    expect(reg.all()[0].id).toBe("third-party");
  });
});
