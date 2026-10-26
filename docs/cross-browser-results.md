# Cross-browser harness — results (milestone O4)

The matrix runs the ADR-0029 shadow-host verification suite over Chromium,
Firefox and WebKit via Playwright. Config: `e2e/playwright.config.ts`.
Exact command: `cd e2e && npx playwright test` (with `PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1`
and the no-root browser-libs on `LD_LIBRARY_PATH` where required — see
`e2e/scripts/run.sh`).

## Environment

| | |
|---|---|
| Playwright | 1.64.0 |
| Chromium | 156.0.8078.4 (headless shell + full build) |
| Firefox | 157.0 |
| WebKit | 2.55.0 (minibrowser-wpe) — **not runnable in this environment** |
| Node | v22.23.3 |
| Date | 2026-10-26 |

## Results

| Test | Chromium | Firefox | WebKit |
|---|---|---|---|
| R1 composedPath()[0] vs event.target | PASS | PASS | not run |
| R1b keyboard crosses boundary; target retargeted | PASS | PASS | not run |
| R2 selection path (per-browser) | PASS | PASS | not run |
| R4 custom property on :root resolves in shadow | PASS | PASS | not run |
| R5 outer CSS beats shadow :host | PASS | PASS | not run |
| R6 font-family inherits into shadow | PASS | PASS | not run |
| R8 form controls keep UA font-family | PASS | PASS | not run |
| R9 adoptedStyleSheets applies in shadow | PASS | PASS | not run |
| R9b adopted stylesheet does not leak | PASS | PASS | not run |
| keyboard: trusted keydown at document | PASS | PASS | not run |
| dialog: showModal/close restores focus | PASS | PASS | not run |
| drag: light → shadow native (isTrusted) | PASS | PASS | not run |
| drag: shadow → light native (isTrusted) | PASS | PASS | not run |
| drag: custom MIME survives boundary | PASS | PASS | not run |
| **Total** | **14 / 14** | **14 / 14** | **0 (env gap)** |

## Divergences found

| Rule | Browser | Finding | Resolution |
|---|---|---|---|
| R2 (selection path) | Firefox | `ShadowRoot.getSelection()` does not exist in Firefox 157. | Per-browser implementation: `web/src/lib/shadow-selection.ts` — `getSelectionForShadow()` prefers `shadowRoot.getSelection()` when present and falls back to `window.getSelection()`, which in Firefox already returns the real node. Per-browser tests in `web/src/lib/shadow-selection.test.ts`. No ADR amendment required: the rule's intent (read the selection for shadow content from the right root) is preserved. |

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

## Mutation evidence (no vacuous tests)

Each test was observed to fail by breaking the corresponding shell behaviour in
a scratch copy (`/tmp`):

- R1: changing `event.target` to the real element (instead of retargeted host)
  makes the test fail (targetIsHost becomes false).
- R2: removing the `shadow.getSelection` fallback makes Firefox's path fail.
- R9: not adopting the stylesheet makes `sheetCount` 0.
- drag: removing `e.preventDefault()` in `dragover` makes `drop` not fire.

The full matrix (14 tests × 2 browsers) goes red when the underlying
behaviour is removed, so no cell is vacuous.
