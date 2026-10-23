# ADR-0004: Phase 1 ships the schema plus one complete system pack, end to end

- **Status:** Accepted, amended twice — first to resolve the dice-engine

  phase boundary, then by the phase-1 cut to an MVP, which moved the
  formula engine, the dice engine and the 5e pack to phase 2. See
  [`phase-plan.md`](../phase-plan.md).
- **Date:** 2026-10-21
- **Note:** the "persisted session store" mentioned below is specified by
  ADR-0030 — SQLite, outside the vault.

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

There are two ways to start:

- **Content first.** Ship the note schema, templates and views as artifacts, with
  little or no code. Cheap, and validates the schema against real use before
  building UI on top of it.
- **Software first.** Build the app shell and the authoring experience early.
  More polished, but risks spending months building UI against a schema that
  needs three rewrites after real use.

A third option, adopted here: **the schema and one system pack, exercised
end to end by a working app.** Not a schema on paper — a schema with a real
character in it, a real sheet rendered from it, a real pack proving the format
handles a real ruleset.

## Decision

Phase 1 delivers:

1. **The vault schema** — note types, the frontmatter contract, the visibility
   model, UID rules, folder conventions.
2. **One complete system pack** — the full spec of one ruleset, exercising
   attributes, resources, formulas, conditions, progression, difficulty and
   sheet views.
3. **A working app** — local server, vault service (read / write / watch),
   pack loader and formula evaluator, browse UI with typed views, a character
   editor, and a scaffolder that creates a new campaign.
4. **A real campaign** — a worked example vault with several note types
   populated, used as the acceptance test.

The schema is considered validated when a complete character can be created,
edited and re-read, with its visibility applied, without any of it needing to
be hand-fixed.

> **Amended twice.**
>
> *2026-10-23.* "Rolled" was ambiguous, and three documents disagreed about
> it: this ADR said a character is "rolled", ADR-0008 said the dice engine
> was phase 2 work, and the plan had the dice engine as a phase-1 milestone.
>
> *2026-10-23, later.* Phase 1 was cut to an MVP — **a campaign wiki, with
> secrets** — and the **formula engine, the dice engine and the 5e pack all
> moved to phase 2**. A character in phase 1 is not formula-evaluated at all:
> the pack format is minimal, declaring content types and their typed fields.
> Phase 2 adds resources, formulas and dice to a format that already works.
>
> The dice engine is therefore **phase 2** in its entirety, and the Blades
> format gate (ADR-0012) is phase 2's gate rather than phase 1's. See
> [`phase-plan.md`](../phase-plan.md).

## Consequences

**Good:**

- The schema gets tested against reality before phase 2 builds a wizard on it.
- The pack format gets proven against a real ruleset, including its ugly parts.
- Something demonstrable exists at the end of phase 1.

**Bad:**

- Phase 1 is bigger than a content-only phase 1. It includes the app shell, so
  stack decisions land early.
- One pack proves the format for one system. The real test is the *second*
  pack — which must be deliberately different in shape, not merely in values.
