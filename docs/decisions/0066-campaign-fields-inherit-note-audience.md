# ADR-0066: Campaign-declared fields inherit the note's audience

- **Status:** Accepted
- **Date:** 2026-10-29
- **Amends:** ADR-0033 (the campaign scope — the serialisation, not the model)
- **Evidence:** phase-6 R3 hostile suite (7/7 per-principal tests in `cmd/demitable/r3_campaign_fields_test.go`, raw-bytes asserts, independent hostile re-verification over all 7 campaign routes), each FAIL-probed (sibling-smuggle, gate-drop, Project-gate probes all red)

## Context

The wiki Home view needs `members`, `packs`, and `home` machine-readable.
Non-reserved fields are never serialised today, so serving any of them is a
visibility-model change: `members` is a list of account ids, and the audience
that may see it had never been decided. Until now the webapp resolved `home:`
where the raw file was readable (GM view) and fell back to the `type: home`
convention otherwise, and never displayed `members` or `packs`.

## Decision

**The three campaign-declared fields (`members`, `packs`, `home`) are served
to exactly the note's own audience — whoever may see the note.** The
serialisation point is single (`applyCampaignFields`, called from `View` and
from the `Visible`-guarded `Project`); every HTTP path serving them reuses the
existing `previewPrincipal` + `auditNoteRead` + `Visible` gate, so an unknown
or hidden campaign note is a bare 404 with an empty body (ADR-0026), and a
non-member, anonymous, or foreign-seat caller sees nothing. No other
non-reserved field crosses the boundary: a sibling custom field stays absent
for non-GM callers, pinned by a committed smuggling test.

**This amends the serialisation, not the model.** Campaign membership, the
grant sets, and the audience computation (ADR-0057) are unchanged. A campaign
note's audience is its campaign's members plus the GM; the fields inherit it
rather than defining a second audience per field. Splitting `members` into a
GM-only field would contradict the single-audience requirement and break the
Home view for the members it names.

**Discovery stays flat.** This decision creates no `discovery` note type and
changes nothing about `discovered`: the flat owner-scoped list (ADR-0050
D4/D5) stands, and R6 records the granularity disposal separately. Keeping the
reserved set at twelve names is a consequence, not a coincidence.

## Consequences

**Good:**

- The Home view resolves `home` from the machine-readable field first; the
  raw-file read and the `type: home` fallback remain only as GM-view
  fallbacks.
- `members` sensitivity is handled by not widening: the ids travel exactly
  as far as the note does, and no further. Threat-model asset row 4
  (party-visible surfaces must not publish what the audience may not see)
  holds by construction.

**Bad:**

- A party member sees the full membership list, including account ids. That
  is the decided audience — widening it further (e.g. public campaigns) or
  narrowing `members` to GM-only each needs its own ADR.

**Neutral:**

- `discovered` is still never emitted to non-owners; the R3 diff adds no
  grant, discovery, or seat rule.
