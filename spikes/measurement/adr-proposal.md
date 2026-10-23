# ADR proposals from the phase-0 measurement spikes

> These are **proposals**, not decisions. The orchestrator lands them; this
> spike does not edit `docs/decisions/`.
>
> Each item names the ADR it contradicts or extends, the evidence, and the
> text that should go into a new ADR (or an amendment). Accepted ADRs are
> immutable, so each of these is a **new** ADR that amends or supersedes.

---

## A1 — The literal index covers section headings

**Amends ADR-0013 (search).** New ADR, status *Proposed*.

### The evidence

Over the SRD corpus's 1,693 notes, the ranked index puts the right note
first for 12 of 15 real GM queries and the literal index for **14 of 15**.
The three the ranked index misses — "stealth", "sleight of hand",
"insight" — are all rule-book terms that live **only** in a note's section
heading: `rules/skills.md` holds all twenty skills under `## Skill`
headings, so "stealth" appears **once** in it and **fourteen times** in
`compendium/items/mithral-armor-xdmg.md`. BM25's length normalisation
punishes the long note and the armors win.

Adding a boosted `headings` column to the FTS5 table moves "sleight of
hand" from a miss to #4 and "insight" to #2; adding the headings to the
**literal** index puts the Skills note first for all three.

ADR-0013's literal index covers "file name, `name` and `aliases`
frontmatter, wikilink targets". Section headings are the same kind of
thing: the name of a thing, inside a note that holds many things.

### The text

> **Amendment to ADR-0013's literal index.** The literal index covers
> exact and prefix matching over:
>
> - file name
> - `name` and `aliases` frontmatter
> - wikilink targets
> - **the note's section headings**
>
> Section headings are included because a rule-book note holds many named
> things under level-2 headings and the heading *is* the name. Measured on
> the 1,693-note SRD corpus: adding the headings to the literal index
> takes the first-place rate on fifteen real GM queries from 12/15 to
> 14/15, and the three queries it fixes are the ones a GM asks for most —
> the name of a skill, a sense, a condition.
>
> The literal index therefore needs the note's headings, which means the
> indexer must keep them. They are one string per note and are cheap.

---

## A2 — The FTS5 table needs columns for the title, the aliases and the headings

**Amends ADR-0030 (SQLite caches).** New ADR, status *Proposed*.

### The evidence

ADR-0013 requires "BM25 over tokenised body text, with the note title and
aliases boosted". ADR-0030's schema is:

```sql
CREATE VIRTUAL TABLE note_fts USING fts5(
  uid UNINDEXED,
  body,
  tokenize = 'unicode61 remove_diacritics 2'
);
```

**There is nowhere to put a boost.** BM25's per-column weights are the only
way FTS5 can boost a field, and this table has one column. The title can
only be boosted by prepending it to the body text, which cannot weight it
separately from the body.

Measured over the same 15 queries:

| Schema | index size | first place |
|---|---|---|
| ADR-0030 verbatim (`uid, body`, title prepended to the body) | 3,878,912 B | **9/15** |
| `uid, title, aliases, tags, body` | 4,186,112 B | **11/15** |
| `uid, title, aliases, headings, tags, body` | 4,284,416 B | **12/15** |

The single-column schema loses three of the fifteen queries to the index
notes, and it is the only one of the three that cannot express ADR-0013's
requirement at all.

### The text

> **Amendment to ADR-0030's FTS5 index.** The table has a column for each
> field that is boosted or filtered:
>
> ```sql
> CREATE VIRTUAL TABLE note_fts USING fts5(
>   uid UNINDEXED,
>   title,
>   aliases,
>   headings,
>   tags,
>   body,
>   tokenize = 'unicode61 remove_diacritics 2'
> );
> ```
>
> The weights are per column and are the only way BM25 can express "the
> title matters more than the body". Measured on the 1,693-note SRD
> corpus: the single-column schema puts the right note first for 9 of 15
> real GM queries; the multi-column schema, 12 of 15.
>
> `tags` is a separate column so that a tag filter is a column filter
> rather than a scan of the body text. `uid` stays unindexed.

---

## A3 — The community's content fences are prose, not code

**Amends ADR-0030 (SQLite caches).** New ADR, status *Proposed*.

### The evidence

ADR-0030's preprocessing rule is "frontmatter stripped, markdown syntax
removed, wikilinks resolved to plain text, **code blocks and comments
skipped**".

