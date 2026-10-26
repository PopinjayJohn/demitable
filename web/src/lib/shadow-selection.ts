// Per-browser selection read for shadow-hosted plugin surfaces (ADR-0029, rule R2).
//
// Chromium retargets `window.getSelection()` to a collapsed range in the host's
// parent for shadow content, so the real selection must come from
// `shadowRoot.getSelection()`. Firefox has no `ShadowRoot.getSelection()` but
// its `window.getSelection()` already returns the real node. Safari historically
// lacked `ShadowRoot.getSelection()`; prefer it when present and fall back to
// the document selection otherwise.
//
// This is the single sanctioned way for app code to read a selection inside a
// plugin's shadow root. Do not call `window.getSelection()` directly for
// shadow content.
export function getSelectionForShadow(shadowRoot: ShadowRoot | null): Selection | null {
  if (!shadowRoot) return window.getSelection();
  const sr = shadowRoot as unknown as { getSelection?: () => Selection | null };
  if (typeof sr.getSelection === 'function') {
    const sel = sr.getSelection();
    if (sel) return sel;
  }
  return window.getSelection();
}

export function getActiveElementInShadow(shadowRoot: ShadowRoot | null): Element | null {
  if (!shadowRoot) return document.activeElement;
  return shadowRoot.activeElement ?? document.activeElement;
}
