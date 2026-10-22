# Shadow DOM vs. sandboxed iframe as the plugin isolation boundary

**Verdict up front: the shadow-root decision is sound, and the specific reasons given for it
(keyboard, IME, clipboard, drag-and-drop) all hold up in a real browser. The two biggest
surprises are (1) `window.getSelection()` is unusable for shadow content, and (2) Tiptap injects
its ProseMirror CSS into `document.head`, where it cannot reach the shadow root.**

---

## Test environment

| | |
|---|---|
| Browser | Chromium **156** (`HeadlessChrome/156.0.8078.4` headless shell; full build `Chrome/156.0.0.0` also verified) |
| Driver | Playwright **1.64.0** (`/tmp/opencode/node_modules/playwright`) |
| Node / npm | v22.23.3 / 10.9.9 |
| Origin | `http://127.0.0.1:<port>` (secure context; `file://` avoided so the Clipboard API is reachable) |
| Editors | `codemirror` 6.0.2, `@codemirror/view` 6.43.15, `@codemirror/state` 6.7.6, `style-mod` 4.1.4, `@tiptap/core` 3.32.0, `@tiptap/starter-kit` 3.32.0, `prosemirror-view` 1.42.6, `prosemirror-model` 1.25.12 |
| Toolchain | `svelte` 5.57.2, `vite` 8.3.5, `@sveltejs/vite-plugin-svelte` 7.3.1, `svelte-check` 4.7.7, `typescript` 7.0.2 (native) / 6.0.3 / 5.9.3 |

Chrome had to be made runnable in this container: 40 shared libraries and the
`fontconfig` + `fonts-dejavu-core` packages were extracted to
`/tmp/opencode/shadowdom-proto/syslibs/root` and wired up with `LD_LIBRARY_PATH` +
`FONTCONFIG_FILE`. Without fonts, Chromium aborts at startup with
`FATAL:third_party/skia/src/ports/SkFontMgr_FontConfigInterface.cpp:163] Not implemented.`

All test sources live in `/tmp/opencode/shadowdom-proto/mine/` (`www/*.html` pages, `t*.mjs`
drivers, `lib/harness.mjs` runner). Nothing was written to `/home/johan/projects/sp-ng`.

---

## C1 — Shadow root is not a document boundary

**Verdict: VERIFIED**

### (a) `keydown` dispatched inside a shadow root is observed by a light-DOM ancestor

**Evidence** (`/tmp/opencode/shadowdom-proto/mine/www/c1.html`, driven by `t1_c1.mjs`):

```js
const shadow = host.attachShadow({ mode: 'open' });
shadow.innerHTML = `<input id="inner" type="text">`;
for (const t of ['keydown','keyup','keypress','compositionstart','beforeinput','input']) {
  shadow.addEventListener(t, e => rec('shadow-root', e));
  document.getElementById('light-ancestor').addEventListener(t, e => rec('light-ancestor', e));
  document.addEventListener(t, e => rec('document', e));
}
// real key press through the browser input pipeline:
await page.evaluate(() => { window.__R.events.length = 0; window.__R.inner.focus(); });
await page.keyboard.press('a');
```

**Observed** (real, trusted key press — `isTrusted: true`):

```
inner-input      keydown  trusted=True composed=True target=input#inner
shadow-root      keydown  trusted=True composed=True target=input#inner
light-ancestor   keydown  trusted=True composed=True target=div#host      <-- retargeted
document         keydown  trusted=True composed=True target=div#host      <-- retargeted
```

`documentSawKeydown: True`, `lightAncestorSawKeydown: True`.
`event.composedPath()` at the document level is
`["input#inner", "#shadow-root", "div#host", "div#light-ancestor", "body", "html", "document", "window"]`
— i.e. the real inner element is always recoverable via `composedPath()[0]`.

Synthetic control — `composed` is the deciding flag, not the event type:

```js
inner.dispatchEvent(new KeyboardEvent('keydown', { key:'K', bubbles:true, composed:true  }));
// -> shadow-root, light-ancestor, document   (3 listeners)
inner.dispatchEvent(new KeyboardEvent('keydown', { key:'K', bubbles:true, composed:false }));
// -> shadow-root ONLY
```

**Implication:** global keyboard shortcuts (`keydown` on `document`) work unchanged for plugin UI.
The only thing you must handle is **retargeting**: `event.target` is the host, not the inner
element. Any code that does `event.target.closest('.my-plugin-thing')` will fail; use
`event.composedPath()[0]` instead.

### (b) `compositionstart` / `beforeinput` from inside the shadow root are observable by the host document

**Evidence** — real IME driven through CDP `Input.imeSetComposition`:

```js
const cdp = await page.context().newCDPSession(page);
await cdp.send('Input.imeSetComposition', { text: 'にほん', selectionStart: 3, selectionEnd: 3 });
```

**Observed:**

```
shadow-root      compositionstart   trusted=True composed=True target=input#inner data=
light-ancestor   compositionstart   trusted=True composed=True target=div#host    data=
document         compositionstart   trusted=True composed=True target=div#host    data=
shadow-root      compositionupdate  trusted=True composed=True target=input#inner data=にほん
light-ancestor   compositionupdate  ...                                      data=にほん
document         compositionupdate  ...                                      data=にほん
```

`documentSawCompositionStart: True`, `lightAncestorSawCompositionStart: True`.

`beforeinput` also crosses, for both `<input>` and `contenteditable`:

```
shadow-root      beforeinput trusted=True composed=True target=input#inner inputType=insertCompositionText
document         beforeinput trusted=True composed=True target=div#host    inputType=insertCompositionText
document         beforeinput trusted=True composed=True target=div#content inputType=insertText
```

**Implication:** IME input (the "Japanese keyboard" case) and any `beforeinput`-based
input handling work identically inside a shadow root. Inside an iframe, IME composition would
be scoped to the iframe document and the host page would see nothing.

### (c) `navigator.clipboard.writeText` and `document.execCommand('copy')`

**Observed — with `clipboard-read` + `clipboard-write` permissions granted:**

```
secureContext: true, hasClipboard: true
writeText: "OK"
readText:  "OK: \"clipboard-from-shadow-test\""
execCommandCopyLegacy (light DOM textarea): true
```

**Observed — with permissions NOT granted** (`permissions: []`):

```
permissionState: "denied"
writeText: "ERROR NotAllowedError: Failed to execute 'writeText' on 'Clipboard': Write permission denied."
readText:  "ERROR NotAllowedError: Failed to execute 'readText' on 'Clipboard': Read permission denied."
```

Note the failure is **not** rescued by a user gesture: the click on `#do-copy` happened and
`writeText` still threw. But:

```
withGesture.execCopyShadowResult: { execCommandReturned: true }   // legacy path still works
```

