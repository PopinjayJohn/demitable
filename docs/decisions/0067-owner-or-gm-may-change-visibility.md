# ADR-0067: Only the GM or the note's owner may change visibility or ownership

- **Status:** Accepted
- **Date:** 2026-11-10
- **Amends:** none (new rule on the PATCH/PUT write path)
- **Evidence:** phase-6 verification finding (R5 follow-up): any principal
  who could view a note could PATCH `visibility` or `player`
  (`cmd/demitable/main.go` PATCH/PUT handlers checked `Visible` only);
  hostile tests `TestVisibilityChangeRequiresOwnerOrGM` and
  `TestPutVisibilityChangeRequiresOwnerOrGM` in
  `cmd/demitable/field_auth_test.go`, each FAIL-probed

## Context

The rail permissions block (visibility quick-select) is client-side-gated to
the GM, and the server already enforces GM-only un-reveal. But the write path
did not: PATCH and PUT required only that the caller could *view* the note,
plus the seat and `submitted` guards. So any viewer could re-audience a note
(`visibility: party` → `gm`, or `gm` → `party` where visible) or take
ownership (`player` → self) and then re-audience it as owner. The second half
is why `player` is gated alongside `visibility`: gating visibility alone
leaves the ownership-transfer bypass open.

## Decision

**`visibility` and `player` are owner-or-GM fields on both write paths.**

- PATCH: if the request sets `visibility` or `player` (any envelope shape,
  case-insensitive) and the caller is neither the GM nor the note's owner,
  reject with 403. (403, not 404: the caller passed the `Visible` check, so
  the note's existence is already known to them.)
- PUT (full-file save): if the incoming content's *effective* visibility or
  owner differs from the note's current values and the caller is neither the
  GM nor the owner, reject with 403. Unparseable content is read as carrying
  no keys, so a body-only save over a defaulted note still lands.
- Owner means: the GM always; otherwise the note's `player` is non-empty
  and equals the caller's account id or, for a seat, its bound character.
  An owner may transfer ownership (set `player`) — that is delegation, and
  the new owner inherits the same rule.

This is a narrowing: no previously legitimate write is refused. GM and owner
writes are unaffected; viewer body edits that leave these two keys (PATCH)
or effective values (PUT) untouched proceed exactly as before.

## Consequences

**Good:**

- The visibility model can no longer be re-audienced by its audience. The
  quick-select is now a convenience over an enforced rule rather than a
  client-side promise.
- The ownership-transfer bypass is closed in the same rule, with the reason
  stated.

**Bad:**

- A non-owner, non-GM integration that used to set `visibility` (e.g. a
  script normalising audiences) now gets 403. That was never a legitimate
  use; it now fails loudly instead of silently succeeding.

**Neutral:**

- `workflow: submitted` and the seat guards are unchanged and compose with
  this rule (each is checked independently).
- Unknown `visibility` values still fail closed to `gm` in the projection;
  this ADR governs who may write the key, not how unknown values read.
