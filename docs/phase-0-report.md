# Phase 0 — The Prototype: what it found

> Six weeks, about sixty hours, and the output was **knowledge rather than
> code**. Ten questions, answered by building disposable spikes. Only the
> frontmatter extractor was promoted; everything else was thrown away, and
> the answers went into the design documents.
>
> **This document is the phase's report.** It says what was found, what
> changed as a result, and what is still open.

---

## The question phase 0 existed to answer

> *Is the design settled enough to build, and is the phase-1 estimate real?*

**The design is now settled enough to build — and the estimate was not
real.** Phase 1's plan was rewritten once already, from thirteen milestones
to eight, and it was still wrong in ways nobody could see from the
documents: a performance criterion that fails, three accepted ADRs with
defects in their rules, and a schema that had never been used to write
anything.

Phase 0 found **fifty-one findings**, of which **fourteen were
contradictions between accepted ADRs** — documents that had been read,
agreed, and recorded, and that did not say what they were supposed to say.

| | |
|---|---|
| Questions answered | **10 of 10** |
| ADRs landed | **8 new** (0049–0056), 4 amended, 1 superseding |
| Findings | **51** |
| Contradictions between accepted ADRs | **14** |
| Criteria that turned out to fail | **1** (the 5,000-note index) |
| Code promoted | **the frontmatter extractor**, 56 tests, and a corpus test |
| Code thrown away | 7 spikes |

---

## The ten answers

### 1. The frontmatter extractor — *what must a write preserve?* → ADR-0049

> **The surgical contract holds for 1,692 of 1,692 files.** Patching one
> field of each real file leaves every other byte of the block unchanged.

And the corpus turned out to be **cleaner than the design assumed**: four
top-level keys, all Obsidian-native, **not one of our twelve reserved
fields**, and not a comment, block scalar, anchor, merge key, flow
collection, nested mapping, wikilink in a property or CRLF file anywhere in
1,692 blocks. It could never have exposed the extractor's edge cases, so
the package carries **twelve fixtures of its own**.

The writer's rules came from measurement: `yaml.v3` and `js-yaml` 4.3.2
were run against 98 candidate scalars and **disagree in seven places**. The
YAML 1.1 boolean words were considered and deliberately **not** quoted —
no tool in the vault's toolchain is a YAML 1.1 reader — and that decision
is recorded, because it is the kind of thing that looks like an oversight
later.

**Three defects were found by writing a test and reading the failure**: a
`for` loop with no post statement in the column-to-byte conversion, which
made every span the writer computed wrong; a one-item list written as
`key: - item`, which is not YAML; and a wikilink and a text value treated
as different kinds, so the verification rejected its own correct output.

### 2. The projection — *does the visibility model produce the right thing?* → ADR-0057

> **The deny side is sound; the allow side was wrong.** 0 of 1,693 SRD notes
> and 0 of 5 GM-only synthetic notes reach any non-GM principal. But the
> **GM cannot see a player's hit points**, a handout cannot be revealed, a
> guest cannot play, and `private` is not private in `draft`.

The cause was structural: **"the first rule that applies wins" means some
rules never run, and the rules that lose are the narrowing ones.** An
audience model has no such failure mode — the audience is the intersection
of every applicable constraint, then widened by the grants permitted to
widen it.

**Fifteen of the twenty findings stand, five are resolved by ADR-0050, three
are new.**

### 3. The interface — *what does this application look like?* → ADR-0052, `docs/design-system.md`

> Four clickable surfaces sharing one token layer, and **ADR-0034's
> colour-independence rule verified in practice** rather than asserted:
> greyscale, flat-hue and deuteranopia proofs, thirteen screenshots, all
> examined, **nothing failed**. Every state separates by glyph, word and
> border pattern, so hue is the only channel allowed to fail.

ADR-0033's own "Bad" consequence is answered: a badge shows the
*effective* visibility in four channels, and an inspector beneath every
opened note lists the rule chain that produced the answer, citing sections.

**One real bug:** the projection's absolute-deny rule was hiding
`visibility: gm` notes from the **GM** as well as from players — but rule 2
says never serialised into a *non-GM* response.

### 4. The renderer — *what does a callout, a stat block and a wikilink look like?* → ADR-0053

> Every construct the corpus contains has a rendering, decided and drawn
> from real excerpts; **what the corpus does not contain is not built.**

The census is the point: **355 stat blocks across 41 layouts**, 120
callouts, 614 block references, 582 tables, 13,285 markdown links, 8,604 raw
HTML tags in 25 files — and **zero wikilinks, zero embeds, zero task lists,
zero ordered lists**.