**`document.execCommand('copy')` on a selection inside the shadow root works:**

```js
const sel = shadow.getSelection();
sel.setBaseAndExtent(span.firstChild, 0, span.firstChild, 8);   // "SpanText"
document.execCommand('copy');   // from a click handler
```
→ `execCommandReturned: true`, and `navigator.clipboard.readText()` afterwards returned
`"SpanText"`. The control (light-DOM selection) returned `"LIGHTSELT"` the same way.

**Implication:** both clipboard paths are available to plugin code. The async Clipboard API needs
the `clipboard-write` permission (denied → `NotAllowedError`); `document.execCommand('copy')` only
needs transient user activation and is the more robust choice for a copy button.

### (d) `window.getSelection()` behaviour for content inside a shadow root

This is the most interesting result of the whole exercise.

**Observed** — with a selection made *inside* the shadow root (`shadow.getSelection().setBaseAndExtent(...)`):

```
docSelection:  anchor=div#light-ancestor anchorOffset=1 focus=div#light-ancestor focusOffset=1
               type="Range" toString="SpanText" rangeCount=1
docRange0:     startContainer=div#light-ancestor startOffset=1
               endContainer=div#light-ancestor endOffset=1
               commonAncestor=div#light-ancestor          <-- COLLAPSED, and not the host
shadowSelection: anchor=text("SpanTextBeta") anchorOffset=0 focus=text("SpanTextBeta") focusOffset=8
                  rangeCount=1, getRangeAt(0) -> real text node
docContainsRealTextNode: true
docSelIsShadowSel: false     (different JS objects)
```

So `window.getSelection()`:
* **does** report the right text via `.toString()` ("SpanText"),
* but its `anchorNode` / `getRangeAt(0)` are **retargeted to a collapsed position in the host's
  parent** (`div#light-ancestor`, offsets 1/1) — *not* to the host, and *not* to the real text node.

Additional probes:

```
// document.execCommand('selectAll')
doc_toString    = "C1\ncopy shadow selection copy light selection\nLIGHTSELTEXT"   <-- shadow content ABSENT
shadow_toString = same string
shadow_rangeCount = 0

// selection spanning the boundary
dsel.setBaseAndExtent(lightTextNode, 0, shadowTextNode, 8)
-> doc_toString = "copy shadow selection copy light selection\n"   (shadow portion silently dropped)
-> shadow_rangeCount = 0
```

**Implication (important):** `window.getSelection()` is **not** a usable API for shadow content.
`.toString()` is fine, but any code that inspects `anchorNode` / `getRangeAt(0)` to find out
*what* is selected will get a collapsed range at a meaningless offset. Plugin code must use
`shadowRoot.getSelection()`. `document.execCommand('selectAll')` does **not** select shadow
content. A selection cannot span the shadow boundary — the shadow part is silently dropped, so
"select all" inside a plugin is a plugin-level operation, not a document-level one.

---

## C2 — HTML5 drag-and-drop crosses a shadow boundary within one document

**Verdict: VERIFIED (both the synthetic event model and the native browser drag path)**

### Caveat, stated up front

The task's caveat is correct as far as it goes, but it is narrower than "synthetic mouse events".
I ran **two** distinct experiments:

1. **Synthetic `DragEvent`s** — `new DragEvent(type, { dataTransfer: new DataTransfer() })`
   dispatched by hand. This validates the **event model and retargeting rules only**.
2. **The native path** — Playwright's `page.dragAndDrop()`, which in Chromium does
   `Input.setInterceptDrags({enabled:true})` → the browser's real `DragController` detects the
   drag and reports `Input.dragIntercepted` with the `dragData` the page's own `dragstart`
   handler produced → Playwright then calls `Input.dispatchDragEvent(...)`, so **the browser
   dispatches the events into the page** with a browser-managed `DataTransfer`.

Evidence for (2) from Playwright's own source
(`/tmp/opencode/node_modules/playwright-core/lib/coreBundle.js:36783-36866`):
`Input.dispatchDragEvent`, `Input.dragIntercepted`, `Input.setInterceptDrags`.

The decisive evidence that the native path really is native: **`event.isTrusted === true`** on
every drag event, and `dataTransfer.getData()` returns the payload that the *page's own*
`dragstart` handler wrote — a synthetic `DataTransfer` cannot do that.

What the native path does **not** cover: the OS-level drag loop (drag image, dragging into
another browser window, dropping onto OS surfaces). Those are irrelevant to the tabletop use case.

### Setup

```html
<div id="light-src" draggable="true">LIGHT_SOURCE</div>
<div id="light-target" class="dropzone">LIGHT_TARGET</div>
<div id="host"></div>
<!-- shadow root: -->
<div id="shadow-src" draggable="true">SHADOW_SOURCE</div>
<div id="shadow-target" class="dropzone">SHADOW_TARGET</div>
```

Realistic app-level handlers (needed, otherwise no `drop` ever fires):

```js
src.addEventListener('dragstart', e => {
  e.dataTransfer.setData('text/plain', payload);
  e.dataTransfer.setData('application/x-monster', JSON.stringify({id:42,name:'Gelatinous Cube'}));
});
tgt.addEventListener('dragover', e => { e.preventDefault(); e.dataTransfer.dropEffect = 'copy'; });
tgt.addEventListener('drop',     e => { e.preventDefault(); });
```

### Observed — light-DOM source → shadow-DOM target

Synthetic:
```
document  dragstart  trusted=False target=div#light-src     path0=div#light-src
shadow-root dragenter/dragover       target=div#shadow-target path0=div#shadow-target
document  dragenter  target=div#host  path0=div#shadow-target  getData='payload-light-to-shadow'
document  dragover   target=div#host  path0=div#shadow-target  getData='payload-light-to-shadow'
document  drop       target=div#host  path0=div#shadow-target  getData='payload-light-to-shadow'
```

Native (`page.dragAndDrop('#light-src', '#shadow-target')`):
```
document  dragstart  trusted=True  target=div#light-src     path0=div#light-src     types=['text/plain','application/x-monster']
shadow-root dragenter trusted=True  target=div#shadow-target
document  dragenter  trusted=True  target=div#host           path0=div#shadow-target types=[...] getData='' (protected mode)
document  dragover   trusted=True  target=div#host           path0=div#shadow-target types=[...] getData='' (protected mode)
shadow-root drop      trusted=True  target=div#shadow-target
document  drop       trusted=True  target=div#host           path0=div#shadow-target types=[...]
                              getData='payload-light-to-shadow'   <-- WORKS
```

### Observed — shadow-DOM source → light-DOM target

Synthetic:
```
shadow-root dragstart target=div#shadow-src
document   dragstart target=div#host  path0=div#shadow-src  getData=''
document   dragenter target=div#light-target  path0=div#light-target  getData='payload-shadow-to-light'
document   drop      target=div#light-target  path0=div#light-target  getData='payload-shadow-to-light'
document   dragend   target=div#host  path0=div#shadow-src
```

