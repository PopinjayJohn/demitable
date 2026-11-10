/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, afterEach, vi } from "vitest";
import { render } from "../test/harness";
import { appState } from "../lib/appstate.svelte";
import { vaultStore } from "../lib/vaultstore.svelte";
import VaultExplorer from "./VaultExplorer.svelte";

vi.mock("../lib/ws", () => ({
  WSClient: class { connect() {} close() {} },
}));
vi.mock("../lib/api", () => ({
  fetchNote: vi.fn(async (id: string) => ({ uid: id, name: "N", content: "", effectiveVisibility: undefined })),
  patchNote: vi.fn(),
}));

describe("VaultExplorer D3 fallback", () => {
  let r: ReturnType<typeof render> | null = null;
  afterEach(() => {
    r?.unmount();
    r = null;
    vaultStore.notes = [];
    appState.selectedUid = null;
    appState.campaignFilter = null;
  });

  const dv = (c: HTMLElement) => Array.from(c.querySelectorAll("[data-vis]")).map((e) => e.getAttribute("data-vis"));

  function setNotes(notes: any[]) {
    vaultStore.notes = notes as any;
  }

  it("projection missing entirely (empty list) renders no permissive badge", () => {
    setNotes([]);
    r = render(VaultExplorer);
    expect(dv(r!.container)).not.toContain("party");
    expect(dv(r!.container)).not.toContain("public");
  });

  it("effective_visibility present → that value used", () => {
    setNotes([{ uid: "u1", name: "N", type: "note", effective_visibility: "party" }]);
    r = render(VaultExplorer);
    expect(dv(r!.container)).toContain("party");
  });

  it("effectiveVisibility (camelCase) present → that value used", () => {
    setNotes([{ uid: "u2", name: "N", type: "note", effectiveVisibility: "public" }]);
    r = render(VaultExplorer);
    expect(dv(r!.container)).toContain("public");
  });

  it("both spellings absent → gm", () => {
    setNotes([{ uid: "u3", name: "N", type: "note" }]);
    r = render(VaultExplorer);
    expect(dv(r!.container)).toContain("gm");
  });

  it("present permissive value renders as given (pass-through)", () => {
    setNotes([{ uid: "u4", name: "N", type: "note", effective_visibility: "public" }]);
    r = render(VaultExplorer);
    expect(dv(r!.container)).toContain("public");
  });

  it("no permissive label on the missing-projection path", () => {
    setNotes([{ uid: "u5", name: "N", type: "note" }]);
    r = render(VaultExplorer);
    expect(dv(r!.container)).not.toContain("party");
    expect(dv(r!.container)).not.toContain("public");
    expect(dv(r!.container)).toContain("gm");
  });
});
