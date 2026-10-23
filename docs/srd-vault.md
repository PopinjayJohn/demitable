# The SRD vault — pipeline, enrichment contract, validation

> How the SRD markdown plus the SRD JSON become a pack and a vault that the
> character creation wizard and the tabletop can actually read.
>
> Governing decision: [ADR-0023](decisions/0023-pack-mechanics-content-in-vault.md).
> Source data: `fixtures/` — 1,692 markdown files and the upstream JSON corpus.

---

## 1. What we have

| | Shape | Notes |
|---|---|---|
| `fixtures/compendium/**`, `fixtures/rules/**` | 1,692 Obsidian markdown files | **SRD only** — sources are `xphb` (712), `xdmg` (445), `xmm` (339) |
| `fixtures/data/**.json` | ~26,900 records, 23,117 index keys | The **full 5e corpus**. The SRD subset is the 3,990 keys in `all-index.json` → `srdKeys` |
| `fixtures/all-index.json` | `type\|name\|source` keys | `srdKeys` (3,990) is the authoritative SRD filter |
| `fixtures/src-index.json` | SRD-resolution subset | Used by the renderer that produced the markdown |

Every entity is split across two JSON files: `fluff-*.json` holds the prose
(already rendered into the markdown body), and the mechanics file holds the
structured fields.

### 1.1 The filter is `srdKeys`, and the SRD *does* have backgrounds

An earlier draft of this document claimed the SRD contained **zero**
backgrounds. That was wrong, and it came from misreading `srdKeys`.

Measured:

```
srdKeys                        : 3,990
  of which x-sourced (2024)     : 2,034   (xphb, xdmg, xmm)
  of which 2014-sourced         : 1,956   (phb 1167, dmg 466, mm 323)
```

**`srdKeys` lists 5 backgrounds**, and the SRD has **4**: Acolyte, Criminal,
Sage and Soldier. `compendium/backgrounds/` holds exactly those four as
`acolyte-xphb.md`, `criminal-xphb.md`, `sage-xphb.md`, `soldier-xphb.md`. The
pack's `background` content type is correct, and the wizard's "pick
background" step has data.

The 16 `xphb` backgrounds in `data/backgrounds.json` are the full 2024 PHB
list; the other 12 are not SRD content.

**And the two filters are not two ways of saying the same thing.** The 2,628
x-sourced keys that `srdKeys` excludes are almost all:

- the *same entity under the other generation's tag* — `action|dash|phb` is in
  `srdKeys`, `action|dash|xphb` is not; and
- prose-only `*fluff` collections — `backgroundfluff|acolyte|xphb` is excluded
  because `backgroundfluff` is not an entity collection.

So **`srdKeys` is a deduplicated union of SRD 5.2.1 and SRD 5.1 content**, and
the source-tag rule alone is **not a filter** — it admits 2,628 keys
`srdKeys` deliberately excludes.

**Therefore the build rule is: emit only entities whose key is in `srdKeys`.**
The source-tag rule is not the licence boundary.

### 1.2 The numbers the design should use

| Quantity | Value | What it counts |
|---|---|---|
| Entities the pipeline emits | **1,692** | Markdown files in the output vault |
| Of which, monsters | **658** | `monster` keys in `srdKeys` |
| Of which, spells | **658** | `spell` keys in `srdKeys` |
| JSON-side filter | **3,990** | `srdKeys` — deduplicated, both SRD generations |
| Corpus totals | 23,117 keys / ~26,900 records | **Not** what we ship |

The earlier figures of "7,012 entities, 4,469 monsters, 936 spells" were
counts of the **whole corpus** and must not appear in the licence argument.

### The join

The markdown and the JSON join on `(type, name, source)`.

Measured coverage of prose → mechanics: **82.2%** with a naive join, and the
misses are systematic rather than random:

| Type | Naive | Cause of miss |
|---|---|---|
| `table` | 0% | JSON names carry a `; subtitle` the markdown drops |
| `item` | 71.8% | same, plus `(Varies)` qualifiers |
| `hazard`, `deck` | ~35% | composite names |
| `monster` | 92.1% | variants and suffixes |
| `spell` | 95.0% | the remainder |
| `class`, `subclass`, `race`, `background`, `feat`, `optionalfeature`, `variantrule` | 100% | — |