Native:
```
document   dragstart trusted=True target=div#host  path0=div#shadow-src types=['text/plain','application/x-monster']
document   drop      trusted=True target=div#light-target path0=div#light-target getData='payload-shadow-to-light'
```

### Answers to the specific questions

| Question | Answer |
|---|---|
| Does `drop` fire? | **Yes**, in both directions, in both synthetic and native paths. |
| Is `event.target` retargeted to the shadow host? | **Yes**, when the drop target is inside the shadow root: `target = div#host` at the document level. When the drop target is in the light DOM, `target` is the light element (no retargeting needed). The `dragstart` of a shadow source is also retargeted to the host. |
| Does `event.composedPath()[0]` give the real inner element? | **Yes** — `div#shadow-target` / `div#shadow-src`, at every level. |
| Does `dataTransfer.getData()` work in the drop handler? | **Yes** — custom MIME types too (`application/x-monster` → `{"id":42,"name":"Gelatinous Cube"}`). |

**Implication:** the tabletop's "drag a monster from the sidebar onto the map" works across the
shadow boundary **in both directions**, including custom drag payloads, with native (trusted)
drag events. The only required discipline is: **always use `event.composedPath()[0]` (or check
`event.target.shadowRoot?.activeElement`) instead of `event.target`**, because `event.target`
is the host. This is a much smaller tax than the iframe alternative, which breaks the drag
entirely across the boundary.

---

## C3 — Focus behaviour

**Verdict: VERIFIED**

### `document.activeElement` / `shadowRoot.activeElement`

```
focus inner input of plain shadow:
  documentActiveElement      = div#host-plain     <-- the host, as expected
  plainShadowActiveElement   = input#in-plain     <-- the real element
  deepActive = ["div#host-plain", "input#in-plain"]
```

The standard walk works:

```js
let el = document.activeElement;
while (el) { chain.push(el); el = el.shadowRoot ? el.shadowRoot.activeElement : null; }
```

### Light-DOM `<dialog>` with `showModal()`

```
before showModal: documentActiveElement=div#host-plain, plainShadowActiveElement=input#in-plain
after  showModal: documentActiveElement=button#dlg-a, plainShadowActiveElement=null
```

So **`showModal()` moves focus out of the shadow root** (`shadowRoot.activeElement` → `null`) and
focuses the dialog's first control. Focus then cycles **inside** the dialog:

```
Tab        -> button#dlg-b
Tab        -> body            (Chrome's modal dialog wraps to the browser chrome; in headless
                               that is <body> — focus does NOT leak into the shadow root)
Shift+Tab  -> button#dlg-b
```

**`dialog.close()` restores focus into the shadow root correctly:**

```
after dialog.close():
  documentActiveElement    = div#host-plain
  plainShadowActiveElement = input#in-plain
```

That is a nice result — Chrome restores focus to the pre-modal element even though it lives
behind a shadow boundary.

**Scope of the modal trap** (tested with the shadow host *inside* vs *outside* the dialog):

```
dialog CONTAINING the shadow host:  Tab -> host-in-dialog (input#in-dlg) -> host-in-dialog
                                     (button#btn-dlg) -> button#d2-b -> body
dialog NOT containing the host:     Tab cycles d3-a -> d3-b -> body; the outside shadow NEVER
                                     receives focus
```

**Implication:** a modal `<dialog>` in the light DOM is a safe focus trap; plugin shadow content
cannot steal focus from it unless the host element is physically inside the dialog.

### `attachShadow({ mode: 'open', delegatesFocus: true })`

```
hostDelegates.focus():
  documentActiveElement        = div#host-delegates    <-- SAME as without delegatesFocus
  delegatesShadowActiveElement = input#in-deleg        <-- DIFFERENT: focus delegated inward
  hostDelegatesTabIndex        = -1
```

* `document.activeElement` is **unchanged** by `delegatesFocus` — it is the host either way.
* What changes is *where inside* the shadow the focus lands.
* A **plain** (non-`delegatesFocus`) host is **not focusable at all**:
  `hostPlain.focus()` → `document.activeElement` stays `body`; `hostPlain.tabIndex === -1`.
* A `delegatesFocus: true` host **is** focusable even with `tabIndex === -1`:
  `hostDelegates.focus()` → host focused, delegated to `input#in-deleg`.
* A **real mouse click** on a `delegatesFocus: true` host focuses it and delegates inward:
  `document.activeElement = div#host-deleg`, `delegShadowActiveElement = input#in-d`.
  A real click on a plain host focuses **nothing** (`document.activeElement = body`).
* A **synthetic** `host.click()` does **not** focus anything (no user activation ⇒ no focus).
* A host with `tabindex="0"` **and no** `delegatesFocus`: `host.focus()` leaves the focus on the
  host (`shadowRoot.activeElement === null`), but **Tab-navigation** into the host moves focus
  *into* the shadow tree. Programmatic focus and sequential navigation behave differently here.

### Can a shadow host participate in a light-DOM focus trap?

**Yes — but only if the trap implements Tab itself.**

Trap implementation (light DOM, capture phase, on `document`):

```js
document.addEventListener('keydown', e => {
  if (e.key !== 'Tab' || trap.hidden) return;
  e.preventDefault();
  trapItems[(trapItems.indexOf(document.activeElement) + 1) % trapItems.length].focus();
}, true);
```

With focus inside the shadow root and the trap open, pressing `Tab`:

```
trap log: { trapSawKey: true, where: "document-capture", target: "div#host-plain" }
           { trapMovedFocusTo: "button#trap-1" }
final state: documentActiveElement = button#trap-1, plainShadowActiveElement = null
```

The trap **did** intercept the `Tab` (because `keydown` is composed) and moved focus out of the
shadow root. But note the trap only sees `target = div#host-plain` — **the host**, not the inner
element. To know what was focused inside the shadow, it must read `host.shadowRoot.activeElement`.

**Native tab order** (no trap): shadow content is *inline* in the light DOM's tab sequence.

```
Tab from #before  -> div#host-plain   (shadow activeElement=input#in-plain)
                  -> div#host-plain   (shadow activeElement=button#btn-plain)
                  -> div#host-delegates (shadow activeElement=input#in-deleg)
                  -> div#host-delegates (shadow activeElement=button#btn-deleg)
                  -> div#host-closed
Shift+Tab from #after -> div#host-tabbable (shadow activeElement=button#btn-tabbable)
```

**Implication (the trap-hole):** plugin shadow content participates in the host page's Tab order.
A light-DOM modal that relies on *native* tab order (rather than handling `Tab` itself) **will
leak focus into plugin shadow content**. A trap that handles `Tab` in a capture-phase
`document` listener can contain it — that is the recommended pattern.

