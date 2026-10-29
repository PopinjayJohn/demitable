# ADR-0064: The UI reads audiences, not ordered rules

- **Status:** Accepted
- **Date:** 2026-10-29
- **Amends:** ADR-0052 (the ordered-rules language), design §5.4 (the ordered-rules phrasing)
- **Evidence:** phase-5 P0c inventory (SURVEY.md, 24/24 surfaces), phase-4 report §"ADRs landed" (proposed but not written)

## Context

ADR-0052 was written against ADR-0033's model: six rules in order, "the first
rule that applies wins", and an inspector that "lists the rule chain that
produced the answer, citing ADR-0033 section numbers". ADR-0057 then replaced
the algorithm: the audience is the intersection of every applicable
constraint, widened only by the grants permitted to widen it. The ordering is
not the algorithm; it never was, and phase 0 proved the ordering is where the
model breaks (seven of twenty projection-spike findings are one defect: "the
first rule that applies wins" means some rules never run, and the rules that
lose are the narrowing ones).

P0c found the residue: twelve locations across ADR-0052, design §5.4, the
threat model §3, api.md §10 and `VisibilityInspector.svelte` still speak of
ordered rules and rule chains. Worse, P0c found four surfaces
(`VaultExplorer`, `NoteEditor`, `Home`, `NoteContext`) that read
`effective_visibility` directly with a fail-open fallback — a missing
projection renders a permissive label. Only `PlayerView`, `Tabletop`, the
`VaultExplorer` badge and the dashboard counts go through the canonical
`surfaceVisibility()` / `assertVisibilityAgreement()` helper.

## Decision

**D1 — The inspector explains constraints, not a rule chain.** ADR-0052's
"rule chain citing ADR-0033 section numbers" becomes: the inspector lists the
applicable constraints (C1–C8) and widenings (W1–W3) per ADR-0057 and the
verdict each produced for the asking principal. It never names a "winning"
rule, because no rule wins. The api.md §10 "inspector decision chain" backlog
item inherits this: any future server-side chain endpoint exposes
per-constraint verdicts, never GM-only rule text to non-GM callers.

**D2 — Surfaces go through the canonical helper or record why not.** Every
surface that displays visibility uses `surfaceVisibility()` (fail on raw
data) and `assertVisibilityAgreement()` (fail when display contradicts the
projection). A surface with a reason not to is recorded-deferred in
SURVEY.md with that reason — the O5 standard, unchanged.

**D3 — A missing projection renders fail-closed.** A surface that has no
server-projected `effective_visibility` for a note renders it as `gm`
(GM-only) — never `party`, never `public`, never a permissive default. The
four P0c fail-open fallbacks (`?? "party"`, bare `effectiveVisibility`,
`catch { return "gm" }` is already correct) are defects against this rule;
fixing them is P0-scope follow-up work, not a new milestone. Rationale: a
label is a permission claim, and a claim made without the projection behind
it must be the narrowest one.

## Consequences

**Good:**

- The UI language matches the resolver: one shape ("intersect, then widen")
  from the projection to the badge to the inspector.
- D3 turns an accidental-class leak (fail-open display default) into a named,
  testable rule.

**Bad:**

- The inspector is still one more surface to keep in step with the
  projection (ADR-0052's own "Bad", unchanged).

**Neutral:**

- design §5.4's ordered-rules text and the threat-model §3 numbering stay as
  history; this ADR is the authoritative reading. A future pass may reword
  them, but no decision changes.
- The `VisibilityInspector` data source (no server-side constraint-verdict
  endpoint) stays a backlog item; the inspector explains from the projected
  verdict it already receives.
