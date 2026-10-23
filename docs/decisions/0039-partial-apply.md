# ADR-0039: The write-back is a partial apply with per-key conflict reporting

- **Status:** Accepted
- **Date:** 2026-10-23
- **Amends:** [`session-writeback.md`](../session-writeback.md) §2, §3, §4 and §8
- **Addresses:**
  [#23](https://github.com/PopinjayJohn/demitable/issues/23) — the contract
  describes two different transactions for the same event

## Context

The write-back contract had **two contradictory transaction models** in it.

§2 said:

> **Atomicity.** A session either lands completely or not at all. A crash
> halfway through must not leave half the party at 3 HP and half at 27.
>
> **Validate.** … If any does not, the whole batch is aborted and the
> conflicts are reported. **Nothing is written.**

§3, three sections later, said:

> A conflict does **not** mean the batch fails. It means the app and the human
> disagree, and only about the keys the session actually touched.

§3 is the one consistent with the machinery M1 is specified to build —
**per-key compare-and-swap against a base content hash**. §2 is the wrong
half, and §2 is where "Atomicity" and "One revert" live, so the wrong
version was the one being quoted.

Three further defects in the same contract:

- **The conflict table was not a decision procedure.** Its "Session's base
  value" column held "—" in two rows and the non-value "differs from base" in
  a third; rows 2 and 3 were the same case stated twice; and **no row
  covered the case where the human's current value already equals what the
  session wants to write** — which would have reported a conflict where
  there is none.
- **The failure-recovery default could not be implemented.** §8 proposed
  applying the batch "to a staging directory and moved into place". A session
  batch touches `Notes/Characters/`, `Sessions/`, `Encounters/` and
  `Notes/World/…` — there is no single directory to move, and a
  cross-directory atomic move does not exist.
- **Idempotency could not detect a re-apply.** §4 detected "the batch id
  already present" in the journal. But ADR-0015 says reverting *writes a new
  journal entry*, so after revert-then-re-apply the batch id is still present,
  and "already written back" is indistinguishable from "written back, undone,
  written back".

## Decision

**A partial apply, with conflict detection and reporting per key. Atomicity
is per note and per key, not per batch.**

### The rule

For each key the session touched:

| The file's current value | Action |
|---|---|
| Equals the session's base value | **Write** the session's value |
| Differs from the base value, and already equals what the session wants | **Not a conflict.** Nothing to write |
| Differs from the base value | **Conflict.** Skip this key, report it, and let the GM choose: the session's value, the human's value, or a third |

Everything else in the file is untouched. A file with three touched keys
where one conflicts gets two keys written and one reported.

**Conflict detection is per key, not per file.** The GM editing an unrelated
field during the session costs nothing.

### Atomicity, redefined

> **Atomicity is per note, per key.** A character's hit points are never
> half-updated, and a note is never left half-written. It is *not* a property
> of the batch.

A session's write-back spans many notes. Making the whole batch atomic
would mean one conflicting field costs the entire evening's changes, for a
guarantee nobody asked for. The guarantee that matters — no character is
left in a broken state — is preserved by writing each key through the
ordinary per-key write path, which is itself atomic per file.

### Recovery is journal-based

There is no staging directory. The journal (ADR-0015) is the record of
intent, and it is what recovery uses.

- If an apply fails partway — a disk error, say — the journal already
  contains entries for the files that landed. The app reports **which files
  landed and which did not**, and the GM can re-run the batch. Because the
  apply is partial by design, a partial failure is a normal outcome rather
  than an exceptional one.
- Reverting a batch means reverting its journal entries, in reverse order.
  Each entry's revert is a per-file write through the ordinary path.

### Batch revert is by id, not by timestamp

Two operations, not three:

| Operation | Meaning |
|---|---|
| **Revert a batch** | Undo every journal entry sharing a `batchId` |
| **Revert one entry** | Undo a single change |

"As of a timestamp" is **dropped**. It swept in unrelated changes — anything
the GM made in the same window — and it was a second, subtly different
notion of the same idea. If a GM wants "undo everything since Tuesday", that
is a filter over entries, not a separate mechanism.

### Idempotency is keyed on (batchId, revision)

A batch carries a `batchId` and a `revision`. The journal records the
revision that was applied.

- A repeat with the **same** revision is skipped: "this session has already
  been written back."
- A repeat with a **higher** revision is a re-apply: the previous revision is
  reverted first, then the new one applied. The revert writes a **tombstone**
  marking that revision as undone, so the two states are distinguishable.

That fixes the defect the old rule could not detect.

### The preview and the apply are one code path

**The preview runs the identical conflict resolution and shows the identical
result.** Not a parallel implementation, not an approximation.

This matters more than it sounds. The write-back *preview* is the answer to
the question the write journal was designed for — "what did the app just do to
my character?" — asked *before* the app does it. A preview that is a
different code path from the apply is a preview that can lie, and a preview
that lies is worse than no preview, because the GM acts on it.

## Consequences

**Good:**

- **One transaction model**, and it is the one the machinery already
  implements. M1's per-key compare-and-swap is the whole implementation.
- **A GM's hand-edit during a session is never silently destroyed.** It is
  skipped and reported.
- **The preview cannot lie**, because it is the apply.
- **Recovery is implementable**, because it is the journal.
- **A double-written character sheet — damage applied twice — is
  impossible**, which is the specific bug the old contract existed to prevent.

**Bad:**

- **"Atomicity" now means something narrower.** A reader of the contract
  expecting all-or-nothing will be surprised. The redefinition is explicit,
  but it is a redefinition.
- **A partial failure is a normal outcome**, and the app must report it
  clearly rather than looking broken.
- **Two ways to revert, not three.** A GM who wanted "undo everything since
  Tuesday" now has to filter entries rather than name a timestamp.
- **The conflict resolution is per key, so a conflict report can be long.** A
  session that touched forty keys and conflicted on twelve produces a
  twelve-item list. The UI must make that scannable, and the GM must be able
  to resolve them in bulk.

**Neutral:**

- The write-back is still **one batch** with one `batchId`, one preview and
  one journal group. Only the *apply* is partial.
- The session note itself — the artefact the write-back produces — is still
  written as a whole, because it is new content rather than a merge.