Also note: a **closed** shadow root still reports `document.activeElement === host`, but the
inner element is unreachable (`host.shadowRoot === null`), so a trap cannot even tell what was
focused. Closed roots therefore make light-DOM focus management much harder.

---

## C4 — Style isolation and custom-property inheritance

**Verdict: VERIFIED**

### Setup

```css
:root { --brand: rgb(0,128,0); --spacing: 12px; --registered-noh: rgb(1,2,3); }
@property --registered-noh { syntax: '<color>'; inherits: false; initial-value: rgb(9,9,9); }
body  { font-family: "DejaVu Sans", sans-serif; color: rgb(1,2,3); line-height: 1.65; }
div   { padding: 3px; }                        /* generic type selector in the LIGHT DOM */
.light-probe { color: rgb(10,20,30); margin: 7px; border: 1px solid red; }
#light-important { color: rgb(4,5,6) !important; }
```

Shadow root of `#host-plain`:

```css
div { color: rgb(0,0,255); padding: 11px; }
.light-probe { color: rgb(0,0,255); margin: 11px; }   /* same class as the light element */
#shadow-brand { color: var(--brand); }
```

### A CSS rule inside the shadow root does not affect light-DOM elements

```
light #light-plain : color rgb(1,2,3)   padding 3px     <-- untouched by the shadow's rules
light #light-probe : color rgb(10,20,30) margin 7px     <-- untouched
```

The shadow's `div { padding: 11px }` and `div { color: rgb(0,0,255) }` did not leak out.

### A light-DOM rule does not affect shadow content except inherited properties

The shadow probe carries the **same class** (`light-probe`) as the light element:

```
shadow #shadow-probe:  color rgb(0,0,255)  (its own shadow rule)
                       margin 11px         (its own shadow rule)
                       border-top-width 0px  <-- light's `border: 1px solid red` did NOT apply
                       padding 11px          <-- light's `div{padding:3px}` did NOT apply
                       font-family "DejaVu Sans", line-height 26.4px   <-- INHERITED from the host
```

The light rule did not *match* the shadow element (color is the shadow's own value, not
`rgb(10,20,30)`), and the non-inherited properties (`margin`, `border`, `padding`) show no trace
of the light rule. Only the **inherited** values (`font-family`, `color`, `line-height`) carry
over — via inheritance, not via rule matching.

### Custom property set on `:root` is readable inside the shadow root

```
--brand on :root            -> rgb(0, 128, 0)
--brand on light #light-probe-> rgb(0, 128, 0)
--brand on shadow host       -> rgb(0, 128, 0)
--brand on shadow #shadow-probe -> rgb(0, 128, 0)
#shadow-brand { color: var(--brand) }  -> computed color rgb(0, 128, 0)   <-- resolves
```

**The theming mechanism works.** But there is a catch:

```
@property --registered-noh { inherits: false; initial-value: rgb(9,9,9) }
  on :root            -> rgb(1, 2, 3)    (the declared value)
  on light #light-probe-> rgb(9, 9, 9)    (initial value - does NOT inherit down)
  on shadow probe     -> rgb(9, 9, 9)    (initial value - does NOT cross the boundary)
```

A **registered** custom property with `inherits: false` does **not** reach into the shadow root.

### `font-family`, `color`, `line-height` inherit into the shadow root

```
shadow host + inner elements: font-family "DejaVu Sans", sans-serif
                               color rgb(1,2,3) (where not overridden)
                               line-height 26.4px
```

(Note: form controls do **not** inherit `font-family` — a `<button>` inside the shadow reported
`font-family: Arial`, the UA default. `font: inherit` is required, as CM6 does for itself.)

### `all: initial` on `:host` resets the inherited values

```css
:host { all: initial; }
```

```
host:  color rgb(0,0,0)          (initial / canvastext)
       font-family "Times New Roman"   (initial)
       line-height "normal"      (initial)
       margin 0px, border-top-width 0px, background-color rgba(0,0,0,0)
       display "inline"          <-- initial value: the host becomes an INLINE box (layout gotcha)
       padding "3px"             <-- NOT reset! see below
inner: color rgb(0,128,0) via var(--brand), font-family "Times New Roman", line-height "normal"
```

Three findings here:

1. `all: initial` **does** reset the inherited values (`font-family`, `color`, `line-height`).
2. `all: initial` does **not** reset **custom properties** — `--brand` still resolves to
   `rgb(0,128,0)` inside. (Confirmed: `all` excludes custom properties.)
3. The host's `padding` stayed `3px` even though `all: initial` should reset it — because the
   **light-DOM** rule `div { padding: 3px }` outranks the shadow's `:host { all: initial }`.

That third point is the cross-scope cascade, which I measured explicitly:

| # | outer-tree declaration | shadow-tree `:host` declaration | winner | host colour |
|---|---|---|---|---|
| t1 | `#t1 { color: rgb(1,1,1) }` normal | `:host { color: rgb(101,101,101) }` normal | **OUTER** | rgb(1,1,1) |
| t2 | `#t2 { color: rgb(2,2,2) }` normal | `:host { color: rgb(102,102,102) }` normal | **OUTER** | rgb(2,2,2) |
| t3 | `#t3 { color: rgb(3,3,3) }` normal | `:host { color: rgb(103,103,103) }` **!important** | **SHADOW** | rgb(103,103,103) |
| t4 | `#t4 { color: rgb(4,4,4) }` normal | `div { color: rgb(104,104,104) }` (no `:host`) | OUTER (host) / SHADOW (inner probe) | rgb(4,4,4) / probe rgb(104,104,104) |
| t5 | `#t5 { color: rgb(5,5,5) }` **!important** | `:host { color: rgb(105,105,105) }` normal | **OUTER** | rgb(5,5,5) |
| t6 | `#t6 { color: rgb(6,6,6) }` **!important** | `:host { color: rgb(106,106,106) }` **!important** | **SHADOW** | rgb(106,106,106) |
| t7 | `#t7 { color: rgb(7,7,7) }` normal | `:host { color: rgb(107,107,107) }` **!important** | **SHADOW** | rgb(107,107,107) |
| t10 | `div { color: rgb(8,8,8) }` normal (**specificity 0,0,1**) | `:host { color: rgb(110,110,110) }` normal (**specificity 0,1,0**) | **OUTER** | rgb(8,8,8) |
| t11 | (nothing targeted it; only `div{...}`) | `:host { color: rgb(111,111,111) }` normal | **OUTER** | rgb(8,8,8) |

Rules that fall out of this:
* For the **host element**, an outer-tree declaration beats a shadow-tree `:host` declaration at
  the same importance level — **scope beats specificity** (t10: outer `div` (0,0,1) beat shadow
  `:host` (0,1,0)).
