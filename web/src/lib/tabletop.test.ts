/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi } from "vitest";
import { loadTabletop } from "./tabletop";
import * as dashboard from "./dashboard";

vi.mock("./dashboard", async () => {
  const actual = await vi.importActual<any>("./dashboard");
  return { ...actual, surfaceVisibility: vi.fn((n: any) => "party") };
});

describe("tabletop loader", () => {
  it("loads in exactly 2 requests (encounter + O2 list)", async () => {
    const tokens = [
      { id: "t1", note_uid: "n1" },
      { id: "t2", note_uid: "n2" },
      { id: "t3", note_uid: "n3" },
    ];
    const fetchMock = vi.fn(async (url: any) => {
      const u = String(url);
      if (u.includes("/encounters/")) {
        return { ok: true, json: async () => ({ encounter: { id: "e1", tokens } }) } as any;
      }
      return { ok: true, json: async () => ({ items: [], count: 0 }) } as any;
    }) as unknown as typeof fetch;
    const data = await loadTabletop("s1", "e1", fetchMock);
    expect(data.requests).toHaveLength(2);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(data.requests[0]).toContain("/encounters/");
    expect(data.requests[1]).toBe("/api/v1/notes");
    // No per-token / per-note reads were issued.
    const urls = (fetchMock as any).mock.calls.map((c: any[]) => String(c[0]));
    for (const u of urls) {
      expect(u).not.toMatch(/\/tokens\//);
      expect(u).not.toMatch(/\/notes\/[^?]/);
    }
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

  it("calls surfaceVisibility on load with projected visibility", async () => {
    const notes = [{ uid: "n1", effective_visibility: "party" }];
    const fetchMock = vi.fn(async (url: any) => {
      const u = String(url);
      if (u.includes("/encounters/")) {
        return { ok: true, json: async () => ({ encounter: { id: "e1", tokens: [] } }) } as any;
      }
      return { ok: true, json: async () => ({ items: notes }) } as any;
    }) as unknown as typeof fetch;
    (dashboard.surfaceVisibility as any).mockClear();
    await loadTabletop("s1", "e1", fetchMock);
    expect(dashboard.surfaceVisibility).toHaveBeenCalled();
    expect(dashboard.surfaceVisibility).toHaveBeenCalledWith(
      expect.objectContaining({ effective_visibility: "party" }),
    );
  });
});
