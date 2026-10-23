# Phase 0 Plan — The Prototype

> **Purpose:** reduce uncertainty before committing to an implementation.
> Phase 0 answers ten questions. Its output is **knowledge**, not code.
>
> **Time box: six weeks at ≈10 hours a week — about sixty hours.**
>
> **The rule:** only the frontmatter extractor is promoted into phase 1.
> Every other spike is disposable; its *answer* is the deliverable and its
> code is thrown away.

---

## 1. Why this phase exists

Phase 1 was cut to an MVP, which was the right call — but it left the
design resting on things nobody has actually built or measured:

- The design set describes **what** the app does and almost nothing about
  **what it is**. No design system, no layout, no information architecture.
- The **frontmatter extractor** is the most load-bearing component in the
  project — every read, search, projection and write depends on it — and its
  hard requirements were all discovered *late*, by research rather than by
  trying.
- The **projection** is the trust boundary. It is well-specified on paper
  (ADR-0033) and has never been run against a real corpus.
- The **plugin API** is supposed to be shaped by two real plugins
  (ADR-0019). It has never been shaped by anything.
- The **performance criteria** in the phase-1 plan are numbers nobody has
  measured.

Phase 0 is where those get built, thrown away, and written down.

### The corpus is a test corpus, not a deliverable

**This is what makes phase 0 cheap.** The SRD pipeline (phase 2's M5) is not
built here. Phase 0 simply *parses* the 1,692 real frontmatters in
`fixtures/` and finds out what breaks. That is minutes of work, not weeks —
and it is the only realistic way to learn the extractor's edge cases.

### Where the spikes live

```
spikes/
  <question-slug>/
    README.md      the question, the answer, and what changed as a result
    ...            disposable code
```

The **answer** is committed to the design documents. The **code** is
disposable, and at the end of phase 0 each spike is either deleted or
archived with a note saying what it settled. Nothing half-built survives.

---

## 2. The ten questions

Each item has one question. It is finished when the question is answered, not
when the code is tidy.

### 2.1 The frontmatter extractor — 12 h — **promoted**

> **What does a real frontmatter block actually contain, and what must the
> writer preserve byte-for-byte?**

Build the extractor as production code, and run it over the corpus.

It must handle: `yaml.Node` round-tripping (folded scalars, CRLF, the `---`
document marker, `!!merge` keys, anchors, duplicate keys, non-string keys),
Obsidian's property model (vault-wide types from `.obsidian/types.json`,
`yes`/`no` as strings, wikilinks in properties as nested lists, dates
decoding differently in Go and js-yaml), and the corpus's own quirks.

**Done when**
- Every one of the 1,692 corpus files parses, and the parser **reports**
  every case it could not handle rather than guessing.
- The corpus's frontmatter shapes are tabulated: which keys appear, which
  types, which are ambiguous.
- A **surgical write** is proven: changing one key leaves every other byte
  of the block identical, including comments, key order, quoting style,
  indentation and line endings.
- The writer emits **type-correct YAML** for the reserved field set
  (ADR-0040): booleans as `true`/`false`, wikilinks quoted, property names
  matched case-insensitively.
- The extractor's findings are written into the schema and the write
  semantics.

### 2.2 The projection prototype — 8 h — disposable

> **Does the visibility model produce the right thing on real data?**

Run the projection over the corpus and produce, for each role, the exact set
of notes and fields that role may see — then read the diffs.

**Done when**
- A per-role visibility diff is produced for the whole corpus and **read by
  a human**, not just asserted.
- Every GM-only note in the corpus is confirmed absent from every non-GM
  role's view.
- The `owner` fallback, the `private` value, the workflow gate and the
  display principal are each exercised on real notes.
- Anything the model gets *wrong* on real data is written into ADR-0033.

### 2.3 The UI/UX prototype — 16 h — disposable

> **What does this application look like?**

Four surfaces, clickable, plus a token layer in CSS custom properties — which
ADR-0029's theming already depends on.

