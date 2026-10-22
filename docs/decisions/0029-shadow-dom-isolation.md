# ADR-0029: Plugin UI is isolated by shadow DOM, not by a frame

- **Status:** Accepted, and **verified empirically** — see
  [`shadow-dom-verification.md`](../shadow-dom-verification.md)
- **Date:** 2026-10-22
- **Supersedes:** the isolation model in ADR-0020 (sandboxed iframes) and
  amends ADR-0021 and ADR-0027

## Context

ADR-0020 put plugin UI in a **sandboxed iframe** and ADR-0027 allowed
in-document components as an exception. The stated reason for the iframe was
isolation.

It is worth being precise about what a same-origin iframe actually
isolates:

- **Not privilege.** A same-origin iframe shares the origin. Its script can
  reach `document`, `localStorage`, and make same-origin `fetch` calls as the
  user. It could always read the whole vault if the user was the GM.
- **Yes: style and layout.** A separate document means no CSS bleed, no DOM
  collision, its own scrolling, and a hard rectangle in the parent's layout.
- **Yes: namespace.** A separate `window`, so globals do not collide.

So the iframe was buying style and layout isolation, and paying for it in
everything an interactive surface needs:

| Broken by the frame boundary | Why it matters here |
|---|---|
| Keyboard events do not bubble across it | Every global app shortcut — `Cmd+K`, `Cmd+S`, Escape — goes dead while focus is in the frame. The fix is forwarding every key over the bridge, putting a message on the hottest path in the app |
| `compositionstart` / `beforeinput` do not cross | IME candidate windows anchor to the frame, not the caret |
| HTML5 drag-and-drop is **blocked** across it | You cannot drag a monster from a bestiary sidebar onto the tabletop. The single most natural interaction in the product |
| Popovers and menus are clipped to the frame | A dropdown must be rendered inside the frame, and `position: fixed` there is relative to the frame, not the viewport |
| The frame is a fixed rectangle | It cannot participate in the app's flex/grid layout, cannot go fullscreen cleanly, and needs a resize protocol to size itself to its content |
| Two nested scroll containers | Scroll chaining and touch momentum fight each other |
| CSS custom properties and inherited styles do not cross | Every plugin re-implements the app's theme; a dark-mode toggle has to be propagated to every frame |
| Focus and `aria` semantics cross awkwardly | Live regions in a frame are not reliably announced; focus trapping needs cooperation |

The tabletop is the worst case for all of this. A VTT is not one surface but a
**composed layout** — the map, the initiative tracker, the party frames, the
dice log, and a bestiary to drag from — which is why putting the board in a
frame forces the bestiary into a different context and breaks exactly the
drag that matters.

The cost of all that was accepted to protect against a threat that did not
exist. A frame is a *document* boundary, and the only reason to want one is
to keep untrusted content out — which a same-origin frame does not do.

## Decision

**Plugin UI is isolated by a shadow root. The iframe is retained only for
embedding external or untrusted content.**

### Three surface kinds

| Kind | Where it runs | Isolation | Who uses it |
|---|---|---|---|
| **Shadow** (default) | A shadow root in the app's document | Styles contained; document shared | Almost every plugin |
| **Light DOM** (opt-in) | The app's document, directly | None | Surfaces that *are* the app — the editor, the table |
| **Frame** (opt-in) | A separate origin, `sandbox`ed | A real document and origin boundary | Embedding a third-party web page, or rendering untrusted HTML |

### The privilege statement, corrected

ADR-0027 said an in-document component "has the same privileges as the person
running the app", and presented that as the exception to the iframe's
sandbox. **That was backwards, and this ADR corrects it.**

> **All plugin UI runs in the app's document, and therefore has the app's
> privileges. What we offer is style and layout isolation, not security.**

A shadow-root plugin can reach `document` exactly as a light-DOM one can.
The difference is that its *styles* are contained and its default placement
is scoped. We were never offering a security boundary to plugin UI, and
saying otherwise was the error.

What *is* a real boundary is unchanged and still load-bearing:

- **The WASM sandbox.** A sandboxed plugin's *logic* cannot touch the
  network, the filesystem, or anything outside its declared capabilities.
  That is the OS-level sandbox (ADR-0020), and it is unaffected by where its
  UI mounts.
- **The capability model.** Data reaches a plugin through the bridge, and the
  bridge enforces the grants.

So the split is honest and worth stating plainly:

> **A plugin's logic is sandboxed. A plugin's UI is trusted.**

That was already true — the iframe never sandboxed the UI either, since it
was same-origin. The difference is that now it is *visible* rather than
accidental, and the import warning can say it in plain words.

### Shadow DOM's sharp edges

Shadow DOM is not free. The behaviours the design must account for:

- **Inherited properties and custom properties cross the boundary; element
  styles do not.** `--color-accent` set on `:root` reaches every shadow
  surface, which is exactly the theming mechanism we want — but the app's
  `button { … }` rules do not, and a plugin wanting the app's exact chrome
  must opt in by setting the inherited values on `:host`.
- **`:host { all: initial }`** resets the inherited values, for a plugin
  that wants the UA defaults instead of the app's theme. It does **not** reset
  custom properties, and it sets `display: inline` — which silently changes
  the host's layout.
- **`event.target` retargets to the shadow host.** A handler that needs the
  real element uses `event.composedPath()[0]`. This matters for drop
  handlers.
- **`adoptedStyleSheets`** is the modern way to inject styles into a shadow
  root; editors that inject into `document.head` by default may need
  adjusting.