In the SRD corpus, the `ad-statblock` fence spans the *whole* stat block
including the creature's Actions and Bonus Actions — for example
`compendium/bestiary/fey/goblin-warrior-xmm.md` lines 45 to 76 — and it
appears in **355 notes, 21% of the corpus**. Applying the rule as written
drops:

```
tokens, raw markdown      413267
tokens after stripping     249937  (60.5% of raw)
tokens with ad-statblock  315440  (+26.2%)
```

A search for "armor class" or "hit points" returns nothing from the
bestiary. Obsidian renders these blocks as content; the ADR-0036 dialects
(`ad-statblock`, `statblock`, `ad-monster`) exist precisely to make them
readable.

### The text

> **Amendment to ADR-0030's preprocessing rule.** Code blocks are skipped,
> **except** the community's content fences — `ad-statblock`, `statblock`,
> `ad-monster` — which Obsidian renders as prose and which carry the
> creature's statistics, actions and bonus actions. They are re-parsed as
> markdown and indexed like the rest of the document.
>
> Measured on the 1,693-note SRD corpus: the exception restores 26.2% of
> the indexable tokens and affects 355 notes (21% of the corpus).

---

## A4 — The search box is its own language, and it needs normalisation and coordination

**Extends ADR-0047 (query language).** New ADR, status *Proposed*.

### The evidence

ADR-0047's predicate language is for `list_by` — a filter over structured
fields. Its operator set was confirmed against 21 observed GM requests:
**no declared operator was unused and no needed operator was missing.**

But the **search box** is a different query, and two things are needed that
ADR-0047 does not say:

1. **Query normalisation.** FTS5's default is a phrase query. "the duke's
   letter" as a phrase matches nothing in the corpus. The query must be
   lowercased, split, stripped of its possessive and of stopwords, and
   matched as a disjunction.
2. **A coordination factor.** Over the corpus, "the duke's letter"
   normalises to `duke OR letter`; "duke" matches **0** notes and "letter"
   matches 2, so the query silently degrades to a single-term search and
   returns the two Cultist notes — the only notes in the SRD containing the
   word "letter". Ranking by *how many* query terms a note matched,
   before the BM25 score, is what fixes it.

And one property the UI must know: **a BM25 score is not comparable across
queries.** The IDF is `log((N − n + 0.5) / (n + 0.5))`, so a term present
in more than half the documents has a negative IDF and the score collapses
to ~1.5 × 10⁻⁶, whatever the corpus size. In the nine-note synthetic
campaign, "duke" scores −0.000002. The relative ordering is still
meaningful; the absolute number is not a quality measure.

### The text

> **Amendment to ADR-0047.** ADR-0047 governs the predicate language used
> by `list_by` and by the MCP `list_by` tool. **The search box is a
> separate surface with its own rules**, and it is not part of the
> predicate language.
>
> A search-box query is normalised before it is executed: it is
> lowercased, split on non-alphanumerics, stripped of a trailing
> possessive, stripped of stopwords, and de-duplicated. The normalised
> terms are matched as a disjunction, and the results are ranked first by
> **how many distinct query terms a note matched** and then by BM25.
>
> The BM25 score is not comparable across queries and must not be shown as
> a quality measure.

---

## A5 — The two-index partition is load-bearing (confirmation, no amendment)

**Confirms ADR-0047's partition.** No text change needed; this is the
evidence for keeping it.

### The evidence

Over the 1,693-note SRD corpus with ADR-0033's pack-declared visibility
defaults applied (1,169 GM-only notes, 524 party-visible), comparing a GM
index against a player index on the same query with the same weights:

- the top result **changes in 3 of 8** real GM queries;
- a note's rank moves by **up to 16 positions**;
- a note's BM25 score changes by **up to 100.6%** (the Darkvision spell
  scores −3.84 in the GM index and −7.70 in the player index).

A single index with a filter after the ranking leaks in two ways: the
hidden note occupies a slot, and its presence changes the statistics of the
visible ones. The partition is required, not defensive.

The residual channel ADR-0047 acknowledges — the player index still holds
the owner-scoped notes of *other* players — is real and much narrower.

---

## A6 — What the watcher and the baselines propose

Added when questions 7 and 9 are answered. See §2 and §3 of
[`README.md`](README.md).
