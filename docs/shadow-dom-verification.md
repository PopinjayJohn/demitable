# Shadow DOM verification

> The empirical basis for [ADR-0029](decisions/0029-shadow-dom-isolation.md).
> Every claim below was tested in a real browser, not read from a spec.
>
> Verified 2026-10-22 by an automated harness. Raw harness and full
> transcripts live outside the repository; this is the distilled result.

---

## 1. Environment

| | |
|---|---|
| Browser | Chromium 156 (headless shell and full build) |
| Driver | Playwright 1.64.0 |
| Node / npm | v22.23.3 / 10.9.9 |
| Editors | CodeMirror 6.0.2, Tiptap 3.32.0, ProseMirror 1.42.6 |
| Toolchain | Svelte 5.57.2, Vite 8.3.5, `@sveltejs/vite-plugin-svelte` 7.3.1, `svelte-check` 4.7.7 |

## 2. Verdicts

| Claim | Verdict |
|---|---|
| A shadow root is not a document boundary | **VERIFIED** |
| HTML5 drag-and-drop crosses a shadow boundary | **VERIFIED** — synthetic *and* native (`isTrusted: true`) |
| Focus behaviour | **VERIFIED** |
| Style isolation and custom-property inheritance | **VERIFIED** |
| CodeMirror 6 in a shadow root | **VERIFIED — zero changes needed** |
| Tiptap / ProseMirror in a shadow root | **VERIFIED — works, but its CSS must be re-homed** |
| Svelte 5 + Vite 8 + plugin-svelte 7 | **VERIFIED** — builds clean |
| `svelte-check` with TypeScript 7.0.2 | **VERIFIED (conditionally)** — see §6 |

**The decision holds.** The reasons for it — keyboard, IME, clipboard and
drag-and-drop — all survive contact with a real browser.

## 3. What was proven, in detail

**Keyboard.** A trusted `keydown` inside a shadow root is observed at
`document` and at light-DOM ancestors. `event.target` is retargeted to the
host; `event.composedPath()[0]` is the real inner element. Synthetic events
cross only if `composed: true` — the flag is the deciding factor, not the
event type.

**IME.** Real composition driven through `Input.imeSetComposition` produced
trusted `compositionstart` / `compositionupdate` observed at `document`, with
the composing text intact. `beforeinput` crosses for both `<input>` and
`contenteditable`.

**Clipboard.** `navigator.clipboard.writeText` succeeds with the
`clipboard-write` permission and throws `NotAllowedError` without it — and a
user gesture does not rescue it. `document.execCommand('copy')` on a selection
*inside* a shadow root works and the text lands on the clipboard. It needs
only transient activation, so it is the more robust choice for a copy button.

**Drag-and-drop.** Verified in both directions — light DOM → shadow root and
shadow root → light DOM — using the **native** browser path
(`Input.setInterceptDrags` + `Input.dispatchDragEvent`), which produces
`isTrusted: true` events with a browser-managed `DataTransfer`. `drop` fires,
`dataTransfer.getData()` works in the drop handler including **custom MIME
types**, and `event.composedPath()[0]` gives the real element.

**Focus.** `document.activeElement` is the host; `shadowRoot.activeElement` is
the inner element. A light-DOM `<dialog>` opened with `showModal()` pulls
focus out of the shadow root and traps it inside the dialog, and
`dialog.close()` **restores focus into the shadow root correctly**.

**Styles.** Shadow rules do not leak out. Light-DOM rules do not match shadow
content — only *inherited* values cross, via inheritance rather than rule
matching. A custom property on `:root` resolves inside the shadow root, so
the theming mechanism works. `adoptedStyleSheets` works on a shadow root, and
`::part()` still lets the app style plugin internals from outside.

**Editors.** CodeMirror 6 in a shadow root needed **no changes at all** — it
auto-detects the shadow root and adopts its theme via `adoptedStyleSheets`,
leaving `document.head` untouched. Tiptap works, but injects its ProseMirror
CSS into `document.head`, where it cannot reach the shadow root, leaving the
editor with `white-space: normal` and a ProseMirror warning. The fix is
`injectCSS: false` plus a `<style>` in the shadow root, or re-homing
`editor.css`.

**Our editor is unaffected.** It is mounted in light DOM by design
(ADR-0021), so Tiptap's CSS reaches it normally. The Tiptap gotcha is a trap
for *third-party* editors in shadow roots, and the CodeMirror result is a
pleasant surprise for the source view.

## 4. The rules that follow

These are implementation requirements, not observations. Each one is a way to
get it wrong.

**R1 — Identify elements with `event.composedPath()[0]`, never `event.target`.**
Every composed event retargets `target` to the shadow host. Any handler doing
`event.target.closest(...)` will silently match the host.

**R2 — Read selections from the shadow root.**
`window.getSelection()` is **unusable** for shadow content: `.toString()`
returns the right text, but `anchorNode` and `getRangeAt(0)` are retargeted
to a *collapsed* range at the host's position in its parent. `containsNode()`
returns `true`, so it looks like it works. Use `shadowRoot.getSelection()`,
found by walking `document.activeElement` → `.shadowRoot`.