* A shadow-tree `:host { ... !important }` beats **everything** in the outer tree, including an
  outer `!important` (t6, t8).

### `adoptedStyleSheets` works on a shadow root

```js
const sheet = new CSSStyleSheet();
sheet.replaceSync('#adopted-probe { color: rgb(7,7,7); --adopted-flag: yes; }');
shadow.adoptedStyleSheets = [sheet];
```

```
shadow.adoptedStyleSheets.length      = 1
shadow.adoptedStyleSheets[0] === sheet = true
colour before = rgb(1,2,3)   ->   after = rgb(7,7,7)
--adopted-flag from the adopted sheet = "yes"
light DOM unaffected (still rgb(1,2,3))
same sheet object adopted into two different shadow roots: allowed (true)
```

### Bonus: `::part()` still works from the host side

```css
/* app stylesheet */
#host-parts::part(primary-btn) { color: rgb(200,0,200); background-color: rgb(1,1,1); }
```
```
shadow button[part=primary-btn]: color rgb(200,0,200), background-color rgb(1,1,1)
```

**Implication:** the theming story works exactly as intended —
custom properties cross the boundary for free (except registered `inherits:false` ones),
`::part()` gives the app a sanctioned channel to style plugin internals, and `adoptedStyleSheets`
gives the plugin a modern, CSP-friendly way to inject its own styles. The one sharp edge is
that the **app can always override the plugin's `:host` styling with a plain CSS rule**, so
plugins must not rely on `:host { ... }` to establish critical layout; use custom properties or
`::part()` for that.

---

## C5 — Can a rich editor live in a shadow root?

**Verdict: VERIFIED — CodeMirror 6 works with zero changes; Tiptap/ProseMirror works but needs its
CSS re-homed into the shadow root.**

### CodeMirror 6

```js
const host = document.createElement('div');
document.body.appendChild(host);
const shadow = host.attachShadow({ mode: 'open' });
const cmHost = document.createElement('div');
shadow.appendChild(cmHost);

const view = new EditorView({
  state: EditorState.create({ doc: '# Hello shadow\n', extensions: [basicSetup, markdown()] }),
  parent: cmHost,
});
```

**Observed:**

```
headStylesBefore: 1, headStylesAfter: 1, headStyleDelta: 0     <-- NOTHING injected into document.head
shadow.adoptedStyleSheets: 1   (146 rules)
shadow.querySelectorAll('style').length: 0                     <-- no <style> tag either
editorInsideShadow: true, editorInDocument: false
view.root === shadow                                            <-- CM6 found the shadow root itself
computed: contentPadding "4px 0px", scrollerFontFamily "monospace", scrollerLineHeight "18.2px",
          editorDisplay "flex", editorHeight "44.375px", guttersPresent true
```

The computed values are **identical** to a control instance mounted in the light DOM
(`headStyleDelta: 1` there — a `<style>` in `document.head`), so the base theme is fully applied
inside the shadow root.

**Nothing had to change.** The mechanism (from `@codemirror/view@6.43.15` / `style-mod@4.1.4`):

```js
// @codemirror/view/dist/index.js:7981
this._root = (config.root || getRoot(config.parent) || document);
// @codemirror/view/dist/index.js:8383
StyleModule.mount(this.root, this.styleModules.concat(baseTheme$1).reverse(), ...);
// @codemirror/view/dist/index.js:753
function getRoot(node) {          // walks up to the Document (9) or ShadowRoot (11 with a host)
  while (node) {
    if (node && (node.nodeType == 9 || node.nodeType == 11 && node.host)) return node;
    node = node.assignedSlot || node.parentNode;
  }
}
```

`style-mod`'s `StyleSet` then takes the `adoptedStyleSheets` branch for a `ShadowRoot`
(`if (!root.head && root.adoptedStyleSheets && win.CSSStyleSheet)`) and constructs a
`CSSStyleSheet` — so the styles land in the shadow root via `adoptedStyleSheets`, not `<style>` tags.

Passing `root: shadow` explicitly gives byte-identical results (`headStyleDelta: 0`,
`adoptedStyleSheets: 1`, same computed styles) — useful if you ever need to override the detection.

**Typing works:**

```
after typing "WORLD": doc = "WORLD# Hello shadow\n"
view.hasFocus = true
document.activeElement = the host
shadowRoot.activeElement = DIV.cm-content
```

### Tiptap / ProseMirror

```js
const editor = new Editor({
  element: pmHost,                 // a div inside the shadow root
  extensions: [StarterKit],
  content: '<p>Hello tiptap</p>',
});
```

**Observed:**

```
pmRootIsShadowRoot: true, pmRootNodeType: 11, pmRootIsHost: true   <-- PM is shadow-DOM aware
pmDomInsideShadow: true, pmDomClass: "tiptap ProseMirror", contenteditable="true"
pmDomWhiteSpace: "normal"          <-- PM's CSS is NOT applied
pmDomPosition:  "static"
headStyleDelta: 1                  <-- Tiptap injected a <style> into document.head
shadow.adoptedStyleSheets: 0, shadow style tags: 0
```

Console warning from ProseMirror:

```
[warning] ProseMirror expects the CSS white-space property to be set, preferably to 'pre-wrap'.
It is recommended to load style/prosemirror.css from the prosemirror-view package.
```

**What had to change — the CSS.** The `<style>` Tiptap injects into `document.head` is:

```css
.ProseMirror { position: relative; }
.ProseMirror { word-wrap: break-word; white-space: pre-wrap; white-space: break-spaces;
               -webkit-font-variant-ligatures: none; font-variant-ligatures: none;
               font-feature-settings: "liga" 0; }
```

`document.head` styles **cannot** reach into a shadow root, so the editor renders with
`white-space: normal`. Tiptap v3 ships **no CSS files at all** (`find node_modules/@tiptap -name '*.css'`
→ empty), so there is nothing to import either.

Three fixes, all verified:

**Fix A — `injectCSS: false` + your own `<style>` in the shadow root (recommended):**
```js
const style = document.createElement('style');
style.textContent = `.ProseMirror { position: relative; word-wrap: break-word;
  white-space: pre-wrap; white-space: break-spaces; }`;
shadow.appendChild(style);

const editor = new Editor({ element: pmHost, extensions: [StarterKit],
                            content: '<p>Fixed</p>', injectCSS: false });
```
→ `headStyleDelta: 0`, `shadowStyleTags: 1`, `whiteSpace: "break-spaces"`, `position: "relative"`,
`wordWrap: "break-word"`, no PM warning.

