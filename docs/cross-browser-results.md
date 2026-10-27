# Cross-browser harness — results (milestone O4)

The matrix runs the ADR-0029 shadow-host verification suite over Chromium,
Firefox and WebKit via Playwright. Config: `e2e/playwright.config.ts`.

## Running the harness

The green cells below were produced by this command, which is also what the
CI `e2e` job runs:

```
cd e2e && npx playwright test --project=chromium --project=firefox
```

On a machine whose browsers need the no-root library tree, run the same
selection through the wrapper (it forwards its arguments to Playwright):

```
cd e2e && SYS=/path/to/syslibs bash scripts/run.sh --project=chromium --project=firefox
```

`SYS` defaults to a scratch path outside this repository; with `SYS` absent or
wrong, `run.sh` warns and falls back to the system loader rather than aborting.
A machine with the normal system dependencies needs neither the wrapper nor
`SYS`:

```
cd e2e && npm ci && npx playwright install --with-deps chromium firefox
```

The **aspirational full command** is `cd e2e && npx playwright test`: it runs
all three configured projects, including WebKit. It is not a reproducible
green command here — WebKit fails to launch for the EGL reason named below, so
use the `--project` form above when you want the two green columns.

## Environment

| | |
|---|---|
| Playwright | 1.64.0 |
| Chromium | 156.0.8078.4 (headless shell + full build) |
| Firefox | 157.0 |
| WebKit | 2.55.0 (minibrowser-wpe) — **not runnable in this environment** |
| Node | v22.23.3 |
| Date | 2026-10-26 |

Libraries: on a rootless machine the browsers' shared libraries come from a
vendored tree (`SYS`), built by `apt-get download` + `dpkg-deb -x` from the
Debian/Ubuntu packages named in `e2e/scripts/run.sh`. That tree is a build
input, like `fixtures/`, and is not committed. The vendored `fonts.conf` names
absolute paths inside its own tree, so a copied tree needs its `fonts.conf`
rewritten (or dropped) to match where it now lives.

## Results

| Test | Chromium | Firefox | WebKit |
|---|---|---|---|
| R1 composedPath()[0] vs event.target | PASS | PASS | not run |
| R1b keyboard crosses boundary; target retargeted | PASS | PASS | not run |
| R2 selection path (per-browser) | PASS | PASS | not run |
| R4 custom property on :root resolves in shadow | PASS | PASS | not run |
| R5 outer CSS beats shadow :host | PASS | PASS | not run |
| R6 `all:initial` on :host sets display:inline | PASS | PASS | not run |
| inheritance: font-family crosses into shadow | PASS | PASS | not run |
| R8 form controls keep UA font-family | PASS | PASS | not run |
| R9 adoptedStyleSheets applies in shadow | PASS | PASS | not run |
| R9b adopted stylesheet does not leak | PASS | PASS | not run |
| keyboard: trusted keydown at document | PASS | PASS | not run |
| dialog: showModal/close restores focus | PASS | PASS | not run |
| drag: light → shadow native (isTrusted asserted) | PASS | PASS | not run |
| drag: shadow → light native (isTrusted asserted) | PASS | PASS | not run |
| drag: custom MIME survives boundary | PASS | PASS | not run |
| **Total** | **15 / 15** | **15 / 15** | **0 (env gap)** |

## Divergences found

| Rule | Browser | Finding | Resolution |
|---|---|---|---|
| R2 (selection path) | Firefox | `ShadowRoot.getSelection()` does not exist in Firefox 157. | Per-browser implementation: `web/src/lib/shadow-selection.ts` — `getSelectionForShadow()` prefers `shadowRoot.getSelection()` when present and falls back to `window.getSelection()`, which in Firefox already returns the real node. Per-browser tests in `web/src/lib/shadow-selection.test.ts`. No ADR amendment required: the rule's intent (read the selection for shadow content from the right root) is preserved. |

**Adoption status.** `web/src/lib/shadow-selection.ts` is currently called by
nothing — only its own definition and unit test reference it. Calling it the
"single sanctioned way" is the intended contract, not an adopted one; it
becomes true when the first shadow-hosted surface mounts and calls it (O5c).
There is no shadow-hosted content in the app today, so no call site is
bypassing it either.

No other divergences: every other rule holds identically in Chromium and Firefox.

## WebKit gap (named)

