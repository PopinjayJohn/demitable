# ADR-0053: The renderer renders constructs, not documents

- **Status:** Accepted
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