| Surface | What it must settle |
|---|---|
| **The GM's campaign browser** | How a campaign of thousands of notes is navigated: the dashboard, typed views, what the GM sees first |
| **The note editor** | The split between the source pane and the live preview, where the frontmatter panel lives, what the reading view looks like |
| **The character panel** | A form rendered from a pack's declared fields, including the parts a player must not see |
| **The player's view** | The same campaign, filtered — and the visual difference between "the GM sees" and "the player sees" |

**Done when**
- All four are clickable and share one token layer.
- The **design system is written down**: the tokens, the type scale, the
  spacing, the colour rules, and **the colour-independence rule** from
  ADR-0034 in practice.
- The two views that face each other — the GM's and the player's — are
  designed **together**, so the projection's consequences show up in the
  interface rather than in a document.
- A real session is walked through the prototype and the friction is
  written down.

### 2.4 The markdown rendering spike — 5 h — disposable

> **What do a callout, a stat block and a wikilink actually look like?**

Render the corpus's own content: the 355 `ad-statblock` blocks, the callout
types with their fold markers, the `^blockid` references, tables, task lists
and wikilinks. Against real data, in the prototype's own styles.

**Done when**
- Every construct the corpus contains has a rendering, decided and drawn.
- The rendering of a **GM-only stat block versus a party-visible one** is
  designed side by side, which is where the projection meets the interface.
- The dialect support in ADR-0036 is confirmed sufficient, or the gaps are
  listed.

### 2.5 The plugin host shape spike — 8 h — disposable

> **What does the manifest actually need to say, and does the bridge carry
> what a plugin needs?**

Build the manifest, the capability model, a shadow-root surface host, the
bridge, and **two plugins: one with a UI surface and one without**.

**Done when**
- The two plugins are written against the manifest, and the manifest is
  **what they actually needed** rather than what was imagined.
- The capability list and the host-function vocabulary are confirmed
  (ADR-0045), and the `api: 1` freeze criteria are written from what the
  two plugins required, not from theory.
- A shadow-root surface and a light-DOM surface are both mounted, and the
  difference is visible.

### 2.6 The search spike — 4 h — disposable

> **Does FTS5 over the real corpus put the right note first?**

The corpus's 1,692 files into FTS5, with the preprocessing — strip
markdown, resolve wikilinks — and then query it with things a GM would type.

**Done when**
- Ranking is judged on real queries: "the duke's letter", "goblin", a
  character's name.
- The preprocessing rules are written down, and the **rank inflation from
  GM-only notes** is measured — the IDF side channel the research found.
- The operator set the query language needs is observed rather than
  invented (ADR-0047).

### 2.7 The watcher spike — 3 h — disposable

> **Does the watcher see everything, on every platform?**

The polling watcher and the inotify path, against the corpus.

**Done when**
- A change made the way Obsidian makes one — write a temp file, rename over
  the original — is **seen**, on Linux and on macOS.
- The macOS path is confirmed to open **zero file descriptors per note**.
- The full-rescan interval is set by measurement, not by guess.

### 2.8 The demo campaign, written by hand — 5 h — **kept as content**

> **What is the schema missing?**

Write a small campaign against the schema: a location, a faction, an item, a
quest, a session log, a secret, a character, a character in draft, a table, a
GM-only note, a discovered location, an undiscovered one. Then a second,
different GM-only note — a character they have not met.

**Done when**
- It is written, and **the schema is corrected wherever it was awkward**.
- The projection test over it passes in both directions.
- It becomes phase 1's demo campaign, so the work is not thrown away.

### 2.9 The baselines — 2 h — disposable

> **What do the numbers actually cost?**

Index 5,000 generated notes; index the real corpus; run the queries; measure
the memory.

**Done when**
- Every performance criterion in the phase-1 plan has a **measured** baseline
  next to it, and the ones that are wrong get corrected rather than carried.

### 2.10 The threat model — 3 h — disposable

> **What is actually being protected, from whom, and by what?**

A written threat model for the projection and the exposure checklist: the
assets, the actors, the attack surfaces, and the mitigations — and **the
things that are not defended**, stated plainly.

