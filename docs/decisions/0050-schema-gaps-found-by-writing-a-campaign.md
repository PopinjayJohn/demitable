# ADR-0050: The schema needs a discovery state, a reserved-layer visibility, and an honest field list

- **Status:** Accepted
- **Date:** 2026-10-23
- **Amends:** ADR-0033 (the visibility model), ADR-0040 (the frontmatter
  field namespace), ADR-0037 (a campaign is a note), ADR-0035 (secret
  callouts), ADR-0048 (display, chat and journals), ADR-0009 (the
  character review workflow), ADR-0049 (the frontmatter I/O contract)
- **Extends:** design doc §5.1 (note types), §5.2 (the frontmatter
  contract) and §5.4 (the visibility model)
- **Addresses:** the gaps found by writing a campaign by hand, in phase 0
  question 8

> **Origin.** Phase 0, question 8: *what is the schema missing?* The
> evidence is a written campaign — `dev/demo-campaign/vault/` — and a
> projection test over it, `dev/demo-campaign/projection_test.go`. Every
> decision below was forced by a specific note in that campaign; the
> README in that directory cites the note.
>
> **A schema you have never used is a schema you have not tested.** Eleven
> of the twenty-one notes needed a field, a value or a shape the twelve
> reserved names do not provide, and one gap could not be closed by any
> field at all.

## Context

A schema that has never been used is a schema that has not been tested.
Phase 0 wrote twenty-one notes of a real campaign — a salt-marsh port
city, a smuggling crew, a debt that comes due — against the schema as
the accepted ADRs define it, and ran a projection over them.

Eleven of the twenty-one notes needed a field, a value or a shape that
the twelve reserved names do not provide. More seriously, the campaign
found **three places where the model cannot express a thing the campaign
needs at all**:

1. **There is no state between "the party knows this" and "the GM knows
   this."** The visibility vocabulary is `public | party | owner |
   private | gm`, and `gm` is an absolute deny that no grant overrides
   (ADR-0033 rule 2). A location the party has not discovered therefore
   has to be `gm` — which makes it invisible for ever. The "Discovery"
   grant listed in `session-writeback.md` §6 is **unreachable text**.
2. **The reserved layer has no visibility of its own.** ADR-0033 rule 7
   gives pack-declared properties a visibility; the reserved fields are
   not pack-declared, so they inherit the note's. A party-visible
   character note therefore publishes the *names* of notes the party has
   not discovered.
3. **A handout has no representable pre-reveal state.** ADR-0033's own
   example note — `visibility: gm` together with `grants` — is invisible
   to the player it was granted to, because rule 2 is absolute.

And three documents disagree about the shape of one field: ADR-0040
requires the reserved layer to be simple-typed, `session-writeback.md` §5
defines `discovered` as a list of objects, and the extractor's `Coerce`
flattens a list of maps to strings when it writes.

---

## Decision

Each decision below names the field or rule it changes.

### D1 — `campaign:` holds a link to the campaign note

**The field.** `campaign`.

```yaml
campaign: "[[The Salt Debt]]"
```

ADR-0037's example is `campaign: stormdark` — a slug. Nothing resolves a
slug: design §5.3 says identity must not move with renames, and that
resolution "goes through the vault index, which maps both filenames and
aliases to uids". A slug is neither. Every other reference in the vault
is a wikilink, and the extractor already reads a quoted wikilink as a
string.

A campaign note's own `campaign:` is the same link, self-referential.

### D2 — `campaign` is a note type

**The field.** `type`.

ADR-0037 makes a campaign "a note of type `campaign`", and design §5.1's
table of note types does not list it. Phase-1 M7a creates "one note of
every core type in §5.1", so the campaign note would never be created
and ADR-0037's one new projection rule would never fire.

`campaign` is added to design §5.1's table and to the reserved `type`
vocabulary.

### D3 — a sixth visibility value: `discovered`

**The field.** `visibility`.

```yaml
visibility: discovered
```

| `visibility` | Who |
|---|---|
| `public` | Anyone, authenticated or not — a link is enough |
| `party` | All members of the note's campaign |
| `owner` | The note's `player`, and the GM |
| `private` | The note's `player`, and nobody else — not even the GM |
| `discovered` | The GM, and any principal who owns a character whose `discovered` list names this note |
| `gm` | The GM only. Never serialised to a non-GM role |

`discovered` is the state between `party` and `gm`: the party may
legitimately not know that a thing exists yet, and finding out is an
event. It makes the "Discovery" grant in `session-writeback.md` §6 live
rather than decorative.

**An unknown `visibility` value must fail closed.** A note whose
`visibility` is not in the vocabulary is treated as `gm`. The projection
must never guess that an unrecognised value means "more visible".

### D4 — `discovered` is a flat list of note references

**The field.** `discovered`.

```yaml
discovered:
  - "[[The Drowned Quarter]]"
  - "[[Cormorant Stair]]"
