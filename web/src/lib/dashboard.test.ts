/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi } from "vitest";
import {
  loadDashboard,
  DASHBOARD_BUDGET_MS,
  isVaultRead,
  assertZeroVaultReads,
  surfaceVisibility,
  assertVisibilityAgreement,
} from "./dashboard";

function makeNotes(n: number) {
  return Array.from({ length: n }, (_, i) => ({
    uid: `n${i}`,
    name: `Note ${i}`,
    type: i % 3 === 0 ? "handout" : "note",
    campaign: i % 2 === 0 ? "stormdark" : "",
    workflow: i % 5 === 0 ? "draft" : "published",
    visibility: i % 4 === 0 ? "gm" : "party",
    effective_visibility: i % 4 === 0 ? "gm" : "party",
  }));
}

describe("dashboard", () => {
  it("loads against SRD-sized vault with zero client vault reads", async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      json: async () => ({ items: makeNotes(1690), count: 1690 }),
    })) as unknown as typeof fetch;
    const data = await loadDashboard(fetchMock, DASHBOARD_BUDGET_MS);
    expect(data.notes.length).toBe(1690);
    expect(data.requests).toHaveLength(1);
    assertZeroVaultReads(data.requests);
    // Only the list endpoint was hit.
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("fails when the budget is exceeded", async () => {
    const fetchMock = vi.fn(async () => {
      await new Promise((r) => setTimeout(r, 20));
      return { ok: true, json: async () => ({ items: makeNotes(10), count: 10 }) };
    }) as unknown as typeof fetch;
    await expect(loadDashboard(fetchMock, 1)).rejects.toThrow(/budget exceeded/);
  });

  it("isVaultRead flags per-note reads", () => {
    expect(isVaultRead("/api/v1/notes/abc")).toBe(true);
    expect(isVaultRead("/api/v1/notes/abc/render")).toBe(true);
    expect(isVaultRead("/api/v1/notes")).toBe(false);
    expect(isVaultRead("/api/v1/notes?limit=10")).toBe(false);
  });

  it("assertZeroVaultReads throws on a vault read", () => {
    expect(() => assertZeroVaultReads(["/api/v1/notes", "/api/v1/notes/x"])).toThrow(
      /client vault reads/,
    );
  });

  it("surfaceVisibility fails on raw (unprojected) data", () => {
    expect(() => surfaceVisibility({ uid: "n1", visibility: "gm" })).toThrow(
      /effective_visibility/,
    );
  });

  it("disagree-fails when surface disagrees with projection", () => {
    expect(() => assertVisibilityAgreement("party", "gm")).toThrow(/disagreement/);
  });

  it("budget constant is positive", () => {
    expect(DASHBOARD_BUDGET_MS).toBeGreaterThan(0);
  });
});