**Fix B — let Tiptap inject, then move `editor.css` into the shadow root:**
```js
const editor = new Editor({ element: pmHost, extensions: [StarterKit], content: '…' });
shadow.appendChild(editor.css);   // <style> is re-parented out of document.head
```
→ `cssParentBefore: "HEAD"`, `cssParentAfter: "SHADOW-ROOT"`, `whiteSpace: "break-spaces"`.

**Fix C — adopt the CSS as a constructed stylesheet:**
```js
const sheet = new CSSStyleSheet();
sheet.replaceSync(`.ProseMirror { position: relative; word-wrap: break-word;
  white-space: pre-wrap; white-space: break-spaces; }`);
shadow.adoptedStyleSheets = [sheet];
const editor = new Editor({ …, injectCSS: false });
```
→ `adoptedStyleSheets: 1`, `whiteSpace: "break-spaces"`.

**Everything else about PM in a shadow root just works.** ProseMirror 1.42 is explicitly
shadow-DOM aware (`prosemirror-view/dist/index.js`):

```js
get root() { … }                 // "the shadow root if the editor is inside one"
return this.root.getSelection(); // uses ShadowRoot.getSelection(), not document.getSelection()
let viewState = view.state, active = view.root.activeElement;
let range = selection.getComposedRanges(view.root)[0];
```

**Typing and selection work:**

```
after typing " TYPED":
  html = "<p>Hello tiptap TYPED</p>"
  text = "Hello tiptap TYPED"
  selection = {"type":"text","anchor":19,"head":19}
  document.activeElement === host        (true)
  shadowRoot.activeElement = DIV.tiptap.ProseMirror.ProseMirror-focused
  view.hasFocus() = true
```

**One leak:** ProseMirror puts its `aria-live` announcement region in the **light DOM**
(`announceDomInLightDom: true` — a `[aria-live]` element found via `document.querySelector`),
even though the editor itself is fully contained (`lightDomProseMirrorNodes: 0`,
`shadowProseMirrorNodes: 1`).

**And the selection caveat from C1(d) bites here too:**

```
document.getSelection().toString()          = ""            <-- USELESS
shadowRoot.getSelection().toString()        = "Hello tiptap TYPED"   <-- correct
```

PM itself uses `view.root.getSelection()`, so PM is fine; but any *host-page* code that tries to
read the plugin's selection (e.g. a "copy selection" toolbar button) must reach into the shadow root.

**Implication:** CM6 is a clean win — zero changes, zero style leakage into `document.head`.
Tiptap works but the plugin must own its ProseMirror CSS (copy the rules into a `<style>` inside
the shadow root, or move `editor.css`), and any host-side code that reads the plugin's selection
must use `shadowRoot.getSelection()`.

---

## Bonus — Frontend toolchain

**Verdict: VERIFIED — Svelte 5 + Vite 8 + vite-plugin-svelte 7 works out of the box. TypeScript
7.0.2 works with `svelte-check`, but only via a specific two-install setup; a plain
`typescript@7.0.2` install breaks `svelte-check` hard.**

### Scaffold and build

```
$ npm create vite@latest spng -- --template svelte --no-interactive
# create-vite v9.3.1 -> Done.
$ npm install
added 39 packages in 6s
$ npm run build
vite v8.3.5 building client environment for production...
✓ 115 modules transformed.
dist/assets/index-CvmmMkxL.js   30.86 kB │ gzip: 12.42 kB
✓ built in 160ms
```

### Exact resolved versions

```
svelte                    5.57.2
vite                      8.3.5
@sveltejs/vite-plugin-svelte  7.3.1
svelte-check              4.7.7
typescript                7.0.2   (native/Go port, installed as @typescript/native)
typescript                6.0.3   (JS compiler — what the template pins by default)
typescript                5.9.3   (tested as the conservative baseline)
@types/node               24.19.2
@tsconfig/svelte          5.0.8
```

Note the current `create-vite` svelte-ts template pins `"typescript": "~6.0.3"`, **not** 5.x and
not 7.x. `svelte-check@4.7.7` accepts `>= 5.0 and <= 6.0` by default.

### Does `svelte-check` work with TypeScript 7.0.2?

**First: `typescript@7.0.2` alone breaks it — hard, with a version-guard error.**

```
$ npm i -D typescript@7.0.2
$ npx svelte-check --tsconfig ./tsconfig.app.json

node_modules/svelte-check/bin/ts-version-check.js:38
    throw new Error(message);
    ^

Error: TypeScript 7 support currently requires both TypeScript 7 and TypeScript 6 installed in your
project, and requires using the --tsgo or --tsgo-experimental-api flag. You can setup both version
with an npm alias via the following command.
npm install --save-dev typescript@~6 @typescript/native@npm:typescript@7

    at checkTypeScriptVersion (.../svelte-check/bin/ts-version-check.js:38:11)
EXIT=1
```

The root cause is visible in `typescript@7.0.2`'s `package.json`:

```json
"exports": { ".": "./lib/version.cjs", "./unstable/sync": "./dist/api/sync/api.js", … }
```

```js
$ node -e "const ts=require('typescript'); console.log(Object.keys(ts), typeof ts.createProgram)"
[ 'version', 'versionMajorMinor' ] undefined
```

**The TS 7 package does not export the JavaScript compiler API at all** — `require('typescript')`
returns only `{ version, versionMajorMinor }`. `svelte-check` needs the full API, hence the guard.

**Second: the documented two-version setup works.**

```
$ npm i -D typescript@~6 @typescript/native@npm:typescript@7
added 1 package, and changed 1 package in 1s

$ npm ls --depth=0
├── @typescript/native@npm:typescript@7.0.2
├── svelte-check@4.7.7
├── svelte@5.57.2
├── typescript@6.0.3
└── vite@8.3.5

$ npx svelte-check --tsconfig ./tsconfig.app.json --tsgo
Loading svelte-check in workspace: …
Getting Svelte diagnostics...
svelte-check found 0 errors and 0 warnings
EXIT=0
```

To prove it was not silently no-op'ing, I injected two deliberate type errors:

```svelte
const broken: number = "not a number"
const alsoBroken: string = 42
```

```
$ npx svelte-check --tsconfig ./tsconfig.app.json --tsgo
src/lib/Counter.svelte:6:9
Error: Type 'string' is not assignable to type 'number'. (ts)
src/lib/Counter.svelte:7:9
Error: Type 'number' is not assignable to type 'string'. (ts)
svelte-check found 2 errors and 0 warnings in 1 file
EXIT=1
```

`--tsgo-experimental-api` works identically. So does **no flag at all** once `typescript` resolves
to 6.0.3 (the guard only fires when the `typescript` package itself is ≥ 7) — in that case
`svelte-check` uses the TS 6 JS compiler and the native port is simply available as `tsc`.

**Third: the conservative baseline works too.**

