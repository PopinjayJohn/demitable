# Phase 3 — The Openings: O5 report (full UI surfaces)

## Summary

Milestone O5 lands the GM dashboard and the remainder of the `spikes/ui/`
surfaces that fit phase 3, with the projection's consequences carried into
the UI. The dashboard loads against the SRD vault through a single O2
query (`GET /api/v1/notes`) — no client vault reads — and renders inside
a stated budget. Landed surfaces (dashboard, note editor, player view,
visibility badge/inspector) all carry the disagree-fails discipline; the
character panel and full markdown construct renderer are deferred to
phase 4 with reasons recorded in `SURVEY.md`.

## Survey checklist

See `SURVEY.md` (canonical). Landed: GmDashboard, NoteEditor, PlayerView,
VisibilityBadge, VisibilityInspector. Deferred: CharacterPanel (phase 4
owner), MarkdownView (partially landed; full renderer phase 4), Icon (cut).

## Predicates

- Dashboard render budget: **250 ms** (stated). Measured full request set
  against the SRD vault (1,690 notes, GM principal, 5 runs):
  80.09, 72.23, 65.22, 65.27, 66.64 ms; p95 (max of 5) = **80.09 ms**.
  Budget met with margin. The client test (`web/src/lib/dashboard.test.ts`)
  fails when the budget is lowered below the measured time.
- Zero client vault reads: asserted by intercepting the request set in
  `loadDashboard` (`assertZeroVaultReads`); the test fails if a per-note
  (`/api/v1/notes/:uid`) or render request is made. Result: **0 vault
  reads** in the dashboard load path; the only request is `/api/v1/notes`.
- Disagree-fails: `surfaceVisibility()` requires a server-projected
  `effective_visibility` and throws on raw data; `assertVisibilityAgreement()`
  throws on mismatch. Covered per surface showing visibility (dashboard,
  player view) by `web/src/lib/dashboard.test.ts`.
- Go: `make lint` + `make test` green. No `fixtures/` in the diff.

## What is now known

- A single full-scan O2 query (~65-80 ms over 1,690 SRD notes) is cheap
  enough to be the dashboard's only request; an N+1 storm is unnecessary
  and would blow any reasonable budget.
- Exposing `effective_visibility` + `defaulted` on `NoteView` lets client
  surfaces display the projection without recomputing it.

## Files changed

- `internal/vault/view.go`, `internal/vault/visible.go` — add
  `EffectiveVisibility`/`Defaulted` to `NoteView` and populate them.
- `web/src/lib/api.ts` — `NoteSummary` carries projected visibility.
- `web/src/lib/dashboard.ts` — dashboard data layer, budget, vault-read
  guard, projection discipline helpers.
- `web/src/lib/dashboard.test.ts` — budget/vault-read/disagree-fails tests.
- `web/src/surfaces/GmDashboard.svelte`, `web/src/surfaces/PlayerView.svelte`.
- `web/src/App.svelte` — surface switching.
- `SURVEY.md`, `docs/phase-3-report.md`.

## Contradictions

None found.

## Not done + why

- Character panel and full markdown construct renderer deferred to phase 4
  (reasons in `SURVEY.md`).

## Unverified

- Cross-browser rendering of the new surfaces is not yet exercised by the
  Playwright matrix (O4 scope); surfaces use the existing shadow-host
  contract.
