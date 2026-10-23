# ADR-0057: The visibility model computes audiences, not rule verdicts

- **Status:** Accepted
- **Date:** 2026-10-23
- **Amends:** ADR-0033 (the rule ordering), ADR-0002 (the trust boundary),
  ADR-0009 (the workflow table), ADR-0038 (the guest seat), ADR-0040 (the
  reserved field set), ADR-0048 (`private`, the display principal)
- **Extends:** ADR-0035 (secret reveals), ADR-0036 (secret syntaxes),
  ADR-0037 (campaigns are notes)
- **Addresses:** the twenty findings of
> [`spikes/projection/`](../spikes/projection/README.md), and the phase-1
> criterion "a GM-only note is provably unreachable by a player"

> **Origin:** phase 0, question 2 — *does the visibility model produce the
> right thing on real data?*

## Context

ADR-0033 completed the visibility model, and it is a good model. It has a
default, a property-level rule, a no-owner rule, a workflow gate, one
definition of `public`, and an absolute deny. It was written as an **ordered
decision procedure**: eight rules, and "the first rule that applies wins".

The phase-0 projection spike ran that model over the SRD corpus and over a
synthetic corpus built to exercise it, and found that the ordering — not the
content — is where the model breaks.

Seven of the twenty findings are the same defect in different clothes:

- **`private` is not private in `draft`.** Rule 3 (the workflow gate) says
  "its owner **and the GM**" and it fires before rule 4 (the base level), so
  a `visibility: private` note in `workflow: draft` is visible to the GM.
  ADR-0048 says `private` means "the note's `player`, and nobody else — not
  even the GM", without exception.
- **The GM cannot see a player's hit points.** Rule 7 says a property
  declared `owner` "resolves against the note's `player`". Rule 4 says the
  note-level `owner` means "the note's `player`, **and the GM**". Rule 7
  does not repeat the second half, so the GM is excluded — against
  ADR-0002, which says explicitly: "a player's HP is visible to them **and
  the GM**, not to the other players".
- **A handout cannot be revealed.** ADR-0033's own example is
  `visibility: gm` + `grants: [p_8f2a]`. Rule 2 says no grant overrides
  `gm`.
- **A guest cannot play.** ADR-0038 says a seat's bound character is
  "implicitly visible to that seat, as if the seat had been granted it" — and
  rule 3 says a grant never overrides the workflow gate, and rule 1 says a
  note in a campaign is visible only to that campaign's members, and a seat
  is not a member. Under the literal ordering the seat sees nothing.
- **`visibility: owner` with no `player` and `visibility: party` with no
  `campaign` are black holes**, because rule 6's default covers an *absent*
  value and these are present values with no audience.

The common shape: **"the first rule that applies wins" means some rules
never run, and the rules that lose are the narrowing ones.** A model of
*narrowing constraints* has no such failure mode: `private` is a constraint,
so it applies whatever the workflow gate says; the GM is a member of the
`owner` audience, so it is a member of the property-level `owner` audience
too.

Two further findings are not about ordering but about the *schema*:

- **ADR-0040's twelve reserved fields are published wholesale.** Every
  top-level field inherits the note's visibility unless a pack declares
  otherwise — and no pack can declare a reserved field, because they are
  universal. So a party-visible player character shows the whole party
  `player:`, `workflow:`, `visibility:`, `grants:` and `discovered:`.
  `discovered` is per-character private state by ADR-0048's own reasoning,
  and it is published.
- **A reveal to one player is a reveal to every player.** ADR-0035's durable
  record is `revealed: [id]` — an id and no audience — while its live record
  has an audience. Measured: a secret revealed to one player reaches four
  non-GM principals the moment it is written back.

And one about the *pipeline*:

- **The pack's content-visibility defaults protect nothing at read time.**
  They are applied when the enrichment emits a note. Measured on the real
  corpus: a pipeline that stamps `campaign:` and forgets `visibility:` gives
  one player all 1,693 notes, 8,407 fields and 2,738,250 body bytes —
  including the 370-note, 1,065,488-byte bestiary. The note-level default
  (a note with a campaign is `party`) is what applies.

## Decision

### The model is a set of audiences

A note's audience is computed, not adjudicated. For a note `n` and a
principal `p`:

> **audience(n, p)** = the set of principals who may see `n`, computed as the
> **intersection** of every applicable constraint, then **widened** by the
> grants that are explicitly permitted to widen it.

The constraints, in the order they are *written down* but not in the order
they are *applied*:

