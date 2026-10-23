# Proposed ADRs from the UI spike

These are proposals only. `docs/decisions/` is untouched; the orchestrator
lands any ADR it accepts.

---

# Proposed ADR: The projection's consequences belong in the UI

- **Status:** Proposed
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

# Proposed ADR: The renderer renders constructs, not documents

- **Status:** Proposed
- **Date:** 2026-10-23
- **Extends:** ADR-0043 (markdown processing), ADR-0036 (community formats)
- **Addresses:** what a callout, a stat block and a wikilink actually look
  like when rendered from the corpus's own content

## Context

The corpus (`fixtures/`, 1,692 SRD markdown files) was measured rather than
assumed:

| Construct | Count |
|-----------|-------|
| `ad-statblock` code blocks | 355 |
| distinct stat-block layouts | 41 |
| Callouts (`quote`, `tldr`, `note`, `example`) | 120 |
| Block references `^id` | 614 occurrences, 204 distinct ids |
| Tables | 582 (358 of them the six-column stat-block grid) |
| Links with title attributes | 1,925 |
| Links with anchors | 3,757 |
| Wikilinks `[[…]]` | 0 |
| Embeds `![[…]]` | 0 |
| Task lists | 0 |
| Ordered lists | 0 |
| Raw HTML tags | 8,604 in 25 files |

The corpus contains raw HTML, some of it malformed (`<th class"level">`
— a missing `=`), callouts appended to the end of paragraphs instead of
starting lines, and links to vault-root-relative `.md` paths with
`#anchor`s. Nothing in the corpus uses wikilinks or task lists.

## Decision

### Every construct that exists has a rendering

The renderer (adapted from ADR-0043's browser parser) decides and draws:

- **Callouts** — `> [!type] Title` with `-` fold markers; `[!tldr]`,
  `[!note]`, `[!example]` render as styled blocks with a glyph, a word and a
  border pattern (ADR-0034).
- **Stat blocks** — `ad-statblock` is prose in a code fence: `title:` option,
  token image, size/type line, labelled field grid, six-column ability table,
  `## Traits` / `## Actions` sections. A GM-only stat block beside a
  party-visible one is designed side by side — the projection meets the
  interface there.
- **Block references** — `^id` renders as an anchored superscript link.
- **Tables** — the six-column grid renders as a stat-block grid; two-column
  dice tables render the `dice:` header as a roll instruction; other columns
  render as tables.
- **Links** — relative `.md` paths resolve against the vault; a link to a note
  the caller may not see renders as present but not followable, not as a 404
  (the link exists; following it is denied by the projection).
- **Raw HTML** — sanitised through an allow-list; malformed attributes
  (`class"level"`) are repaired, not silently dropped; `script`, `style`,
  `javascript:` URLs are removed and reported.

### What does not exist in the corpus is not built

Wikilinks, task lists, ordered lists, footnotes, highlights and math do not
appear in the corpus, so they have no rendering here. If a pack or a user's
vault needs them, they are added when measured.

## Consequences

**Good:**

- The renderer was tested on 1,692 real files, not invented content.
- A GM-only stat block and a party-visible stat block are drawn side by side
  — the projection's interface consequence is visible, not documented.

**Bad:**

- Two callouts in the corpus append `[!note]` to the end of a paragraph; they
  render as literal text because a callout is a block and that is not one.
  The renderer reports this rather than guessing.

**Neutral:**

- The corpus's links are relative `.md` paths with URL-encoded anchors, not
  Obsidian wikilinks; the resolver handles both shapes.