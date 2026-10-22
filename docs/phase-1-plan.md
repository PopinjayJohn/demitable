# Phase 1 Plan — The Wiki

**Goal:** a working local app whose **minimal core** opens a campaign vault,
browses it as a typed campaign database, edits notes (prose *and* structure),
interprets a system pack, and serves a role-filtered view to players — with
nothing but a markdown directory on disk, and with every feature delivered as
a plugin.

**Definition of done:** a complete character can be created, edited, have a
formula evaluated, be seen by its owner and not by another player, and survive
a round-trip through Obsidian without corruption — through the plugin API, not
around it.

---

## Milestones

### M0 — Skeleton and build pipeline

**Work**
- Go module `demitable`, `cmd/demitable` entrypoint.
- `internal/server` serving `web/dist` via `go:embed`, with a `/api/health`.
- Svelte + Vite shell, `npm run build` → `web/dist`, embedded at compile time.
- Config: `--vault <dir>`, `--port`, `--host`, `--no-browser`.
- Opens the default browser on start.
- The **exposure configuration surface** (ADR-0031), so it exists before
  anything depends on it: `--expose`, `--public-url`, `--trust-proxy`,
  `--allow-http-import`, `--no-import`. The defaults are the safe ones —
  import off, loopback bind, forwarded headers ignored — and `--expose`
  changes them together.
- **Operating it for months** (ADR-0031), because the server now runs 24/7:
  - **Graceful shutdown** on `SIGTERM` and `SIGINT` — stop accepting
    connections, finish in-flight writes, checkpoint the SQLite WAL, close
    cleanly. A campaign vault must never be left half-written by a restart.
  - **`/api/health`** — a liveness probe, and the thing a load balancer or
    a `systemd` `WatchdogSec` checks.
  - **Structured logs to stdout**, one JSON object per line, so the GM can
    pipe them into `journald` and rotate them without the app managing log
    files. The app does **not** write its own log files.
  - **A `SIGHUP` reload** of configuration, so changing the public URL or
    turning off `--expose` does not need a restart.

**Done when** `go build ./cmd/demitable && ./demitable --vault ./example`
serves the shell on one port and no second process is needed.

---

### M1 — Vault service, indexes and the write journal

**Work**
- `internal/vault`
  - `Note`: path, raw bytes, frontmatter (parsed *and* raw), body.
  - Frontmatter parse/serialise: **`yaml.Node` round-trip**, so unknown keys,
    comments and key order survive untouched.
  - Index: uid → note, filename → note, alias → note, link graph
    (outgoing + backlinks), tag index.
  - Watcher (fsnotify) with debounce; re-index on change; emit events.
  - Content hash (sha256 of raw bytes) as the note version, used for
    optimistic concurrency.
  - Read: `Get`, `List(query)`, `Write`, `PatchProperties`.
  - **Per-key compare-and-swap** against a base content hash. This is what
    makes a session write-back safe against a human editing the same file
    mid-session — see [`session-writeback.md`](session-writeback.md). The
    mechanism lands in phase 1; the session write-back that uses it is phase
    2.
  - **Asset resolution** (ADR-0026): a note references an attachment by path;
    the index records which visible notes reference which asset, so the
    projection can answer "may this caller see this file?".
- `internal/assets`
  - `GET /asset/<vault-relative-path>`, session-token required.
  - **An attachment is visible to a player if a note visible to that player
    references it, or if it is explicitly marked public.** Assets referenced
    by nothing are GM-only by default.
  - **404, not 403**, for anything the caller may not see — a 403 confirms
    the path exists.
  - Streamed with `Content-Length` and a content-hash `ETag`; `If-None-Match`
    yields a 304. Range requests supported, so the board can load a large map
    progressively.
  - Paths are resolved against the vault root; anything that escapes it is
    refused.
- `internal/search`
  - **Literal index** (in memory): prefix-anchored, case-insensitive, over file
    name, `name`, `aliases` and wikilink targets. Substring fallback. Exact
    lookup wants a map, not a database.
  - **Ranked index** (SQLite FTS5, ADR-0030): a persisted FTS5 virtual table
    over note bodies, in the user-level app directory, keyed by vault id —
    **never in the vault**. The body is preprocessed before indexing:
    frontmatter stripped, markdown removed, wikilinks resolved to plain text,
    code blocks and comments skipped. BM25, prefix and phrase queries, and
    snippet generation come from FTS5 rather than from code we write.
  - Both updated incrementally by the watcher. The FTS index is a cache: a
    missing, corrupt or stale file triggers a rebuild, never an error.