With normalising away `; subtitle` and parentheticals, and falling back to a
source-scoped normalised-name match, the join closes above 95%. **The
remaining misses must be reported, not silently dropped** — see §7.

---

## 2. Outputs

The pipeline produces three things:

```
fixtures/  ──┬──▶  packs/dnd5e.yaml              the mechanics pack — ships
             │
             └──▶  dev/srd-vault/                  the enriched vault — does not ship
                             │
                             └──▶  a build report
```

The **pack** is small: formulas, resources, difficulty, dice workflows, sheet
views, and the `content:` declarations that describe the shape of a class, a
species, a background, a spell, a monster. It ships in the binary, because it
is small, it is our own work, and it cites the SRD.

The **vault** is the enriched markdown, and it serves four purposes — all of
them *development* purposes:

- **Development template** — open it and the app is immediately interesting.
- **Development demo** — a populated world, not an empty vault.
- **Validation corpus** — 1,692 real-world files with real-world messiness,
  used by the test suite (§6).
- **Versioned artefact** — committed, diffable, and checked against the
  pipeline (§3).

**None of it ships.** The SRD content is not installed by the scaffolder and
not bundled in the binary.

**A community member hosts the archive.** We document the format and the
build; somebody else publishes and hosts the enriched archive at a stable
URL. We host no index, no registry and no artefact of our own — ADR-0011's
position taken literally. The project distributes *how to build* the content,
not the content.

The consequence is worth stating plainly: **a new user gets no content until
someone publishes it.** The first-run experience is therefore "download the
archive and unzip it into the vault" — which, because the archive is a zip of
plain files, needs no server feature at all. `demitable import <url>`
(ADR-0025) is a convenience that arrives in phase 1.5, not the distribution
mechanism.

The licence travels with the content: in the manifest, inside the archive, and
in every emitted note.

### 2.1 The source is transient

`fixtures/` is a **build input, not a repository artefact**
([ADR-0024](decisions/0024-transient-source.md)).

```
  manual acquisition  ──►  fixtures/   (gitignored, transient)
        + srdcheck           │
                          srdenrich pipeline
                                 │
                 ┌───────────────┴───────────────┐
                 ▼                               ▼
     packs/dnd5e.yaml                   dev/srd-vault/
     (the mechanics pack)               (the enriched content vault)
                 └────────────►  committed  ◄────────────┘
```

`fixtures/` is never committed and never shipped in the binary.

**A pinned, checksummed copy is retained outside git** — as a release asset,
or at a documented mirror with a recorded SHA-256 — and `srdcheck` verifies
it before the pipeline runs. The working-tree copy is transient; the pinned
copy is not, because the corpus is manually acquired and deleting it would
make the pipeline one-shot and any later enrichment bug unfixable
(ADR-0024).

The pin is a **hash of the corpus**, not a repository URL and a commit hash.
That is not a stylistic choice: the upstream's canonical repository is no
longer publicly resolvable, so a commit hash could only ever have been
pinned against an unofficial mirror, and a hash of a mirror is not
evidence of anything. A hash of the artifact is, and it survives the
upstream disappearing — which it already has. `srdcheck` verifies the tree,
the markdown tree and the data tree against
[`tools/srdcheck/provenance.json`](../tools/srdcheck/provenance.json), and
reads the corpus's version out of the corpus's own changelog, so the version
is self-declared by the artifact rather than asserted by this repository.

The licence does not rest on the upstream at all. The published vault ships
only the SRD subset, and the SRD's own terms — CC-BY-4.0, Wizards of the
Coast — are authoritative and independently checkable. The upstream's terms
matter only for the non-SRD material, and the build ships none of it. See
[`corpus-provenance.md`](corpus-provenance.md) for the full record and the
reasoning.

The reason the filter exists at all is the licence, and the numbers make it
plain: the corpus's data holds 23,117 entities, of which only **3,990 are
SRD**. The other 19,127 belong to copyrighted sourcebooks. Developing
against the corpus would mean developing against content we do not own.

### 2.2 The corpus is manual

**The corpus is version 2.28.1, and it is only manually available.** There is
no URL to fetch it from. So there is no fetch script — instead there are two
things:

