# ADR-0047: One fixed predicate language for `list_by`, shared by REST and MCP

- **Status:** Accepted
- **Date:** 2026-10-23
- **Extends:** ADR-0014 (MCP) and ADR-0030 (the SQLite caches)
- **Addresses:** a blocking open decision — the filter language for
  `list_by`, and the cursor format

## Context

Two consumers need to query notes: the REST API's `GET /api/v1/notes`, and
the MCP plugin's `list_by` tool. Two languages for the same job is a bug
waiting to happen, and the design already notes they should share one.

There is also a leak to close. **FTS5 computes BM25 — and therefore IDF —
over the whole corpus**, including GM-only notes. A term that appears only in
a hidden note changes the *ranking* of visible notes. That is a side channel,
and the research found it unanalysed.

## Decision

### A small, fixed predicate language

Not a general query language. A fixed grammar, a fixed operator set, a fixed
path vocabulary:

```
type == character
and system == dnd5e
and data.attributes.str >= 14
and tags contains "npc"
```

- **Operators:** `==`, `!=`, `>`, `>=`, `<`, `<=`, `contains`, `in`, `and`,
  `or`, `not`, parentheses.
- **Paths:** the universal fields, plus `data.<path>` into the note's
  system-specific data.
- It compiles to SQL over a `notes` table in the SQLite cache — so the query
  language and the FTS index are the same machinery.

### The projection is applied after the query, and it does not adjust the counts

**A query never reveals that something was hidden.** Results a caller may not
see are dropped, and the total count reflects what was returned, not what
matched. Otherwise the count is a perfect oracle for what the GM has hidden.

### The FTS index is partitioned so hidden notes do not distort ranking

**Two indexes.** One over the whole corpus, for the GM. One over content
that is not GM-only, for players. A player's search runs against the
player index, so a term that appears only in a GM-only note cannot change
the ranking of their results.

The residual is that the player index still contains notes that *this* player
cannot see — an `owner`-scoped note, or one granted to somebody else. That
is a much smaller and narrower channel than GM-only content, and it is
acknowledged rather than solved, because solving it exactly would mean a
per-player index.

### Cursors are opaque

A cursor is an opaque, signed token encoding the last row's sort key. It is
not a database offset, so it does not break when the result set changes
underneath it, and it does not leak position information to a caller who
should not know how many notes matched.

## Consequences

**Good:**

- **One language, two consumers**, and it compiles to SQL so it is fast
  without a second query implementation.
- **The count is not an oracle.**
- **Hidden content does not distort the ranking** of visible results, which
  was a real leak and is now closed for the common case.

**Bad:**

- **A fixed language is not a general one.** "Find me notes whose
  frontmatter contains this phrase" is not expressible — and that is a
  deliberate limit, not an oversight.
- **Two FTS indexes cost storage and write amplification**, and every
  indexed note is written twice.
- **The residual ranking channel persists** for owner-scoped notes.

**Neutral:**

- The grammar is small enough to implement in a page of Go and to document
  in a page of Markdown, and it is a superset of what a first query needs.