- `internal/store`
  - The SQLite connection and schema: `note_fts` (FTS5), and the session
    tables (`session`, `entity`, `initiative`, `log`).
  - WAL mode, one writer and many readers — all writes go through the server
    process, so there is exactly one writer.
  - Files live in `~/.local/share/demitable/vaults/<vault-id>/`, never in the
    vault: a binary database in a synced folder is unmergeable, and SQLite on
    a network filesystem has broken locking.
  - The driver is `modernc.org/sqlite` — pure Go, no cgo, FTS5 built in.
  - **WAL checkpointing on a schedule**, so `search.db-wal` does not grow
    without bound over months. And **session retention**: old sessions are
    pruned, because a 24/7 server accumulates them (ADR-0031).
  - **The DSN pragmas and a bounded pool**
    ([#27](https://github.com/PopinjayJohn/demitable/issues/27)):
    `journal_mode(wal)`, `busy_timeout(5000)`, `synchronous(NORMAL)`, and
    `SetMaxOpenConns(1)` on the writer. Measured: without them, 243 of 400
    concurrent writes failed with "database is locked". Never mix
    `db.Conn()` with `db.Exec()` on a one-connection pool — it deadlocks.
  - **A polling watcher as the primary mechanism on macOS and the universal
    backstop everywhere**
    ([#28](https://github.com/PopinjayJohn/demitable/issues/28)). fsnotify's
    kqueue backend opens one file descriptor **per file**, so a 50,000-note
    vault cannot be watched on macOS at all. Watch directories, debounce,
    re-read, compare a **content hash** rather than an mtime, reconcile
    removals by `stat`, and run a periodic full rescan.
  - **Type-aware frontmatter emission**
    ([#29](https://github.com/PopinjayJohn/demitable/issues/29)): emit
    booleans as `true`/`false` and never as `yes`/`no`; always quote
    wikilinks in properties, or YAML reads them as nested lists; keep
    frontmatter flat; key property names case-insensitively; and emit the
    YAML *type* that matches `.obsidian/types.json`, not an equal-looking
    string.
  - **A vault is untrusted input**
    ([#33](https://github.com/PopinjayJohn/demitable/issues/33)): budgets for
    file size, frontmatter size, depth and key count; never `node.Decode`
    untrusted YAML into a Go value; resolve every served path with
    `EvalSymlinks` and verify it is still under the vault root.
  - **A Go markdown parser** for the FTS5 body preprocessing and the link
    checker, both of which run in the core process
    ([#33](https://github.com/PopinjayJohn/demitable/issues/33)). The
    JavaScript parser cannot serve them.
  - **A `campaign` field on every note** (ADR-0033), and the scratch log the
    in-session capture needs
    ([#5](https://github.com/PopinjayJohn/demitable/issues/5)) — session-tier,
    disposable, reconciled into the write-back at the end.
- `internal/journal`
  - Append-only journal under `.demitable/journal/`, one file per day.
  - Content-addressed prior versions under `.demitable/blobs/`.
  - Every app-initiated write is recorded: actor, source, op, uid, path,
    before/after hash, and a key-level diff for structured edits or a line
    diff for body edits.
  - Revert one entry; revert a batch as of a timestamp.
  - Reverting writes a new journal entry. The journal is never rewritten.
- Frontmatter boundary rule: the frontmatter block is `---` delimited at the
  top of the file. `---` inside the body is not frontmatter.

**Done when**
- A vault of 5 000 notes indexes in under a second.
- A file changed on disk appears in the index within 200 ms of the change.
- Writing a property to a note with 40 unknown frontmatter keys leaves those
  keys byte-identical.
- Reverting a journalled write restores the exact prior bytes.
- A search for a known note finds it; a search for a body phrase finds it.
- The FTS index survives a restart: the app opens without re-tokenising the
  vault.
- A deleted or corrupt `search.db` is detected and rebuilt, rather than
  producing wrong results or an error.

**Design note:** *never re-serialise the whole frontmatter block when patching
one key.* Parse into `yaml.Node`, mutate the node, re-emit only that block.
This is the single most important correctness detail in the whole project.

---

### M2 — Formula engine

**Work**
- `internal/formula`: lexer, Pratt parser, evaluator per ADR-0003's grammar.
- Values: `nil | bool | number | string | list | map`.
- Scope resolution: dotted paths into the entity's data, plus other formulas,
  resolved lazily with cycle detection.
- Whitelisted builtins only. No assignment, no user functions, no
  `__proto__`/`constructor`/`prototype` access, depth- and step-limited.
- Errors carry the formula name and the expression position.
- Tests: a table of expression → expected value, plus a suite of *malicious*
  expressions that must be rejected rather than evaluated.

**Done when**
- The malicious suite passes: `constructor.constructor("return 1")()`,
  `a.__proto__`, deeply nested literals, and a self-referencing formula are
  all rejected with a useful error, not evaluated.
- A formula referencing another formula resolves in dependency order.

**Design note:** write this as if it will run on untrusted YAML forever,
because it will be.

---

### M3 — Dice engine, faces and tables

**Work**
- `internal/dice`, per ADR-0008 and ADR-0022.
- **Faces carry payloads**: `id`, `label`, `symbol`, `asset`, `value`,
  `rank`, `tags`. `value` and `rank` are both optional.
- **Tables** are the unified concept: a named set of faces. Tables can be
  pack-provided or live in the vault as `type: table` notes with faces in
  `data.faces`.
- Die sets: numeric (`1d20`), table-backed (`rune`, `omen`), and mixed.
- Notation: `NdM`, flat modifiers, keep-highest / keep-lowest (`4d6kh3`),
  exploding (`6d6>=5`), Fudge dice (`4dF`), wild percentile, success
  thresholds, opposed pools.
- Result types derived from the definition: `numeric`, `symbol`, `count`,
  `named`.
- Counting in pools from tagged faces or a rank threshold. Comparison requires
  ranks — an unordered die in a comparison is an **error with a clear
  message**, never a silent fallback.
- The **dice log**: roller, workflow, expression, individual die results,
  modifier sources, target, outcome, timestamp.

**Done when**
- `4d6kh3` correctly keeps the highest three.
- A rune die produces a symbol result, not a number.
- A pool of runes counts successes from tagged faces.
- Comparing two unordered dice errors with a message naming the fix.
- A mixed d6 (five numbers, one critical face) rolls and logs correctly.

---

### M4 — Packs: model, loader, validator, linter

**Work**
- `internal/pack`
  - Types for the pack YAML (ADR-0003): attributes, resources, formulas,
    conditions, progression, difficulty, dice, views.
  - Loader: parse, resolve formula dependencies, compile all formulas once.
  - Validator: unknown keys, missing labels, formulas referencing undefined
    identifiers, cycles, duplicate keys, malformed YAML.
  - Linter: enforce the **schema invariants** (§6.1 of the design doc).
  - Registry: built-in packs (embedded) + packs in `Packs/` in the vault,
    which shadow built-ins by id.
- **Content declarations gain a `visibility` default per content type**
  (ADR-0033): `spell` and the character-creation content is `party`; `monster`,
  `item` and the rules content is `gm`. This is what stops the whole SRD
  bestiary from shipping player-visible and enumerable
  ([#21](https://github.com/PopinjayJohn/demitable/issues/21)).
- **An `inventory` field on characters**
  ([#15](https://github.com/PopinjayJohn/demitable/issues/15)) — the schema
  change, so it lands in phase 1 rather than being retrofitted.
- **Secrets in the schema** (ADR-0035, ADR-0036): the `revealed` field on
  a note, the block-identity rule, and the `secrets` vocabulary — which
  callout types and which YAML keys mean "GM-only" is pack configuration,
  not a hardcoded list, so a GM's own admonition custom types keep working.
  The projection must **strip** unrevealed secret blocks, which makes it a
  block-aware filter rather than a pure note-level one — the one piece of
  code whose job is to be obviously correct, so it grows.
- **The bestiary reads two stat block dialects** (ADR-0036): the corpus's
  `ad-statblock`, which is *prose* and stays prose, and the community's
  `statblock`, which is *structured YAML* and seeds a token directly. A
  homebrew monster the GM wrote last night must work with no import step.
- The **D&D 5e SRD 5.2 pack**, as data: attributes, resources (hp, hp_max,
  ac, speed, initiative, spell slots by level, exhaustion), formulas
  (ability modifiers, proficiency bonus, max HP, DC, saves, initiative,
  attack bonus, passive perception), conditions, progression (levels 1–20),
  difficulty (XP budget with five thresholds), dice workflows, and sheet
  views.
- SRD 5.2.1 is CC-BY-4.0 — ship attribution in the pack header and in the
  app's about screen.

**Done when**
- `demitable pack lint packs/dnd5e.yaml` is clean.
- A deliberately broken pack (unknown formula target, cycle, missing label)
  produces a precise, actionable error.
- The 5e pack evaluates a level-5 fighter correctly: modifier +1, prof +3,
  max HP 38, AC 18, passive perception 13.

---

### M5 — The SRD pipeline and the content vault

**Work**
- `tools/srdenrich/` — a build-time tool. Not a plugin, not shipped in the
  binary.
- **`tools/srdcheck/`** — verifies a manually acquired corpus before the
  pipeline runs. The corpus is **version 2.28.1 and only manually available**
  (ADR-0024), so there is no fetch step — there is a check:
  - version is 2.28.1;
  - the expected file inventory is present;
  - checksums of the index and a sample of entity files match the recorded
    ones;
  - no unexpected top-level entries.
  A failed check stops the build. Running the pipeline against the wrong
  corpus would produce a vault that looks fine and is subtly wrong.
- The pipeline then:
  1. Reads the SRD mechanics JSON and the SRD markdown from the working
     directory.
  2. **Filters to the SRD only** — by **`srdKeys` membership**, not by source
     tag. Non-SRD entities are never written anywhere, and a non-SRD entity
     reaching the output is a hard build error. This is the licence boundary
     (SRD 5.2.1 and SRD 5.1 are CC-BY-4.0; the rest of the corpus is not
     ours to ship). The source-tag rule alone is *not* a filter — it admits
     2,628 keys `srdKeys` deliberately excludes, mostly the same entity
     under the other generation's tag.
  3. Joins prose ↔ mechanics on `(type, name, source)`, normalising away
     `; subtitle` and parentheticals. Measured: 82.2% with a naive join,
     above 95% with normalisation.
  4. Converts relative markdown links to wikilinks.
  5. Translates the 11 distinct upstream `<$…$>` expressions into pack
     formulas (ADR-0003's language).
  6. Emits `packs/dnd5e.yaml` — the mechanics pack, plus the `content:`
     declarations for class, species, background, feat, spell, monster,
     optionalfeature, variantrule.
  7. Emits `dev/srd-vault/` — the enriched vault. **Frontmatter only;
     the body is never modified.**
  8. Emits a build report: errors, unjoined files, unused entities, and every
     formula conversion, so it can be reviewed by hand.
- The enrichment adds `type: content`, `content_type`, `uid`, `system`,
  `source`, `license`, `attribution`, and the pack-specific scalars to each
  file's frontmatter. Prose stays in the body; scalars go in frontmatter.
- **Uids are derived deterministically from the source path**, and every
  emitted collection is explicitly sorted, so the build is **byte-identical**
  for a given input (ADR-0024).
- **The source directory is deleted after a successful build.** It is
  gitignored and never committed.

**Done when**
- The pipeline runs end to end and emits a pack, a vault and a report.
- The build is **deterministic**: two runs on the same source produce
  byte-identical output.
- `srdcheck` rejects a corpus with the wrong version, a missing file, a
  changed checksum, or an unexpected top-level entry, with a clear message.
- Every wikilink in the output resolves.
- Every content note validates against its pack's `content:` schema.
- Every class's formulas evaluate across levels 1–20.
- No non-SRD entity appears anywhere in the output, and the build fails if one
  does.
- Unjoined files are listed in the report and still present, unenriched.
- `fixtures/` is gone, and the repository still contains everything needed to
  rebuild it.
- The published archive is **the committed vault, zipped** — byte-identical to
  `dev/srd-vault/`, with the manifest and attribution (ADR-0025).
- The import **refuses a URL that resolves to a private address**, and
  **refuses to reconnect to a different address than it validated** (ADR-0031).
- The import is **disabled unless explicitly enabled**, and its fetch
  requires https.

**CI note.** The corpus is manually available, so CI cannot re-run the
pipeline. CI runs every check that works against the *committed* vault — the
byte-identical round-trip, the validation suite, the link checker, and a scan
for non-SRD entities — and the "does the pipeline still reproduce this?"
check is a maintainer task, run locally when the enrichment rules change.

**Note:** this milestone also produces the project's three test suites
([`srd-vault.md`](srd-vault.md) §6) — the editor's golden corpus, the pack
validation suite, and the link checker. The corpus is deliberately *not*
curated: 24 of the class-progression files carry 504 malformed `class"…"`
attributes each, which is exactly what a round-trip test needs.

---

### M6 — Plugin host and capability model

**Work**
- `internal/plugin`
  - Registry: manifest parsing, service resolution by id and version range,
    mount points, command and shortcut registration.
  - **Capability model** (ADR-0019, ADR-0020): per plugin, per role. The core
    never hands a plugin the vault — only a scoped view exposing the declared
    operations, filtered by role.
  - **`dom.inline`** (ADR-0027, ADR-0029): a separately-granted capability
    letting a plugin mount into the app's own DOM rather than a shadow root.
    Compiled-in plugins have it implicitly; a WASM plugin must declare it.
    It controls *style and layout participation*, not privilege — all plugin
    UI runs in the app's document.
  - **The wazero runtime, configured defensively**
    ([#26](https://github.com/PopinjayJohn/demitable/issues/26)):
    `WithCloseOnContextDone(true)` is **mandatory** — wazero has no fuel
    meter and no epoch ticker in any release, and `Runtime.Close()` does
    **not** stop a spinning guest, so a runaway plugin is a hung server
    without it. Plus a per-call context deadline the host owns, a
    `WithMemoryLimitPages` cap (the default is 4 GiB per instance), an
    explicit `WithCoreFeatures(api.CoreFeaturesV2)`, and a test that a module
    importing `wasi_thread_start` fails to instantiate.
  - **The ABI version handshake**
    ([#31](https://github.com/PopinjayJohn/demitable/issues/31)): a
    `demitable_abi_version()` export checked before any other call, a
    host-side supported **range**, dispatch on the guest's version, an
    `abi:` field in `plugin.yaml`, and a golden ABI test that imports every
    host function the host offers.
  - **Surface hosts** (ADR-0029): the app provides named host elements; the
    default host carries a **shadow root**, and plugin styles are contained
    while custom properties on `:root` inherit in. The import warning calls
    `dom.inline` out separately from the WASM sandbox reassurance.
  - **The `when` condition** of a manifest is an expression in the formula
    language (ADR-0003) against a small note context. No second expression
    language.
  - **The `config` block** is rendered by the core into a settings form, so
    every plugin gets one for free. A plugin may register its own surface,
    which takes precedence.
  - **The WASM ABI** is Extism's shape — one exported entry point taking and
    returning JSON, plus host functions — implemented on wazero rather than
    via the Extism SDK, so the pure-Go single-binary property holds and a
    plugin written against the Extism PDK would work unchanged.
  - **Tiers**: compiled-in (Go, registered via a generated file), WASM
    (wazero, pure Go, no cgo), vault CSS (inert).
  - WASM host ABI: one exported entry point taking and returning JSON, plus
    host functions `vault.read`, `vault.write`, `vault.query`, `http.fetch`,
    `log`, `now` — granted only if declared. No filesystem preopens, no
    sockets.
  - Asset serving for plugin CSS/JS/images, with vault CSS scoped under a
    plugin root attribute and ordered by manifest.
  - The browser bridge: versioned, role-scoped `postMessage` API
    (`call`, `on`, `emit`).
- The **REST and WebSocket contract** ([`api.md`](api.md)): `/api/v1/*` with a
  stable error envelope, cookie-or-bearer authentication, the compute
  endpoints, and the versioned WebSocket protocol. Written down so that a
  client — the browser app today, a native app later — is a consumer of a
  contract rather than of an implementation.
- The shell: a minimal Svelte app that mounts plugin surfaces at the
  declared mount points, and can run with **no plugins at all** — a vault
  browser.

**Done when**
- A plugin can register a service, a mount point and a command, and be
  called through it.
- A plugin declaring only `vault.read` cannot write, and cannot read a note
  hidden from the calling role.
- A vault CSS plugin restyles its own surface and cannot restyle the shell.
- A plugin with `dom.inline` mounts into a named host element; one without
  it cannot.
- The shell boots with every plugin disabled.

**Design note:** this milestone is deliberately **before** the editor, because
the editor will change it. What is provisional here is expected to change; the
capability *model* is not.

---

### M7 — The editor plugin

**Work**
- `plugins/editor/` — a **CodeMirror 6** plugin (ADR-0021), compiled in and
  mounted in **light DOM** (ADR-0027, ADR-0029). The editor is the app's own
  content surface, it must work on first run, and focus, IME, clipboard and
  the native spellcheck UI all suffer behind a boundary.
- **The buffer is the source.** The editor does not re-serialize; saving
  writes the buffer back. This is what makes the byte-identical contract
  satisfiable by construction (ADR-0021).
- **Block widgets** for structured content: `Decoration.replace` for a
  block, `Decoration.widget` for an inline element. This is Obsidian's
  model, and it is the sanctioned way for a plugin to contribute structured
  content to a note.
- **Callout support, including `> [!secret]`** (ADR-0035). The callout is
  rendered as a callout, and a `secret` one renders with the GM's reveal
  affordance. It must round-trip **byte-identically** like everything else —
  including the fold marker (`-`/`+`) and any trailing `^blockid`.
- **The community's block formats are native, not imported** (ADR-0036):
  Obsidian callouts (with admonition's types and aliases), `ad-*` code
  blocks, and `statblock` blocks. The corpus is *already* in these dialects
  — 355 `ad-statblock` blocks, 204 `^blockid` references — so this is
  reading what we already have. A `statblock` renders as a live stat block
  widget.
- **The character sheet is a form panel**, rendered from the pack's
  `views.sheet` — not a document.
- **Frontmatter is a structured form panel**, patched surgically and never
  re-encoded.
- **The golden corpus**: a directory of deliberately awkward markdown
  fixtures. Every one must round-trip byte-identically.
- Write-side verification: re-parse the serialized output and compare it to
  the editor's document. A mismatch blocks the write and shows a diff.
- An extension registry so other plugins can contribute block and inline
  widgets.
- A source view alongside, because people who know markdown expect to see it.
- **The cross-browser harness, committed to the repository**
  ([#32](https://github.com/PopinjayJohn/demitable/issues/32)). The
  verification is Chromium-only today, and `ShadowRoot.getSelection()` does
  not exist in Firefox or Safari. The harness runs against Chromium, Firefox
  and WebKit in CI and asserts all ten rules in
  [`shadow-dom-verification.md`](shadow-dom-verification.md) §4 in each.
- **A sanitisation policy for rendered markdown**
  ([#33](https://github.com/PopinjayJohn/demitable/issues/33)). The app
  renders a GM's *and* a player's files, alongside plugin UI that is trusted
  with the app's privileges, and the SRD corpus contains raw HTML. Something
  has to sanitise, strip or sandbox the renderer before the editor ships.

**Done when**
- Every corpus fixture round-trips byte-identically.
- A note containing an unknown Dataview block is preserved exactly through
  open → edit elsewhere → save.
- A lossy edit is **rejected with an explanation**, not silently applied.
- A plugin can contribute a block widget, and it renders in the note.
- The editor is usable with no plugins installed beyond it, and it mounts
  into the note surface in light DOM, with no frame boundary.
- **A shadow-root plugin surface exists and works**: a plugin's styles are
  contained, the app's element styles do not reach in, and custom properties
  on `:root` do. A plugin mounted in a shadow root can still receive
  keyboard events and take part in native drag-and-drop with the app's
  light-DOM surfaces.
- **The core's own shadow-boundary rules hold** (ADR-0029, verified in
  [`shadow-dom-verification.md`](shadow-dom-verification.md)):
  - a drop handler identifies the element with `event.composedPath()[0]`,
    never `event.target`;
  - the app's "copy selection" reads the selection from the active shadow
    root, not from `window.getSelection()`;
  - a modal dialog traps Tab in a capture-phase `keydown` listener, so
    plugin shadow content cannot steal focus;
  - the theme's tokens are **unregistered** custom properties on `:root`.
- The same harness runs against **Firefox and WebKit** before the plugin
  host is considered done. Chromium-only is not enough: the
  `getSelection()` retargeting behaviour is the most likely to differ.

---

### M8 — Accounts, roles and projection

**Work**
- `internal/accounts`
  - Accounts in `.demitable/players.yaml` (ADR-0009): id, name, argon2id
    passphrase hash, created, owned character uids.
  - Join codes: GM creates a player, server issues a single-use 24-hour code,
    player exchanges it for a passphrase at `/join`.
  - Session cookie: httpOnly, SameSite=Strict, host-only. The `Secure` flag
    follows the configured public scheme, so the same build works behind an
    HTTPS proxy and on a plain-HTTP LAN.
  - **Bearer tokens as a first-class alternative to the cookie** (ADR-0031,
    [`api.md`](api.md)), so a native client is not forced into browser-shaped
    auth. Both are accepted on every endpoint; the browser app uses the
    cookie, everything else uses the header.
  - **Rate limiting and backoff on the auth endpoints** — join, login, and
    the GM secret. Strictly tighter under `--expose`.
  - **Mandatory GM authentication under `--expose`.** The GM secret is a
    credential like any other, not "it is localhost".
  - **An audit log** of authentication events, and — under `--expose` — of
    reads of GM-only notes.
  - **Sessions and seats** (ADR-0009, [`api.md`](api.md) §3): a seat is who
    is at this table right now, playing this character or watching. A
    **session code** — short, unambiguous, rotating per session — lets a
    guest take a seat without an account; an account holder joins with their
    passphrase and chooses a character. A seat is **not** a privilege: the
    projection applies to it, and a seat naming a character the caller does
    not own is rejected. The token a join returns is scoped to that session
    and that seat.
- `internal/projection`
  - Role model: `gm`, `player`, and later `observer`.
  - Visibility evaluation per §5.4: note-level `visibility`, property-level
    pack declarations, `display` masks.
  - Ownership: the `owner` universal field resolves to a player id.
  - Workflow gating: a note in `draft` or `submitted` is visible only to its
    owner and the GM, regardless of its `visibility`.
  - **The projection is the only path from the vault to a non-GM response.**
    Enforce this structurally: the HTTP layer for player-scoped requests
    receives a `*projection.View`, not the vault.
- The GM's own UI never goes through the projection — it sees everything,
  which is the point.

**Done when**
- A property test asserts: for every note, every property and every role, the
  projection never contains a `gm`-visible field.
- A player sees their own `draft` character, sees another player's `draft`
  character as nothing at all, and sees an `active` character per its
  `visibility`.
- A wrong passphrase, a used join code and an expired join code are all
  rejected.

---

### M9 — Browsing and search plugins

**Work**
- `plugins/browser/`: typed views over the index — characters, NPCs,
  locations, factions, items, quests, sessions, tables, events. Table, card
  and board layouts. Filters by tag, faction, location, status, system.
  Backlinks panel, outgoing links, unlinked mentions. Campaign dashboard.
  Character sheet rendered from the pack's `views.sheet`, with live formula
  values.
- `plugins/search/`: the search UI over the two core indexes.
- A relation view (who knows whom, faction membership) — simple graph or
  expandable rows. Decide at implementation time; do not build a
  force-directed graph editor in phase 1.

**Done when**
- 5 000-note vault: every view filters and sorts in under 100 ms.
- Clicking a wikilink in prose opens the note in the app.
- A character's sheet shows the same numbers Obsidian shows in raw
  frontmatter.

---

### M10 — Scaffolding, the demo campaign, and the worked example

**Work**
- `internal/scaffold`: create a campaign — folders, `Home.md`, one template
  per note type, an installed pack, an example character.
- **The archive import** (`demitable import <url>`, ADR-0025) has
  **moved to phase 1.5.** It is security-sensitive, network-facing, and its
  only phase-1 user would be the maintainer publishing the archive — which a
  community member now does. Phase 1 distributes content by **unzipping a
  downloaded archive into the vault**, which needs no server feature at all.
  The SSRF defence, the extraction safety rules and the plugin warning move
  with it.
- **The demo campaign** (`dev/demo-campaign/`) — a small campaign beside the
  SRD vault. Not filler: it exists to **exercise the projection layer** over
  content notes, so it must contain a visited location *and* an unvisited
  one, a GM-only note, a character in `draft`, a played encounter using an
  SRD monster, a session log, a `table` note, and an asset. The full spec is
  in [`srd-vault.md`](srd-vault.md) §8.
- **A published schema and starter Bases**
  ([#13](https://github.com/PopinjayJohn/demitable/issues/13)): a
  machine-readable `schema.md` listing every universal field, its type and
  its vocabulary — the formal version of the community's "Vault Conventions"
  note — plus starter Bases and Dataview queries shipped with the
  scaffolder, so a new vault comes with working views.
- Templates: parameterised note templates used by "new note" and by the
  phase-2 wizard.
- **The example campaign**: a real, small campaign used as the acceptance
  test. Two characters (a PC and an NPC), one location, one faction, one
  item, one quest, one session log, one encounter plan, one **symbol table**,
  a 5e pack copy, and deliberately awkward content — a note with unknown
  frontmatter keys, a note with Dataview blocks, a note with `%%comments%%`,
  a note with an embed.

**Done when**
- The example campaign passes a round-trip check: open every note, save every
  note, and diff the vault — **zero unintended changes**.
- A fresh campaign can be created and used without touching the docs.
- The demo campaign's projection test passes: a player sees the visited
  location and not the unvisited one, sees their own draft character and not
  another player's, and sees nothing marked GM-only.
- An SRD monster in the demo encounter becomes a token; an SRD class appears
  as a wizard option.
- `demitable import <url>` downloads an archive and merges it into a vault,
  reporting what was imported and what was rejected.
- An archive with a path-traversal entry, an absolute path or a symlink is
  refused, and nothing is written.
- An archive declaring a size or compression ratio beyond the caps is
  rejected before extraction.
- An archive containing `.demitable/players.yaml` does **not** import those
  accounts; the entry is rejected and reported.
- An archive containing plugin bundles produces the warning listing each
  bundle's capabilities, and an interactive import requires confirmation.
- A non-interactive import **skips** plugin bundles unless
  `--include-plugins` is passed.
- A repository that is obviously not a vault is refused with a clear message.

---

### M11 — The campaign export plugin (the second plugin)

**Work**
- `plugins/export/`: a **non-UI** plugin that walks the campaign through the
  core API and produces a static, self-contained archive — a single HTML
  document, or a directory of HTML plus assets.
- It exercises the API the way a *reader* would: notes, computed character
  values, the link graph, and assets. It reads nothing the editor did not
  already read, and it writes nothing at all.
- Exports the campaign's public material only, honouring the projection: a
  player-scoped export contains what a player may see.

**Done when**
- An export of the demo campaign contains every note the exporting role may
  see, and nothing it may not.
- The export renders without the app — a static HTML file, no server, no
  JavaScript required to read it.
- The export is a *second, independent* consumer of the API, so the API is
  exercised by something the editor did not shape.

**Note:** this is the second, **non-UI** plugin, and its purpose is to prove
the plugin API is not accidentally editor-shaped (ADR-0019). It is also the
milestone at which **`api: 1` is frozen** — by then two plugins have been
written against it, one of them with no relation to the editor.

**The MCP plugin has moved to phase 1.5.** It drags an external protocol into
phase 1 for a feature the design itself rates Medium risk and which is off by
default, and any second non-UI plugin would have served the same purpose.
It remains a good plugin to write — it is just not one to write *first*.

---

### M12 — The Blades pack: the format gate

**Work**
- `packs/blades.yaml`, per ADR-0012: ratings, roles, harm as a condition
  track, clocks and tokens as resource kinds, crew actions, dice-pool
  workflows, and an opposition-based difficulty model.
- Mechanics and structure only. No rulebook prose is reproduced.
- Lint against the schema invariants of §6.1.

**Done when** — the six proofs in ADR-0012 all hold:

1. A pack with no `levels` block loads and lints clean.
2. A pack with no HP-like resource expresses a character fine.
3. A dice-pool workflow rolls correctly: rating dice vs opposition dice,
   highest count wins, with position and escalation.
4. Clocks work as a resource kind with size, progress and segmentation.
5. Harm works as a condition track, not a numeric pool.
6. The encounter difficulty model is opposition dice and clock pressure, with
   no XP budget anywhere in the core.

**If any of these need a change to the *core* rather than the pack, the pack
format was not generic enough and phase 2 waits.**

---

## Where the research findings land

Every finding from the three research passes, and the phase and milestone it
is assigned to, is tabulated in
[`research-findings.md`](research-findings.md). The assignments above are the
phase-1 rows of that table; the phase-2, 3 and 4 rows name the milestone
each gap still needs.

## Cross-cutting

**Definition of done for phase 1** (all must hold):

1. A character can be created, edited, and re-read, with no manual fixing.
2. Formulas evaluate correctly and unsafe expressions are rejected.
3. The vault round-trips without corruption, including unknown syntax —
   **byte-identical** for every golden-corpus fixture.
4. A GM-only note, a GM-only property and an unapproved character are
   provably unreachable by players, and unreachable by a **player-scoped**
   plugin instance, and unreachable by a sandboxed plugin's **logic**.
   *Not* unreachable by a plugin's UI or by a compiled-in plugin — plugins
   are trusted at the user's privilege level (ADR-0019, ADR-0027).
5. The 5e pack lints clean and produces correct values for a test character.
6. The app runs as a single binary with no runtime dependencies.
7. A player can join with a code, log in, and see exactly what they should.
8. Every app-initiated write is journalled and revertible.
9. A second, non-UI plugin (the campaign export) reads the campaign through
   the API — and `api: 1` is frozen, because two plugins now exist.
10. The Blades pack proves the format is not 5e-shaped (M12).
11. **Every feature above the core is a plugin**, and the core runs with all
    of them disabled.
12. The SRD content vault builds, validates, and contains no non-SRD content.

**Explicitly not in phase 1:** dice rolling UI, encounters, the board, audio,
player-facing character creation (the *accounts and review workflow* that
player creation depends on are in scope; the wizard itself is phase 2).

### Phase-1 exit criteria for internet exposure

The app may be internet-exposed (ADR-0031). Before it is declared safe to
expose, all of these must hold:

- [ ] The import endpoint refuses a URL that resolves to a private, loopback
      or link-local address, **and** refuses to reconnect to an address other
      than the one it validated (DNS rebinding).
- [ ] The import requires https, is **off by default**, and enforces size,
      time and redirect caps.
- [ ] Authentication is rate-limited and backed off, on every auth endpoint.
- [ ] The GM authenticates with a credential, not by being local.
- [ ] Absolute URLs come from the configured public base URL, never the
      `Host` header.
- [ ] Forwarded headers are trusted only from a configured proxy address.
- [ ] The `Secure` cookie flag follows the configured public scheme.
- [ ] CORS is off by default; the API is same-origin.
- [ ] The MCP endpoint is off by default, and logs a warning if enabled.
      *(MCP moves to phase 1.5; the import and the export plugin carry the
      phase-1 exposure surface.)*
- [ ] The app **starts and works with the network interface down**, and with
      DNS unavailable.
- [ ] There is **no automatic update** path. The app may notice a release
      and say so.

This list is deliberately checkable rather than aspirational, because
"internet-exposed" is the deployment where every one of these matters.

## Sequencing and parallelism

```
M0 ──► M1 ──┬──► M7 (editor plugin) ──┬──► M10 ──► M11 (export plugin)
            └──► M2 ──► M3 ──► M4 ────┘      ▲          │
                        │                      │          ▼
                        ├──► M12 (Blades gate) │      api:1 frozen
                        │    └─ gates phase 2  │
                        │                      │
                        └──► M6 (plugin host) ─┘
                                  │
M8 (accounts, roles, projection) ◄┘
        │
M9 (browser + search plugins) ◄──┘

M5 (SRD pipeline) ── depends on M1 (index), M2, M4
        │                └── produces the golden corpus, the pack
        │                    validation suite and the link checker
        ▼
M5 ──► M7        the editor needs the golden corpus, so M5 precedes M7
```

**M6 must precede M7.** The plugin host exists so the editor can be written
against it, and the editor is what shapes the API.

**M2 → M3 → M4** and **M1 → M7** are largely independent and can run in
parallel. M8 needs M1's index and M4's pack visibility declarations.

**M5 sits early on purpose.** The SRD vault is the project's test corpus, and
the editor's golden corpus does not exist until it does. Running M5 before M7
turns the editor milestone from "build an editor and hope" into "build an
editor against 1,692 files that already break it".

**M12 — the Blades gate — now runs immediately after M4**, before the editor
and before the schema is frozen by the 5e pack. A gate that fires *after* the
thing it is gating is not a gate. If clocks and harm tracks have no home in
the schema, we want to know that in week six, not in month nine.

**M11 is where `api: 1` freezes.** By then two plugins exist, one of them
with no relation to the editor.

## Phase 1.5 — what was deferred, and why

| Deferred | Why | ADR |
|---|---|---|
| **Archive import** (`demitable import <url>`) | Security-sensitive, network-facing, off by default, and its only phase-1 user would have been us publishing an archive that a community member now hosts. Content reaches phase-1 users by **unzipping a downloaded archive**, which needs no server feature at all | 0025 |
| **The MCP plugin** | Drags an external protocol into phase 1 for a Medium-risk, off-by-default feature. Any second non-UI plugin proves the API is not editor-shaped — the campaign export plugin does that, and better | 0014 |

**The editor stays in phase 1.** The adversarial review's recommendation to
defer it was predicated on it costing 12–16 weeks of Tiptap work
([#35](https://github.com/PopinjayJohn/demitable/issues/35)). With
CodeMirror 6 and block widgets it is a fraction of that, and ADR-0006's
argument — the editor is the product's floor, and building it later means
retrofitting it into every feature — applies with *more* force, not less.

## Open items to resolve during M1–M6

- Whether the server binds the LAN by default or only on request
  (ADR-0010 says LAN by default, with `--host` to override).
- Where accounts live in the vault. Draft answer: `.demitable/players.yaml`,
  app-private, hidden from the note browser, but inside the vault so it
  travels with the campaign.
- Frontmatter key naming for the universal fields: `type` vs `sp_type`,
  `uid` vs `sp_uid`. A namespace prefix keeps the vault clean but collides
  with Obsidian's Dataview conventions, which expect bare `type`.
- Whether one campaign may span several vaults.
- How maps and other binary assets are referenced and delivered to players.
- The concrete plugin API surface. See [`plugin-api.md`](plugin-api.md),
  which is **provisional by design** — its open questions are listed at the
  end of that document, and it is not frozen until two plugins have shaped it.
- Whether the WASM host ABI should follow Extism's shape exactly, or be
  simplified now that only one guest language is actually targeted.
