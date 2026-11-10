/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, afterEach } from "vitest";
import { render } from "./harness";
import { mount, unmount } from "svelte";

// A trivial component defined inline via a .svelte file is not available;
// instead exercise the harness with a minimal component created at runtime
// is not possible, so we assert the container plumbing directly.
describe("component test harness", () => {
  let r: ReturnType<typeof render> | null = null;
  afterEach(() => { r?.unmount(); r = null; });

  it("creates and removes a container", () => {
    // Use a dynamic component import of VisibilityBadge as the smoke target.
    return import("../components/VisibilityBadge.svelte").then((m) => {
      r = render(m.default, { visibility: "gm" });
      expect(r.container.querySelector("[data-vis]")?.getAttribute("data-vis")).toBe("gm");
      r.unmount();
      r = null;
    });
  });
});
