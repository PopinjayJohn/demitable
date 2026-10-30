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

## D3 — web-fallback accepted gap

- **Locations:** `web/src/surfaces/Home.svelte:122`,
  `web/src/surfaces/NoteEditor.svelte:32`, plus pre-existing
  `web/src/components/NoteContext.svelte:28`.
- **Status:** Fail-closed correct (`?? "gm"` default) but no component test;
  no surface test harness exists in the repository.
- **Owner:** maintenance track.
- **Reopen trigger:** first component-test harness.