**Manual acquisition.** The corpus is placed into a gitignored working
directory by hand. This is a step in the build instructions, not a command.
The build instructions must say exactly where it goes and what to call it.

**Verification — `tools/srdcheck`.** Because acquisition is manual, the
pipeline must not trust what it was handed:

| Check | Why |
|---|---|
| Version is 2.28.1 | A different corpus produces a different vault |
| Expected file inventory present | Catches a partial or wrong acquisition |
| Checksums of the index and a sample of entity files match the recorded ones | Catches a modified or substituted corpus |
| No unexpected top-level entries | Catches a corpus that has acquired extra content |

A failed check stops the build with a clear message. **Running the pipeline
against the wrong corpus would produce a vault that looks fine and is subtly
wrong**, which is worse than producing nothing.

### 2.3 What CI can and cannot check

Because the corpus is manual-only, CI cannot re-run the pipeline.

| Check | Where | Needs the corpus? |
|---|---|---|
| The committed vault round-trips byte-identically | **CI** | No |
| Every content note validates against its pack | **CI** | No |
| Every wikilink resolves | **CI** | No |
| The committed vault contains no non-SRD entity | **CI** | No — the vault carries its own `source` markers |
| The pipeline still reproduces the committed vault | **A maintainer, locally** | Yes |

The CI checks are the valuable ones — they run against the committed artefact
and catch any mutation. The reproducibility check is a maintainer task, run
when the enrichment rules change.

### 2.4 Where the outputs live

```
packs/dnd5e.yaml              the mechanics pack — committed, ships
dev/srd-vault/                the enriched content vault — committed, does not ship
fixtures/                      the source corpus — gitignored, transient
tools/srdenrich/               the pipeline — committed
tools/srdcheck/                the corpus verifier — committed
```

The committed vault is also the **golden corpus** (ADR-0021). Because it
lives in git, an accidental mutation of a corpus file shows up in the diff
rather than silently weakening the round-trip test.

### 2.5 Provenance and licence

**One licence, and it is permissive.**

| What | Licence |
|---|---|
| The SRD 5.2.1 content | **CC-BY-4.0**, Wizards of the Coast |
| The upstream rendering and JSON database, at version 2.28.1 | **CC-BY-4.0** — the same terms |

Both halves are CC-BY-4.0, so the enriched vault is a CC-BY-4.0 derived work.
It may be redistributed, including commercially, with attribution. There is
no share-alike obligation and nothing constrains how the repository is shared.

The corpus is manually acquired, so the terms are **recorded here** rather
than discovered from a licence file at build time:

| Field | Value |
|---|---|
| Corpus version | 2.28.1 |
| Acquisition | Manual only |
| Upstream licence | CC-BY-4.0 |
| SRD content licence | CC-BY-4.0 |
| Does the upstream impose share-alike? | No |
| Does the upstream permit redistribution of derived output? | Yes, with attribution |

### Attribution

Every emitted note carries it, and so does the pack header:

```yaml
source: SRD 5.2.1
license: CC-BY-4.0
attribution: Wizards of the Coast — D&D System Reference Document 5.2.1
```

The converter emits it; it is not optional.

### The SRD-only filter stays

Even though the upstream's own terms now cover the whole corpus, the build
still emits **only** the SRD subset — `xphb`, `xdmg` and `xmm` — and a
non-SRD entity reaching the output remains a hard error.

The reason is that it costs nothing and removes an entire category of doubt:
every byte we emit traces back to the SRD, whatever the upstream's licence
file happens to say. The 19,127 non-SRD entities have no use to us — they
belong to sourcebooks we cannot ship, and they would bloat the output by an
order of magnitude.

The licence no longer *forces* this rule. It is still the right one.

---

## 3. The enrichment contract

> **Frontmatter is what machines compute with. The body is what humans read.**

The body is never modified. Only the YAML frontmatter block is extended. This
keeps the byte-identical round-trip contract (ADR-0021) intact by
construction — there is no new prose to corrupt.

### What goes in frontmatter

Universal fields, per design doc §5.2:

```yaml
type: content
content_type: monster
uid: 01JBX…
name: Goblin
system: dnd5e
visibility: party
tags: [monster, humanoid]
```

Then the pack-specific scalars under `data:`. Per content type:

| Content type | Frontmatter `data:` holds |
|---|---|
| `class` | `hit_die`, `primary_abilities`, `saving_throws`, `spellcasting_ability`, `caster_progression`, `prepared_spells`, `cantrip_progression`, `proficiencies`, `features[]` |
| `species` | `size`, `speed`, `ability_bonuses`, `languages`, `traits[]` |
| `background` | `ability_choices`, `skill_proficiencies`, `tool_proficiencies`, `starting_equipment`, `feats[]` |
| `feat` | `prerequisites`, `ability[]`, `effects` |
| `spell` | `level`, `school`, `classes[]`, `casting_time`, `range`, `components`, `duration`, `classes`, `damage`, `healing` |
| `monster` | `size`, `type`, `alignment`, `ac`, `hp`, `speed`, `abilities{}`, `senses`, `passive_perception`, `cr`, `cr_xp`, `immune`, `resist`, `vulnerable`, `condition_immunities`, `languages` |
| `optionalfeature` | `class`, `prerequisites`, `choices` |
| `variantrule` | `rule_type`, `rule` |

**Long prose stays in the body.** A monster's `trait`, `action`, `reaction` and
`legendary` entries are prose and are already rendered there — they are not
duplicated into frontmatter.

**The stat block is preserved verbatim.** The corpus's monsters already carry
`ad-statblock` blocks — 355 of them — in the community's own dialect, along
with 204 `^statblock` block references. The pipeline does not convert them
(ADR-0036): it is already the community's format, and its version is *prose* —
a stat block a human reads. The *structured* mechanics come from the JSON and
land in the note's `data:`, which is what the tabletop turns into a token. A class's feature descriptions likewise stay in
the class body; the frontmatter carries only an *index*:

```yaml
data:
  features:
    - { name: Rage, level: 1 }
    - { name: Unarmored Defense, level: 1 }
    - { name: Reckless Attack, level: 2 }
```

The wizard resolves a feature's prose by extracting the matching section from
the class note's body. That is a direct consequence of *markdown is the
database*: the prose is a first-class part of the note, addressable by heading.

**Features are not split into individual notes.** The 2,632 features stay as
sections in their class note. Splitting them would make them individually
linkable, but it is the one transformation that alters the source prose, and
it puts the byte-identical round-trip contract (ADR-0021) at risk for no gain
the wizard actually needs — it reads the prose from the body either way.

### Spell lists are derived, not stored

A class note stores its casting progression rule; the spell notes store their
own level, school and class list. The wizard **computes** the list when it
needs one.

Nothing is duplicated, so the list cannot drift from the spells, and adding a
spell to a campaign updates every class that can cast it with no second edit.
The cost is that every list view does work, so it is cached in memory — and
never written back, because a cache that is written back is a second copy
that can go stale.

### Derived values are formulas, not numbers

Where the source data has a formula, the pack gets a formula and the note
gets the inputs. The upstream JSON embeds its own tiny expression language:

```
"preparedSpells": "<$level$> / 2 + <$int_mod$>"
"spell_dc__int":  (implicit, base 8)
"to_hit__str":    (implicit)
```

The corpus contains **11 distinct expressions across 84 uses**, all trivial.
The converter translates them into the pack's formula language (ADR-0003):

| Source | Demitable |
|---|---|
| `<$level$>` | `level` |
| `<$str_mod$>` | `attributes.str_mod` |
| `<$int_mod$>` | `attributes.int_mod` |
| `<$spell_dc__int$>` | `spell_dc` = `8 + proficiency + attributes.int_mod` |
| `<$damage_avg__2.5+str$>` | `damage_avg` = `2.5 + attributes.str` |
| `<$to_hit__str$>` | `to_hit` = `attributes.str_mod + proficiency` |

The `__` convention means "the named quantity equals this expression". The
converter resolves it into a named formula plus its dependencies.

`<$short_name$>` and `<$title_short_name$>` are display fields, not formulas,
and are translated as metadata.

---

## 4. Note shapes for the two consumers

### The character creation wizard

The wizard is generic. It reads:

1. **The pack's `content:` declarations** — what a class is, which fields it
   has, which are choices, which are required.
2. **The vault's content notes** — the actual choices: which classes exist,
   which species, which backgrounds.
3. **The pack's `formulas:`** — how to compute the result.

