/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, afterEach, vi } from "vitest";
import { render } from "../test/harness";
import { appState } from "../lib/appstate.svelte";
import { vaultStore } from "../lib/vaultstore.svelte";
import { fetchNote } from "../lib/api";
import Rail from "../components/Rail.svelte";
import NoteEditor from "./NoteEditor.svelte";

vi.mock("../lib/ws", () => ({
  WSClient: class { connect() {} close() {} },
}));
vi.mock("../lib/api", () => ({
  fetchNote: vi.fn(),
  saveNote: vi.fn(),
  patchNote: vi.fn(),
}));

describe("NoteEditor D3 fallback", () => {
  let r: ReturnType<typeof render> | null = null;
  let r2: ReturnType<typeof render> | null = null;
  afterEach(() => {
    r?.unmount(); r = null;
    r2?.unmount(); r2 = null;
    appState.selectedUid = null;
    vaultStore.notes = [];
  });

  const dv = (c: HTMLElement) => c.querySelector("[data-vis]")?.getAttribute("data-vis") ?? null;

  async function mountWith(doc: any) {
    (fetchNote as any).mockResolvedValue(doc);
    appState.selectedUid = "u1";
    r = render(NoteEditor);
    await new Promise((res) => setTimeout(res, 100));
    r2 = render(Rail);
    await new Promise((res) => setTimeout(res, 50));
  }

  it("projection missing entirely (null doc) renders no permissive badge", async () => {
    await mountWith(null);
    // doc null → no badge rendered (fail-closed: nothing permissive leaks)
    expect(dv(r2!.container)).toBeNull();
    expect(r2!.container.textContent).not.toMatch(/\bParty\b|\bPublic\b/);
  });

  it("effectiveVisibility present → that value used", async () => {
    await mountWith({ uid: "u1", name: "N", content: "", effectiveVisibility: "party" });
    expect(dv(r2!.container)).toBe("party");
  });

  it("effective_visibility (snake) present → that value used", async () => {
    await mountWith({ uid: "u1", name: "N", content: "", effective_visibility: "public" });
    expect(dv(r2!.container)).toBe("public");
  });

  it("both spellings absent → gm", async () => {
    await mountWith({ uid: "u1", name: "N", content: "" });
    expect(dv(r2!.container)).toBe("gm");
  });

  it("present permissive value renders as given (pass-through)", async () => {
    await mountWith({ uid: "u1", name: "N", content: "", effectiveVisibility: "public" });
    expect(dv(r2!.container)).toBe("public");
  });

  it("no permissive label on the missing-projection path", async () => {
    await mountWith({ uid: "u1", name: "N", content: "" });
    expect(dv(r2!.container)).toBe("gm");
    expect(r2!.container.textContent).not.toMatch(/\bParty\b|\bPublic\b/);
  });
});
