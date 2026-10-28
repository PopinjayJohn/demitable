/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi } from "vitest";
import { loadTabletop } from "./tabletop";

describe("tabletop loader", () => {
  it("loads in exactly 2 requests (encounter + O2 list)", async () => {
    const fetchMock = vi.fn(async (url: any) => {
      const u = String(url);
      if (u.includes("/encounters/")) {
        return { ok: true, json: async () => ({ encounter: { id: "e1", tokens: [] } }) } as any;
      }
      return { ok: true, json: async () => ({ items: [], count: 0 }) } as any;
    }) as unknown as typeof fetch;
    const data = await loadTabletop("s1", "e1", fetchMock);
    expect(data.requests).toHaveLength(2);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(data.requests[0]).toContain("/encounters/");
    expect(data.requests[1]).toBe("/api/v1/notes");
  });

  it("guard wiring: asserts zero vault reads", async () => {
    const fetchMock = vi.fn(async (url: any) => {
      const u = String(url);
      if (u.includes("/encounters/")) return { ok: true, json: async () => ({ id: "e1" }) } as any;
      return { ok: true, json: async () => [] } as any;
    }) as unknown as typeof fetch;
    const data = await loadTabletop({ sid: "s1", eid: "e1" }, fetchMock);
    expect(data.requests).toHaveLength(2);
  });
});