**R3 — Dialogs must trap Tab themselves.**
Shadow content is inline in the host page's native tab order, so a modal
relying on native tab order **leaks focus into plugin content**. A
capture-phase `keydown` listener on `document` sees the composed Tab and can
contain it — but it only sees the host, so it must read
`host.shadowRoot.activeElement` to know where focus was.

**R4 — Theme tokens must be unregistered custom properties**, or `@property`
with `inherits: true`. A registered property with `inherits: false` does not
cross the boundary; the shadow sees the initial value.

**R5 — Plugins must not rely on `:host` rules for layout-critical styling.**
An outer-tree declaration beats a shadow `:host` declaration at equal
importance — **scope beats specificity**. The app can always restyle a
plugin surface, which is a feature: the app controls the chrome. Plugins
should express intent through custom properties, not `:host` rules. A
`:host { … !important }` does beat everything, including an outer
`!important`.

**R6 — If a plugin uses `all: initial` on `:host`, re-set `display`.**
`all: initial` resets inherited values and custom properties are *not* reset,
but it also sets `display: inline`, silently changing the host's layout.

**R7 — "Select all" is a plugin-level operation.**
`document.execCommand('selectAll')` excludes shadow content entirely, and a
selection cannot span the boundary — the shadow portion is silently dropped.

**R8 — Form controls do not inherit `font-family`** into a shadow root; they
need `font: inherit`.

**R9 — `adoptedStyleSheets` is how a plugin injects its own styles** into its
shadow root, and it is CSP-friendly. CodeMirror 6 already does this.

**R10 — A plugin's shadow root is not complete a11y containment.** ProseMirror
puts its `aria-live` announcement element in the light DOM even when the
editor is fully contained. Harmless for CSS, but worth knowing.

## 5. The rules the *app* must follow

- The app's own dialogs use a capture-phase Tab trap (R3).
- The app's "copy selection" command reads the selection from the active
  shadow root, not the document (R2).
- The app's theme is expressed as **unregistered** custom properties on
  `:root` (R4), and it may restyle plugin surfaces freely (R5).
- The app provides the shadow root, so a plugin's styles are contained by
  construction and `::part()` is the sanctioned channel for the app to style
  plugin internals.

## 6. TypeScript 7

`typescript@7.0.2` **alone breaks `svelte-check`**, with a hard
version-guard error. The cause is visible in the package: `require('typescript')`
returns only `{ version, versionMajorMinor }` — the package exports **no
JavaScript compiler API**, and the API lives only under `./unstable/*`. Every
tool that embeds the TS compiler needs the native binary explicitly.

Two working configurations:

```jsonc
// A. Recommended today
"devDependencies": { "typescript": "~5.9.3" }        // or ~6.0.3, what create-vite scaffolds

// B. With the native compiler's speed
"devDependencies": {
  "typescript": "~6.0.3",
  "@typescript/native": "npm:typescript@7"
},
"scripts": { "check": "svelte-check --tsconfig ./tsconfig.app.json --tsgo" }
```

Configuration B was verified to work — including catching deliberately
injected type errors, so it was not silently no-op'ing. But it is fragile:
`svelte-check` gates it behind an unguarded version check, it depends on an
npm-alias trick the TS team has not standardised, and the two installs fight
over `node_modules/.bin/tsc`.

**Decision: configuration A.** TypeScript 5.9.x (or 6.x) is the primary
`typescript` dependency. The native compiler can be added later, deliberately.

## 7. What is still unverified

| Item | Why it matters | How to settle it |
|---|---|---|
| **Firefox and Safari** | Chromium only. `ShadowRoot.getSelection()` (Safari historically lacked it), `document.getSelection()` retargeting, `adoptedStyleSheets` in shadow roots, and ProseMirror's own Safari shadow-selection workaround (it is in `prosemirror-view`'s source) all differ | Repeat these pages in Playwright's `firefox` and `webkit` projects **before committing to shadow DOM** |
| Native drag gesture on a desktop OS | The browser path was exercised through CDP drag interception, which is browser-generated but bypasses the OS drag loop | Only matters if tokens may be dragged out of the browser window |
| A real OS IME | Composition was driven through CDP, producing genuine trusted events | Test on a machine with a real IME |
| Plugin → plugin drag between *two* different shadow roots | The mechanism matches the tested cases | Extend the harness with two shadow hosts |
| Performance with many plugin surfaces | No measurement of the cost of ~50 shadow roots | Benchmark |

**The Firefox/Safari gap is the one that matters.** The `getSelection()`
retargeting behaviour is the most likely to differ, and R2 depends on it.

## 8. Two incidental findings worth keeping

- **Playwright's selector engine pierces open shadow roots; `document.querySelector`
  does not.** `document.querySelector('#shadow-target')` is `null`, but
  `page.locator('#shadow-target').count()` is `1`. E2E tests will "just work"
  where app code cannot reach in at all.
- **A classic-script bundle can clobber a global.** Loading an esbuild ESM
  bundle with `<script src>` (no `type="module"`) ran ProseMirror's top-level
  `var DOMParser = class _DOMParser {…}` as a global, overwriting
  `window.DOMParser` and breaking Tiptap. Worth knowing if we ever ship a
  bundled plugin.
