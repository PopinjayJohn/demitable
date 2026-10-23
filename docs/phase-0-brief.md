# Phase 0 — The Prototype

> The brief for the phase-0 agent. Self-contained: a fresh session needs
> nothing else. The plan of record is [`phase-0-plan.md`](phase-0-plan.md);
> where they disagree on phase-0 scope, this brief wins.

## What the project is

Demitable is a local-first campaign operating system for tabletop RPGs. A
campaign is **a plain directory of markdown files**. A single Go binary serves
a web app over it: the GM browses and edits their campaign, and their players
get a *filtered* view of the same campaign through a server-side projection
layer that hides GM-only content. System-specific rules (character sheets,
dice, monsters) come from **packs** — YAML files that describe a game system.
The core is small and everything the user experiences is a plugin.

Phases: **0** prototype (this phase) · **1** the MVP wiki · **2** the game
system (packs, formulas, dice) · **3** the tabletop · **4** session recording.

**You are not building the application.** You are not building phase 1.

## Read these first

`docs/design.md` (the living design) · `docs/phase-0-plan.md` (the plan of
record) · `docs/phase-1-plan.md` (what phase 1 will build, so your findings
land somewhere) · `docs/phase-plan.md` (the phase map)

Then these ADRs, which your findings will challenge or confirm:
`0021` the editor and the round-trip contract · `0033` the visibility model ·
`0036` the community's stat-block and callout formats · `0040` the reserved
frontmatter fields · `0043` markdown processing · `0045` the plugin ABI ·
`0047` the query language · `0009` accounts · `0012` the Blades format gate.

`docs/decisions/README.md` indexes all of them.

## The rules

1. **Phase 0 produces knowledge, not code.** Every spike is disposable
   *except* the frontmatter extractor, which is load-bearing enough that
   writing it twice is worse than writing it early. If a spike's code is good
   enough to keep, promote it — and say why in the report.
2. **The list is fixed.** Ten questions, no new items. If something important
   turns up, it becomes a phase-1 item. That is fine.
3. **A spike is finished when its question is answered** — not when the code is
   tidy, and not when it is pretty.
4. **Answers go in the design documents**, not only in a spike's README. A
   finding that never reaches an ADR has not been done. Update the relevant
   ADR or add a new one.
5. **Accepted ADRs are immutable.** If a finding contradicts a decision, write
   a *new* ADR that supersedes it, or record a proposed amendment in the
   write-up. Do not edit an accepted ADR in place.
6. **Build against real data.** The corpus is `fixtures/` — 1,692 real
   Obsidian markdown files from the D&D SRD, plus JSON in `fixtures/data/`.
   It is gitignored and it is a *test corpus*, not a deliverable. Use it.
7. **Commit as you go**, one commit per spike, with the question in the
   message. Push only at the end. Never force-push, never rewrite history.

## The ten questions

### 1. The frontmatter extractor — **production code, promote it**

> *What does a real frontmatter block contain, and what must the writer
> preserve byte-for-byte?*

Build the extractor and run it over the whole corpus.

It must handle: `yaml.Node` round-tripping (folded scalars, CRLF, the `---`
document marker, `!!merge` keys, anchors, duplicate keys, non-string keys);
Obsidian's property model (vault-wide types in `.obsidian/types.json`,
`yes`/`no` as strings, wikilinks in properties as nested lists, dates
decoding differently in Go and js-yaml); and the corpus's own quirks.

*Accept when:* every corpus file parses and the parser **reports** what it
cannot handle rather than guessing; the corpus's frontmatter shapes are
tabulated (keys, types, ambiguities); a **surgical write** is proven — changing
one key leaves every other byte identical, including comments, key order,
quoting style, indentation and line endings; and the writer emits
**type-correct YAML** for the reserved field set (ADR-0040): booleans as
`true`/`false`, wikilinks in properties quoted, property names matched
case-insensitively.

### 2. The projection prototype — disposable

> *Does the visibility model produce the right thing on real data?*

Run the projection over the corpus and produce, per role, the exact set of
notes and fields that role may see. Then **read the diffs**.

*Accept when:* a per-role visibility diff for the whole corpus has been read
by a human; every GM-only note is confirmed absent from every non-GM role's
view; the `owner` fallback, the `private` value, the workflow gate and the
display principal are each exercised on real notes; and anything the model
gets wrong on real data is written into ADR-0033.

### 3. The UI/UX prototype — disposable

> *What does this application look like?*

