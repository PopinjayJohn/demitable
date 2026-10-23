# ADR-0052: The projection's consequences belong in the UI

- **Status:** Accepted
- **Date:** 2026-10-23
- **Extends:** ADR-0033 (the visibility model), ADR-0034 (accessibility and
  localisation), ADR-0035 (secret callouts)
- **Addresses:** ADR-0033's "Bad" consequence — "the effective visibility
  depends on six rules in order ... it needs a 'why can this role see this
  note?' inspector in the UI"

## Context

The visibility model resolves a note per principal through six ordered rules:
scope, absolute deny, workflow gate, base level, grants, default. A GM who
cannot tell *why* a note is shared will eventually publish something meant to
stay private. ADR-0033 asks the UI to explain the answer, and ADR-0034 forbids
encoding state in colour alone.

## Decision

### Visibility badge — four channels

Every note badge shows the **effective** visibility, not the declared one, in
four channels:

1. **A glyph** — lock / users / globe / user / eye-off
2. **A word** — "GM only" / "Party" / "Public" / "Owner" / "Private"
3. **A border pattern** — solid / dashed / double-dash / long-gap dash /
   dash-gap-dash — the channel that survives greyscale and colour-blindness
4. **A hue** — the only channel allowed to fail

A defaulted value (no `visibility` field; ADR-0033 §6) carries a `*` marker
and a tooltip, so a GM sees at a glance that a note is shared by default rather
than by choice.

### The inspector

Beneath every opened note, the inspector lists the rule chain that produced
the answer, citing ADR-0033 section numbers — scope, absolute deny, workflow
gate, base level, grant, default — so a GM can audit *why this role sees it*.

### Secrets

A `> [!secret]` block inside a shared note is stripped from the non-GM
projection; the text never reaches a non-GM client (ADR-0035). The GM's own
view renders it distinctly — a lock affordance and a reveal control — never
relying on colour alone (ADR-0034).

## Consequences

**Good:**

- The six-rule model is *legible*: a GM can point at the inspector and name
  the rule that decided.
- The badge's four channels satisfy ADR-0034's colour-independence rule;
  verified against greyscale, flat-hue and deuteranopia proofs.

**Bad:**

- The inspector is one more UI surface to keep in step with the projection
  implementation.

**Neutral:**

- A campaign-less note defaults to `gm`; the scaffolder must stamp `campaign:`
  on new campaign notes, or they are invisible to the table (ADR-0033's
  neutral consequence).

---