**One defect was found in the screenshots and not explained:** a link inside
a table cell renders with a line through it. Nothing in the stylesheet asks
for it. It is recorded as an **open defect** rather than quietly fixed,
because the screenshots are the evidence that it was there.

### 5. The plugin host — *what does the manifest need to say?* → ADR-0056

> **ADR-0045's central rule is false.** "A capability with no host function
> does not exist" — but three of the nine capabilities have none, because
> one is a stream and two are declarations. The rule as written would have
> deleted those kinds.

Capabilities now come in **three kinds**; the four host primitives
(`context.get`, `pref.get`, `pref.set`, `command.register`) are frozen with
`api: 1`; the manifest field is **`api`**, not `abi`; and the freeze lands
at **M8**, not M11, because the export plugin is the second non-UI consumer
ADR-0019 requires.

**The bridge hands out a projected `NoteView`, never a raw note.** The
first version returned the note wholesale, which would have given plugins
the `player`, `grants` and `discovered` fields for notes the principal may
not see — ADR-0050's owner-scoped fields leaking through the plugin
boundary.

### 6. Search — *does FTS5 put the right note first?* → ADR-0054

> **Three accepted ADRs were wrong.** ADR-0030's preprocessing rule was
> discarding **26.2% of the corpus's indexable tokens**, because
> `ad-statblock` is content, not code — so a search for "hit points"
> returned **nothing from the bestiary**. ADR-0030's FTS5 schema had
> **nowhere to put a boost** (9 of 15 first place, against 12 of 15). And
> ADR-0013's literal index was **missing section headings**, which is where
> a rule-book note keeps the name of a thing (14 of 15, against BM25's 12 —
> and the three BM25 missed are a skill, a sense and a condition).

And the one that came out right turned out to matter more than assumed:
ADR-0047's two-index partition was called **defensive**, and it is
**load-bearing** — with 1,169 GM-only notes the top result changes in 3 of 8
queries, a note moves by **16 positions**, and a note's score changes by
**100.6%**.

### 7. The watcher — *does it see everything, on every platform?* → ADR-0055

> **Yes on Linux, and the macOS path is chosen rather than assumed.** An
> Obsidian-style atomic save — a temp file renamed over the original, which
> **changes the inode** — is seen on the target path in **577 µs**, and the
> watch survives it. The polling watcher holds **zero file descriptors** for
> 1,693 notes.

The Linux number **cannot** be transferred to macOS, and that is the point:
inotify needs no descriptor per path, kqueue opens one per file. The spike
ships the command that takes the macOS number, and says plainly that it has
not been taken.

### 8. The demo campaign — *what is the schema missing?* → ADR-0050

> **Writing twenty-one real notes found eleven that needed a field, a value
> or a shape the twelve reserved names do not provide** — and one gap no
> field could close: **there is no state between "the party knows this" and
> "the GM knows this"**, so an undiscovered location was invisible for ever
> and the Discovery grant in `session-writeback.md` §6 was **unreachable
> text**.

Two leaks fell out of it: the reserved layer had **no visibility of its
own**, so a party-visible character note published the *names* of places the
party had not found; and **Obsidian's `%%comments%%` leak**, because they
are hidden in preview but were serialised into the projection — which makes
them the most likely place for a GM's aside to be written by accident.

### 9. The baselines — *what do the numbers cost?* → ADR-0055

> **The phase-1 criterion fails: 1.23 s, not the 1 s allowed. And the index
> is only 12% of it.** The insert is 144 ms; the *parse* is 1,080 ms. The
> criterion is restated as parse plus insert, because "index 5,000 notes in
> under 1 s" conflated the cheap half with the expensive one.

The real corpus is not the problem: **1,693 notes in 0.30 s**. Peak RSS for
a 5,000-note index: **78.9 MiB**.

### 10. The threat model — *what is actually protected?* → ADR-0051

> **Defended:** the GM's vault content and the players' private content,
> from unauthorised readers — and no more.
>
> **Not defended, stated plainly:** a compromised GM host; a GM-installed
> plugin's UI tier, which runs in the app's document with the app's
> privileges; credential theft inside its window; search-ranking side
> channels; markdown XSS; plugin UI in Firefox and Safari; phase 4's live
> surfaces.

And the generalisation the model turns on: **hiding was partly a rendering
concern and partly a serialisation concern, and only the second one
matters.** The design said a GM aside was "hidden in preview"; the
projection serialised it anyway. The leaks are not in the designed
mechanism — they are in the accidental one.

---