A wizard step for "choose a class" is not 5e-specific. It is
`content.class`, and the pack says what that means.

Concretely, the wizard for a 5e character is: pick species → pick background →
pick class → allocate ability scores (standard array, point buy or roll, per
the pack) → pick skills from the class and background grants → pick a feat if
the background grants one → choose starting equipment → compute hit points,
proficiency bonus, spell save DC and armour class from the pack's formulas.

Every one of those inputs is a note in the vault.

### The tabletop

The VTT reads monster notes: `ac`, `hp`, `speed`, `abilities`, `senses`,
`passive_perception`, `cr`, immunities — all in frontmatter. Token size comes
from `size`. The stat block the player reads is the body.

A monster dragged onto the board becomes a token with a portrait, HP bar,
initiative modifier (from `dex`), and the conditions it starts with.

**One file, two consumers.** The same note is a wizard option and a token.

---

## 5. Links

The markdown uses **relative markdown links** (`./barbarian-xphb.md`,
`rules/variant-rules/sphere-area-of-effect-xphb.md`) and Obsidian-specific
`[[…]]` is not used.

Two options:

- **Keep them relative.** Valid markdown, valid in Obsidian, and they work
  when the compendium is copied into a vault unchanged. But the app's link
  graph wants wikilinks or at least a resolvable path, and Obsidian's
  `[[…]]` gives rename-tracking for free.
- **Convert to wikilinks** (`[[Sphere]]`, with `|alias` where the text
  differs). Then renames are tracked by Obsidian, links resolve by name
  rather than path, and the vault is idiomatic.

**Recommendation: convert to wikilinks.** The conversion is mechanical and
testable — every link must resolve, and the validator checks it. The
counter-argument is that relative links survive a folder restructure, whereas
wikilinks survive a *rename* — and renames are far more common than folder
restructures in a campaign vault.

Conversion happens in the pipeline, before enrichment, so the enriched output
is already idiomatic.

---

## 6. The validation suite

The SRD vault is the project's test corpus, and it is a very good one
precisely because it is *not* curated. It contains:

- raw HTML tables with **malformed attributes** — 24 class-progression files
  carry 504 occurrences each of `class"level"`, `class"pb"` and `class"feature"`
  where `class="…"` was meant. A markdown parser will normalise these, and
  the byte-identical round-trip contract will fail on them. That is the
  point: it proves the corpus is real.
- Obsidian `cssclasses`, `obsidianUIMode`, nested tag hierarchies.
- upstream entity references — `{@damage 1d10}`, `{@condition poisoned}`,
  `{@actSave wis}` — remaining in JSON prose.
- reference links, nested blockquotes, tables with alignment, task lists,
  footnotes, emoji, entity escapes.

### The three suites

**A. Editor round-trip (ADR-0021).** Every one of the 1,692 files must
survive `parse → serialize` byte-identically. Failures are either a parser
bug or a missing raw-node case — both are real bugs, and the malformed HTML
will exercise the raw-node path exactly as intended.

**B. Pack validation.** Every content note validates against its pack's
`content:` schema. Every class's formulas evaluate across levels 1–20. Every
monster has the fields a token needs. Every spell's level and school are
present. A content note whose `system` names a pack that does not declare its
`content_type` is an error.

**C. Link integrity.** Every internal link resolves — after the wikilink
conversion, every `[[Target]]` must name a note that exists, and every
relative path must exist. The two `phb`-sourced list pages are excluded as
index pages.

---

## 7. The build report

The pipeline emits a report, and **the build fails on a class of error**:

| Severity | Meaning |
|---|---|
| **Error** | The build fails. Non-SRD content would ship; a link does not resolve; a note is missing a required field |
| **Unjoined** | A markdown file with no matching mechanics entity. Reported, listed, and **not** enriched — the file is still copied, still prose, still useful |
| **Unused** | A mechanics entity with no markdown file. Reported; for SRD entities this is a signal that a file is missing |
| **Converted** | A formula translation, listed so it can be reviewed by hand |

**Unjoined files are not failures.** A monster with no mechanics is still a
readable stat block. The enrichment is additive, and the base vault is useful
without it.

But **a non-SRD entity reaching the output is a hard error.** The filter is
the licence boundary:

- Only entities whose source is in the SRD set may be emitted.
- The converter asserts this on every entity it writes.
- The full `fixtures/data/` corpus is never copied — only the SRD subset is
  read, and the non-SRD entities are never written anywhere.

### Attribution

SRD 5.2.1 is **CC-BY-4.0**. The vault carries attribution in the pack header
and in each content note:

```yaml
source: SRD 5.2.1
license: CC-BY-4.0
attribution: Wizards of the Coast — D&D System Reference Document 5.2.1
```

The converter emits it; it is not optional.

**The upstream licence is attributed too.** The SRD content and the community
rendering are different works with different terms. If the upstream is
share-alike, the enriched template inherits that, and it may constrain
bundling the template into a released binary. The template is committed to
the repository, where attribution and share-alike terms are straightforward
to honour; whether it can also ship inside the binary is a separate decision,
deferred until distribution is settled
([ADR-0024](decisions/0024-transient-source.md)).

---

## 8. Why this is the right corpus

The SRD vault does four jobs at once, which is why it is worth the pipeline:

1. **It is the content the wizard and the VTT need.** 4,469 monsters and
   936 spells are a real bestiary and a real spell list.
2. **It is the golden corpus for the editor** (ADR-0021). Real-world
   markdown with real-world bugs, which is exactly what a hand-made fixture
   set lacks.
3. **It is the pack's validation suite.** 3,990 entities exercising the
   `content:` schemas and the formula language.
4. **It is the demo.** A new user opens a world.

And it is the strongest available test of the system-agnostic claim: **if the
5e content can be expressed as vault notes that a generic wizard and a
generic tabletop read, then the format is genuinely generic.** The Blades
pack (ADR-0012) does the same thing with content that has no 5e equivalent.

### The demo question

The enriched SRD is a **bestiary and a spell list, not a world.** As a
development demo it proves the data works, but it does not show what a
campaign looks like — no character with a backstory, no location with
history, no encounter that was played.

So a small **demo campaign** lives beside it, in `dev/demo-campaign/`. It is
not filler; it has a job, and the job is to **exercise the projection layer**
over content notes. A demo that only shows pretty data would not test
anything.

#### What it must contain

| Element | What it demonstrates |
|---|---|
| A player character with a sheet | The pack's formulas, the sheet view, a computed character |
| An NPC, and a character in `draft` | The review workflow — a draft is visible to its owner and the GM, and to nobody else |
| A location the party has visited | A discovery record; the location is visible to the party |
| A location the party has *not* visited | The same location type, invisible to the party — the negative case, which is the one that matters |
| A GM-only note | The absolute deny: hidden regardless of discovery or ownership |
| A faction, an item, a quest | The world-note types, each with exactly one writer |
| A played encounter | The encounter record, a dice log, and an SRD monster pulled in as a token |
| A session log | The session note, and a rolled-up view of the campaign |
| A `table` note | Symbol dice in use — a rune or oracle set, and a roll against it |
| An asset | A map image referenced by a location, reachable only through the asset endpoint |

#### What it must prove

1. **A player sees exactly what they should.** The projection test runs over
   the demo campaign as a fixture.
2. **A player sees nothing they should not.** Same fixture, negative cases.
3. **Content notes work in play.** A monster from the SRD becomes a token; a
   class from the SRD becomes a wizard option.
4. **A round-trip survives.** Every note in the demo campaign round-trips
   byte-identically, like the SRD corpus.
5. **The whole thing is small enough to read.** A demo that takes an hour to
   understand has failed.

It also stays a development artefact — it is in the repository, not shipped,
not published as an archive.

---

## 9. Open questions

- **Adventuring gear.** `items.json` is 2,417 entries, many of which are
  mundane. The SRD subset may warrant a curated shortlist.
- **Decks and cards.** 711 card entries and 34 decks exist in the JSON. The
  compendium has 3 deck files. These are a natural first test of the symbol
  dice and table model (ADR-0022).
- **The `phb` list pages.** Two compendium list pages come from the 2014
  PHB. They are index pages and are excluded, but they should be verified
  against the SRD rather than dropped on a filename rule.
- **Whether the vault should stay a development artefact.** The licence now
  permits redistributing it, bundling it in the binary, or publishing it as an
  importable archive. It is **published as an archive** (ADR-0024); bundling
  is deliberately not.
