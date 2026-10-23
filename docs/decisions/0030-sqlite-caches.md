# ADR-0030: SQLite holds the derived caches — the search index and live session state

- **Status:** Accepted
- **Amended by:** ADR-0054 — the FTS5 table gains a column per boosted or
  filtered field, and the community's content fences are indexed as prose
  rather than skipped as code
- **Date:** 2026-10-22
- **Amends:** ADR-0004 (Rule 2) and ADR-0013 (search)

## Context

The project's data model is "markdown is the database" (Rule 1), which is a
statement about **where truth lives**, not a prohibition on databases. Two
things nonetheless need somewhere to live:

1. **A ranked full-text index over note bodies.** ADR-0013 specified BM25
   over tokenised body text, maintained in memory by the vault watcher. That
   works, but it is the one index expensive enough to be worth *not*
   rebuilding on every launch, and it re-implements a search engine.
2. **Live session state** — current HP, conditions, token positions,
   initiative order, the dice log. ADR-0004 called this "in-memory + a
   persisted session store" and left the second half unspecified.

Both are **derived**. Neither is truth. The question is only what to derive
them into.

## Decision

**SQLite holds the derived caches. They live outside the vault, in the
user-level application directory, keyed by vault.**

### The three tiers, by how expensive they are to rebuild

This is the distinction that makes the whole thing coherent:

| Data | Home | Why |
|---|---|---|
| **Note metadata** — uid, name, aliases, tags, link graph, parsed frontmatter | **In memory** | Frontmatter is tiny; 5,000 notes parse in milliseconds. The projection layer (ADR-0002) needs this *in memory* anyway, because visibility is a recursive mix of grants, denies, ownership and discovery — painful to express in SQL and wrong to split across two stores |
| **Literal search** — exact and prefix over names, aliases, wikilink targets | **In memory** | Exact, instant, no tokenizer. A map is the right data structure for "is this string a key" |
| **Ranked body search** | **SQLite FTS5** | The expensive-to-rebuild index. FTS5 gives a correct, tuned BM25, prefix and phrase queries, and snippet/highlight generation — all things we would otherwise write |
| **Live session state** | **SQLite** | The mutable thing. Crash-safe without a hand-rolled journal, queryable without hand-rolling queries |

So we do **not** mirror the vault's structured data into SQL. Only the two
things that genuinely need it.

### Where the files live

```
~/.local/share/demitable/
  vaults/<vault-id>/
    search.db     the FTS5 index — a rebuildable cache
    session.db    live state — disposable
```

**Not in the vault.** Four reasons, in order of severity:

1. **A binary database in a synced folder is a corruption hazard.** A campaign
   vault is very likely to be in Dropbox, Obsidian Sync, Syncthing or git. An
   FTS5 index is unmergeable and a SQLite file written concurrently by two
   machines is corruptible. The app must never put a file in the vault that
   a sync client will try to merge.
2. **SQLite on a network filesystem has broken locking.** NFS and some SMB
   configurations do not provide the byte-range locks SQLite requires.
3. **A derived thing inside the vault can be mistaken for truth.** Rule 1
   exists to keep exactly one source of truth. A cache in the same directory
   invites a user to edit it, or a tool to sync it.
4. **It should not be backed up.** It is a cache, and it is rebuildable from
   the vault in one pass.

The `vault-id` is stored in the vault's own `.demitable/config.yaml`, so a
campaign that moves between machines or is renamed keeps its session.

### The driver

**`modernc.org/sqlite`** — a pure-Go port of SQLite, no cgo.

- **No cgo is not optional here.** `mattn/go-sqlite3` is the popular driver
  and it requires cgo, which breaks both the single static binary and
  cross-compilation (ADR-0005, tech stack §8).
- FTS5 is **compiled into `modernc.org/sqlite` by default** — no extension to
  load, no build tag.
- The cost is binary size: a transpiled SQLite adds a few megabytes.

The database is a **cache**, so a missing, corrupt or stale file is not an
error — it triggers a rebuild.

### The FTS5 index

```sql
CREATE VIRTUAL TABLE note_fts USING fts5(
  uid UNINDEXED,
  body,
  tokenize = 'unicode61 remove_diacritics 2'
);
```

