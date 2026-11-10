# R5 trust remainders — acceptance record

## Item 4 — `session-log-proposals.json` segment text at rest

- **Decision:** GM-local acceptance (not redacted).
- **Reason:** The sidecar stores the last proposed segment text so the
  ownership guard can detect a GM rewrite and preserve it across re-runs
  (`TestP3ReviewEndToEnd`). Redacting the text would defeat that contract.
  The sidecar lives under `<vault>/.demitable/`, is written with mode 0600,
  is never served by any endpoint to non-GM principals, and is read only on
  the GM code path (`p3IsGM`).
- **Owner:** session-log subsystem maintainer.
- **ADR-0058 posture:** No segment text is logged; the sidecar is at-rest
  storage, not a log line, and ADR-0058 governs log lines. ADR-0058's
  redaction handler is unaffected.

## Item — `verifyToken` constant-time compare

- **Location:** `cmd/demitable/auth.go:111` (`subtle.ConstantTimeCompare`).
- **Reason:** Timing side channel on signature comparison closed. Property is
  not black-box-testable (timing behaviour is not observable through the API
  surface), so acceptance is recorded rather than asserted as a test.
- **Owner:** maintenance track.

## P1–P3 hand-review sign-off

- **Reviewer:** phase-6 session 2026-10-30.
- **Per-path verdicts:** all clean.
  - `cmd/demitable/p1_audio.go`: zero `slog` calls.
  - `cmd/demitable/p2_transcribe.go`: zero `slog` calls.
  - `cmd/demitable/p3_session_log.go`: zero `slog` calls.
  - `internal/audio`: zero `slog` calls.
  - `internal/transcribe`: zero `slog` calls.
  - Sidecar written with mode `0600`, GM-only.
- **Outcome:** accepted.

## D3 — web fallbacks (PINNED 2026-11-10, maintenance T1b)

- **Locations:** `web/src/surfaces/Home.svelte:122`,
  `web/src/surfaces/NoteEditor.svelte:35` (`effectiveVisibility ??
  effective_visibility ?? "gm"`), `web/src/components/NoteContext.svelte:33`,
  plus `web/src/surfaces/VaultExplorer.svelte:34` (`try surfaceVisibility
  catch → "gm"`).
- **Status:** PINNED. The first component-test harness
  (`web/src/test/harness.ts`, vitest + jsdom) carries one pin file per
  surface (`NoteContext.test.ts`, `Home.test.ts`, `NoteEditor.test.ts`,
  `VaultExplorer.test.ts` — 24 D3 assertions, 88 web tests green), each
  FAIL-probed against a `"party"`-flipped fallback (8 FAILs, 2 per surface).
  Production bundle verified unaffected (323-module baseline; test-only
  `resolve.conditions` scoped out of `vite build`).
- **Owner:** maintenance track.
- **Reopen trigger:** met — acceptance closed by the pins above.

## Maintenance re-affirmation (2026-11-10, T3)

- **Item 4 sidecar:** re-affirmed, owner unchanged (session-log subsystem
  maintainer). Hostile re-probe: canary segment text driven through
  review-update + propose-apply + promote; zero hits in slog/audit output
  (bite check confirmed the grep bites); sidecar created mode 0600;
  non-GM principals receive zero segment bytes on every route (raw-bytes;
  member-visible applied session-log content follows the audience model per
  ADR-0057). Adjacent surface now named explicitly: content-addressed journal
  blobs under `.demitable/blobs/` are likewise GM-local at-rest storage
  (GM-only serve path), unchanged since R5 — no behavior change, no ADR
  amendment.
- **`verifyToken` constant-time:** re-affirmed, owner maintenance track.
  Source byte-identical since R5; full-repo grep finds no new
  token/secret comparison sites (pre-existing GM-secret `!=` checks on
  rate-limited endpoints noted as a future-track surface, out of scope).
- **D3:** moved to PINNED above; no longer an acceptance.