```
$ npm i -D typescript@5.9.3
$ npx svelte-check --tsconfig ./tsconfig.app.json
svelte-check found 0 errors and 0 warnings
EXIT=0
$ npm run build
✓ built in 146ms
```

**`npm run build` is unaffected by all of this** — it succeeded with TS 7.0.2 + 6.0.3, with
5.9.3, and with the plain JS template.

### Recommendation

**Ship TypeScript 5.9.x (or 6.x) now; do not make TypeScript 7 the primary `typescript` dependency.**

* **Recommended today:** `"typescript": "~5.9.3"` (or `"~6.0.3"`, what `create-vite` currently
  scaffolds). `svelte-check` and `tsc` both work with zero flags, zero ceremony, and it is the
  configuration every Svelte tool (svelte-language-server, Vitest, eslint typescript parsers,
  `svelte2tsx`) currently assumes.
* **If you want the native compiler's speed**, adopt the *two-install* form and keep TS 6 as the
  `typescript` dependency:

  ```jsonc
  "devDependencies": {
    "typescript": "~6.0.3",
    "@typescript/native": "npm:typescript@7",
  },
  "scripts": {
    "check": "svelte-check --tsconfig ./tsconfig.app.json --tsgo && tsc -p tsconfig.node.json"
  }
  ```

  This works today (verified above), but it is fragile: `svelte-check` gates it behind a
  version check that throws an *unguarded* exception, and it depends on an npm-alias trick that
  the TS team has not standardised. It is also not yet the default that `create-vite` produces.
* **Do not** put `"typescript": "^7"` in `devDependencies` as the only TypeScript — `svelte-check`
  dies with the version-guard error above and there is no compiler API to fall back on.

### Two toolchain gotchas found while testing

1. **The two TS installs fight over the `tsc` bin.** After installing both, `node_modules/.bin/tsc`
   symlinks to `../@typescript/native/bin/tsc` (TS 7), *not* `../typescript/bin/tsc` (TS 6). So
   `npm run check`'s `tsc -p tsconfig.node.json` silently runs the **native** compiler, and
   `npx tsc --version` reports `7.0.2`. After `npm uninstall @typescript/native`, npm removed the
   `.bin/tsc` symlink **entirely** instead of restoring TS 6's link, and `npm run check` failed with
   `sh: 1: tsc: not found` until a plain `npm install` re-created it.

2. **Bundle format matters when you self-host a library.** My first CodeMirror/Tiptap test bundle
   was loaded with a plain `<script src="bundle.js">` instead of `type="module"`. esbuild's ESM
   output then ran as a classic script, so the bundle's top-level `var DOMParser = class _DOMParser
   {...}` (ProseMirror's parser) **overwrote the global `window.DOMParser`**, and Tiptap blew up
   with `TypeError: Cannot read properties of undefined (reading 'forEach')` inside
   `new _DOMParser(...)`. Worth knowing if you ever ship a bundled plugin.

---

## Summary table

| Claim | Verdict | One-line evidence |
|---|---|---|
| **C1** Shadow root is not a document boundary | **VERIFIED** | Trusted `keydown`/`compositionstart`/`beforeinput` all observed at `document`; clipboard write + `execCommand('copy')` of a shadow selection succeed; `getSelection()` retargeted to a collapsed range. |
| **C2** HTML5 DnD crosses a shadow boundary | **VERIFIED** (synthetic **and** native, `isTrusted: true`) | `drop` fires both ways; `event.target` → host, `composedPath()[0]` → real element; `dataTransfer.getData()` works in the drop handler. |
| **C3** Focus behaviour | **VERIFIED** | `document.activeElement` = host, `shadowRoot.activeElement` = inner; `showModal()` pulls focus out and `close()` restores it into the shadow; `delegatesFocus` changes only the *inner* focus target; light-DOM traps can intercept Tab from shadow content. |
| **C4** Style isolation + custom-property inheritance | **VERIFIED** | Shadow rules don't leak out; light rules don't match shadow content (only inherited values cross); `--brand` on `:root` resolves inside; `all:initial` on `:host` resets inheritance but not custom props; `adoptedStyleSheets` works. |
| **C5** Rich editor in a shadow root | **VERIFIED** | **CodeMirror 6: works with zero changes** (auto-adopts its theme into the shadow root, `document.head` untouched). **Tiptap/PM: works**, but you must re-home the ProseMirror CSS into the shadow root (`injectCSS:false` + own `<style>`, or move `editor.css`) — Tiptap otherwise injects it into `document.head` where it cannot apply. |
| **Bonus** Svelte 5 + Vite 8 + vite-plugin-svelte 7 | **VERIFIED** | `npm create vite@latest -- --template svelte` installs; `npm run build` succeeds. Versions: svelte 5.57.2, vite 8.3.5, vite-plugin-svelte 7.3.1, svelte-check 4.7.7. |
| **Bonus** `svelte-check` with TypeScript 7.0.2 | **VERIFIED (conditional)** | `typescript@7.0.2` alone → hard failure (no compiler API exported). Works with `typescript@~6` + `@typescript/native@npm:typescript@7` **and** `--tsgo`. **Recommend TS 5.9.x/6.x as the primary dependency.** |

---

## Still INCONCLUSIVE / not tested

| Item | What is missing | How to settle it |
|---|---|---|
| **Native drag gesture in a real (headed) browser** | The native path was exercised through Playwright's CDP drag interception (`Input.setInterceptDrags` + `Input.dispatchDragEvent`), which produces **browser-generated, `isTrusted: true`** drag events with a browser-managed `DataTransfer`. But it bypasses the OS-level drag loop: no drag image, no drag-out-of-window, no drop onto native OS targets. | Run the same page headed on a desktop OS and drag with a physical mouse. Only matters if you plan to support dragging a token *out of* the browser window. |
| **Firefox / Safari** | Everything here is Chromium 156 only. Firefox and Safari differ on: `ShadowRoot.getSelection()` (Safari historically lacked it), `document.getSelection()` retargeting, `adoptedStyleSheets` in shadow roots (Safari 16.4+ supports it), and ProseMirror's Safari-specific shadow-selection workaround (visible in `prosemirror-view`: `safari && this.root.nodeType == 11 && deepActiveElement(...) && safariShadowSelectionRange(...)`). | Repeat the C1–C4 pages in Firefox and Safari via Playwright's `firefox`/`webkit` projects. **Do this before committing** — the `getSelection()` retargeting behaviour in C1(d) is the one most likely to differ. |
| **Native IME** | Real IME composition was driven through CDP `Input.imeSetComposition`, which does produce genuine trusted `compositionstart/update` events. A real OS IME (ibus/fcitx/macOS) was not used. | Test on a machine with a real IME installed. |
| **Drag-and-drop *within* one shadow root** (plugin→plugin) | Not tested; the mechanism is identical to the light→shadow case, but two plugins in two *different* shadow roots dragging onto each other was not exercised. | Extend the C2 page with two shadow hosts. |
| **Performance** | No measurement of the cost of many shadow roots, or of `adoptedStyleSheets` per plugin. | Benchmark with ~50 plugin instances. |
| **`svelte-check` under TS 7 with the `svelte2tsx`/language-server path** | `svelte-check --tsgo` was verified; the standalone `svelte-language-server` was not exercised against TS 7. | Install `svelte-language-server` and point an editor at it. |

---

## Surprises / gotchas

Ordered by how likely they are to bite.

1. **`window.getSelection()` is unusable for shadow content.** `.toString()` returns the right
   text, but `anchorNode` / `getRangeAt(0)` are **retargeted to a collapsed range at the host's
   position in its parent** (`div#light-ancestor`, offsets 1/1) — not to the host, not to the real
   text node. `containsNode(realTextNode, true)` returns `true`, which makes it look like it works.
   **Always use `shadowRoot.getSelection()`.** This will silently break any "copy the selected
   text" feature, and any ProseMirror/CodeMirror-adjacent selection logic in host code.

