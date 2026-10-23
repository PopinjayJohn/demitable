# ADR-0054: The search index, measured — headings, columns, and content fences

- **Status:** Accepted

- **Date:** 2026-10-23
- **Amends:** ADR-0013 (search), ADR-0030 (the SQLite caches), ADR-0047 (the
  query language)
- **Origin:** phase 0, question 6 — *does FTS5 over the real corpus put the
  right note first?* The evidence is
  [`spikes/measurement/`](../../spikes/measurement/README.md), fourteen result
  files, and an index over the 1,692-file corpus.

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

ADR-0013 requires two indexes: a literal one over names, and a ranked one
over body text with the title boosted. ADR-0030 gives the FTS5 schema and
the preprocessing rules. ADR-0047 gives the predicate language for
`list_by`.

None of the three had been run against a real corpus. Phase 0 pointed them
at the 1,692 SRD files and asked fifteen questions a GM would actually
type.

Three assumptions turned out to be wrong, and one turned out to be right in
a way that matters more than it looks.

## Decision

### 1. The literal index covers section headings

**Amends ADR-0013.** The literal index covers exact and prefix matching
over:

- file name
- `name` and `aliases` frontmatter
- wikilink targets
- **the note's section headings**

Section headings are included because a rule-book note holds many named
things under level-2 headings, and **the heading *is* the name**.

Measured on the 1,692-note corpus: the ranked index puts the right note
first for **12 of 15** real GM queries and the literal index for **14 of
15**. The three the ranked index misses — "stealth", "sleight of hand",
"insight" — are all rule-book terms that live only in a heading. The corpus
keeps every skill in one note (`rules/skills.md`, twenty `## Skill`
headings), so "stealth" appears **once** there and **fourteen times** in
`compendium/items/mithral-armor-xdmg.md`. BM25's length normalisation
punishes the long note and the armours win.

Adding the headings to the literal index fixes all three, and they are the
three a GM asks for most: the name of a skill, a sense, a condition.

### 2. The FTS5 table has a column per boosted or filtered field

**Amends ADR-0030.** ADR-0030's schema is `note_fts(uid UNINDEXED, body)`.
**There is nowhere to put a boost.** BM25's per-column weights are the only
way FTS5 can boost a field, and that table has one column — so the title
can only be boosted by prepending it to the body text, which cannot weight
it separately from the body.

```sql
CREATE VIRTUAL TABLE note_fts USING fts5(
  uid UNINDEXED,
  title,
  aliases,
  headings,
  tags,
  body,
  tokenize = 'unicode61 remove_diacritics 2'
);
```

Measured over the same fifteen queries:

| Schema | Index size | First place |
|---|---|---|
| ADR-0030 verbatim (`uid, body`, title prepended to the body) | 3,878,912 B | **9/15** |
| `uid, title, aliases, tags, body` | 4,186,112 B | **11/15** |
| `uid, title, aliases, headings, tags, body` | 4,284,416 B | **12/15** |

The single-column schema loses three of the fifteen queries *and* is the
only one of the three that cannot express ADR-0013's requirement at all.

`tags` is a separate column so that a tag filter is a column filter rather
than a scan of the body. `uid` stays unindexed.

### 3. The community's content fences are prose, not code

**Amends ADR-0030's preprocessing rule.** Code blocks are skipped,
**except** the community's content fences — `ad-statblock`, `statblock`,
`ad-monster` — which Obsidian renders as prose and which carry the
creature's statistics, actions and bonus actions. They are re-parsed as
markdown and indexed like the rest of the document.

ADR-0030's rule as written — "code blocks and comments skipped" — is
wrong for the corpus. The `ad-statblock` fence spans the *whole* stat
block, including the creature's actions (`goblin-warrior-xmm.md` lines
45–76), and it appears in **355 notes, 21% of the corpus**. Applying the
rule as written drops:

```
tokens, raw markdown      413267
tokens after stripping     249937  (60.5% of raw)
tokens with ad-statblock  315440  (+26.2%)
```

**A search for "armor class" or "hit points" returns nothing from the
bestiary.** Those are a GM's two most common lookups.

### 4. The search box is its own language

**Extends ADR-0047.** ADR-0047 governs the predicate language used by
`list_by` and by the MCP `list_by` tool. **The search box is a separate
surface with its own rules**, and it is not part of the predicate language.

