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
