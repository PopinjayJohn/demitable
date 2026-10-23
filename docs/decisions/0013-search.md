# ADR-0013: Search is literal for names and links, ranked for body text

- **Status:** Accepted
- **Amended by:** ADR-0054 — the literal index also covers the note's section
  headings, amended by **ADR-0030** — the ranked body index is
  SQLite FTS5, persisted outside the vault; the literal index stays in memory
- **Date:** 2026-10-21

## Context

The app is the primary way to browse a campaign of thousands of notes. Search
is the primary way to find things, and it has to behave in a way a user can
predict.

Obsidian's search is literal substring matching over file names, aliases and
body text. It is predictable and fast, and on a large vault it returns a wall
of partial matches — a "gol" hits "goliath", "gold", and "Angolmois" alike.

Full-text search with ranking is better at finding prose but is bad at the
case that matters most: you know the character's name and you want the note.

## Decision

**Two mechanisms, one search box.**

**Literal index** — exact and prefix matching over:
- file name
- `name` and `aliases` frontmatter
- wikilink targets

Matching is case-insensitive, prefix-anchored, with substring fallback. This
is what runs on every keystroke and what fills the results list first. It
returns the note you meant.

**Ranked index** — BM25 over tokenised body text, with the note title and
aliases boosted. This is what surfaces the note you *didn't* know you were
looking for: "the scene where the brothers argued about the toll road".

Both indexes are built incrementally by the vault watcher (M1) and are updated
on change rather than rebuilt from scratch.

**Results are merged**: literal hits first, ranked body hits after, with the
ranked ones visually distinguished so the user knows why each appeared.

> **Amended by ADR-0030.** The two indexes are now implemented differently,
> because they are different kinds of thing:
>
> - The **literal index stays in memory.** It is exact lookup over names,
>   aliases and wikilink targets — a map is the right structure for "is this
>   string a key", and it is fast enough not to need help.
> - The **ranked index moves to SQLite FTS5** (ADR-0030), persisted outside
>   the vault. It is the one index expensive enough to be worth not
>   rebuilding on every launch, and FTS5 gives a correct, tuned BM25, prefix
>   and phrase queries, and snippet generation — all of which we would
>   otherwise have written ourselves.
>
> The body is **preprocessed** before indexing: frontmatter stripped, markdown
> syntax removed, wikilinks resolved to plain text, code blocks and comments
> skipped. `[[Ilia Vane]]` and `<!-- comment -->` should not be indexed as
> literal noise, and the preprocessing is needed with any engine.

## Consequences

**Good:**

- Predictable for the common case, useful for the uncommon one.
- Both indexes are simple. BM25 is a well-understood algorithm and the
  tokeniser does not need to be clever.
- Incremental updates keep both fast on a 5 000-note vault, with no
  re-index-then-wait step.

**Bad:**

- **Two indexes to keep in sync with the watcher.** Every create, edit, rename
  and delete must update both. A bug here shows up as "search says it does not
  exist" or, worse, "search shows a deleted note".
- The BM25 score is not explainable. Users will ask why a note ranked first
  and there is no honest answer beyond "it matched those words".
- Tokenisation needs to handle the wikilink syntax without treating
  `[[Ilia` and `Vane]]` as separate tokens.
- **A stale or corrupt FTS index** is now a possibility rather than a
  non-event, because the index is a file on disk. It must be detected and
  rebuilt rather than allowed to produce wrong results.

**Neutral:**

- The two mechanisms compose well with the projection layer: both indexes
  must be filtered by role before results reach a player, so a search must
  never be a path around the visibility rules. This makes search part of the
  security boundary, not just a feature.
