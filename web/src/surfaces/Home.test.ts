/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, afterEach, vi } from "vitest";
import { render } from "../test/harness";
import { appState } from "../lib/appstate.svelte";
import { vaultStore } from "../lib/vaultstore.svelte";
import Rail from "../components/Rail.svelte";
import Home from "./Home.svelte";

vi.mock("../lib/api", () => ({
  fetchNote: vi.fn(async (id: string) => ({ uid: id, name: "Camp", content: "", effective_visibility: undefined, effectiveVisibility: undefined })),
  createNote: vi.fn(),
}));

describe("Home D3 fallback", () => {
  let r: ReturnType<typeof render> | null = null;
  let r2: ReturnType<typeof render> | null = null;
  afterEach(() => {
    r?.unmount(); r = null;
    r2?.unmount(); r2 = null;
    appState.campaignFilter = null;
    vaultStore.notes = [];
  });

  const dv = (c: HTMLElement) => c.querySelector("[data-vis]")?.getAttribute("data-vis") ?? null;

  async function mountWith(note: any) {
    vaultStore.notes = [note];
    appState.campaignFilter = String(note.campaign ?? note.name ?? "Camp");
    r = render(Home);
    await new Promise((res) => setTimeout(res, 100));
    r2 = render(Rail);
    await new Promise((res) => setTimeout(res, 50));
  }

  it("projection missing entirely (null campaignNote) renders no badge", async () => {
    vaultStore.notes = [];
    appState.campaignFilter = "Camp";
    r = render(Home);
    await new Promise((res) => setTimeout(res, 100));
    r2 = render(Rail);
    await new Promise((res) => setTimeout(res, 50));
    // campaignNote null → railContent not published? appState.rail may be previous; but no badge expected
    expect(r2!.container.textContent).not.toMatch(/\bParty\b|\bPublic\b/);
  });

  it("effective_visibility present, effectiveVisibility absent → present value used", async () => {
    await mountWith({ uid: "c1", name: "Camp", type: "campaign", campaign: "Camp", effective_visibility: "party" });
    expect(dv(r2!.container)).toBe("party");
  });

  it("effectiveVisibility present, effective_visibility absent → present value used", async () => {
    await mountWith({ uid: "c1", name: "Camp", type: "campaign", campaign: "Camp", effectiveVisibility: "public" });
    expect(dv(r2!.container)).toBe("public");
  });

  it("both spellings absent → gm", async () => {
    await mountWith({ uid: "c1", name: "Camp", type: "campaign", campaign: "Camp" });
    expect(dv(r2!.container)).toBe("gm");
  });

  it("present permissive value renders as given (pass-through)", async () => {
    await mountWith({ uid: "c1", name: "Camp", type: "campaign", campaign: "Camp", effective_visibility: "party", effectiveVisibility: "party" });
    expect(dv(r2!.container)).toBe("party");
  });

  it("no permissive label on the missing-projection path", async () => {
    await mountWith({ uid: "c1", name: "Camp", type: "campaign", campaign: "Camp" });
    expect(dv(r2!.container)).toBe("gm");
    expect(r2!.container.textContent).not.toMatch(/\bParty\b|\bPublic\b/);
  });
});
