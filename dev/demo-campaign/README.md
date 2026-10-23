# The demo campaign — phase 0, question 8

> *What is the schema missing?*

**The question.** Write a small campaign against the vault schema and see
where the schema is awkward.

**The answer, in three sentences.** Writing twenty-one notes of a real
campaign found **eleven notes that needed a field, a value or a shape the
twelve reserved names do not provide** — five of them needing a lifecycle
word (`status`), three a link to another note or an asset, two a date and
an attendance list, two a `discovered` list the extractor cannot write
back, and one a `visibility` value that does not exist. The schema can
express *who may see a note* and almost nothing else: it has **no state
between "the party knows this" and "the GM knows this"**, so the discovery
grant that
[`session-writeback.md` §6](../../docs/session-writeback.md) lists as a
grant source is **unreachable text**. The reserved layer needs its own
visibility defaults, and the visibility vocabulary needs one more value.

Everything below is the evidence. The corrections are landed in
[ADR-0050](../../docs/decisions/0050-schema-gaps-found-by-writing-a-campaign.md),
which amends ADR-0033, ADR-0037, ADR-0035, ADR-0037, ADR-0040, ADR-0048 and
ADR-0009, and the design document's §5.1, §5.2 and §5.4.

---

## What is here

```
dev/demo-campaign/
  README.md              this file
  doc.go                 package doc
  projection_test.go     the projection test over the campaign
  vault/                 the campaign itself — phase 1's demo campaign
```

The campaign is **The Salt Debt**: a salt-marsh port city, a smuggling
crew, and a debt that comes due at the spring tide. It is written to be
read, because the prose is what makes the prototype worth looking at.

Run the test:

```
go test ./dev/demo-campaign/
```

Fifteen tests. Every one of them is a claim about the schema, and the
failing ones are the findings.

---

## The campaign

Saltmere stands where the sea comes in twice a day. The only dry road out
of it is the Crown's Causeway, and the Causeway is taxed; salt is the one
thing the marsh makes, and untaxed salt is worth three times taxed salt.
The difference is the whole economy, and it belongs to a crew called **the
Brinebacks**, who keep their debts in a ledger written in tides. On the
night of the wreck the party took a job, lost a lighter's worth of
untaxed salt into the water, and woke up owing four hundred marks. They
do not know yet that the salt was *meant* to be lost.

### The notes

| Note | `type` | `visibility` | `workflow` | `player` |
|---|---|---|---|---|
| `Home.md` | `home` | `party` | `active` | — |
| `Campaigns/saltdebt/The Salt Debt.md` | `campaign` | `party` | `active` | — |
| `Notes/World/Locations/The Drowned Quarter.md` | `location` | `party` | `active` | — |
| `Notes/World/Locations/Cormorant Stair.md` | `location` | **`discovered`** | `active` | — |
| `Notes/World/Factions/The Brinebacks.md` | `faction` | `party` | `active` | — |
| `Notes/World/Items/The Tideglass.md` | `item` | `party` | `active` | — |
| `Notes/Quests/Pay the Tide.md` | `quest` | `party` | `active` | — |
| `Notes/Quests/The Old Salt Road.md` | `quest` | `party` | `retired` | — |
| `Sessions/Session 01 - The Cormorant Wreck.md` | `session` | `party` | `active` | — |
| `Notes/World/Timeline/The Night of the Cormorant.md` | `event` | `party` | `active` | — |
| `Notes/Characters/Hessa Cray.md` | `character` | `party` | `active` | — |
| `Notes/Characters/Bel Orrery.md` | `character` | `party` | `active` | — |
| `Notes/Characters/Ilva Sarn.md` | `character` | `party` | `active` | `p_ilva` |
| `Notes/Characters/Orson Pike.md` | `character` | `party` | `active` | `p_orson` |
| `Notes/Characters/Wren Lock.md` | `character` | `party` | **`draft`** | `p_wren` |
| `Notes/Characters/Corvin Sallow.md` | `character` | **`gm`** | `active` | — |
| `Notes/Encounters/GM Only - The Reeve Collects.md` | `encounter` | **`gm`** | `active` | — |
| `Notes/Handouts/The Bond of Salt.md` | `handout` | `owner` | `active` | `p_ilva` (+ grant `p_orson`) |
| `Campaigns/saltdebt/Journals/Ilva - Salt and Iron.md` | `journal` | `private` | `active` | `p_ilva` |
| `Tables/Saltmere Faces.md` | `table` | `party` | `active` | — |
| `Notes/Handouts/The Causeway Notice.md` | `handout` | `public` | `active` | — (no campaign) |

