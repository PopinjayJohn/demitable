/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, afterEach } from "vitest";
import { render } from "../test/harness";
import NoteContext from "./NoteContext.svelte";

describe("NoteContext D3 fallback", () => {
  let r: ReturnType<typeof render> | null = null;
  afterEach(() => { r?.unmount(); r = null; });

  const dv = (c: HTMLElement) => c.querySelector("[data-vis]")?.getAttribute("data-vis") ?? null;

  it("projection missing entirely (null note) renders no permissive badge", () => {
    r = render(NoteContext, { note: null, secrets: [], onReveal: () => {}, onUnreveal: () => {} });
    expect(dv(r!.container)).toBeNull();
    expect(r!.container.textContent).not.toMatch(/Party|Public/);
  });

  it("effective_visibility present, effectiveVisibility absent → present value used", () => {
    r = render(NoteContext, {
      note: { uid: "u1", name: "N", effective_visibility: "party" } as any,
      secrets: [], onReveal: () => {}, onUnreveal: () => {},
    });
    expect(dv(r!.container)).toBe("party");
  });

  it("effectiveVisibility present, effective_visibility absent → present value used", () => {
    r = render(NoteContext, {
      note: { uid: "u1", name: "N", effectiveVisibility: "public" } as any,
      secrets: [], onReveal: () => {}, onUnreveal: () => {},
    });
    expect(dv(r!.container)).toBe("public");
  });

  it("both spellings absent → gm", () => {
    r = render(NoteContext, {
      note: { uid: "u1", name: "N" } as any,
      secrets: [], onReveal: () => {}, onUnreveal: () => {},
    });
    expect(dv(r!.container)).toBe("gm");
  });

  it("present permissive value renders as given (pass-through)", () => {
    r = render(NoteContext, {
      note: { uid: "u1", name: "N", effective_visibility: "party", effectiveVisibility: "party" } as any,
      secrets: [], onReveal: () => {}, onUnreveal: () => {},
    });
    expect(dv(r!.container)).toBe("party");
  });

  it("no permissive label reaches the missing-projection path", () => {
    r = render(NoteContext, {
      note: { uid: "u1", name: "N" } as any,
      secrets: [], onReveal: () => {}, onUnreveal: () => {},
    });
    expect(dv(r!.container)).toBe("gm");
    expect(r!.container.textContent).not.toMatch(/\bParty\b|\bPublic\b/);
  });
});
