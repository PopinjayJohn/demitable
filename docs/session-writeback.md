# The session write-back contract

> How a session turns live state into vault changes.
> It has two consumers: the live table itself, and the between-sessions
> player view (ADR-0017). A contract with one consumer tends to be shaped by
> that consumer, so both are stated here.

---

## 1. What a session produces

A session run produces seven kinds of change. Not every session produces all
seven.

| # | Artefact | Where it lands | Notes |
|---|---|---|---|
| 1 | **Session note** | `Sessions/<date>-<name>.md`, type `session` | **Always a new note.** Created if absent. Date, attendees, scene beats, NPCs introduced, decisions, loot, loose threads |
| 2 | **Character sheet updates** | The participant's own note | `data:` values: resources, conditions, progression, inventory |
| 3 | **Encounter record** | `Encounters/<name>.md`, type `encounter` | Participants, outcome, the dice log, the map used |
| 4 | **Dice log** | Inside the session note's marked section | Or its own note, for a long session |
| 5 | **Discovery records** | The participant's own note | Which locations were visited, which NPCs were met. Feeds the between-sessions view |
| 6 | **World effects** | Item, faction and location notes | Loot gained or lost, faction standing changed, a location altered |
| 7 | **Quest updates** | Quest notes | Status transitions, progress, notes |

**Artefact 5 is the one that is easy to skip and impossible to add later.**
It is what makes ADR-0017's between-sessions view possible, and it has to be
*recorded during* the session — it cannot be inferred afterwards from prose.

### One note per session, always

A session is never merged into an existing note. A campaign of forty sessions
is forty files, dated, in `Sessions/`.

The alternative — one running note per campaign, appended to — was considered
and rejected: it makes a single file grow without limit, makes a write-back
conflict touch the entire campaign history, and turns "the session where we
met the king" into a link to a heading rather than a link to a note.

The cost is that the arc of a campaign is fragmented across many files, and
reading the whole campaign is a search problem rather than a scroll. That is
the right trade: a note is a thing you can link, quote, version and diff.

### The world is the GM's

Players may change **their own character note** and the personal notes within
it. They may not change the world — not quests, not locations, not factions,
not items.

So the review workflow of ADR-0009 applies to **characters only**. There is no
proposal queue for world changes, and no player-facing "propose an edit"
affordance. The world notes have exactly one writer.

This is a deliberate narrowing. It keeps the write-back contract simple (one
writer for world state, so no merge), keeps the "GM-only" guarantee
unambiguous, and keeps the between-sessions view read-only — which is what
ADR-0017 promised.

---

## 2. The write-back transaction

All seven kinds of change are applied as **one batch at the end of the
session**, not incrementally during play.

### Why a batch

- **Atomicity.** A session either lands completely or not at all. A crash
  halfway through must not leave half the party at 3 HP and half at 27.
- **One revert.** The whole session's changes undo with one action. This is
  the "undo the session write-back" operation from ADR-0015.
- **A preview.** The GM sees exactly what will change before it changes.

### The batch

```json
{
  "batchId": "01JBX…",
  "sessionId": "ses_7f2a",
  "actor": "gm",
  "source": "session-writeback",
  "operations": [
    { "op": "update", "uid": "…", "path": "Notes/Characters/Ilia Vane.md",
      "base": { "sha256": "9f2a…" },
      "changes": [ { "key": "data.resources.hp", "from": 27, "to": 23 } ] },
    { "op": "update", "uid": "…", "path": "…",
      "base": { "sha256": "4c71…" },
      "changes": [ { "key": "discovered", "from": [], "to": [ … ] } ] }
  ]
}
```

`base` is the content hash of the file **as it was when the session started**.
It is the basis for conflict detection.

### Write order

1. **Validate.** Every target note still exists and its current hash matches
   `base`. If any does not, the whole batch is aborted and the conflicts are
   reported. Nothing is written.
2. **Dry run.** Produce a per-file diff and show it to the GM.
3. **Apply.** Each file is written through the ordinary per-key write path, so
   the journal, the watcher and the frontmatter surgery all behave exactly as
   they do for any other write.
4. **Journal.** One journal entry per file, all sharing the batch id, so the
   batch reverts atomically.

---

## 3. Conflicts

A conflict is a file whose hash no longer matches `base` — the GM edited it in
Obsidian during the session, or another device wrote to it.

### Per-key compare-and-swap

A conflict does **not** mean the batch fails. It means the app and the human
disagree, and only about the keys the session actually touched.

So for each changed key:

| Current file value | Session's base value | Action |
|---|---|---|
| equals base | — | Write the session's value |
| differs | — | **Conflict.** The session's value is not written. The key is reported, and the human's value is kept |
| — | differs from base | The key was changed by something else *and* by the session. Same as above — a conflict |

The result: the app writes only the keys it is confident about, and leaves the
human's edits to keys it did not touch completely alone. A character sheet the
GM edited by hand during the session keeps the GM's edits, and the session's
HP update lands only if the GM did not also set HP by hand.

### Reporting

A conflicting batch is never silent. The GM sees, per file:

```
Ilia Vane.md
  ✓ data.resources.hp     27 → 23
  ⚠ data.resources.mana   8 → 0    conflict: you changed this during the session
  ✓ discovered            [] → [2 locations]
```

And can resolve the conflict by taking the session's value, keeping theirs,
or setting a third value.

### Aborting

The GM may abort a batch entirely. The abort itself is journalled.

---

## 4. Idempotency

Re-running a write-back for a session that has already been written must not
apply anything twice.

Each batch carries a `batchId` and a `sessionId`. The journal records both. On
a repeat, the core detects the batch id already present and:

- **skips** by default, reporting "this session has already been written back",
- or **re-applies** on explicit request, which first reverts the previous
  batch and then applies the new one.

A double-written character sheet — damage applied twice — is the kind of bug
that destroys trust in a campaign tool, so the skip is the default and the
re-apply is deliberate.

---

## 5. Discovery records

Artefact 5. What a character has seen, so the between-sessions view (ADR-0017)
can show them a world they have actually visited.

This is a **universal** field, not a pack-specific one. It is app-level —
"what does this character know" — and it does not vary by ruleset.

```yaml
discovered:
  - uid: 01JBX8K2M9Q
    type: location
    at: 2026-10-21T19:04:00Z
    how: visited
  - uid: 01JBX9P4R2L
    type: character
    at: 2026-10-21T19:41:00Z
    how: met
```

`how` is a short free-text or a small vocabulary (`visited`, `met`,
`heard-of`, `fought`, `served-by`). Packs may extend the vocabulary but the
field itself is universal.

Records are appended, never removed. A character who once met an NPC still
knows them.

---

## 6. The visibility model, formalised

ADR-0002 and design doc §5.4 describe the visibility levels. The write-back
contract needs a precise version, because discovery interacts with them.

**A note is visible to a player if any *grant* applies. `gm` is an absolute
*deny* that overrides every grant.**

### Grants

| Grant | Source |
|---|---|
| Public | `visibility: public` |
| Party | `visibility: party` |
| Owner | `visibility: owner` and the caller owns the note |
| Discovery | the note's uid appears in a character the caller owns, in `discovered` |
| Type-level | a core-defined rule, e.g. a player's own character note |

### Denies

| Deny | Source |
|---|---|
| GM | `visibility: gm` — **absolute**, regardless of discovery or ownership |

The deny is absolute on purpose. A GM-only note that a character once
discovered must still be hidden — otherwise "discovered" becomes a way to leak
secrets, and a secret that leaks once is a secret that leaks.

### Property-level

Property-level visibility (a pack declaring `hp` visible to its owner) applies
within a visible note, in the same order: a property marked `gm` is never
serialised into a player response.

---

## 7. The preview

Before anything is written, the GM gets a full preview: per file, the keys
that change, the values before and after, and any conflicts.

This is the answer to the question the write journal was designed to answer
(ADR-0015) — "what did the app just do to my character?" — asked *before* the
app does it, which is strictly better than being able to answer it
afterwards.

The preview is rendered by a plugin, not by the core, because presentation is
a plugin concern. The data is a core service.

---

## 8. Open questions

- **Who writes the prose summary?** The transcript pipeline (phase 4) proposes
  one; the GM edits it. Is the proposal a marked section that the GM rewrites,
  or a draft the GM accepts wholesale? *Default: a marked section, so the
  GM's rewrite and the machine's proposal never fight over the same text.*
- **Discovery granularity.** Is a location "discovered" when the character
  enters it, when they learn of it, or both? `how` exists to distinguish them,
  but the UI must make the distinction matter. *Default: both, recorded
  separately — `visited` and `heard-of` are different facts and a player
  should be able to know of a place they have never been.*
- **Failure recovery.** If the batch fails validation halfway through
  applying — a disk error, say — what is the recovery? The journal must be
  sufficient to reconstruct the intended end state. *Default: the batch is
  applied to a staging directory and moved into place, so a partial apply is
  impossible rather than recoverable.*