The body is **preprocessed** before indexing: frontmatter stripped, markdown
syntax removed, wikilinks resolved to plain text, code blocks and comments
skipped. This is necessary regardless of the engine — `[[Ilia Vane]]` and
`<!-- comment -->` should not be indexed as literal noise — and it means the
built-in tokenizer is sufficient.

A custom tokenizer is possible through the driver's virtual-table API, and is
the escape hatch if preprocessing proves insufficient.

### The session store

```sql
CREATE TABLE session (
  id TEXT PRIMARY KEY, created TEXT, updated TEXT, state TEXT
);
CREATE TABLE entity (
  id TEXT PRIMARY KEY, session_id TEXT, kind TEXT, name TEXT,
  x REAL, y REAL, layer INTEGER, hp INTEGER, hp_max INTEGER, ac INTEGER,
  conditions TEXT, player_id TEXT, hidden INTEGER, data TEXT
);
CREATE TABLE initiative (
  session_id TEXT, entity_id TEXT, ord INTEGER, value INTEGER
);
CREATE TABLE log (
  id INTEGER PRIMARY KEY, session_id TEXT, ts TEXT, kind TEXT,
  actor TEXT, payload TEXT
);
```

WAL mode, one writer and many readers. **All writes go through the server
process**, so there is exactly one writer — the "GM changes HP while a player
moves a token" case is goroutine-level concurrency inside the server, not a
multi-process database problem. The clients never touch the database; they
talk to the server over the WebSocket.

## Consequences

**Good:**

- **The app opens instantly on a large vault.** The FTS index is persisted
  rather than rebuilt at every launch.
- **We stop writing a search engine.** FTS5's BM25 is correct and tuned, and
  prefix queries, phrase queries, `NEAR`, and snippet generation come free.
- **Live state survives a crash.** A server restart mid-session no longer
  loses the table's state — which is what "a cache that vanishes" was always
  threatening.
- **Session state becomes queryable.** "Every token in this room", "the
  initiative order", "every roll this session" are SQL, not hand-written
  traversal code.
- **The trust model is unaffected.** Nothing in the vault is mirrored, so
  there is still exactly one source of truth and the projection layer still
  has one place to enforce visibility.

**Bad:**

- **A significant dependency.** A transpiled SQLite in the binary, a few
  megabytes, and a transpilation layer that can carry bugs the C version
  does not.
- **Two representations of the body text exist** — the markdown file and the
  indexed copy. They can drift. Mitigated by the watcher updating the index
  on every change, and by the fact that the index is a cache with a rebuild
  path.
- **The database files are opaque.** A user who finds `search.db` in their
  app directory and edits it corrupts the index. The app must detect and
  rebuild rather than fail.
- **WAL side files** (`-wal`, `-shm`) appear next to the database, and need
  periodic checkpointing or the file grows without bound.
- **SQLite's locking model is subtle.** WAL mode plus a busy timeout is the
  configuration; getting it wrong produces intermittent "database is locked"
  errors that are painful to diagnose.
- **We are now partly maintaining SQLite's behaviour** in our heads, rather
  than owning a few hundred lines of BM25.

**Neutral:**

- Rule 1 is refined, not broken: the vault is the source of truth, and
  everything else is a rebuildable cache. That was always the intent.
- Rule 2 is strengthened: live state is still a cache, but it is now a cache
  that survives a crash.
- The MCP server can offer the FTS index as a search tool without extra work.
- The vault is still the only thing worth backing up.

## The line

The test for whether something belongs in SQLite is:

> **Can it be rebuilt from the vault, in one pass, without the vault?**

- The FTS index: yes → cache → SQLite.
- Live session state: yes → cache → SQLite.
- Note metadata and frontmatter: yes, but it is needed *live* and it is
  cheap → in memory.
- A character's HP after a session: **no** → it is written back to the
  character note and lives in the vault.
- A player's account: no → it is in `.demitable/players.yaml` (ADR-0009).
- A pack: no → it is a YAML file in the vault (ADR-0003).

If it cannot be rebuilt from the vault, it does not go in SQLite.