WebKit (Playwright's `webkit` project, minibrowser-wpe) **cannot run in this
environment**. Launch fails with:

```
Could not create EGL display: no supported platform available. Aborting...
```

The headless MiniBrowser requires an EGL/GL display which is unavailable in
this container (no GPU, no virtual framebuffer with EGL). The browser
binaries and system libraries were installed (see `e2e/scripts/run.sh` and
the no-root syslibs), but the display dependency is environmental and cannot
be satisfied by extracting further packages.

Consequence: the matrix is green in **two of three** browsers here. The WebKit
column is recorded as `not run` with the gap named, not as green. CI on a
runner with a working GL/EGL stack (or a headed display) must close this
column; the harness itself is unchanged and WebKit is a configured project in
`e2e/playwright.config.ts`.

## F0 probe (2026-10-27)

Re-probed in the F0 environment on 2026-10-27 to confirm the WebKit column
before any F2 surface work starts. Result: **red with reason — environmental;
harness unchanged; no divergence data exists because nothing ran.**

- WebKit binaries present: 2.55.0 (minibrowser-wpe), matching the
  Environment table above.
- Launch attempt: MiniBrowser aborts with
  `Could not create EGL display: no supported platform available. Aborting...`
- Environment: no `DISPLAY`, no `/dev/dri`, no `Xvfb`; container has no GPU
  and no EGL-capable virtual framebuffer. This matches
  `docs/cross-browser-results.md` (WebKit gap section) and `e2e/scripts/run.sh`
  lines 3–5.
- No test executed, so no per-rule divergence data exists for WebKit; the
  column remains `not run` (not "failed").

**Unblock requirement (precise).** Run the WebKit column on a runner that
provides one of:

1. A GL/EGL-capable runner (GPU or Mesa software rasteriser with EGL), or
2. `Xvfb` plus Mesa (`Xvfb :99 -screen 0 1280x1024x24` with
   `DISPLAY=:99` and Mesa's `libEGL`/`libGLESv2` on the loader path), and
   install WebKit with its system libraries via either
   `npx playwright install --with-deps webkit` (as root) or the no-root
   syslibs recipe in `e2e/scripts/run.sh`.

Until one of those is available the column stays red-with-reason; the gate
decision (waiver vs. blocker before F2) is the orchestrator's, not this
document's. No ADR is filed for this infra gap: there is zero behavioral
evidence to record (phase-3 precedent).

## Deferred with reason

The harness covers the rules that can be asserted against the fixture today.
The items below are part of ADR-0029's verification set or O4a's brief and are
**not** in the matrix. They are named here so nothing is silently dropped, with
the owner that closes them.

| Item | Why it is not in this matrix | Closed by |
|---|---|---|
| IME (`compositionstart`, `compositionupdate`, `beforeinput`) | Phase-0 proved it in Chromium with CDP's `Input.imeSetComposition`; CDP is Chromium-only and Playwright has no portable IME driver for Firefox, so it cannot be a cross-browser cell yet. No divergence has been observed. | **O5c**, together with the "real OS IME" row in `shadow-dom-verification.md` §7. If the editor stays light-DOM (ADR-0021), only a third-party editor in a shadow root ever needs it. |
| Clipboard (`navigator.clipboard.writeText`, `execCommand('copy')` on a shadow selection) | Needs a permission grant and a user gesture; phase-0 covered Chromium only. | **O5c**, when the app's copy-selection command (the R2 consumer) exists — that is the code worth testing. |
| `::part()` | The fixture exports no part; phase-0 verified it from the host side in Chromium only. | **O4 follow-up** (cheap fixture addition) before **O5c** mounts the first plugin surface whose internals the app styles. No known divergence. |
| R3: dialogs must **trap Tab** themselves | The dialog test asserts `showModal()`/`close()` focus restore (what O4a names), not Tab containment. The trap is app code, and the fixture has no trap to test. | **O5c**, with the first app modal's component test: a capture-phase `keydown` listener on `document` reading `host.shadowRoot.activeElement`. |
| R7: "select all" is a plugin-level operation | `document.execCommand('selectAll')` excludes shadow content; the behaviour matters only once the app has a select-all command. | **O5c** (editor command suite). The editor is light-DOM today, so R7 has no shadow consumer yet. |
| R10: a shadow root is not a11y containment | The observation is specific to ProseMirror/Tiptap, which put their `aria-live` node in the light DOM; no shadow-hosted editor ships in phase 3. | **O5c**, only if a shadow-hosted editor is proposed; otherwise not applicable. |
| CodeMirror 6 in a shadow root | Phase-0 verified zero changes needed in Chromium; `e2e/` has no CodeMirror dependency, and the app mounts its editor in light DOM (ADR-0021). | **O5c**, only if a shadow-hosted source view is proposed. |
| OS-level drag gesture (dragging out of the browser window) | `page.dragAndDrop` is the browser's input path (`isTrusted: true` is asserted), but it bypasses the OS drag loop. | **Phase 4** (tabletop), and only if tokens may be dragged outside the window; see `shadow-dom-verification.md` §7. |
| Plugin → plugin drag across two different shadow roots | The mechanism matches the tested cases; the app has no two plugin surfaces yet. | **Phase 4** (bestiary → tabletop), where the interaction first exists. |
| Safari / WebKit column | Named in the section above: the environment cannot create an EGL display. | **O4b follow-up** on a runner with GL/EGL or a headed display. The harness needs no change. |

## Mutation evidence (no vacuous tests)

Each test was observed to fail by breaking the corresponding behaviour in a
scratch copy (`/tmp`):

- R1: changing `event.target` to the real element (instead of retargeted host)
  makes the test fail (targetIsHost becomes false).
- R2: removing the `shadow.getSelection` fallback makes Firefox's path fail.
- R6: removing `all: initial` from the reset host leaves `display: block`, so
  `resetDisplay` fails.
- R9: not adopting the stylesheet makes `sheetCount` 0.
- drag: removing `e.preventDefault()` in `dragover` makes `drop` not fire.

Each mutation above turns the corresponding cell red, so those cells are not
vacuous. The remaining cells read a computed value or an event flag that the
fixture does not synthesise (the browser produces it), and each fails if the
fixture stops producing the behaviour — R6 and the `isTrusted` assertions were
added on that basis.