Every case the brief asked for is here, and a few more: a **journal** with
`visibility: private`, a **handout** revealed to two players and not a
third, a **`public`** document, an **`event`** on the timeline, a
**table** of faces (ADR-0022's `data.faces`), and three **assets** — a
map and two portraits, of which one is referenced only by a GM-only note
and is therefore GM-only (ADR-0026).

### The projection, by hand

| Principal | Sees |
|---|---|
| `gm` | 20 of 21 — everything except the private journal, which is private even from the GM (ADR-0048) |
| `p_ilva` | 17 — the 14 shared notes, the public notice, the bond, and her own journal |
| `p_orson` | 17 — the 14 shared notes, the public notice, the bond, and the Cormorant Stair he discovered |
| `p_wren` | 16 — the 14 shared notes, the public notice, and his own draft character |
| `display` | 14 — the shared notes, and **nothing else** |
| `anonymous` | 1 — the notice nailed to the tide wall |

The 14 shared notes are: `Saltmere` (the `Home.md` dashboard),
`The Salt Debt`, `The Drowned Quarter`, `The Brinebacks`,
`The Tideglass`, `Pay the Tide`, `The Old Salt Road`,
`Session 01 - The Cormorant Wreck`, `The Night of the Cormorant`,
`Hessa Cray`, `Bel Orrery`, `Ilva Sarn`, `Orson Pike`, `Saltmere Faces`.

Three of these are the interesting ones:

- **Wren Lock's character is `visibility: party` and `workflow: draft`**,
  and no other member of the party can see it. The workflow gate is a
  grant-suppressing deny, and it is the only thing hiding it — `party`
  would have shown it.
- **The Cormorant Stair is `visibility: discovered`** and only Orson can
  see it, because only Orson's character has it in `discovered`. The
  shared display never sees it, even though two people can.
- **The Bond of Salt is `visibility: owner` with a grant**, so Ilva sees
  it, Orson sees it, Wren does not, and the shared display does not.

---

## Every schema correction

Each of these was forced by a specific note in the campaign. The
proposals are written up in [`docs/decisions/0050-schema-gaps-found-by-writing-a-campaign.md`](../../docs/decisions/0050-schema-gaps-found-by-writing-a-campaign.md); this is
the short form.

### 1. `campaign:` must be a link, not a slug

**The field.** `campaign`.

**What I wrote.** `campaign: "[[The Salt Debt]]"`.

**Why.** ADR-0037's example is `campaign: stormdark` — a slug. But
design §5.3 says identity must not move with renames, and that
"resolution goes through the vault index, which maps both filenames and
aliases to uids". A slug is neither a filename nor an alias, so nothing
in the system can resolve it. Every other link in the campaign is a
wikilink, and the extractor already treats a quoted wikilink as a string
rather than a nested sequence.

**Correction.** `campaign:` holds a wikilink to the campaign note. The
campaign note's own `campaign:` is the same link, self-referential.

### 2. `type: campaign` is not in the type vocabulary

**The field.** `type`.

**What I wrote.** `type: campaign`.

**Why.** ADR-0037 makes a campaign "a note of type `campaign`", and gives
that note `members` and `packs`. But design §5.1's table of note types
has fourteen entries and `campaign` is not one of them, and phase-1 M7a
says `demitable init` produces "one note of every **core** type in §5.1
of the design document" — so the scaffolder would not create the campaign
note, and ADR-0037's one new projection rule ("a campaign note's `party`
resolves against its own `members` list") would never fire.

**Correction.** Add `campaign` to §5.1's table and to the reserved
`type` vocabulary.

### 3. There is no way to say "the party has not discovered this yet"

**The field.** `visibility`.

**What I wrote.** `visibility: discovered` on the Cormorant Stair.

**Why.** The vocabulary is `public | party | owner | private | gm`. A
location the party has not found must be hidden, and the only value that
hides it is `gm` — which ADR-0033 rule 2 makes an **absolute deny that
no grant overrides**. So the location is hidden *for ever*: it can never
become visible to a player, not by a grant, not by a reveal, not by the
discovery that is supposed to cause it.

This makes **the "Discovery" grant in
[`session-writeback.md` §6](../../docs/session-writeback.md) dead text**:
it says a note is visible when "the note's uid appears in a character the
caller owns, in `discovered`", but there is no visibility level a
not-yet-discovered note can hold that a discovery could override.

**Correction.** A sixth value, `discovered`: visible to the GM, and to
any principal who owns a character whose `discovered` list names this
note. That is the state between `party` and `gm`, and it makes the
grant real.

**The cost of not having it.** Under the schema as it stands, the
Cormorant Stair is invisible to *everyone*, including Orson, who has
discovered it. The test asserts both behaviours:
`TestTheDiscoveryGrantIsUnreachableToday`.

### 4. `discovered` cannot be the list of records it is defined as

**The field.** `discovered`.

**What I wrote.** `discovered: ["[[Cormorant Stair]]"]`.

**Why.** [`session-writeback.md` §5](../../docs/session-writeback.md)
defines it as a list of objects — `uid`, `type`, `at`, `how` — and says
`how` distinguishes `visited` from `met` from `heard-of`. ADR-0040
requires the reserved layer to be simple-typed, because Obsidian's Bases
cannot query nested properties. The extractor's `Reserved` declares
`discovered` a `KindList`, and its `Coerce` flattens each element with
`String(e.String())` — so a list of maps would be written back as
`{at: 2026-03-14, how: visited}`. Three documents, three shapes.

**Correction.** `discovered` is a flat list of note references. The
`how`/`at` granularity has nowhere to live: it is either lost — and the
open question in §8 of the same document ("is a location discovered when
the character enters it, when they learn of it, or both?") becomes
unanswerable — or it moves to a separate note type. The demo records the
reference and loses the granularity, and says so in the prose.

### 5. The reserved layer has no visibility of its own

**The fields.** `discovered`, `grants`.

**What I wrote.** `discovered: ["[[Cormorant Stair]]"]` on Orson Pike's
note, which is `visibility: party`.

**Why.** ADR-0033 rule 7 gives *properties* a visibility, declared in the
pack, inheriting the note's otherwise — and `discovered` is not a pack
property, it is a reserved field, so no rule applies to it and it
inherits the note's. The consequence: **Orson Pike's character note, which
the whole party can read, publishes the name of a location the party has
not found.** Not the location's contents — its name.

**Correction.** The reserved set declares a visibility per field, as the
pack's content types already do. `discovered` and `grants` are
**owner-scoped** (visible to the note's `player` and the GM), with
ADR-0033 rule 7's "no owner, fall back to the note's visibility" applying
to a GM-created note that has no `player`.

### 6. `workflow: retired` means two different things

**The field.** `workflow`.

**What I wrote.** `workflow: retired` on a finished quest.

**Why.** ADR-0009's lifecycle table says `retired` is visible to "Author,
GM" — that is, hidden from the party. ADR-0033's rule 3 gates only
`draft` and `submitted`, so under it a retired note with
`visibility: party` **stays** party-visible. The two accepted ADRs
disagree about a note that is in the demo campaign.

**Correction.** One of them has to change. The proposal keeps ADR-0033's
rule 3 (`retired` is not a visibility gate) and amends ADR-0009's table,
because rule 3 is the authoritative visibility ADR and because
`workflow` is the *review* gate — its four values were written for a
player's character, not for a GM's quest board. The alternative — gate
`retired` too — has a consequence the GM would not expect: **closing a
quest would silently remove it from the players' board.**

The test asserts the disagreement:
`TestRetiredIsGatedOrNot`.

### 7. A handout's pre-reveal state is not representable

**The field.** `visibility`, on a `handout`.

**Why.** ADR-0033 says a handout "is normally `gm` and becomes visible to
the table when the GM reveals it", and its example is:

```yaml
type: handout
visibility: gm
grants: [p_8f2a]      # after the reveal
```

which is **invisible to the player it was granted to**, because rule 2
says `visibility: gm` is absolute and no grant overrides it. The example
and the rule are in the same ADR.

**Correction.** State the two states separately — pre-reveal
`visibility: gm`, post-reveal `visibility: party` (or `owner` plus
`grants`) — and amend the example so it does not show both at once. The
demo writes the post-reveal state: `visibility: owner`, `player: p_ilva`,
`grants: [p_orson]`.

### 8. There is no `status`, and phase 1 has no pack to declare one

**The field.** `status` — which ADR-0040 deliberately removed.

**What I wrote.** `status: alive` on Hessa Cray, `status: dead` on Bel
Orrery, `status: open` on the quest, `status: closed` on the retired
quest.

**Why.** ADR-0040 removed the generic `status` and said a pack declares
the fields its own content types need, with the example of a `quest`
content type declaring `stage` and `outcome`. **Phase 1's pack does not
declare them.** M2a's example pack declares one content type,
`character`, with two text fields. So in phase 1:

- a **quest** has no stage the app can read;
- an **item** has no attunement, no charges, no rarity;
- a **faction** has no standing;
- a **location** has no population.

Every one of those has to live in a GM-owned field the app never reads,
which is what `status:` is. The app's typed views (M6a: "the filters are
over the universal fields and the pack's declared fields") cannot filter
by any of them.

**Correction.** Either M2a declares a second content type — `quest` with
a `stage` field is the smallest that works — or ADR-0040's "a pack
declares the fields its content types need" is acknowledged as covering
characters only in phase 1, and the published schema says so.

### 9. M5c's property test is vacuous against this campaign

**The rule.** phase-1 M5c: "for every note in the demo campaign, every
**declared property**, and every non-GM principal, the serialised
projection contains no field whose visibility is `gm`, and no `owner`
field for a caller who does not own the note."

**Why.** M2a's pack declares no resources — no formulas, no resources, no
dice, no sheets. So the demo campaign declares **no properties with a
visibility**, and the property half of the property test iterates over
nothing.

**Correction.** M2a must declare at least one owner-scoped and one
gm-scoped property, or M5c must drop the property half until phase 2.
The demo campaign has no `data:` block that the pack does not declare, so
it cannot exercise the rule either way.

### 10. There is no type for a plain document

**The field.** `type`.

**What I wrote.** The reeve's notice, nailed to the tide wall, is a
`handout`.

**Why.** A notice is a document, and the fourteen types have no document
in them. `handout` is the nearest — "a prop revealed to the table, with a
journalled reveal event" — and it is wrong in two ways: this notice is
not revealed, it is nailed up; and a handout whose visibility is *mutable
at runtime* is a strange home for a thing that is `public` for ever.

**Correction.** Either a `document` type, or `handout` is split into
"prop revealed later" and "document that is simply there". Small, but the
demo needed it: a port city has notices, letters, ledgers and wanted
posters, and all four have to be a `handout` today.

### 11. A session log has no date field and no attendee field

**The fields.** `date`, `played` — the GM's own.

**What I wrote.**

```yaml
date: "2026-03-14"
played: [p_ilva, p_orson]
```

**Why.** The reserved set has no date. `date:` is the GM's own field —
which is allowed — but two things bite:

- **The value must be a quoted string.** An unquoted `2026-03-14` is a
  YAML timestamp, and the extractor reports it as an issue. The
  extractor's own writer quotes date-shaped scalars for exactly this
  reason (js-yaml resolves *any* `YYYY-MM-DD` to a timestamp, where
  yaml.v3 leaves an out-of-range one a string). So the GM has to know
  that a session's date is a *string* and not a date.
- **The same is true of a checkbox.** A top-level boolean is reported by
  the extractor as an issue (`IssueBool`), so a GM-owned checkbox
  property — the exact spelling ADR-0040 tells the app to write — is
  unrepresentable without a nested workaround. The Tideglass wanted an
  `attuned: false` and lost it.

**Correction.** The published schema (`schema.md`, M7c) must say what
type a GM-owned `date` is. The extractor's issue-reporting should be
raised only for a *reserved* field whose declared type is not boolean;
for an unreserved field, a boolean is a type the app can carry.

### 12. `faction`, `location` and `portrait` are shown as universal and are not reserved

**The fields.** `faction`, `location`, `portrait`.

**Why.** Design §5.2's example block puts them in the same list as the
twelve reserved names, and the demo needs all three: `faction:` on
Hessa Cray, `portrait:` on the two characters who have portraits, and a
`map:` on the Drowned Quarter — a fourth field with no home at all.

None of them is reserved. The app will never read them, and M6a's
filters cannot filter by them.

**Correction.** Either they join the reserved set — which makes it
fourteen or fifteen, and ADR-0040 promised twelve — or §5.2's example
stops showing them as universal. The proposal takes the second road and
adds a `map` to the same list, because a *reference to another note* and
*a reference to an asset* are both things the app needs to resolve, and
neither is a pack concern.

### 13. `visibility: public` and the campaign dimension are not reconciled

**The field.** `visibility`.

**Why.** Rule 1 (scope) is applied before rule 4 (the base level), so a
`public` note **inside a campaign** is not public: an anonymous caller
is not a member of any campaign and is refused at rule 1. And ADR-0048's
display rule says the shared display "sees exactly what `visibility:
party` allows", so a public document is not on the shared screen either.

The demo therefore puts the notice **outside** the campaign — it is a
document of the city, not campaign lore — which is a workaround, not a
decision.

**Correction.** Say which rule wins. The proposal keeps scope first (a
campaign is a boundary, and a note inside it may not leak out of it) and
adds: a `public` note *with* a campaign is public *within* the campaign,
which is what `public` is for inside a campaign boundary. The display's
rule is left alone and flagged for phase 3.

### 14. An Obsidian comment is not a secret mechanism

**The construct.** `%%…%%`.

**Why.** ADR-0035's secret vocabulary is "a callout of a secret type, an
admonition code block, a `gm: true` option on any code block, or an
explicit marker block". A comment is none of those — and design §5.6
lists `%%comments%%` as "Preserved, hidden in preview", which is a
*rendering* guarantee, not a *projection* guarantee. The body is
serialised into the response, so a comment is serialised too.

The demo hit this: the GM-only note in the session log was written as a
comment, and it leaked. It is now an unrevealed `> [!secret]` callout,
which is the correct mechanism — and the leak itself is proved by
`TestObsidianCommentsLeak`, on a synthetic note, so that the demo
campaign itself contains no comment in a party-visible note.

**Correction.** Either extend the secret vocabulary to `%%…%%`, or strip
comments from non-GM projections. Stripping is better: a comment is
already invisible in preview, so nothing player-facing is lost, and a
comment is the community's default place for a GM's aside.

### 15. Design §5.2's example block would fail the extractor's own check

**The rule.** the example YAML in design §5.2.

**Why.** It contains `visibility: party` **twice**. A note copied from
it has a duplicate key, and the extractor reports
`duplicate-key` — the one issue kind the writer refuses to patch
around. The demo campaign has no duplicate keys; all 21 notes parse with
**zero issues**.

### 16. A link to a note the caller may not see is not a rule anywhere

**The construct.** `[[…]]` in a body.

**What I wrote.** The dashboard links to `[[The Bond of Salt]]`, which is
`visibility: owner` with a grant — so Ilva and Orson can open it and
Wren cannot.

**Why.** The projection strips secret *blocks* from a body (ADR-0035)
and says nothing about secret *links*. The result is that a
party-visible note can link to a note most of the party cannot open, and
the player gets a 404, which reads as a broken link rather than as "you
are not allowed to read that". There is no rule for it in ADR-0033, and
no rule for it in the API.

**Correction.** A link whose target the caller may not see must render
as *unavailable* — present, not following — rather than as a 404. That
is a projection rule (and a rendering one), not a field, and it is
listed here because writing the campaign produced the case and nothing
else did.

---

## What is now known that was previously assumed

1. **The reserved set of twelve covers identity and visibility, and
   nothing else.** Eleven of the twenty-one notes needed something it
   does not provide. Counting the needs rather than the notes: five notes
   needed a lifecycle word (`status`), three a link to another note or an
   asset (`faction`, `portrait`, `map`), two a date and an attendance
   list (`date`, `played`), two a `discovered` list in a shape the
   extractor cannot write back, and one a `visibility` value that does not
   exist. (Hessa Cray and Corvin Sallow each needed two of these, so the
   needs outnumber the notes.)
2. **There is no state between "the party knows this" and "the GM knows
   this".** The vocabulary has four values and one of them is an absolute
   deny, so a not-yet-discovered location is invisible for ever, and the
   discovery grant in `session-writeback.md` §6 can never fire. It is
   dead text today.
3. **The reserved layer has no visibility of its own, and inherits the
   note's.** A party-visible character note publishes the *names* of
   locations the party has not found.
4. **`discovered` cannot be a list of records.** ADR-0040 (flat,
   simple-typed), `session-writeback.md` §5 (a list of objects) and the
   extractor's `Coerce` (which flattens maps to strings) are three
   incompatible shapes for one field.
5. **The secret mechanism covers blocks and not comments.** `%%…%%` is
   hidden in the preview and serialised into the projection.
6. **A public note and a campaign boundary have not been reconciled**,
   and the shared display's rule excludes public documents.
7. **`workflow` is doing two jobs** — the review gate and "no longer
   active" — and the two accepted ADRs disagree about what `retired`
   does to visibility.
8. **A handout has no representable pre-reveal state.** ADR-0033's own
   example note is invisible to the player it was granted to.
9. **The extractor is clean on hand-written notes** — 21 notes, 21 clean
   parses, zero issues — **but only because no note carries a top-level
   boolean or an unquoted date**, and both are ordinary Obsidian
   frontmatter. A GM who writes `attuned: false` gets a report.
10. **Phase 1's pack is not enough to make a campaign.** With one content
    type and two text fields, a quest, an item, a faction and a location
    have no machine-readable fields at all, and M5c's property test has
    nothing to iterate.
11. **A hand-written campaign can be checked.** The projection test in
    this directory resolves ADR-0033's model over the campaign and
    asserts the exact set of notes each of six principals may see — and
    the expectations are written down, so a change in the model is a
    change in a table rather than a change in a feeling.

---

## Contradictions found

| # | Contradiction | The ADRs that now disagree |
|---|---|---|
| 1 | A handout's example note (`visibility: gm` + `grants`) is invisible to the player it was granted to | **ADR-0033** rule 2 against **ADR-0033**'s own handout section |
| 2 | `workflow: retired` hides the note from the party, or does not | **ADR-0009**'s lifecycle table against **ADR-0033** rule 3 |
| 3 | `campaign: stormdark` (a slug) cannot be resolved by anything | **ADR-0037**'s example against design §5.3 (identity must not move; the index maps filenames and aliases to uids) |
| 4 | A campaign is a note of type `campaign`, and `campaign` is not a note type | **ADR-0037** against design §5.1's type table, and against phase-1 M7a |
| 5 | `discovered` is a list of records / a flat list / a list the writer flattens | **ADR-0040** against [`session-writeback.md` §5](../../docs/session-writeback.md), and against `internal/vault/frontmatter`'s `Reserved` and `Coerce` |
| 6 | The "Discovery" grant cannot fire | [`session-writeback.md` §6](../../docs/session-writeback.md) against **ADR-0033** rule 2 |
| 7 | The "Campaign" deny is absolute, but a guest seat's grant crosses campaigns | [`session-writeback.md` §6](../../docs/session-writeback.md) against **ADR-0038** ("anything explicitly granted to it") — not exercised by the demo, but the two cannot both hold |
| 8 | `faction`, `location` and `portrait` are shown as universal, and are not reserved | design §5.2's example against **ADR-0040**'s twelve |
| 9 | The app writes booleans as `true`/`false`, and a top-level boolean is reported as an issue | **ADR-0040** against `internal/vault/frontmatter`'s `IssueBool` |
| 10 | `internal/vault/frontmatter/value.go` cites "ADR-0049" for the js-yaml measurement, and there is no ADR-0049 in `docs/decisions/` | a dangling reference; the proposal is numbered **0050** to avoid the collision, and the orchestrator should check whether 0049 exists elsewhere |

---

## What phase 1 should change

| Milestone | Change |
|---|---|
| **M2a** | The example pack declares a second content type (`quest`, with a `stage` field) and at least one owner-scoped and one gm-scoped resource, or M5c's property test is vacuous. Its `id` must match the demo campaign's `system:` values. |
| **M5b** | The projection implements `visibility: discovered`; declares a visibility for every reserved field (`discovered` and `grants` owner-scoped); fails **closed** on an unknown `visibility` value; and states whether `workflow: retired` gates. |
| **M5b** | Non-GM projections strip Obsidian comments. |
| **M5c** | The property test adopts this campaign's expectation tables — six principals, twenty-one notes, exact sets — plus two assertions this spike added: *no GM-only character is named anywhere in a player's view*, and *no unrevealed secret's text appears in any non-GM projection*. |
| **M7b** | `dev/demo-campaign/vault/` becomes the shipped demo campaign, unchanged. |
| **M7c** | `schema.md` publishes: the reserved set with a visibility per field; `discovered` as a flat list of references; `campaign` as a wikilink; `type: campaign`; `date` as a GM-owned *string*; and the fact that a GM's own `status:` is untouched by the app. |
| **M1a** | The index resolves `campaign:` through the vault index like every other link, so a renamed campaign note still resolves. |

---

## What I would do differently

- **Write the secret first.** The reveal changes the *shape* of the note
  that holds it — a `revealed:` list appears in its frontmatter — and I
  only found that at the end, when the projection test asked where
  `saltgate` was.
- **Write the draft character last.** The workflow gate interacts with
  `visibility`, and the interesting case — `visibility: party` with
  `workflow: draft`, where the gate is the only thing hiding the note —
  only became clear once I had written the other four characters.
- **Write the expected projection before the resolver.** I wrote the
  resolver first and the expectation table after, and the display's and
  `public`'s cases changed twice. The table is the specification; it
  should have come first.
- **Read `session-writeback.md` §5 before writing the campaign.** That
  one section — the discovery records — would have surfaced the
  `discovered` contradiction at the start instead of at the end.
- **Give the campaign a third player who has played.** With two players
  in the sessions, the display-is-not-the-union rule and the grant rule
  both need a third case to be interesting, and I had to invent the
  third member (Wren) to get one.
- **Check the demo campaign against the extractor before writing the
  prose.** The first draft of the session log had an unquoted date and a
  boolean; both would have been reported as issues, and both would have
  been wrong.

---

## What failed, honestly

- **The `%%` comment leak is proved on a synthetic note, not on the
  campaign.** I wrote a GM comment inside a party-visible note, the
  projection leaked it, and I moved the material into an unrevealed
  secret. The demo campaign is clean; the leak is in the test.
- **The `public` document had to be moved out of the campaign** to mean
  anything, because rule 1 refuses an anonymous caller before rule 4 is
  reached. The demo works around it rather than settling it.
- **"Visited on 14 March" could not be said at all.** The demo records
  that Orson found the Cormorant Stair, and nothing else.
- **The quest's stage, the item's attunement and the faction's standing
  do not exist**, and the demo had to invent GM-owned fields for them
  that the app will never read.
- **The projection test is a disposable re-implementation of the model.**
  It proves the *campaign* is consistent with ADR-0033 and that the model
  produces the right thing for this campaign. It does not prove the real
  projection is right — the real one does not exist yet. Phase 1 must
  re-implement it and re-run these cases; the expectation tables are the
  part worth keeping.
- **The extractor has no opinion about the schema.** It parses, and it
  reports constructs it cannot represent. It cannot tell me that
  `visibility: discovered` is not in the vocabulary, or that `status` is
  not reserved. Those were found by reading, not by running.