- **Focus crosses the boundary the same way events do.** `document.activeElement`
  is the host; `shadowRoot.activeElement` is the inner element.
- **Outer-tree CSS beats the plugin's `:host` rules** at equal importance —
  *scope beats specificity*. The app can always restyle a plugin surface.
  That is a feature: the app controls the chrome.

### What was verified, and how

Every claim above was tested in Chromium 156 with Playwright, not read from a
spec. The results are in
[`shadow-dom-verification.md`](../shadow-dom-verification.md), and they hold:

- Keyboard, IME (`compositionstart`/`beforeinput`) and clipboard all cross the
  boundary, observed at `document`.
- **HTML5 drag-and-drop crosses the boundary in both directions**, verified on
  the *native* browser path with `isTrusted: true` events and a
  browser-managed `DataTransfer`, including custom MIME types. So a monster
  can be dragged from a bestiary onto the map.
- `dialog.close()` restores focus into the shadow root correctly.
- Custom properties on `:root` resolve inside the shadow root; `::part()`
  works from the host side; `adoptedStyleSheets` works.
- **CodeMirror 6 works in a shadow root with zero changes.** **Tiptap works,
  but its ProseMirror CSS must be re-homed** into the shadow root — it is
  injected into `document.head` by default, where it cannot apply.

Our editor is mounted in **light DOM** (ADR-0021), so the Tiptap CSS issue
does not affect it. It matters for third-party editors in shadow roots.

### The rules that follow

The verification produced ten implementation rules, recorded in full in the
verification document. The four that shape the core's own code:

1. **Identify elements with `event.composedPath()[0]`, never `event.target`.**
   Every composed event retargets `target` to the host.
2. **Read selections from the shadow root.** `window.getSelection()` is
   unusable for shadow content — `anchorNode` and `getRangeAt(0)` collapse to
   a meaningless offset in the host's parent, while `containsNode()` still
   returns `true`, so it looks like it works. The app's "copy selection"
   command must walk `document.activeElement` → `.shadowRoot`.
3. **Dialogs must trap Tab in a capture-phase `keydown` listener.** Shadow
   content is inline in the host page's native tab order, so a modal relying
   on native tab order leaks focus into plugin content.
4. **Theme tokens must be unregistered custom properties**, or `@property`
   with `inherits: true`. A registered property with `inherits: false` does
   not cross the boundary.

### What is still unverified

Chromium only. **Firefox and Safari are the open risk** — `ShadowRoot.getSelection()`
(Safari historically lacked it), `document.getSelection()` retargeting, and
`adoptedStyleSheets` in shadow roots all differ, and ProseMirror ships its own
Safari shadow-selection workaround, which is a signal. Rule 2 above depends on
this behaviour.

The shadow-DOM decision should be **revisited if Firefox or Safari behave
differently**, and the cross-browser harness should exist before the plugin
host is built.

### What this means for the two surfaces that matter

- **The editor** (ADR-0021) is a **light-DOM** surface. It is the app's own
  content surface, it must compose with the app's chrome, and it must own
  focus, IME and the native spellcheck UI without a boundary in the way. It
  was already in-document; this ADR makes the placement explicit.
- **The table** is a **light-DOM** surface too, for the same reason: it is a
  whole-screen mode of the app composed of the map, the initiative tracker,
  the party frames, the dice log and the bestiary. Native drag-and-drop
  between them is the point.

Both are still plugins. The tier model is unchanged; only the isolation
primitive changed.

## Consequences

**Good:**

- **Native drag-and-drop everywhere.** A monster can be dragged from a
  bestiary onto the map, and a token moved around it, with no custom drag
  protocol and no bridge on the drag path.
- **Keyboard, IME, clipboard and selection all behave natively.** No
  forwarding, no per-keystroke message, no per-frame postMessage.
- **The table can be a real layout** — flex, grid, fullscreen, the app's own
  chrome — rather than a rectangle with panels arranged around it.
- **Theming works by inheritance.** Custom properties on `:root` reach every
  plugin surface automatically.
- **The isolation story is honest.** We stopped implying a security boundary
  we never had, which makes the actual boundaries — the WASM sandbox and the
  capability model — clearer, not weaker.

**Bad:**

- **Plugin UI is trusted, explicitly.** A malicious plugin's UI can read and
  change everything the user can see. Under the "this is just a game"
  posture, that is acceptable; it is not acceptable for a vault containing
  real secrets, and the import warning must say so.
- **A plugin can still break the app's styles** if it escapes its shadow root
  deliberately, or if it ships a light-DOM surface.
- **Shadow DOM is not uniformly supported by every browser** for the more
  recent parts (`adoptedStyleSheets`, `::part`). The baseline is fine; the
  niceties are progressive enhancements.
- **Editor-in-shadow-root is the risky case** — IME, the native spellcheck
  UI, and selection across the boundary. This is why the editor is
  light-DOM. (Verified: CodeMirror 6 has no such problem; Tiptap needs its
  CSS re-homed.)
- **Firefox and Safari are unverified.** Chromium only so far. The
  `getSelection()` retargeting behaviour is the most likely to differ, and a
  rule depends on it.

**Neutral:**

- The bridge is still needed. Shadow DOM gives a plugin the DOM, not the
  app's data model. Reading notes, rolling dice and subscribing to live state
  still go through the capability-scoped bridge.
- The frame remains available for embedding a third-party web page — a real
  feature that needs a real origin boundary and a `sandbox` attribute. It is
  simply no longer the *default* plugin surface.
- The WASM tier, the capability model, the mount points and the service
  registry are all unchanged.