A search-box query is normalised before it is executed: lowercased, split
on non-alphanumerics, stripped of a trailing possessive, stripped of
stopwords, and de-duplicated. The normalised terms are matched as a
disjunction, and the results are ranked first by **how many distinct query
terms a note matched**, and then by BM25.

The coordination factor is not decoration. "the duke's letter" normalises
to `duke OR letter`; "duke" matches **0** notes and "letter" matches 2, so
without the coordination factor the query silently degrades to a
single-term search and returns the two Cultist notes — the only notes in
the SRD containing the word "letter".

**The BM25 score is not comparable across queries and must not be shown as
a quality measure.** The IDF is `log((N − n + 0.5) / (n + 0.5))`, so a term
present in more than half the documents has a negative IDF and the score
collapses to ~1.5 × 10⁻⁶ whatever the corpus size. The relative ordering
is meaningful; the absolute number is not.

### 5. The two-index partition is load-bearing

**Confirms ADR-0047.** No change; this is the evidence for keeping it, and
it is stronger than the decision assumed.

Over the corpus with ADR-0033's pack-declared visibility defaults applied
(1,169 GM-only notes, 524 party-visible), comparing a GM index against a
player index on the same query with the same weights:

- the top result **changes in 3 of 8** real GM queries;
- a note's rank moves by **up to 16 positions**;
- a note's BM25 score changes by **up to 100.6%** — the Darkvision spell
  scores −3.84 in the GM index and −7.70 in the player index.

A single index with a filter *after* the ranking leaks in two ways at once:
the hidden note occupies a slot, and its presence changes the statistics of
the visible ones. **The partition is required, not defensive.**

The residual channel ADR-0047 acknowledges — the player index still holds
the owner-scoped notes of *other* players — is real, and much narrower.

### 6. The predicate language's operator set is confirmed

ADR-0047's operator set was checked against **21 observed GM requests**:
**no declared operator was unused, and no needed operator was missing.**

Two observations the corpus adds:

- **`contains` has two semantics, and the corpus pins both down.** `tags` is
  a YAML *list*, so `contains` is list membership; the values are
  hierarchical (`monster/cr/1-4`), so a substring reading would make
  `contains "monster/cr"` a hit and `contains "monster/cr/1-4"` a different
  one. `obsidianUIMode` is a *scalar*, so `contains` there is substring.
  One operator, two meanings, decided by the field's type — which is what
  ADR-0040's type registry is for.
- **The comparison operators (`>`, `>=`, `<`, `<=`) and `not` are
  unexercisable by the corpus**, which has no numeric and no boolean field.
  They can only be tested against a pack-declared field. That is a gap in
  the corpus, not in ADR-0047.

## Consequences

**Good:**

- **Three defects in accepted ADRs are fixed by measurement rather than by
  reading**: the literal index was missing headings, the FTS5 schema had
  nowhere to put a boost, and the preprocessing rule was discarding 26% of
  the corpus.
- **A GM's two most common lookups work.** "hit points" and "armor class"
  return the bestiary.
- **The two-index partition has a number behind it**, so it survives a
  future argument about whether it is worth the cost.
- **The search box's rules are stated**, instead of being whatever the
  implementation happened to do.

**Bad:**

- **The index is bigger and the note is written twice.** The multi-column
  schema costs 4.2 MB against 3.9 MB for the corpus, and the partition
  doubles the FTS file.
- **The literal index now needs the headings**, so the indexer must keep
  them. One string per note; cheap.
- **Ranking by matched-term-count is a blunt instrument.** It fixes "the
  duke's letter" and it will occasionally favour a note that mentions
  every term shallowly over one that answers the question.

**Neutral:**

- The ranked index still misses the three heading-only queries. The fix is
  to have the literal index answer them, and the merged index to prefer the
  literal hit — which is what ADR-0013 already specifies.
- The corpus's frontmatter holds only four Obsidian-native keys, so it can
  exercise a list-membership operator over `tags` and `aliases` and nothing
  else. The rest of the operator set is observed from what a GM asks for
  against ADR-0040 and ADR-0033.

## A note on what this cost

The corpus could not answer the visibility question on its own — it has no
`visibility`, no `campaign` and no `type`, so the split had to be
manufactured two ways: by applying ADR-0033's pack-declared defaults to
the real corpus, and by writing a nine-note synthetic campaign with real
`visibility`, `campaign`, `player` and `workflow` fields. Both are in
`spikes/measurement/testdata/`.

This is the same lesson as ADR-0050's: **the corpus is a test corpus, not
a schema.** Every measurement that needs our own fields has to build them.