2. **Tiptap v3 injects its ProseMirror CSS into `document.head`** (hard-coded:
   `document.getElementsByTagName("head")[0].appendChild(styleNode)`), where it **cannot reach a
   shadow root**. The editor then renders with `white-space: normal` and ProseMirror logs
   *"expects the CSS white-space property to be set, preferably to 'pre-wrap'"*. Tiptap v3 also
   ships **zero `.css` files**, so there is nothing to import. Fix: `injectCSS: false` + your own
   `<style>` in the shadow root, or `shadow.appendChild(editor.css)` after construction.
   (CodeMirror 6 gets this right — it auto-adopts into the shadow root.)

3. **Shadow content participates in the host page's native Tab order.** Tab from a light-DOM button
   lands *inside* the plugin's shadow root (`document.activeElement` = host,
   `shadowRoot.activeElement` = the inner input). A light-DOM modal that relies on native tab
   order will leak focus into plugin content. The fix works (a capture-phase `keydown` on
   `document` sees the composed Tab) but the trap only sees `event.target === host`, so it must
   read `host.shadowRoot.activeElement` to know where focus was.

4. **Outer-tree CSS beats the plugin's `:host` rules — scope beats specificity.** A light-DOM
   `div { color: … }` (specificity 0,0,1) overrides a shadow `:host { color: … }` (0,1,0).
   Combined with `all: initial` on `:host` not resetting `padding` (because the light `div{padding:3px}`
   rule won), a plugin **cannot** rely on `:host` styling to establish critical layout.
   A shadow `:host { … !important }` does beat everything, including outer `!important`.

5. **`all: initial` on `:host` does not reset custom properties**, but it *does* reset
   `font-family`/`color`/`line-height` — **and it sets `display: inline`**, silently changing the
   host's layout. If you use `all: initial` for a clean slate, re-set `display` explicitly.

6. **Registered custom properties with `inherits: false` do not cross the shadow boundary.**
   `@property --x { inherits: false }` → the shadow root sees the *initial value*, not the value
   set on `:root`. Use unregistered custom properties (or `inherits: true`) for theming.

7. **`delegatesFocus` does not change `document.activeElement`** — it is the host either way. What
   it changes is *which* element inside the shadow gets focus. Also:
   * a plain shadow host is **not focusable at all** (`host.focus()` is a no-op, `tabIndex === -1`);
   * a `delegatesFocus` host **is** focusable despite `tabIndex === -1`;
   * a **synthetic** `host.click()` does not focus anything (no user activation);
   * a host with `tabindex="0"` and **no** `delegatesFocus`: `host.focus()` leaves focus on the
     host, but **Tab-navigation into it descends into the shadow tree**. Programmatic focus and
     sequential navigation disagree.

8. **A selection cannot span the shadow boundary.** `setBaseAndExtent(lightTextNode, 0,
   shadowTextNode, 8)` silently selects the *light-DOM* range between the two and drops the
   shadow portion. `document.execCommand('selectAll')` also excludes shadow content entirely.

9. **Form controls don't inherit `font-family` into a shadow root.** A `<button>` in the shadow
   reported `font-family: Arial` (UA default) while a `<div>` reported the inherited
   `"DejaVu Sans"`. Needs `font: inherit` (which CodeMirror does for itself).

10. **ProseMirror leaks an `aria-live` announcement element into the light DOM** (`document.body`)
    even when the editor lives entirely inside a shadow root. Harmless for CSS, but it means the
    shadow root is not a *complete* containment boundary for a11y helper DOM.

11. **TypeScript 7's npm package has no JavaScript compiler API.** `require('typescript')` under
    `typescript@7.0.2` returns only `{ version, versionMajorMinor }` — the package `main` is
    `lib/version.cjs` and the API is only under `./unstable/*`. Any tool that embeds the TS
    compiler (svelte-check, svelte-language-server, eslint parsers) will break unless it opts into
    the native binary explicitly.

12. **`typescript` and `@typescript/native` fight over `node_modules/.bin/tsc`.** With both
    installed, `.bin/tsc` points at the native one; after uninstalling `@typescript/native`, npm
    removed the symlink entirely and `npm run check` failed with `sh: 1: tsc: not found` until a
    plain `npm install` repaired it.

13. **A classic-script bundle can clobber a global.** Loading an esbuild ESM bundle with
    `<script src>` (no `type="module"`) ran its top-level `var DOMParser = class _DOMParser{…}`
    (ProseMirror's parser) **as a global**, overwriting `window.DOMParser` and breaking Tiptap with
    `TypeError: Cannot read properties of undefined (reading 'forEach')`.

14. **Playwright's selector engine pierces open shadow roots; `document.querySelector` does not.**
    `document.querySelector('#shadow-target')` → `null`, but `page.locator('#shadow-target').count()`
    → `1`. Useful for tests; a reminder that E2E selectors will "just work" while app code
    cannot reach into a shadow root at all.

15. **`event.target` is retargeted to the host for *every* composed event**, including `keydown`,
    `beforeinput`, `dragstart`, `dragover` and `drop`. Any host-page handler that does
    `event.target.closest(...)` or `event.target.matches(...)` to identify which plugin element
    was involved **will silently match the host instead**. Use `event.composedPath()[0]`.

16. **`dialog.close()` restores focus into the shadow root correctly** (`shadowRoot.activeElement`
    is the pre-modal element again) — this one is a pleasant surprise and is worth relying on.

17. **Modal `<dialog>` focus wrapping goes to `<body>`, not into shadow content.** Tab past the
    last control in a modal dialog lands on `document.body` in headless Chromium (Chrome's
    "browser chrome" focus stop). It does **not** leak into plugin shadow roots outside the dialog.
