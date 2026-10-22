# ADR-0015: The server keeps a write journal with revert

- **Status:** Accepted
- **Date:** 2026-10-21

## Context

The app writes to the vault: character sheets are updated at session end,
encounter summaries are written, session logs are filed. A bug in any of those
write paths corrupts a real campaign. And the user has no way to tell what
changed, because the app is a program making changes to their files.

The user's actual question is not "did you corrupt my file" but **"what did
you just do to my character?"** — and today the honest answer is "look at the
git diff, if you use git".

## Decision

**Every write the app performs is recorded in an append-only journal, with
before and after content, and the GM can revert any of it.**

### Storage

Two pieces inside the vault, under `.demitable/`:

```
.demitable/
  journal/
    2026-10-21.log        append-only, one file per day
  blobs/
    01JBX8K2M9Q7r...      content-addressed previous versions
```

The journal entry references blobs rather than embedding them, so the same
prior content is stored once no matter how many times it was replaced.

### Entry shape

```json
{
  "ts": "2026-10-21T19:04:11Z",
  "actor": "gm",
  "source": "session-writeback",
  "op": "update",
  "uid": "01JBX8K2M9Q",
  "path": "Notes/Characters/Ilia Vane.md",
  "before": "sha256:9f2a…",
  "after": "sha256:4c71…",
  "changes": [
    { "key": "data.resources.hp", "from": 27, "to": 23 },
    { "key": "body", "from": "…", "to": "…" }
  ]
}
```

`changes` is a key-level diff for structured edits and a line diff for body
edits, so the journal is readable without diffing files by hand.

### Reverting

- **Revert one change** — restore that entry's prior content for the file.
- **Revert a batch** — restore the state of a set of files as of a timestamp.
  This is the one that matters: "undo the session write-back" restores every
  file that write-back touched, atomically.
- Reverting writes a **new** journal entry rather than erasing history. The
  journal is append-only, always. An undo is itself an event.

### Scope

Only **app-initiated** writes are journaled. Edits made in Obsidian are not
app-initiated, so they are not in the journal — but they are not reverted
either, because the journal only restores content the app itself replaced.

This is the important nuance: reverting a journal entry restores the content
the app overwrote, which may be stale relative to edits made afterwards in
Obsidian. The UI must warn when the file has changed since the journalled
write, and offer a diff rather than a blind restore.

### Retention

Journals are kept for 90 days by default and blobs are garbage-collected when
no live journal entry references them. Configurable, and never automatically
deleting a journal entry that a revert would still need.

## Consequences

**Good:**

- Answers the question users actually ask, in the place they will look.
- "Undo the session write-back" becomes a button instead of a git archaeology
  exercise, which matters for users who do not use git.
- The journal doubles as an audit trail of everything the app touched, which
  is reassuring given the app edits files by itself.
- The session write-back contract becomes auditable: each run produces
  journal entries that show exactly what changed on every character.

**Bad:**

- Storage growth. Blobs are deduplicated by content hash, so it is bounded by
  the number of distinct prior versions, not by the number of writes — but it
  is not zero, and it lives in the vault where it gets synced.
- **The journal directory is inside the vault**, so it must be excluded from
  the note index, the search indexes, and the MCP resource list. A file the
  app cannot see is a file the app cannot corrupt.
- Reverting is not free of judgement: the file may have changed since. The
  UI has to handle that honestly rather than pretending it is a simple undo.

**Neutral:**

- The journal is a natural place to hang future features — a per-session
  summary, an "what changed" digest, an undo across an entire session.