```

The reserved layer is simple-typed, because Obsidian's Bases cannot query
nested properties (ADR-0040). `session-writeback.md` §5's list of
records — `uid`, `type`, `at`, `how` — is therefore not writable: the
extractor's `Coerce` flattens a list of maps to strings.

**The consequence is stated rather than hidden.** The `how` of a
discovery — `visited`, `met`, `heard-of` — and the `at` of it have
nowhere to live in the reserved layer, and the open question in
`session-writeback.md` §8 ("is a location discovered when the character
enters it, when they learn of it, or both?") is unanswerable until they
do. If the granularity is needed, it becomes a **separate note type**
(`discovery`), not a nested field in a reserved one.

### D5 — the reserved set declares a visibility per field

**The fields.** `discovered`, `grants`.

ADR-0033 rule 7 gives pack-declared properties a visibility and lets
them inherit the note's otherwise. The reserved fields are not
pack-declared, so the same fallback is applied to them — with two
exceptions, because the fallback leaks:

| Field | Visibility |
|---|---|
| `discovered` | `owner` — the note's `player`, and the GM |
| `grants` | `owner` — the note's `player`, and the GM |
| every other reserved field | the note's visibility |

ADR-0033 rule 7's fallback applies: **a note with no `player` falls back
to the note's visibility.** A GM-created note has no `player`, so its
`discovered` and `grants` lists are as visible as the note is — which is
correct, because there is nobody else they could be hiding from.

### D6 — `workflow: retired` is not a visibility gate

**The field.** `workflow`.

ADR-0033 rule 3 gates `draft` and `submitted`. ADR-0009's lifecycle table
says `retired` is visible to "Author, GM". The two disagree about a note
that is in the demo campaign.

**ADR-0033's rule 3 stands**, and ADR-0009's table is amended: `retired`
is the *review* gate's end state, not a visibility gate. The rule 3 gate
is about who may speak for a thing — a player's own character — and a GM
closing a quest must not silently remove it from the players' board.

The reserved `workflow` vocabulary stays `draft | submitted | active |
retired`, and the GM's own `status:` is untouched, as ADR-0040 says.

### D7 — a handout's two states are stated separately

**The field.** `visibility`, on a `handout`.

| State | `visibility` | What else |
|---|---|---|
| Before the reveal | `gm` | no grants; the note is nobody's but the GM's |
| After the reveal | `party`, or `owner` + `grants` | the reveal is journalled (ADR-0033, ADR-0015) |

ADR-0033's example note, which shows `visibility: gm` and `grants` in the
same block, is amended: the two states are one state or the other. Rule 2
is absolute, and a reveal that leaves the note `gm` reveals nothing.

### D8 — the reserved set is unchanged, and the design's example stops implying otherwise

**The fields.** `faction`, `location`, `portrait`, `map`.

Design §5.2's example block shows `faction`, `location` and `portrait`
beside the twelve reserved names, and the demo campaign needs all three
plus a `map`. None of them is reserved, so the app never reads them and
M6a's filters cannot filter by them.

**The reserved set stays at twelve.** ADR-0040's "twelve names is a
promise" is the right call, and the fields are not universal: they are
the GM's, and the app leaves them alone, as ADR-0040 says it leaves
`status:` alone.

What changes is design §5.2's example block: it stops showing them as
universal, and §5.1's "the reserved set" table gains a sentence saying
that a reference to another note and a reference to an asset are the GM's
own fields, resolved by the index and the asset endpoint respectively.

### D9 — `type: document`, or `handout` split

**The field.** `type`.

A notice nailed to a tide wall is not a prop that was revealed to the
table, and it is not an item, an event or a message. The demo campaign
needed one and had to call it a `handout`.

Either a `document` type is added — a thing that is simply there: a
notice, a letter, a wanted poster, a ledger page — or `handout` is split
into "prop revealed later" and "document that is present". This decision
does not change the projection; it changes what the app is asked to find.

### D10 — the published schema states the type of a GM-owned `date`

**The field.** `date` (the GM's own).

There is no reserved date field, so a session log's date and a timeline
entry's date are the GM's own `date:`. **It must be a quoted string.**
An unquoted `2026-03-14` is a YAML timestamp, which the extractor reports
as an issue and which js-yaml resolves to a date where yaml.v3 may not.
The extractor's own writer quotes date-shaped scalars for exactly this
reason.

M7c's `schema.md` says so, and says it of every date-shaped GM field.

### D11 — the extractor reports a boolean only for a reserved field

**The rule.** `IssueBool`.

ADR-0040 tells the app to write booleans as `true`/`false`, and Obsidian
has a `checkbox` property type. The extractor reports **every**
top-level boolean as an issue, so a GM-owned checkbox property — the
spelling ADR-0040 mandates — is reported as unclean.

The issue should be raised only when the field is **reserved** and its
declared type is not boolean. For an unreserved field, a boolean is a
type the app can carry and there is nothing to report.

(This is a change to `internal/vault/frontmatter`, which phase 0 did not
touch. The extractor was not wrong about YAML; it was asking a question
about the *schema* that it has no way to know the answer to.)

### D12 — Obsidian comments are stripped from non-GM projections

**The construct.** `%%…%%`.

ADR-0035's secret vocabulary is a callout, an admonition, a `gm: true`
code-block option, or an explicit marker block. A comment is none of
those, and design §5.6's "Preserved, hidden in preview" is a rendering
guarantee, not a projection guarantee: the body is serialised, so the
comment is serialised.

Comments are stripped from a non-GM projection. A comment is already
invisible in preview, so nothing player-facing is lost, and a comment is
the community's default place for a GM's aside — which makes it the
*most likely* place for a secret to be written by accident.

### D13 — `public` and the campaign boundary

**The field.** `visibility`.

Rule 1 (scope) is applied before rule 4 (the base level), so a `public`
note inside a campaign is not visible to an anonymous caller — the caller
is not a member of any campaign. That stands: **a campaign is a
boundary, and a note inside it may not leak out of it.**

What is added is the other half: **a `public` note with a campaign is
public within that campaign** — visible to every member of it, signed in
or not, and to nobody in another campaign. `public` means "not private
to the party", not "outside the boundary".

A `public` note with **no** campaign is public absolutely, and is the
only kind of note an anonymous caller can see. (ADR-0033's rule 6 already
makes a campaign-less note `gm` by default, so this must be said
explicitly.)

ADR-0048's display rule is left alone: the shared display sees
`visibility: party` content. A `public` document is therefore not on the
shared screen, which is worth a decision in phase 3 rather than a
clarification here.

### D14 — the design's example block is corrected

**The rule.** design §5.2's example YAML.

It contains `visibility: party` **twice**. A note copied from it has a
duplicate key, and the extractor reports `duplicate-key` — the one issue
the writer refuses to patch around. The example is corrected.

### D15 — a link to a note the caller may not see renders as unavailable

**The construct.** `[[…]]` in a body.

The projection strips secret *blocks* from a body (ADR-0035) and says
nothing about secret *links*. The demo campaign's dashboard links to
`[[The Bond of Salt]]`, which is `visibility: owner` with a grant, so two
members of the party can open it and the third cannot.

A link whose target the caller may not see is rendered as **present but
not followable** — not as a 404, which reads as a broken link rather
than as "you are not allowed to read that". The projection knows which
targets the caller may see, because it has already resolved the note
set; it is a rendering rule and an API rule, and it is stated here
because it is missing from both.

---

## Consequences

**Good:**

- The campaign can be written. A not-yet-discovered location is a note
  with `visibility: discovered`, and a character that found it records
  the reference; the discovery grant in `session-writeback.md` §6 fires.
- A party-visible character note stops publishing the names of places the
  party has not found.
- The reserved set keeps its promise of twelve names, and the fields that
  were only pretending to be universal stop pretending.
- The three documents that disagreed about `discovered` now agree, and
  the loss of the `how`/`at` granularity is stated rather than hidden.
- `public`, `party`, `owner`, `private`, `discovered` and `gm` are six
  values that a GM can choose between without consulting the ADRs, and
  every one of them is exercised by a note in the demo campaign.

**Bad:**

- **The reserved set grows a per-field visibility table.** Twelve fields
  each with a default is more to document, more to lint, and more for the
  inspector to explain.
- **A sixth visibility value is a new thing for every reader of the
  model to learn**, and it is one more rule in the one piece of code
  whose job is to be obviously correct.
- **`discovered` loses its granularity**, and until a `discovery` note
  type exists, "visited" and "heard of" are the same fact.
- **D6 changes ADR-0009's table**, which some readers will take as a
  regression: a retired character is still visible to its owner and the
  GM under ADR-0009, and under ADR-0033 rule 3 it is visible to the
  party as well.

**Neutral:**

- D1, D2, D8, D9, D10 and D14 change documents and the app's vocabulary,
  not the projection's behaviour.
- The demo campaign is written against the schema *as it stands* wherever
  the schema could carry it, and against the schema *as proposed* only
  where it could not: the single `visibility: discovered` in it is the
  one note that a phase-1 reader must understand as a proposal.

---

## What this decision does not settle

- **The granularity of discovery** — whether a location is discovered by
  the character who walked there or by the party that heard about it. D4
  removes the ability to record it and leaves the question open.
- **Whether the shared display shows `public` documents** (D13).
- **Whether a guest seat's grant crosses a campaign boundary.**
  `session-writeback.md` §6 lists the campaign deny as absolute, and
  ADR-0038 says a guest sees "anything explicitly granted to it" — and a
  guest is not a campaign member. The demo campaign does not exercise
  this, and the two rules cannot both hold.
- **The published schema's list of GM-owned fields.** D8 says the reserved
  set stays at twelve; it does not enumerate what a GM may put in the
  rest of the frontmatter, and M7c's `schema.md` will have to.
