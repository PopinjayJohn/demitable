# ADR-0035: Secrets are `[!secret]` callouts, revealed in realtime

- **Status:** Accepted
- **Date:** 2026-10-22
- **Extends:** ADR-0033 (the visibility model) and ADR-0021 (the editor)
- **Addresses:** [#8](https://github.com/PopinjayJohn/demitable/issues/8)
  ("blind" content at paragraph level)

## Context

ADR-0033 promised "revealable secrets" — a block inside a note that is
visible to the party, which the GM can promote in place. It did not say how
such a block is written, how it is identified, or how the reveal reaches the
players.

Two things are now specified:

**The syntax should be Obsidian's.** Obsidian callouts are
`> [!type] Title` followed by `>`-prefixed body lines, and `> [!type]-` folds
the callout. `[!secret]` is a *custom callout type* — Obsidian renders it as
a generic callout, and the community styles it with CSS snippets. So a vault
using `> [!secret]` already means something to an Obsidian user, and it
round-trips through the editor's callout handling.

**A reveal must reach the players immediately.** "The GM reveals it and the
table sees it" is the entire point of the feature. A reveal that lands in a
file and reaches the players through the vault watcher is not a reveal — it is
a file write with a delay, and on macOS the watcher's backstop is a
5–15 second poll.

## Decision

### Syntax

A secret is a **callout of type `secret`**:

```markdown
> [!secret] The vault key
> It is under the third stone, north of the well.

> [!secret]- Folded until revealed
> Another one.
```

Or, for content that is not a callout, an explicitly marked block:

```markdown
<!-- secret:begin -->
The steward is called Harlan.
<!-- secret:end -->
```

Both are **stripped from the projection** for a non-GM role until revealed.

**The text never reaches a non-GM client.** The server strips it. This is not
a CSS hiding rule and not a "the client knows not to render it" — the bytes
are never serialised into a player response, so a player who inspects the
network, the DOM, or the WebSocket frames sees nothing.

### Block identity

A secret block needs a stable identity, so the reveal can be recorded against
it.

- **By default, a content hash.** The id is the first eight hex digits of the
  SHA-256 of the block's normalised source. It survives reordering and
  unrelated edits elsewhere in the note.
- **Or explicitly, with an Obsidian block reference.** `> [!secret] The key
  ^vaultkey` gives the block a stable id the author chooses, using Obsidian's
  own block-reference syntax.

**The trade, stated plainly:** editing a revealed secret's *text* changes its
hash, so **the block reverts to hidden**. That is a defensible rule — if the
GM changes what the secret says, it is a different secret and the party
should see the new wording — but it is surprising, and a revealed secret
should get an explicit `^id` if the GM expects to keep editing it.

### The reveal is a live event, not a file write

```
GM client ──► secret.reveal { note_uid, block_id, audience }
                              │
                    server applies it, persists to the session tier,
                    and broadcasts
                              │
        every client ◄───── secret.revealed { note_uid, block_id, audience, by }
```

- The **server is authoritative**. Clients do not apply a reveal optimistically
  in a way that can diverge.
- A `secret.revealed` event carries the note uid and block id; **clients
  refetch that note**. One note is cheap to refetch, and it avoids a second
  serialisation path that could drift from the first.
- The audience may be **everyone in the campaign**, or **specific players** —
  `audience: { players: [p_8f2a] }`.
- **Revealing a secret also grants visibility of the containing note.** A
  secret inside a GM-only note is invisible however many times it is revealed,
  which is confusing; so the reveal implies the note becomes visible to the
  audience. One action, one consistent result.

### Durability

A reveal must survive the session, because the party remembers it three
sessions later. Two layers:

- **Live**: the revealed set lives in the session tier (ADR-0030), which is
  what the WebSocket broadcast and the projection read.
- **Durable**: at session write-back, the revealed ids are written into the
  note's frontmatter, and the reveal is **journalled** (ADR-0015). An undo is
  itself an event.

```yaml
type: location
campaign: stormdark
visibility: party
revealed: [a3f91c2e, 77bd0f14]
```

The note's frontmatter records *which* secrets are revealed; the journal
records *when* and *to whom*.

### The GM's view

In the GM's view a secret renders distinctly — a lock affordance, a distinct
shape, and a control offering **reveal to the table** and **reveal to a
player**. The marking must not rely on colour alone (ADR-0034).

### Handouts and inline secrets compose

An inline secret is convenient but lives inside a note, so it is only
revealable to the audience that can already see the note. A **handout** is a
separate note with its own grant (ADR-0033), so it can be given to one player
independently of the note it came from.

So: **a secret can be promoted to a handout.** The GM reveals it to one
player, and the app offers to lift the block into a handout note with its own
`grants`, so the party keeps a durable copy and the source note stays as it
was.

## Consequences

**Good:**

- **The syntax is Obsidian-native.** A vault already using `> [!secret]`
  means the same thing in our app, and the editor handles it as a callout it
  already knows how to render.
- **The reveal is instant.** It is a WebSocket broadcast, not a file write, so
  the table sees it as it happens.
- **The secret text never leaves the server** for a non-GM role.
- **The reveal is durable and auditable** — journalled, and persisted into
  the note.
- **A secret is ordinary markdown.** It is greppable, diffable, and readable
  in any editor. It is not a special file format.

**Bad:**

- **Block identity is imperfect.** A content hash un-reveals a secret whose
  text is edited. An explicit `^id` avoids it, but that is friction the GM has
  to know about.
- **The projection must strip blocks**, which means the projection is no longer
  a pure note-level filter. That is a real increase in the complexity of the
  one piece of code whose job is to be obviously correct.
- **A refetch per reveal** is simpler than a patch, but it is a request per
  reveal, and a GM revealing five secrets in a row makes five requests. A
  batched or patched variant may be needed later.
- **Obsidian does not know about any of this.** In Obsidian, `> [!secret]` is
  text and a player with file access sees it. That is acceptable under our
  threat model — the vault is GM-only and players get projections, not files
  (ADR-0002) — but it must be stated, because "secret" in our app and
  "secret" in Obsidian are not the same guarantee.
- **The `revealed` list in frontmatter is app state in a user file.** It is
  small and human-readable, but it is not something the user asked for and
  it is not something Obsidian understands.

**Neutral:**

- The `<!-- secret:begin -->` form exists for content that is not a callout,
  but the callout form is the one to teach.
- The GM's reveal control and the shared display in phase 3 are the same
  affordance at different scales.
