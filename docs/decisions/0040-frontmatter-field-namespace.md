# ADR-0040: The app's frontmatter fields are bare, with a tightened reserved set

- **Status:** Accepted
- **Date:** 2026-10-23
- **Amends:** the universal field list in design doc §5.2
- **Addresses:** a pre-phase-1 decision — the field names are written into
  every user's file on first save

## Context

The app writes a set of fields into every note it touches. Those names are a
**wire format**: they land in the user's files, and changing one later is a
breaking vault-format change (ADR-0031).

The research on Obsidian interop settled one thing firmly. **Obsidian's Bases
cannot query nested properties** — "Properties are deliberately flat. Nested
properties are not supported in the properties editor." A `sp:` block would
therefore make every app-owned field invisible to Bases, and since the
community's convention is to query by a bare `type` property, our vault would
be the one vault in the ecosystem that does not match.

So the names must be **top-level and simple-typed**. The remaining question is
whether they are bare or prefixed — and, before that, whether the set is
right.

### Two names were already wrong

Looking at the set with fresh eyes surfaced two problems that have nothing to
do with the bare/prefixed question.

**`status` and `workflow` were both lifecycle fields.** `status` was
"free-ish vocabulary, defined per type"; `workflow` was
`draft | submitted | active | retired`. A user reading `status: active` next
to `workflow: active` cannot tell them apart. And "free-ish vocabulary,
defined per type" is not a specification — it is a placeholder that hands the
app an unbounded set of possible values on a field it is about to write.

**`owner` meant two different things.** Ours meant "the player account that
owns this note". A GM's `owner:` on an item note would naturally mean the
item's *possessor*, which might be a character. We would clobber it.

## Decision

**The fields are bare and top-level, and the reserved set is tightened to
twelve names that do not collide.**

| Field | Meaning |
|---|---|
| `type` | The note type. The community's convention, and the one Bases users query |
| `uid` | Stable identity. Never changes |
| `name` | The display name, where it differs from the filename. Obsidian's own convention |
| `system` | Which pack interprets this entity |
| `campaign` | Which campaign this belongs to (ADR-0037) |
| `kind` | `pc` \| `npc` \| `creature` |
| `player` | **The player account that owns this note.** Was `owner` |
| `workflow` | `draft` \| `submitted` \| `active` \| `retired`. The review state, and the only lifecycle field the app owns generically |
| `visibility` | `public` \| `party` \| `owner` \| `gm` |
| `grants` | Additional parties who may see this note |
| `revealed` | The secret blocks in this note that have been revealed |
| `discovered` | What this character has seen and met |

### `status` is gone

There is no generic status. A pack defines the fields its own content types
need, in its `content:` declarations:

```yaml
content:
  - id: quest
    fields:
      - { key: stage, label: Stage, type: string }
      - { key: outcome, label: Outcome, type: string }
```

A GM who wants `status: alive` on a character keeps it, and the app never
touches it — because the app no longer has a status to write.

### `owner` is now `player`

The field holds an **account id**, and it is called `player` because that is
what it holds. "The owner of this note" and "the item's possessor" are
different facts, and the field is about the first.

The **visibility value** stays `owner`, because there it names a
*relationship* — "the note's `player`" — and `visibility: player` would read
as "visible to players", which is `party`.

### Each reserved field has a visibility

> **Amended by ADR-0050, decision D5.** The reserved set declares a
> visibility per field. `discovered` and `grants` are `owner`-scoped; every
> other reserved field inherits the note's visibility. A note with no
> `player` falls back to the note's visibility, so a GM-created note's
> lists are as visible as the note is — which is right, because there is
> nobody else they could be hiding from.

### The app never writes a field it does not understand

If a note already has a field in the reserved set with a value the app does
not recognise, the app does not overwrite it. It is a reservation, not a
takeover — and a user who wants their own `campaign:` meaning gets to keep it
by not using a value the app knows.

## Consequences

**Good:**

- **The vault looks like every other vault in the ecosystem.** Bare `type`,
  bare `name`, bare `tags` — so a user's existing Bases views, Dataview
  queries and community plugins work on our vault without modification.
- **The schema is published as data** (ADR-0034, M10) and is a contract
  another tool can read, because it is not a dialect.
- **The two collision-prone names are gone.** A GM's `status: alive` and
  their `owner: [[Bram]]` are theirs.
- **Twelve names is a short list** to document, to explain, and to reserve.
- **Packs get better at describing their own content**, which is the point of
  a pack.

**Bad:**

- **A GM loses a generic `status` field.** Quests, characters and factions
  each need their own vocabulary, declared by the pack.
- **`player` is a slightly awkward name for ownership**, and it will read
  oddly on an item note — "who is this note owned by in the app", rather than
  "who owns this item".
- **The reserved set is a promise.** Twelve names are taken out of a user's
  vocabulary forever, and the app has to behave well when a user uses one
  for something else.

**Neutral:**

- `tags` and `aliases` are Obsidian's own conventions and are untouched.
- The `uid` format stays ours. A ULID is distinctive enough that collision
  is not a practical concern.
