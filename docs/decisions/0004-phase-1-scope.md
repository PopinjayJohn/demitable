# ADR-0004: Phase 1 ships the schema plus one complete system pack, end to end

- **Status:** Accepted
- **Date:** 2026-10-21
- **Note:** the "persisted session store" mentioned below is specified by
  ADR-0030 — SQLite, outside the vault.

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
edited, rolled and re-read without any of it needing to be hand-fixed.

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