Four clickable surfaces sharing one token layer in CSS custom properties
(which ADR-0029's theming already depends on): the **GM's campaign browser
and dashboard**, the **note editor** (source pane, live preview, where the
frontmatter panel lives), the **character panel** (a form rendered from a
pack's declared fields, including what a player must not see), and **the
player's view**.

Design the GM's view and the player's view **together**, so the projection's
consequences show up in the interface rather than in a document.

*Accept when:* all four are clickable and share one token layer; the design
system is written down (tokens, type scale, spacing, colour rules, and
ADR-0034's colour-independence rule in practice); and a real session has
been walked through the prototype with the friction recorded.

> **If this item overruns:** build the **player's view and the GM's view as
> one artefact**, and drop the note editor and the character panel to phase 1.
> Those two are conventional; the pair is the differentiated part, and the
> pair is where the projection's consequences become visible. Say in the
> report that the two were deferred and why.

### 4. The markdown rendering spike — disposable

> *What do a callout, a stat block and a wikilink actually look like?*

Render the corpus's own content: the 355 `ad-statblock` blocks, the callout
types with their fold markers, the `^blockid` references, tables, task
lists, wikilinks.

*Accept when:* every construct the corpus contains has a rendering, decided
and drawn; the rendering of a **GM-only stat block beside a party-visible
one** is designed side by side; and ADR-0036's dialect support is confirmed
sufficient or its gaps listed.

### 5. The plugin host shape spike — disposable

> *What does the manifest actually need to say, and does the bridge carry
> what a plugin needs?*

Build the manifest, the capability model, a shadow-root surface host, the
bridge, and **two plugins — one with a UI surface, one without**.

*Accept when:* the two plugins are written against the manifest and the
manifest is **what they actually needed**; the capability list and
host-function vocabulary are confirmed (ADR-0045) and the `api: 1` freeze
criteria are written from what the plugins required; and both a shadow-root
and a light-DOM surface mount and the difference is visible.

### 6. The search spike — disposable

> *Does FTS5 over the real corpus put the right note first?*

Load the corpus into FTS5 with the preprocessing — strip markdown, resolve
wikilinks — and query it with things a GM would type.

*Accept when:* ranking is judged on real queries ("the duke's letter",
"goblin", a character's name); the preprocessing rules are written down; the
**rank inflation from GM-only notes** is measured (the IDF side channel);
and the operator set the query language needs is observed, not invented.

### 7. The watcher spike — disposable

> *Does the watcher see everything, on every platform?*

The polling watcher and the inotify path, against the corpus.

*Accept when:* a change made the way Obsidian makes one — write a temp file,
rename over the original — is **seen**, on Linux and on macOS; the macOS path
is confirmed to open **zero file descriptors per note**; and the rescan
interval is set by measurement.

### 8. The demo campaign, written by hand — **keep as content**

> *What is the schema missing?*

Write a small campaign against the schema: a location, a faction, an item, a
quest, a session log, a secret, a character, a character in draft, a table, a
GM-only note, a discovered location, an undiscovered one, and a second
GM-only note for a character they have not met.

*Accept when:* it is written and the schema is corrected wherever it was
awkward; the projection test over it passes in both directions; and it becomes
phase 1's demo campaign, so the work is not thrown away.

### 9. The baselines — disposable

> *What do the numbers actually cost?*

Index 5,000 generated notes; index the real corpus; run the queries; measure
memory.

*Accept when:* every performance criterion in the phase-1 plan has a
**measured** baseline beside it, and the wrong ones are corrected rather than
carried.

### 10. The threat model — disposable

> *What is actually being protected, from whom, and by what?*

A written threat model for the projection and the exposure checklist: assets,
actors, attack surfaces, mitigations, and **the things that are not defended**,
stated plainly.

*Accept when:* it exists; the phase-1 exposure checklist is derived from it
rather than being a list someone liked the look of; and every claim in the
trust model is checked against it, with anything the model does not actually
defend corrected in the ADRs.

## Order

Week 1: **1**, then **2** and **10**. Weeks 3–4: **3**, with **4** and **6**
alongside it. Week 5: **5** and **7**. Week 5–6: **8** and **9**. Week 6:
write up.

The dependencies: **1** first, because everything else reads notes through
it. **8** late, because it needs the schema to have already been corrected.
**4** before **3** finishes, because it feeds the visual design.

## Where things live

```
spikes/<question-slug>/     each with a README: the question, the answer,
                            and what changed as a result
dev/demo-campaign/          the demo campaign (question 8) — kept
internal/vault/frontmatter/  the extractor (question 1) — promoted
```

`fixtures/` is gitignored. Do not commit it.

If the extractor is a real Go package, it goes under `internal/` with tests.
Everything else under `spikes/` is disposable — at the end, delete it or
archive it with a note saying what it settled.

## Out of scope

The SRD enrichment pipeline (phase 2 — phase 0 only *parses* the corpus) · the
5e pack · the formula and dice engines · the WASM plugin tier · any live
session, seat or board (phase 4) · the archive import endpoint (phase 2) ·
**and any of the actual application**.

## Stop and report if

- the extractor's round-tripping is **impossible** for some construct the
  corpus contains — that changes ADR-0021 and ADR-0043;
- the projection cannot be expressed for a real case in the corpus — that
  changes ADR-0033;
- the manifest cannot express what two real plugins need — that changes
  ADR-0045;
- the UI prototype contradicts the visibility model — that changes ADR-0033;
- the measured baselines contradict the phase-1 estimates;
- you conclude the phase-0 list itself is wrong.

In any of these, **stop that item, record the finding, and continue with the
rest.** Do not redesign around it silently.

## When you are done

1. Every answer is in a design document (ADR, or a proposal in the write-up).
2. The extractor is in `internal/`, tested, with its findings written into the
   schema and the write semantics.
3. The demo campaign exists in `dev/demo-campaign/`.
4. The four UI/UX prototypes exist and a **design-system document** has been
   written.
5. The threat model and the baselines exist as documents.
6. Spikes are committed; nothing is half-promoted without a stated reason.

Then report back in this shape:

- **A one-paragraph summary of what phase 0 changed about the design.**
- **Per question: the answer, in three sentences or fewer, and the design
  document it changed.** If a question's answer was "the plan was right",
  say that — it is a result.
- **What is now known that was previously assumed.** This is the most valuable
  section. Be specific: "the corpus's frontmatter has 14 distinct key
  shapes, and 3 of them cannot be round-tripped by `yaml.Node`" beats
  "the extractor is solid".
- **Contradictions found, with the ADR that now disagrees.**
- **What phase 1 should change** — concrete, with the milestone.
- **What you would do differently, given what you now know.**

Be honest about what failed. A phase 0 that found nothing is a phase 0 that
did not test anything.
