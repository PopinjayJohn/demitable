# ADR-0033: The campaign dimension, grant sets, and a completed visibility model

- **Status:** Accepted
- **Date:** 2026-10-22
- **Amends:** ADR-0002 (trust boundary), ADR-0009 (accounts),
  ADR-0017 (between-sessions access), and the visibility model in
  [`session-writeback.md`](../session-writeback.md) §6
- **Addresses:** [#3](https://github.com/PopinjayJohn/demitable/issues/3)
  (handouts), [#8](https://github.com/PopinjayJohn/demitable/issues/8)
  (paragraph-level secrecy), [#11](https://github.com/PopinjayJohn/demitable/issues/11)
  (no campaign dimension), and
  [#24](https://github.com/PopinjayJohn/demitable/issues/24) (the visibility
  model has no default and no property-level resolution)

## Context

Three things came out of the research that the visibility model could not
express.

**A campaign is not a vault.** The Obsidian community's convention is *one
vault, many campaigns* — one folder per campaign, plus a shared setting
folder. Our design assumed one server, one vault, one campaign, and ADR-0002's
`visibility: party` means "all players in the session". With two campaigns in
one vault, "party" would **span both**, which is a leak. Fixing it is a
*schema* change, and the design's own rule says structure is expensive to
change later.

**A handout is not an attachment.** An asset is visible if a note visible to
the player references it. But a handout's whole point is that the note
referencing it is *GM-only*, and the handout must become visible *at a chosen
moment* to a *chosen subset* of players. All three major rivals have this as
a named feature; none of our four visibility levels can express "these three
players".

**Secrets can be inside a shared note.** A letter the players can read, where
the GM's copy also says something else. Foundry has a first-class "Secret"
paragraph format for exactly this. Our model is note-level and value-level,
so it has nowhere to put it — and worse, the one absolute rule we have
(`visibility: gm` is absolute) makes it impossible to *reveal* such a thing
later.

And the model was **incomplete** in ways the review found: no default for an
absent `visibility`, no property-level grants at all, no rule for a property
whose owner does not exist, no `workflow` in the formal model, and `public`
defined three different ways.

## Decision

### The campaign dimension

A new **universal field**, `campaign`, on every note:

```yaml
type: character
campaign: stormdark
```

- A note **with** a campaign belongs to that campaign.
- A note **without** one is campaign-less — a personal or global note.
- The server serves **one vault**, holding any number of campaigns. The player
  joins a *campaign*; a join code names it.
- `visibility: party` means **"all players in this note's campaign"** — not
  "in this session". This is the leak fixed.

### The completed visibility model

A note's visibility is resolved in this order. The first rule that applies
wins.

**1. Scope.** Is this note in a campaign the caller belongs to?

- A note with `campaign: X` is visible only to members of X, plus the GM.
- A note with no `campaign` is visible to the GM, and to anyone explicitly
  granted.

**2. The absolute deny.** `visibility: gm` — the note, or a property of it —
is **never** serialised into a non-GM response. There is no grant that
overrides it. This rule is absolute and is what makes the model predictable.

**3. The workflow gate.** A note whose `workflow` is `draft` or `submitted`
is visible **only to its owner and the GM**, *regardless* of its visibility
and regardless of any grant. This is a grant-suppressing deny.

**4. The base level.**

| `visibility` | Who |
|---|---|
| `public` | Anyone, authenticated or not — a link is enough |
| `party` | All members of the note's campaign |
| `owner` | The note's `owner`, and the GM |
| `gm` | The GM only. Never serialised to a non-GM role |

**5. Grants.** `grants: [p_8f2a, p_91bc]` — *additional* parties who may
see the note, on top of the base level. A grant never overrides rule 2 or
rule 3.

**6. The default.** When `visibility` is **absent**:

- a note **with** a `campaign` defaults to `party`;
- a note **without** one defaults to `gm`.

So a note inside a campaign is shared with the table unless it says
otherwise, and a note outside one is private unless it says otherwise. The
UI must show the *effective* visibility on every note, so a GM can see at a
glance that a note is shared.

**7. Property level.** A property may declare its own visibility, in the
pack. It **inherits the note's** otherwise.

- `owner` on a property resolves against the note's `owner`. **If the note has
  no owner, the property falls back to the note's visibility.** This is the
  rule that was missing, and its absence meant the shipped 5e pack hid every
  NPC's hit points from players — `hp` was `owner`-scoped and GM-created NPCs
  have no owner.
- `gm` on a property is absolute, like rule 2.

**8. Masking.** `display:` overrides the rendered value per audience. Applied
**last**, after grants and denies have decided *whether* a value is present.

### Handouts, and grants that can be given at runtime

A **`handout`** is a note type whose visibility is *mutable at runtime*, and
whose reveal is an **event**.

```yaml
type: handout
campaign: stormdark
visibility: gm
grants: [p_8f2a]      # after the reveal
```

- A handout is normally `gm` and becomes visible to the table when the GM
  reveals it.
- **Revealing is journalled** (ADR-0015), so "when did the party learn about
  the Duke's letter?" is answerable.
- A reveal can target **everyone**, **one player** (`grants: [p_8f2a]`), or
  **the shared display** (phase 3).
- A revealed handout must be **referable afterwards** — it appears in the
  between-sessions projection and is findable in search by the players who
  received it. That is why it is a note and not an attachment.

### Revealable secrets

A **secret block** inside a note that is itself visible to the party. It has
its own visibility until it is revealed, and revealing it promotes it in
place.

> **Specified in [ADR-0035](0035-secret-callouts.md)**: the syntax is
> Obsidian's `> [!secret]` callout, the reveal is a live WebSocket broadcast
> rather than a file write, and the text is stripped server-side so it never
> reaches a non-GM client.

- It is a **different field** from note-level `visibility`, deliberately, so
  the absolute deny of rule 2 is not weakened. A note is never "partly gm".
- It lives in the body, in a marked block, so it round-trips through the
  editor's raw-block escape hatch (ADR-0021).
- A reveal is a journalled event, like a handout's.

The distinction that makes this coherent: **`gm` is a deny; a secret is a
deferred grant.** A deny is absolute and permanent. A deferred grant is a
thing that starts hidden and can be given later, and the two must not be
confused in the schema.

### Content notes declare their own default visibility

A pack's `content:` declaration gains a `visibility:` default per content
type, so the SRD's own content is neither a spoiler nor invisible:

| Content type | Default | Why |
|---|---|---|
| `class`, `species`, `background`, `feat` | `party` | Players need them for the wizard |
| `spell` | `party` | A player must be able to read their own spells |
| `monster` | `gm` | A stat block is the GM's reference. Reading it early is a spoiler |
| `item` | `gm` | A magic item's properties are the GM's to reveal |
| `variantrule`, `optionalfeature` | `gm` | Same reasoning |

**This closes the open question in
[#21](https://github.com/PopinjayJohn/demitable/issues/21)**: the visibility of
the emitted SRD content notes was undefined, and "everything is `party`" would
have shipped the whole bestiary and spell list as player-visible and
enumerable. It is now declared per content type by the pack, and the
enrichment applies it.

A player who *should* see a specific spell gets it by grant — which is the
same mechanism a handout reveal uses.

## Consequences

**Good:**

- **One vault can hold many campaigns**, matching the Obsidian community's
  convention, and "party" no longer leaks across campaigns.
- **Handouts and secrets exist**, with per-player targeting and a journalled
  reveal — the vocabulary of *reveal*, which the phase-3 interaction is built
  on.
- **The visibility model is complete.** It has a default, a property-level
  rule, a no-owner rule, workflow gating, and one definition of `public`.
- **The NPC hit-point bug is fixed** by the no-owner fallback.
- **Content visibility is declared by the pack**, so the SRD ships with the
  right defaults rather than an accident.

**Bad:**

- **Two more fields** (`campaign`, `grants`) and a third concept (secret
  blocks) on every note. The model is more capable and more complex.
- **The effective visibility depends on six rules in order**, which is harder
  to reason about than one lookup. It needs a **"why can this role see this
  note?" inspector** in the UI, or the complexity will bite.
- **Secret blocks are a new body construct** the editor must round-trip
  safely. They are marked blocks, so the raw-block escape hatch covers them —
  but the editor must not treat them as ordinary content.
- **The pack now declares visibility**, so a pack author can get it wrong.
  The linter should check that a content type's declared default is
  sensible, and the UI should show the effective result.

**Neutral:**

- Rule 2 (the absolute deny) survives everything, which is what makes the
  model predictable. Everything else is a grant, and grants are revocable and
  auditable.
- A campaign-less note being `gm` by default means the *scaffolder* must
  stamp a `campaign:` on new campaign notes, or they will be invisible to
  the table. That is a real trap, and the scaffolder is where it is handled.
