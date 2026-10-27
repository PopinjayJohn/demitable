# ADR-0063: F0 closes with WebKit red-with-reason; F2 surfaces wait for green

- **Status:** Accepted (landed 2026-10-27; records the F0 gate outcome for the
  WebKit column — an environmental gap with zero behavioral evidence)
- **Date:** 2026-10-27
- **Amends:** nothing (it constrains phase 4, wave 3 — it does not change a
  prior decision)

## Context

The phase-4 plan's F0 gate requires the WebKit column of the O4 shadow-host
matrix to run "green, or red with an ADR number — before any F2 surface work
starts." Phase 3 closed O4 at 15/15 Chromium and 15/15 Firefox with WebKit
`not run` (EGL gap), explicitly filing no ADR for an infra gap with zero
behavioral evidence. F0 re-probed it twice (2026-10-26 baseline, 2026-10-27
F0c): the WebKit 2.55.0 binaries launch MiniBrowser, which aborts with
`Could not create EGL display: no supported platform available` — no
`DISPLAY`, no `/dev/dri`, no Xvfb in this environment. The harness is
unchanged; there is no divergence data because nothing ran. Full record in
`docs/cross-browser-results.md` (F0 entry).

## Decision

- F0 closes with the WebKit column **red-with-reason (environmental)** and no
  behavior claim in either direction. Filing a behavioral ADR on zero runs
  would repeat what phase 3 correctly refused to do.
- **No F2 surface work that builds on the shadow host (F2c tabletop, F5
  surfaces) starts until the O4 matrix runs green on a GL/EGL-capable runner**
  (or Xvfb+Mesa), via `npx playwright install --with-deps webkit` as root or
  the `e2e/scripts/run.sh` syslibs recipe. The requirement is documented in
  `docs/cross-browser-results.md`, not invented here.
- F2a (the session-tier cadence measurement) and F5a server-side work do not
  touch the shadow host and are not gated by this.

## Consequences

- A Safari divergence found after the tabletop is built costs more than one
  found now — that is why the gate sits before F2 surfaces, not after them.
- When the green run exists, it lands as a matrix update in
  `docs/cross-browser-results.md`, not as a further ADR, unless it diverges —
  in which case the divergence table plus an amending ADR is the deliverable
  (phase-4 brief §8).
