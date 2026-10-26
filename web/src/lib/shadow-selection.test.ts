/**
 * @vitest-environment jsdom
 */
import { describe, it, expect } from "vitest";
import { getSelectionForShadow, getActiveElementInShadow } from "./shadow-selection";

describe("shadow-selection", () => {
  it("falls back to document.getSelection when shadowRoot is null", () => {
    const sel = getSelectionForShadow(null);
    expect(sel).toBe(document.getSelection());
  });

  it("uses shadowRoot.getSelection when available", () => {
    const host = document.createElement("div");
    document.body.appendChild(host);
    const shadow = host.attachShadow({ mode: "open" });
    // jsdom may not implement getSelection on ShadowRoot; emulate it.
    const fakeSel = {} as Selection;
    (shadow as any).getSelection = () => fakeSel;
    expect(getSelectionForShadow(shadow)).toBe(fakeSel);
  });

  it("falls back to document.getSelection when shadowRoot.getSelection is missing", () => {
    const host = document.createElement("div");
    document.body.appendChild(host);
    const shadow = host.attachShadow({ mode: "open" });
    // ensure no getSelection
    delete (shadow as any).getSelection;
    expect(getSelectionForShadow(shadow)).toBe(document.getSelection());
  });

  it("getActiveElementInShadow returns shadow activeElement when present", () => {
    const host = document.createElement("div");
    document.body.appendChild(host);
    const shadow = host.attachShadow({ mode: "open" });
    const inner = document.createElement("button");
    shadow.appendChild(inner);
    // emulate activeElement
    Object.defineProperty(shadow, "activeElement", { value: inner, configurable: true });
    expect(getActiveElementInShadow(shadow)).toBe(inner);
  });

  it("getActiveElementInShadow falls back to document.activeElement", () => {
    const host = document.createElement("div");
    document.body.appendChild(host);
    const shadow = host.attachShadow({ mode: "open" });
    Object.defineProperty(shadow, "activeElement", { value: null, configurable: true });
    expect(getActiveElementInShadow(shadow)).toBe(document.activeElement);
  });
});
