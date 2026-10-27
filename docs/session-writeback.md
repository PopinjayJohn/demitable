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

**A partial apply, with conflict detection and reporting per key.** Settled in
[ADR-0039](decisions/0039-partial-apply.md).

> **Atomicity is per note, per key.** A character's hit points are never
> half-updated, and a note is never left half-written. It is *not* a property
> of the batch.

For each key the session touched: write it if the file's current value still
equals the session's base value; otherwise skip it and report it. Everything
else in the file is untouched, and the rest of the batch still lands.

### The write order

1. **Resolve.** For every touched key, compare the file's current value with
   the session's base value. Produce the set of writes and the set of
   conflicts.
2. **Preview.** Show the GM exactly what will be written and every conflict.
   **The preview runs the identical resolution and produces the identical
   result** — it is the same code path, not a parallel one, so it cannot lie
   about what will happen.
3. **Apply.** Each write goes through the ordinary per-key write path, so
   the journal, the watcher and the frontmatter surgery behave exactly as they
   do for any other write.
4. **Journal.** One entry per file, all sharing the batch id, so the batch
   reverts as a unit.

### Recovery

There is no staging directory — a session batch touches several directories
and a cross-directory atomic move does not exist. The journal is the record of
intent and recovery uses it.

If an apply fails partway, the journal already holds entries for the files that
landed. The app reports **which files landed and which did not**, and the GM
can re-run the batch. A partial apply is a normal outcome, not an exceptional
one.

## 3. Conflicts

> A conflict is **per key**, not per file.

For each key the session touched:

| The file's current value | Action |
|---|---|
| Equals the session's base value | **Write** the session's value |
| Differs from the base, and already equals what the session wants | **Not a conflict.** Nothing to write |
| Differs from the base value | **Conflict.** Skip the key, report it, and let the GM choose: the session's value, the human's value, or a third |

Everything else in the file is untouched. A file with three touched keys
where one conflicts gets two keys written and one reported.

A GM editing an unrelated field during a session costs nothing, because
conflict detection is not per file.

### Reporting

A conflicting batch is never silent:

```
Ilia Vane.md
  ✓ data.resources.hp     27 → 23
  ⚠ data.resources.mana   8 → 0    conflict: you changed this during the session
  ✓ discovered            [] → [2 locations]
```

The GM can resolve conflicts individually or in bulk. A session that touched
forty keys and conflicted on twelve produces a twelve-item list, so the UI has
to make it scannable.

## 4. Idempotency

A batch carries a `batchId` and a `revision`, and the journal records the
revision that was applied.

- A repeat with the **same** revision is **skipped**: "this session has
  already been written back."
- A repeat with a **higher** revision is a **re-apply**: the previous
  revision is reverted first, then the new one applied. The revert writes a
  **tombstone** marking that revision as undone.

The tombstone is what makes the two states distinguishable. Without it, a
revert-then-re-apply leaves the same batch id in the journal, and "already
written back" is indistinguishable from "written back, undone, written back".

### Reverting

Two operations, not three:

| Operation | Meaning |
|---|---|
| **Revert a batch** | Undo every journal entry sharing a `batchId`, in reverse order |
| **Revert one entry** | Undo a single change |

"As of a timestamp" is **dropped**. It swept in unrelated changes — anything
the GM made in the same window — and it was a second, subtly different
notion of the same idea. "Undo everything since Tuesday" is a filter over
entries, not a separate mechanism.

## 5. Discovery records

Artefact 5. What a character has seen, so the between-sessions view (ADR-0017)
can show them a world they have actually visited.

This is a **universal** field, not a pack-specific one. It is app-level —
"what does this character know" — and it does not vary by ruleset.

```yaml
discovered:
  - "[[The Drowned Quarter]]"
  - "[[Ilia Vane]]"
```

A **flat list of note references**, and that is a change: the records this
used to be — `uid`, `type`, `at`, `how` — are not writable. The reserved
layer is simple-typed, because Obsidian's Bases cannot query nested
properties (ADR-0040), and the extractor's writer flattens a list of maps
to strings.