## A note on how the last contradiction was found

The projection spike's work was re-checked against ADR-0050 and ADR-0051 by
a second agent, and the answers were landed. **A third agent, which had been
superseded and was still running, surfaced the last contradiction on its way
out** — by continuing to work on a branch that had already been replaced.

It was a real defect, and it is the kind that reading cannot find: **two
ADRs, landed hours apart, disagreed about the shape of one field.**
ADR-0050's D4 makes `discovered` a flat list of *note references*, and the
demo campaign writes `- "[[The Drowned Quarter]]"`. ADR-0057's W2 described
the widening rule in terms of the note's **uid**. A wikilink never equals a
uid, so the discovery rule **could not fire** — not for the synthetic corpus,
not for the demo campaign, not for anything.

The spike's own synthetic corpus had used bare uid strings, which is why its
tests passed. The shipped format was wikilinks. The corpus and the campaign
disagreed, and the ADR had picked a side without noticing.

It is fixed by resolving the reference through the vault index, as design
§5.3 says it must, and the synthetic corpus now writes the format the demo
campaign writes. The regression test fails without the fix.

---

## What changed as a result

| | |
|---|---|
| **8 new ADRs** | 0049 the frontmatter I/O contract · 0050 the schema gaps · 0051 the threat model · 0052 the projection in the UI · 0053 the renderer · 0054 the search index · 0055 the watcher and the index · 0056 the plugin manifest · 0057 the audiences model |
| **4 amended** | ADR-0009 (the workflow table) · ADR-0033 (the rule ordering) · ADR-0038 (the guest seat) · ADR-0045 (the plugin ABI) · plus ADR-0030 and ADR-0013 and ADR-0025 |
| **3 design documents** | `docs/threat-model.md` · `docs/design-system.md` · `docs/corpus-provenance.md` |
| **1 promoted package** | `internal/vault/frontmatter/` — 56 tests, and a test that runs against the real corpus |
| **2 committed corpora** | `dev/demo-campaign/` (21 notes) and `spikes/measurement/testdata/` (9 notes) |
| **13 screenshots** | the colour-independence proofs, kept because prose cannot replace them |

---

## What is still open

- **The corpus cannot exercise the schema.** 0 of its 1,693 notes carry any
  of the twelve reserved fields. Every measurement and every fixture that
  needs our own fields has to build them, and that cost is now in the
  design rather than hidden in a plan.
- **The macOS watcher numbers are not measured.** The command is written and
  ready; it has not been run on a Mac.
- **The plugin UI's browser matrix is unverified** — Chromium only.
- **The discovery granularity is gone.** `discovered` is a flat list of note
  references, so "visited" and "heard of" are the same fact until a
  `discovery` note type exists.
- **The 5,000-note parse still needs 1.08 s.** The criterion now says where
  the budget goes; whether phase 1 meets it by parallelising or by caching
  the parse is phase-1 work.
- **A renderer draws over its own text** in the prototype, and the
  mechanism was never established.

---

## The three things phase 0 would tell the next person

**One: a schema you have never used is a schema you have not tested.**
Reading the ADRs said the visibility model was complete. Writing twenty-one
notes found a gap no field could close. The gap was there the whole time,
in the *absence* of a value rather than in the presence of a rule.

**Two: the expensive part is rarely the part you measured.** The index
criterion failed by 23%, and the index was 12% of the cost. The corpus
looked clean because a clean corpus is what a *pipeline* produces, not what a
*user* writes. And the plugin manifest's rule about host functions was
confidently wrong, and would have been caught by one plugin that needed a
stream.

**Three: the leaks are in the accidental class.** The designed mechanisms —
the absolute deny, the secret callout, the `owner` scope — all worked. What
leaked was `%%comments%%`, a grant list, an owner's discovery list, a
plugin's view of a note. **Every one of those is a thing nobody designed to
be a security decision.** That is where to look next.

---

## A note on how the phase ran

It did not run as planned. The first dispatch was cut off part-way through
the extractor — **and the extractor had never been run**, which is how three
defects survived in it. The second wave ran four agents in parallel and
every one of them hit a provider limit: the subscription tier was exhausted,
one key was capped at $0, another had a per-minute token cap, and the free
tiers deprioritised under load. Four of the ten questions were finished by
hand rather than by delegation, because the delegation kept failing and the
work was not going to wait for it.

That is worth recording for the next phase: **the phase's output did not
depend on the agents.** The spikes were disposable, the answers were
questions with numbers attached, and those numbers could be re-measured by
anyone. The hardest part was never the model — it was running anything at
all.