**Done when**
- It exists, and the exposure checklist in the phase-1 plan is derived from
  it rather than being a list someone liked the look of.
- Every claim in the trust model is checked against it, and anything the
  model does not actually defend is corrected in the ADRs.

---

## 3. The schedule

| Week | | Hours |
|---|---|---|
| 1 | **2.1** the frontmatter extractor, and the corpus audit | 12 |
| 2 | **2.2** the projection prototype · **2.10** the threat model | 11 |
| 3–4 | **2.3** the UI/UX prototype, four surfaces, and the token layer | 16 |
| 4 | **2.4** the markdown rendering spike · **2.6** the search spike | 9 |
| 5 | **2.5** the plugin host shape spike · **2.7** the watcher spike | 11 |
| 5–6 | **2.8** the demo campaign · **2.9** the baselines | 7 |
| 6 | Write up. Move every answer into the design documents. Decide what phase 0 changes | — |
| | **Total** | **≈66 h** |

Two things deliberately overlap: the rendering spike runs alongside the
UI/UX work because it feeds it, and the demo campaign runs late because it
needs the schema to have already been corrected by everything above it.

### The discipline

- **The list is fixed.** No new items are admitted. If something important
  appears, it becomes a phase-1 item, and that is fine.
- **Every spike is disposable** except the extractor, and even that is
  disposable if it turns out to be wrong — it just gets rewritten.
- **A spike is finished when its question is answered.** Not when the code is
  complete, and not when it is pretty.
- **Answers go in the design documents**, not in the spike's README alone.
  A spike whose finding never reaches an ADR has not been done.

---

## 4. What is explicitly not in phase 0

| Not in phase 0 | Why |
|---|---|
| The SRD enrichment pipeline | It is phase 2's deliverable. Phase 0 only *parses* the corpus |
| The 5e pack | Phase 2 |
| The formula and dice engines | Phase 2 |
| The WASM plugin tier | Phase 2 |
| Any live session, seat or board | Phase 3 |
| The import endpoint | Phase 1.5 |
| Writing the actual app | That is phase 1 |

---

## 4a. What happened

**The phase is complete.** All ten questions were answered, and the answers
are in [`phase-0-report.md`](phase-0-report.md).

| | |
|---|---|
| Questions answered | **10 of 10** |
| Findings | **51** |
| Contradictions between accepted ADRs | **14** |
| Criteria that turned out to fail | **1** |
| New ADRs | **8** (0049–0057) |
| ADRs amended | **6** |
| Promoted code | `internal/vault/frontmatter/` — 56 tests |

**The phase's own question** — *is the design settled enough to build, and is
the phase-1 estimate real?* — is answered in the report: the design is
settled enough to build, and **the estimate was not real**. One performance
criterion fails, and three accepted ADRs say something other than what they
were supposed to say.

---

## 5. What phase 0 decides

At the end of it, these are answered — and each answer changes something
that exists:

| Question | What it changes |
|---|---|
| What is really in a frontmatter block | The schema (ADR-0040), the write semantics (design doc §5.5) |
| What does the projection actually expose | The visibility model (ADR-0033), the inspector, the property test |
| What the app looks like | A new design-system document, and every surface's layout |
| What a callout and a stat block look like | ADR-0036's dialect support |
| What a plugin actually needs | The manifest, the capability list, the `api: 1` freeze criteria (ADR-0045) |
| What FTS5 finds and ranks | The query language's operators (ADR-0047) |
| What the watcher misses | The watcher strategy in the phase-1 plan |
| What the schema is missing | The schema, and phase 1's demo campaign |
| What the numbers cost | Every performance criterion |
| What is actually defended | The trust model, the exposure checklist, and any over-claim in the ADRs |

**And the question phase 0 exists to answer:** is the design settled enough
to build, and is the phase-1 plan's estimate real?

If the answer is no, that is a success, not a failure. Finding out in six
weeks that the projection does not behave on real data is worth more than
finding out in month four.