**The consequence is stated rather than hidden.** The `how` of a discovery —
`visited`, `met`, `heard-of` — and the `at` of it have nowhere to live, and
the open question in §8 ("is a location discovered when the character enters
it, when they learn of it, or both?") is unanswerable until they do. If the
granularity is needed, it becomes a **separate note type**, not a nested
field in a reserved one (ADR-0050, decision D4).

Records are appended, never removed. A character who once met an NPC still
knows them.

---

## 6. The visibility model, formalised

**Completed by [ADR-0033](decisions/0033-campaign-and-grants.md)**, which
is now authoritative. It adds the campaign dimension, grant sets, revealable
secrets, the default, and the no-owner fallback.

> **A note is visible to a player if any *grant* applies. `gm` is an absolute
> *deny* that overrides every grant.**

### Grants

| Grant | Source |
|---|---|
| Public | `visibility: public` |
| Party | `visibility: party` — all members of the note's **campaign** |
| Owner | `visibility: owner` and the caller owns the note |
| Explicit | `grants: [p_8f2a]` — named parties, on top of the base level |
| Discovery | the note is `visibility: discovered` and its uid appears in a character the caller owns, in `discovered` |
| Type-level | a core rule, e.g. a player's own character note |

### Denies

| Deny | Source |
|---|---|
| GM | `visibility: gm` — **absolute**, regardless of discovery, ownership or grants |
| Workflow | `workflow: draft` or `submitted` — owner and GM only, regardless of visibility or grants |
| Campaign | the note's `campaign` is one the caller does not belong to |

> **Amended by ADR-0050.** A note that is `visibility: discovered` is *not*
> denied by the GM rule, so the Discovery grant above is reachable. Before
> this, a discovery-gated note had to be `gm`, and `gm` is an absolute deny
> that no grant overrides — so the grant could never fire.
>
> The campaign deny is absolute: a note inside a campaign may not leak out
> of it. `public` therefore means "not private to the party", not "outside
> the boundary". A `public` note **with** a campaign is public within that
> campaign; a `public` note with **no** campaign is public absolutely, and
> is the only kind of note an anonymous caller can see.
>
> **Phase-4 note:** the grant/deny ordering in this section predates
> ADR-0057 and is kept for history, not for implementation. The
> authoritative algorithm is the audience computation (intersection of
> every applicable constraint, widened only by permitted grants). Implement
> seat and session visibility from ADR-0057 — implementing it from the
> ordering above rebuilds the defect phase 0 fixed.

The deny is absolute on purpose: a GM-only note that a character once
discovered must still be hidden, or "discovered" becomes a way to leak
secrets, and a secret that leaks once is a secret that leaks.

### Property-level

Property-level visibility (a pack declaring `hp` visible to its owner) applies
within a visible note, in the same order: a property marked `gm` is never
serialised into a player response.

**A property with no owner falls back to the note's visibility.** Without
that rule the shipped 5e pack hid every NPC's hit points from players, because
`hp` was owner-scoped and GM-created NPCs have no owner.

### The default

A note **with** a campaign defaults to `party`; a note **without** one
defaults to `gm`. A missing `visibility` is never left to whatever the code
happens to do.

---

## 7. The preview

Before anything is written, the GM gets a full preview: per file, the keys
that change, the values before and after, and any conflicts.

This is the answer to the question the write journal was designed to answer
(ADR-0015) — "what did the app just do to my character?" — asked *before* the
app does it, which is strictly better than being able to answer it
afterwards.

**The preview runs the identical resolution as the apply**, so it cannot lie
about what will happen. This matters more than it sounds: the preview is the
journal's question asked *before* the app acts on it, and a preview that is a
separate code path is a preview that can be wrong.

The preview is *rendered* by a plugin, because presentation is a plugin
concern. The *data* is a core service, and it is the same service the apply
uses.

---

## 8. Open questions

- **Who writes the prose summary?** The transcript pipeline (phase 5) proposes
  one; the GM edits it. Is the proposal a marked section that the GM rewrites,
  or a draft the GM accepts wholesale? *Default: a marked section, so the
  GM's rewrite and the machine's proposal never fight over the same text.*
- **Discovery granularity.** Is a location "discovered" when the character
  enters it, when they learn of it, or both? `how` exists to distinguish them,
  but the UI must make the distinction matter. *Default: both, recorded
  separately — `visited` and `heard-of` are different facts and a player
  should be able to know of a place they have never been.*
- **Failure recovery.** *Settled by ADR-0039: recovery is journal-based.*
  There is no staging directory — a session batch touches several
  directories and a cross-directory atomic move does not exist. The journal
  records what landed, the app reports which files landed and which did not,
  and the GM can re-run the batch.