| # | Constraint | Effect |
|---|---|---|
| C1 | **Scope** | a note with `campaign: X` has an audience inside X (plus the GM). A note without one has an audience of "nobody but the GM, plus explicit grants" |
| C2 | **The absolute deny** | `visibility: gm` ⇒ the audience is empty. No grant, discovery, seat binding or workflow state reopens it |
| C3 | **The workflow gate** | `workflow: draft`/`submitted`/`retired` ⇒ the audience is {the note's `player`} ∪ {the GM} |
| C4 | **The base level** | `public` ⇒ everyone · `party` ⇒ the campaign's members · `owner` ⇒ {the note's `player`} ∪ {the GM} · `private` ⇒ {the note's `player`} · `gm` ⇒ {the GM} |
| C5 | **The default** | `visibility:` absent ⇒ `party` if the note has a campaign, `gm` if it does not. A value that is **present but unknown** has no audience (fail closed) and is a **write-time error** |
| C6 | **The owner's exclusions** | `private` removes the GM, at note level and at property level, whatever C3 says |
| C7 | **The property level** | a property's declared visibility replaces C4 for that property, inside a visible note. `owner` resolves against the note's `player` **and the GM**; with no `player` it falls back to the note's visibility |
| C8 | **Masking** | `display:` replaces the *value* per audience, after the audience is known. It never changes *whether* a value is present |

Then, and only then:

| # | Widening rule | Effect |
|---|---|---|
| W1 | **Grants** | `grants: [p_x]` adds `p_x` — **including across a campaign boundary** |
| W2 | **Discovery** | the note's uid appears in a character the caller owns ⇒ the caller is added |
| W3 | **The seat's binding** | the seat's bound character is added, and the seat is added to any note granted to it |

**W1–W3 are applied after C1–C8, and they cannot reopen C2 or C3.** The
difference from ADR-0033 is that C1 is a constraint on the *base level*, not
on the grants: a grant may cross a campaign boundary, and that is
deliberate. It has to be, or ADR-0038's guest case and ADR-0033's handout
case are inexpressible.

### The amendments this forces

**A1 — `owner` includes the GM, at both levels.** *(still open — ADR-0050 D5 scoped only the reserved fields `discovered` and `grants` to player+GM; pack-declared `owner` (rule 7) still omits the GM, so finding 9 survives)*
Rule 7 becomes: *"`owner` on a property resolves against the note's
`player` **and the GM**. If the note has no `player`, the property falls
back to the note's visibility."* This matches ADR-0002 and rule 4, and it
makes the GM able to see a player's hit points, which is the GM's job.

**A2 — `private` excludes the GM, always.** *(partially landed — ADR-0050 D3 keeps `private` excluding the GM; the amendment to ADR-0038's "GM's view is never filtered" is still open, so finding 6 survives)*
Rule 4 becomes: *"`private` — the note's `player`, and nobody else, not even
the GM."* And ADR-0038's "the GM's view is never filtered" is amended to
"the GM's view is filtered by exactly one thing: `visibility: private`."

**A3 — `workflow: retired` is gated.** *(resolved the other way — ADR-0050 D6 lands on ADR-0033's side: retired is NOT a visibility gate; finding 8 resolved)*
ADR-0009's workflow table is right and ADR-0033's rule 3 was incomplete:
`retired` joins `draft` and `submitted` in the gate. A retired character is
the author's and the GM's.

**A4 — a seat's binding is not a grant.** *(still open — finding 14/15 survive)*
ADR-0038's "as if the seat had been granted it" is replaced: **the seat's
bound character is part of the seat's identity, and the workflow gate
applies to accounts, not to the seat the GM assigned.** A guest may play a
character that is still in draft, because the GM assigned it.

**A5 — the campaign filters the base level, not the grants.** *(still open — finding 14 survives; ADR-0050's "does not settle" list leaves the guest's cross-campaign grant open)*
Rule 1 becomes: *"A note with `campaign: X` resolves its `party` and
`owner` audiences inside X. A grant, a discovery or a seat binding may
cross that boundary; the GM has said so explicitly by writing the id down."*
This is the one place where the campaign is not a hard boundary, and it is
recorded as a decision rather than left to the order of two rules.

**A6 — a note-level owner fallback, and two write-time errors.** *(partially landed — ADR-0050 D3 lands the "unknown value fails closed" write-time error and the `discovered` value; the `party`-with-no-`campaign` linter error and the note-level `owner` fallback are still open)*
* `visibility: owner` on a note with no `player` falls back to the note's
  campaign scope (`party`), mirroring rule 7's property fallback.
* `visibility: party` on a note with no `campaign` is a **linter error**.
  The model has no way to say "shared with everyone in the vault", and a
  silent black hole is worse than a refused write.
* A `visibility:` value outside the vocabulary is a **linter error** and a
  read-time fail-closed.

**A7 — the reserved fields get visibility rules.** *(partially landed — ADR-0050 D5 lands the table, but only for `discovered` and `grants` (`owner`-scoped); the other ten fields inherit the note's visibility, which is the landed decision)*
ADR-0040's twelve names are universal, so no pack can declare them, and
today they all inherit the note's visibility. They should not:

| Field | To whom, inside a visible note |
|---|---|
| `type`, `uid`, `name`, `system`, `campaign`, `kind` | everyone who can see the note |
| `player` | the note's `player` and the GM |
| `workflow` | the note's `player` and the GM |
| `visibility` | everyone who can see the note |
| `grants` | the note's `player` and the GM — **a grant list is a targeting decision** |
| `revealed` | the note's `player` and the GM |
| `discovered` | **the note's `player` only** — it is per-character state (ADR-0048) |

This is the smallest change that stops a party-visible player character from
publishing its owner's discovery list to the table.

**A8 — `revealed:` carries an audience.** *(landed — ADR-0050 D8, with `players:` absent meaning the whole campaign; finding 16 fixed)*
ADR-0035's durable record becomes:

```yaml
revealed:
  - id: whisper
    players: [p_8f2a]      # absent = everyone in the campaign
```

and the secret block's id is **scoped to its note** — because the corpus has
355 blocks that all carry the id `^statblock`, and a global id list would
reveal all 355 at once.

**A9 — the pack's content-type default is a projection-time fallback.** *(still open — finding 2 survives)*
The projection falls back to the pack's declared `visibility:` for a
`content_type` when the note carries no `visibility:` of its own. Today it
does not, and the measured consequence of that omission is the entire SRD
going to one player.

**A10 — masking gets a syntax, and the audiences are named.** *(still open — finding 18 survives)*
`display:` is a map on the property, keyed by audience:

```yaml
data:
  attributes:
    str:
      value: 10
      display:
        party: "??"
```

The audiences are `gm`, `party`, `owner` and `public`, resolved in that
order of specificity. **A masked property is a mapping, not a scalar**, which
is an exception to ADR-0040's "simple-typed, top-level, Bases-queryable"
rule and must be recorded as one — or masking moves to a sibling field
(`str_display`), which keeps the type simple at the cost of the shape.

**A11 — ADR-0035's block-id example is corrected.** *(partially landed — ADR-0050 D12 specifies comment stripping, but block-id location is still unspecified; finding 17 survives)*
Obsidian writes a block reference on the line **after** the block. All 614
of the corpus's block references are written that way, and ADR-0035's
example (`> [!secret] The key ^vaultkey`) is not. The example is corrected,
the hash's "normalised source" is defined, and the note says that a block
id is only unique **within its note**.

## Consequences

**Good:**

- **The GM can see a player's hit points.** A one-word fix to rule 7, found
  by running the model on a character sheet.
- **`private` means what ADR-0048 says it means**, in every state.
- **The guest can play.** ADR-0038's guest case and ADR-0033's handout case
  both become expressible.
- **A reveal to one player is a reveal to one player.**
- **The SRD cannot leak by omission.** A pipeline that forgets a
  `visibility:` gets the pack's declared default instead of the party.
- **The reserved fields stop publishing the table's private state.**
- **The model has one shape.** "Intersect the constraints, then widen" is a
  sentence a human can hold in their head, which is what ADR-0033's own
  "why this needs a UI" note asks for.

**Bad:**

- **It is a rewrite of the resolver**, not an amendment to it. The eight
  rules become eight constraints plus three widenings, and the "first rule
  that applies wins" phrasing — which the phase-1 inspector and the
  linter are both built against — has to go.
- **A9 makes the projection depend on the pack.** Today the projection
  needs no pack; after A9 it does, and a note whose content type no
  installed pack declares falls back to the note-level default. That is a
  new coupling between the vault service and the pack loader.
- **A7 gives ADR-0040 a second table.** The reserved set is no longer
  "twelve names with types and vocabularies"; it is twelve names *with
  visibility semantics*, and the published schema has to say so.
- **A8's `revealed:` shape is not a list of strings**, so the writer's
  type-correct YAML rules (ADR-0040) need a case for it, and the extractor's
  round-trip needs to keep it.
- **A10's exception to "simple-typed" has to be explained** to anyone whose
  Bases view stops working on a masked field.

**Neutral:**

- **The campaign is still the multi-tenant boundary** for `party`; it is no
  longer a boundary for explicit grants. A GM who wants to share a note
  across campaigns writes the id down, and the model records that it was
  deliberate.
- **The default rule is unchanged.** A note with a campaign is `party`; a
  note without one is `gm`.
- **`visibility: public` is unchanged**: anyone, authenticated or not.

## What phase 1 should change because of this

| Milestone | Change | Why |
|---|---|---|
| **M2** (pack format) | The content type's declared `visibility:` becomes a **projection-time fallback**, not only a write-time default | Measured: the alternative publishes 2,738,250 bytes of SRD to one player |
| **M5b** (the projection) | The resolver is rewritten as constraints-and-widenings; the property-level `owner` includes the GM; `private` excludes the GM; the seat's binding beats the workflow gate; the reserved fields get their own visibility rules | The findings above; the "provably unreachable" criterion is not testable until the model has one shape |
| **M5c** (the property test) | The property test gains: *no non-GM principal sees a `gm` note or a `gm` property*; *no non-GM principal sees an `owner` field it does not own*; *no non-GM principal sees a `discovered` list it does not own*; *a seat sees its bound character*; *a reveal to one player does not reach another* | Each is a finding turned into an assertion |
| **M5c** (the linter) | `visibility: party` with no `campaign`, and a `visibility:` outside the vocabulary, are **write-time errors** | Two silent black holes |
| **M7** (the demo campaign) | The demo campaign must cover the 33 cases in the synthetic corpus | The SRD corpus cannot; this is the only corpus that can |
