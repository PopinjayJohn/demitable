# ADR-0048: A display principal, chat as notes, and the rest of the projection's shape

- **Status:** Accepted

- **Date:** 2026-10-23
- **Extends:** ADR-0033 (the visibility model), ADR-0035 (secrets),
  ADR-0037 (a campaign is a note), ADR-0038 (principals)
- **Addresses:** recommendation 9 of the external review — settle the
  player/GM gaps as schema decisions now, because a projection rule is
  expensive to retrofit

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

The external review's finding was sharp: several player-facing features are
cheap as a *schema* decision now and expensive as a *feature* later, because
the expensive part is not the interface but the projection rule behind it.

Five of them are in that category:

| Feature | What is expensive to add later |
|---|---|
| The shared display | A new **kind of principal** the projection has never resolved for |
| Chat | Whether messages are notes or a separate store — and therefore whether they are searchable, archivable and projectable |
| A personal player journal | Whether "only me" is expressible at all |
| The scratch log | Whether a third tier of state exists between the vault and the session |
| The campaign lifecycle | Whether handover and archival are operations on notes or on the server |

All five are settled here, as schema. **None of them builds a feature.**

## Decision

### A display principal

**A shared display — one screen for the whole party — is a principal**, and it
resolves to **party-visible content only**.

It does **not** see the union of what the party members can see, because that
union leaks: if player A owns a note, the display would show A's private note
in front of B. So the display sees exactly what `visibility: party` allows,
and nothing that is `owner`-scoped, granted, or discovered by one particular
character.

This is the one of the five that genuinely changes the projection's *shape*,
which is why it could not wait for phase 3. The feature is still phase 3; the
rule is now.

### Chat is notes

**A chat message is a note.** Type `message`, with the campaign, the sender,
and a visibility that follows the conversation:

| Channel | The note |
|---|---|
| Table channel | `visibility: party` |
| GM → one player | `visibility: owner`, `player: <recipient>` |
| Player → GM | `visibility: owner`, `player: <the GM>` |
| Out of character | `visibility: party`, with a `channel: ooc` field |

No new mechanism, and three things fall out for free: chat is **searchable**,
it is **archivable** with the campaign, and it is **projected** by the same
rules as everything else. The interface is phase 2 or 3; the shape is now.

### One more visibility value: `private`

`owner` means *the owner and the GM* — which is right for a character sheet,
where the GM must see the player's character.

But a player asked for a **personal** journal: private from the GM too. That
is not expressible today, and it is the kind of gap that is trivial to fill
now and awkward later.

So there are two owner-scoped values:

| Value | Who |
|---|---|
| `owner` | The note's `player`, and the GM |
| `private` | The note's `player`, and nobody else — not even the GM |

`private` is a narrow grant-suppressing rule on the same pattern as the
workflow gate: it narrows `owner`, and it never widens anything.

### The personal player journal

**A note.** Type `journal`, `player: <the owner>`, `visibility: private` by
default, with `owner` available for a journal the GM may read.

### The scratch log is a session-tier table

**Not notes. Not in the vault.** A table in the session store (ADR-0030):
append-only, timestamped, free-text with optional tags.

It is disposable by design, and it is **reconciled at write-back** — the GM
sees the scratch log beside the diff and either folds entries into the batch
or discards them. Entries can be *promoted* to durable artefacts, and a
promotion happens inside the same atomic batch as the rest of the write-back.

A third state tier is a real addition to the model, and it is worth having
because the alternative is that the GM has nowhere to put a half-formed fact
at minute 40 of a session.

### The campaign lifecycle

Operations on notes, not on the server.

| Event | How |
|---|---|
| Retirement | The campaign note's `workflow` becomes `retired`. Its notes stay. The server stops offering it in campaign pickers |
| Handover to a new GM | The export plugin emits a bundle: the vault's notes, the packs, and **a fresh `.demitable/`** — so the account hashes do not travel with the campaign |
| Archival | A retired campaign that is no longer served. The files are still there, still readable, still in git |

Handover is the reason the export plugin exists in phase 1 rather than phase 2.

## Consequences

**Good:**

- **The display, chat and journals are schema, so the features are cheap when
  they arrive.**
- **Chat is searchable and archivable** for free, because it is notes.
- **A private journal is possible**, which was not before.
- **The scratch log gives the GM somewhere to put a fragment**, which is the
  single most-repeated complaint about note-taking at a table.
- **Handover is an export**, and the export plugin is already in phase 1.

**Bad:**

- **Chat as notes means chat is indexed and searchable**, which is good for
  finding things and bad for a busy table — a session's messages are a lot of
  notes. The search must handle it, and there may be a case for excluding
  `type: message` from the default result set.
- **Chat as notes means chat is in the vault**, and the vault is what the GM
  syncs and shares. A table's conversation is not campaign canon, and putting
  it in the same directory as the lore blurs that.
- **A third state tier complicates the model.** The vault, the session tier
  and the scratch tier are three things, and "which one is this in?" will be
  asked repeatedly.
- **`private` is a new value in the vocabulary**, and the linter, the
  projection and the UI all have to learn it.

**Neutral:**

- The scratch log is *not* a note type, so it never appears in the vault's
  index, in search, or in an export. That is the point of it.
- Chat's interface is still unscheduled. This ADR decides its *shape*, not its
  phase.
