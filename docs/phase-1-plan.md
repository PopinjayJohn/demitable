# Phase 1 Plan — The Wiki (MVP)

> **Goal:** a working local app whose **minimal core** opens a campaign
> vault, browses it as a typed campaign database, edits it as markdown, and
> serves a role-filtered view to players — with nothing but a directory of
> files on disk.
>
> **The MVP in one line:** *a campaign wiki, with secrets.*
>
> **Definition of done:** a GM can create a campaign, write notes in it,
> search it, and see their players' filtered view of it; a GM-only note is
> provably unreachable by a player; and the app never damages a file it
> cannot undo.
>
> **And one thing it does not claim.** A GM-only note is unreachable by a
> *player*, not by a *plugin*. A compiled-in plugin is the program, and a
> plugin's UI runs in the app's document with the app's privileges
> (ADR-0029). The capability model constrains accidental overreach; it is
> not a defence against a hostile plugin. See
> [`threat-model.md`](threat-model.md) §8–§9.

Phase membership and what moved: [`phase-plan.md`](phase-plan.md).

---

## 1. What this phase costs

**Eight milestones, twenty-one sub-milestones.** The previous plan was
thirteen milestones and thirty-seven sub-milestones, at ~70–90 weeks
full-time. The cuts below are the difference.

| Block | Sub-milestones | Full-time | Part-time (≈10 h/week) |
|---|---|---|---|
| M0 — skeleton, build, config | 1 | 1–2 wks | 1 month |
| M1 — vault service, index, watcher, journal, assets, store | 5 | 7–9 wks | 4–5 months |
| M2 — the minimal pack format | 1 | 2–3 wks | 1–2 months |
| M3 — the plugin host's core | 4 | 6–8 wks | 4–5 months |
| M4 — the editor | 3 | 4–6 wks | 3–4 months |
| M5 — accounts, campaign, projection | 3 | 5–6 wks | 3–4 months |
| M6 — browser and search plugins | 3 | 5–6 wks | 3–4 months |
| M7 — scaffolder, demo campaign, published schema | 3 | 5–6 wks | 3–4 months |
| M8 — the export plugin | 2 | 2–3 wks | 1–2 months |
| **Phase 1 total** | **21** | **~37–49 weeks** | **~2–2.5 years** |

**Roughly half the previous estimate.** Still not a sprint, but it is now a
phase that ends, and the parts of it that are hard are the parts that matter
— the vault, the trust boundary, and the editor's safety.

**The most interesting saving** is the editor. With source editing and a live
preview, **the write path transforms nothing** — the buffer is the file — so
the round-trip contract is satisfied trivially and the corpus suite becomes a
regression test rather than a research project. The rich layer, with block
widgets and the extension registry, moves to phase 2.

---

## 2. What is in this phase, and what is not

**In:** the shell and the core UI/UX · the vault service, index, watcher,
journal and asset endpoint · the store and rated search · the minimal pack
format · accounts, the campaign join code, the principal model and the
projection · source editing with live preview and the frontmatter panel · the
typed views and search · the scaffolder, the demo campaign and the published
schema · the export plugin.

**Out, deliberately:**

| Out | Where |
|---|---|
| The formula engine, the dice engine | Phase 2 |
| The 5e pack, the SRD content vault, the Blades gate | Phase 2 |
| The creation wizard, character sheets | Phase 2 |
| Block widgets, the extension registry | Phase 2 |
| The archive import endpoint | Phase 2 |
| The WASM plugin tier, the host ABI | Phase 3 |
| The query language | Phase 3 |
| The MCP plugin | Phase 3 |
| The cross-browser harness | Phase 3 |
| Live sessions, seats, session codes, the scratch log | Phase 4 |
| The encounter runner, the tabletop, chat, the shared display | Phase 4 |
| The session write-back contract | Phase 4 |

---

## 3. Milestones

### M0 — Skeleton, build pipeline and configuration

**M0a** — Go module, `cmd/demitable`, the server serving `web/dist` via
`go:embed`, the Svelte shell, the config surface.

> **Where M0 starts.** The repository already has the Go module, the
> `cmd/demitable` entrypoint, the `Makefile` and the CI workflow. The
> entrypoint today sets up the logger and the audit log (ADR-0058), answers
> `GET /healthz`, and shuts down cleanly on `SIGTERM` — which is the
> graceful-shutdown criterion below, proved early rather than at the end.
> It cross-compiles for all five platforms with `CGO_ENABLED=0`,
> and `make release` builds the matrix.
>
> **Still to do:** the config surface below, `--vault`, `--port 0`,
> `--no-browser`, the `go:embed` of `web/dist`, the Svelte shell, and
> `GET /api/health` (the endpoint is at `/healthz` today; the plan's path is
> the one that ships).

**Done when**
- `go build ./cmd/demitable` produces a binary; started with `--vault <tmp>
  --port 0 --no-browser`, it serves `GET /` containing the built bundle's
  hash and `GET /api/health` with 200, **from one process, with no `node` on
  `PATH`**.
- The configuration surface exists with safe defaults: `--expose`,
  `--public-url`, `--trust-proxy`, `--allow-http-import`, `--no-import`
  (ADR-0031, ADR-0025). The import itself is phase 2; the flags are here
  so the deployment story is settled before anything is exposed.
- `CGO_ENABLED=0` builds succeed for `linux/amd64`, `linux/arm64`,
  `darwin/amd64`, `darwin/arm64`, `windows/amd64`, each **statically
  linked** — one file, no runtime dependencies (ADR-0060). **There is no
  size limit**; the size is reported, not gated.
- Graceful shutdown on `SIGTERM`: in-flight writes finish, the SQLite WAL is
  checkpointed, and the process exits 0.

### M1 — The vault service, the index, the watcher, the journal, the assets, the store

**M1a — the vault.** Read, index, watch, frontmatter.
**M1b — the write journal.** Append-only, content-addressed blobs, revert.
**M1c — the asset endpoint.** Role-scoped, ETag-cached.
**M1d — the store.** SQLite, FTS5, the `notes` table.
**M1e — the watcher.** Debounce, hash-compare, rescan backstop.

> **M1a's frontmatter half already exists.** Phase 0 promoted the extractor
> into `internal/vault/frontmatter/` — 56 tests, and a corpus audit that
> measures the round-trip contract against the 1,692 real files. M1a wires it
> into the vault service; it does not write it again. Read
> [ADR-0049](decisions/0049-frontmatter-io-contract.md) before changing
> anything in that package.

**Done when**
- A generated vault of 5,000 notes, each with 40 unknown frontmatter keys and
  a 2 KB body, is **parsed** — frontmatter parsed to `yaml.Node`, link graph
  built, prose extracted — in **under 1 s**, and the resulting documents are
  **inserted** into FTS5 in **under 250 ms**, p95 over 5 runs, on the CI
  runner. The generator is committed. **Measured on the phase-0 machine:
  1.08 s parse, 0.14 s insert, 1.23 s wall clock** — the index is 12% of the
  cost and the parse is 88%, which is why the criterion is stated in two
  parts (ADR-0055). The real corpus, 1,693 notes, indexes in **0.30 s**.
- After `touch`-modifying a note, a `GET /api/v1/notes/<uid>` issued 200 ms
  later returns the new content, **measured from the fsnotify event**.
- A single-key frontmatter edit on a note with 40 unknown keys leaves those
  40 keys **byte-identical**, including comments, key order, quoting style,
  indentation and CRLF.
- A frontmatter write **never re-encodes the block**: a folded scalar, a
  `---` document marker, a `!!merge` key and a `^escaped` value all survive
  (ADR-0021).
- A duplicate key is detected on load, and the writer refuses to patch rather
  than patching the wrong occurrence.
- **Measured against the SRD corpus** (phase 0, ADR-0049): patching one
  field of each of the 1,692 real files leaves **every other byte of the
  block unchanged**, in 1,692 of 1,692 cases. The corpus audit is committed
  and runs as part of the suite.
- A list or a map is **never written on its key's line** — `key: - item` is
  not YAML — and the block's own sequence style is reproduced, compact or
  indented (ADR-0049).
- A string is quoted exactly when yaml.v3 or js-yaml would not read it back
  as a string; the YAML 1.1 boolean words (`yes`, `no`, `on`, `off`, `y`,
  `n`) are deliberately left plain (ADR-0049).
- Reverting a journal entry restores the prior bytes exactly; reverting a
  batch reverts every entry sharing its `batchId`, in reverse order.
- An asset a player may not see returns **404, not 403**; the same asset
  returns 200 with an `ETag` for a caller who may, and 304 on
  `If-None-Match`.
- The store's DSN sets `journal_mode(wal)`, `busy_timeout(5000)` and
  `synchronous(NORMAL)`, and the writer pool is `SetMaxOpenConns(1)`.
  **400 concurrent inserts complete with zero "database is locked" errors**
  (measured: 243 of 400 failed without these).
- The watcher uses **content hashes, not mtimes**; reconciles a removal
  reported as `CHMOD` by stat-ing the path; and runs a full rescan every
  **30 seconds** — measured, not guessed: a rescan of the 1,693-note corpus
  costs 9 ms stat-only or 52 ms with hashes, so 30 s is a **0.03% / 0.17%
  duty cycle** (ADR-0055).
- On **macOS** the watcher is a polling watcher and opens **zero file
  descriptors** per note. A 50,000-note vault indexes and watches there
  without `EMFILE`. **Measured on Linux: the polling watcher holds +0 fds
  for 1,693 notes, and fsnotify holds one per *directory*, not per file.**
  The Linux number does not transfer — the same fsnotify call opens a
  descriptor per file on kqueue — so the macOS number must be taken on a
  Mac, and the spike ships the command that takes it (ADR-0055).
- A change made **the way Obsidian makes one** — a temp file renamed over
  the original, which changes the inode — is seen **on the target path**,
  and the watch survives it (ADR-0055).
- A vault is treated as untrusted input: a note larger than 8 MB, a
  frontmatter block larger than 1 MB, a frontmatter deeper than 32 levels and
  a frontmatter with more than 10,000 keys are each rejected with a clear
  error, not a crash.
- A YAML alias bomb is refused: `node.Decode` is never called on untrusted
  frontmatter, and the tree walk is bounded.

### M2 — The minimal pack format

**M2a** — the loader, the content type declarations, the example pack. The
example pack's content type `id` matches the demo campaign's `system:`, and
it declares **at least one owner-scoped and one gm-scoped resource**, or
M5c's property test passes vacuously (ADR-0050).

Phase 1's pack format is a **deliberate subset**: it declares content types
and their typed fields, and it carries the visibility default. **No formulas,
no resources, no dice, no sheets.** Those are phase 2, and they are additive
to a format that already works.

```yaml
id: example
name: Example
version: 1.0.0
content:
  - id: character
    label: Character
    visibility: party
    fields:
      - { key: species, label: Species, type: text }
      - { key: occupation, label: Occupation, type: text }
```

**Done when**
- A pack declaring a content type with typed fields loads, and a note of that
  `content_type` validates against it.
- A pack declaring a **missing** field, an **unknown** field type, or a
  **duplicate** content type is rejected with a precise error naming the
  field.
- A pack declaring a `visibility` default is applied to the notes the
  pipeline emits, and a content type with no default is `gm` (ADR-0033).
- A pack that declares a `formula` or a `dice` block in phase 1 is
  **rejected with a message saying those are phase 2** — so a pack written
  for phase 2 fails loudly rather than half-working.
- The example pack's `character` type round-trips through the editor.

### M3 — The plugin host's core

**M3a** — registry, capabilities, services. Capabilities come in **three
kinds** — invocable, subscription, declaration — and the four host
primitives (`context.get`, `pref.get`, `pref.set`, `command.register`) are
frozen with `api: 1` (ADR-0056).
**M3b** — surface hosts, the bridge, the shell. The bridge hands out a
**projected `NoteView`**, never a raw note: the owner-scoped fields
(`player`, `grants`, `discovered`) are stripped unless the principal is the
GM or the note's player (ADR-0056).
**M3c** — the API contract. The manifest field is **`api`**, not `abi`, and
the host checks it **before loading the module** (ADR-0056).
**M3d** — the compiled-in plugin registration path, manifest-driven
(ADR-0056).

**No WASM in phase 1.** The sandbox is phase 4 (ADR-0020, ADR-0045). What
phase 1 has is the *shape* of a plugin host — a manifest, a capability
model, a service registry, surface hosts, a bridge — with compiled-in
plugins only. That is what keeps the core a core rather than a monolith, and
it is what makes the phase-3 sandbox an addition rather than a rewrite.

**Done when**
- A plugin can register a service, a mount point and a command, and be
  called through it.
- A plugin declaring only `vault.read` **cannot** write, and **cannot** read
  a note hidden from the calling role.
- The capability and host-function vocabularies are the same list, and the
  manifest validator rejects a capability with no host function. The WASM host
  functions themselves are phase 3; the *vocabulary* is settled now, so the
  manifest does not change shape when they arrive.
- A plugin with `dom.inline` mounts into a **light-DOM host**; one without
  mounts into a **shadow-root host** the app provides (ADR-0026). The app can
  style the shadow surface with `::part()`; the plugin cannot seal the root
  away from the app.
- The core renders a plugin's declarative `config` block into a settings
  form, and a plugin's own config surface takes precedence.
- The shell boots with **every plugin disabled** and still opens a vault.
- A plugin that declares `abi: 2` is refused with a legible message before
  its module is loaded.

### M4 — The editor

**M4a** — the buffer, the live preview, the corpus.
**M4b** — the frontmatter panel, write-side verification.
**M4c** — the source view, the callout and stat-block rendering.

**Source editing with a live preview.** The buffer is the file, so the
round-trip contract is satisfied trivially — nothing transforms the text. The
rich layer — block widgets and the extension registry — is phase 2; the
cross-browser harness, which gates the plugin host's UI tier, is phase 3.

**Done when**
- **Every corpus fixture round-trips byte-identically.** The fixtures include
  tight and loose lists, setext headings, reference links, aligned tables,
  HTML blocks, autolinks, task lists, footnotes, emoji, zero-width
  characters, CRLF, a BOM, no trailing newline, a file that is nothing but a
  Dataview block, the 355 `ad-statblock` blocks, callouts with fold markers,
  and the 204 `^statid` references.
- **The write path transforms nothing.** Opening a note and saving it
  unchanged leaves the file byte-identical — asserted by `git diff --exit-code`
  over the corpus.
- A note containing an unknown Dataview block is preserved exactly through
  open → edit elsewhere → save.
- A lossy edit is **rejected with an explanation**, not silently applied.
- The write-side check re-parses the serialised output and compares it to the
  editor's document; a mismatch **blocks the write** and shows a diff.
- The frontmatter panel edits the reserved set of ADR-0040 and emits
  **type-aware YAML**: booleans as `true`/`false` and never as `yes`/`no`,
  wikilinks in properties always quoted, property names matched
  case-insensitively, and the value's YAML *type* matching
  `.obsidian/types.json` where the user has registered one.
- The live preview renders the corpus's `ad-statblock` blocks and callouts
  **read-only**, with the fold marker honoured.
- The two parsers agree: a conformance suite parses the corpus with the Go
  parser and the browser parser and compares the extracted plain text and
  links. **A disagreement fails CI** (ADR-0043).

### M5 — Accounts, the campaign, the projection

**M5a** — accounts, the campaign join code, tokens.
**M5b** — the projection.
**M5c** — the property test and the exposure checklist.

**No sessions, seats or session codes in phase 1.** Those need live state,
which is phase 4 (ADR-0038). What phase 1 has is the *account* and the
*campaign* — which is the whole premise.

**Done when**
- A wrong passphrase, a **used** campaign code and an **expired** campaign
  code are each rejected, with distinct messages.
- The projection resolves against a **principal** — `gm`, `account` or
  `anonymous` — not a role string (ADR-0038). A `seat` principal joins in
  phase 4.
- A campaign note's `party` resolves against **its own `members` list**, and
  a member id that does not exist is tolerated rather than fatal.
- A note with `visibility: private` is visible to its `player` and to nobody
  else, not even the GM (ADR-0048, ADR-0057).
- **The projection computes audiences, not rule verdicts** (ADR-0057): the
  audience is the intersection of every applicable constraint, then widened
  by the grants that are permitted to widen it. The property-level `owner`
  rule **includes the GM**, so a GM can see a player's hit points — which
  ADR-0002 requires and ADR-0033's rule 7 did not deliver.
- A seat's **binding is part of the seat's identity**, not a grant, so it
  survives the workflow gate: a guest may play a character still in draft,
  because the GM assigned it (ADR-0057).
- `visibility: party` on a note with **no campaign** is a **linter error**,
  not a silent black hole. A `visibility:` value outside the vocabulary is
  both a linter error and a read-time fail-closed (ADR-0057).
- The reserved fields carry **their own visibility**: `player`, `workflow`,
  `grants` and `revealed` to the note's `player` and the GM, `discovered` to
  the note's `player` alone, and `type`, `uid`, `name`, `system`,
  `campaign`, `kind` and `visibility` to everyone who can see the note
  (ADR-0057, extending ADR-0050's table).
- A note with `visibility: discovered` is visible to the GM and to any
  principal who owns a character whose `discovered` list names it — and to
  nobody else (ADR-0050).
- **An unrecognised `visibility` value fails closed**: it is treated as
  `gm`. The projection must never guess that an unknown value means *more*
  visible.
- The reserved fields declare a visibility per field: `discovered` and
  `grants` are `owner`-scoped, everything else inherits the note's
  (ADR-0050). A note with no `player` falls back to the note's visibility.
- **Obsidian's `%%comments%%` are stripped from a non-GM projection**
  (ADR-0050). They are hidden in preview but were serialised into the
  projection, which makes them the most likely place for a GM's aside to
  leak by accident.
- A note in `workflow: draft` is visible to its owner and the GM **and to
  nobody else**, regardless of its `visibility` and its grants.
- A **display principal** — the shared screen for the party — resolves to
  **party-visible content only**. It never sees `owner`-scoped, granted or
  character-discovered content, and it is **not** the union of what the party
  members can see, because that union would leak one player's private note to
  the rest (ADR-0048).
- **The property test:** for every note in the demo campaign, every declared
  property, and every **non-GM** principal, the serialised projection
  contains no field whose visibility is `gm`, and no `owner` field for a
  caller who does not own the note. **The GM principal is asserted
  separately** — it *does* contain every `gm` field.
- **The demo campaign's expectation tables are adopted**: the exact note set
  each of six principals may see, in both directions, from
  `dev/demo-campaign/projection_test.go`. Two assertions are added to them:
  **no GM-only character is named anywhere in a player's view**, and **no
  unrevealed secret's text appears in any non-GM projection**.
- **The 36 cases of the projection spike's synthetic corpus**
  (`spikes/projection/testdata/`) join them: the corpus cannot exercise the
  model at all — **0 of its 1,693 notes carry any of ADR-0040's twelve
  reserved fields** — so the model's cases have to be written (ADR-0057).
- **The GM can see a player's `owner`-scoped fields.** No non-GM sees an
  `owner` field it does not own, and no non-GM sees a `discovered` list it
  does not own (ADR-0057).
- The example pack declares **at least one owner-scoped and one gm-scoped
  resource**, or the property test iterates over nothing and passes
  vacuously (ADR-0050).
- The count returned by a query reflects what was **returned**, not what
  matched, so it is not an oracle for what is hidden.
- An FTS query from a player runs against the **player index**, so a term
  appearing only in a GM-only note cannot change the ranking of their
  results.
- An audit log records authentication events, and — under `--expose` — reads
  of GM-only notes.

### M6 — The browser and search plugins

**M6a** — typed views and filters.
**M6b** — the sheet and form rendering.
**M6c** — the search interface.

**Done when**
- A 5,000-note vault: every view filters and sorts in under 100 ms.
- A character's panel renders **exactly** the values
  `GET /api/v1/characters/<uid>?expand=1` returns, and every value also
  present in the note's frontmatter is rendered identically to the frontmatter
  value. The set of rendered fields equals the pack's declaration for that
  content type.
- The literal index resolves a name, alias or wikilink target exactly, and
  the ranked index returns results in BM25 order.
- The filters are over the universal fields and the pack's declared fields.
  **The query language of ADR-0047 is phase 3**; phase 1's filters are
  equality over those fields, evaluated against the in-memory index.

### M7 — Scaffolding, the demo campaign, the published schema

**M7a** — the scaffolder and the templates.
**M7b** — the demo campaign — `dev/demo-campaign/vault/`, already written in
phase 0, unchanged.
**M7c** — the published schema and starter Bases.

**Done when**
- `demitable init /tmp/x` produces a vault containing one note of every
  **core** type in §5.1 of the design document, each with a `uid`, a `type`
  and resolvable relative links, and it opens with **zero errors logged**. A
  scripted test performs the init, the open, and the open of one note of
  each type.
- Opening and saving every note in the demo campaign without modifying it
  leaves the vault **byte-identical** — `git diff --exit-code` is empty.
- The demo campaign contains **both** a visited and an unvisited location, a
  GM-only note, a character in `draft`, a session log, a `table` note and an
  asset — and the projection test over it passes in both directions.
- The published `schema.md` lists every reserved field, **its visibility**,
  its type and its vocabulary, and the starter Bases and Dataview queries
  work against a freshly scaffolded vault (ADR-0050).
- The published schema says that `campaign:` holds a **link** to the
  campaign note, that `discovered` is a **flat list** of note references,
  that `campaign` and `document` are note types, and that a GM-owned `date`
  must be a **quoted string** — an unquoted one is a YAML timestamp
  (ADR-0050).

### M8 — The export plugin

**M8a** — the transport and the resources.
**M8b** — the tools.

**Done when**
- An export of the demo campaign contains every note the exporting principal
  may see, **and nothing it may not**.
- The export renders **without the app** — a static HTML file, no server, no
  JavaScript required to read it.
- The export is a second, independent consumer of the API, so the API is
  exercised by something the editor did not shape.
- **`api: 1` is frozen at this milestone** (ADR-0045). The manifest's `abi:`
  field is checked before the module loads, and a mismatched plugin is
  refused with a legible message.

---

## 4. Sequencing

```
M0 ──► M1 ──┬──► M4 (editor) ──┬──► M7 ──► M8 (export, api:1 frozen)
            └──► M2 (pack format) ──► M3 (plugin host) ──► M4
                                          │
M5 (accounts, campaign, projection) ◄─────┘
        │
M6 (browser + search) ◄──┘
```

- **M3 precedes M4.** The plugin host exists so the editor can be written
  against it, and the editor is what shapes the API.
- **M1 precedes M2** — a pack declares fields, and a field is only meaningful
  against the schema the vault defines.
- **M5 needs M1's index** and M2's field declarations.
- **M7 needs M1, M2 and M5.**

**The critical path is M0 → M1 → M2 → M3 → M4 → M7 → M8**, with M5 and M6
alongside.

---

## 5. The exposure checklist, and who owns it

The app may be internet-exposed (ADR-0031). Before it is declared safe to
expose, each item must hold, **and the milestone that owns it must be
complete**. The import rows are phase 2, because the import is.

> **This checklist is derived from [`threat-model.md`](threat-model.md)**, and
> that document is its source of truth (ADR-0051). Every row below maps to a
> section of the threat model, and every defence in the threat model maps to a
> row or a milestone criterion. The rows the model exposes but the checklist
> lacked — the session code, the GM secret, the audit log, the credential
> windows and the MCP token — were added in phase 0, question 10.
>
> The threat model's **not-defended list is binding**. An item on it is an
> accepted risk or a known gap, and phase 1 must either close it or record it
> as accepted, with the milestone that owns the closure.

| Check | Owner |
|---|---|
| Authentication is rate-limited and backed off on every auth endpoint | M5 |
| **A campaign code is single-use, expires, and accepted only by the account endpoint** | M5 |
| **The session-code endpoint is rate-limited and counts attempts** | Phase 4 |
| **The GM secret is rotatable, and rotation invalidates outstanding sessions** | M5 |
| **Every read of a GM-only note is written to the audit log** | M5 |
| **A guest seat sees its bound character, the live table, and nothing else** | Phase 4 |
| **A seat token grants nothing outside its session** | Phase 4 |
| The GM authenticates with a credential, not by being local | M5 |
| Absolute URLs come from the configured public base URL, never the `Host` header | M3 |
| Forwarded headers are trusted only from a configured proxy address | M3 |
| The `Secure` cookie flag follows the configured public scheme | M5 |
| CORS is off by default; the API is same-origin | M3 |
| **A player's FTS query is filtered before BM25/IDF ranking, so a GM-only note cannot shift a visible note's rank** | M1d / M6 |
| **Markdown is sanitised before the editor renders it** | M4 |
| The MCP endpoint is off by default, token-gated, and logs a warning if enabled | Phase 3 |
| The import refuses a private address and refuses to reconnect elsewhere | Phase 2 |
| The import requires https and is off unless explicitly enabled | Phase 2 |
| The app starts and works with the network interface down, and with DNS unavailable | M0 |
| There is no automatic update path | M0 |

---

## 6. Open items

Carried forward from the previous plan, and settled by decision:

- The formula language's scope — ADR-0042, phase 2.
- The Go markdown parser — ADR-0043, phase 1 (the indexer and the link
  checker).
- The Blades clock and harm representation — ADR-0044, phase 2.
- The plugin ABI, the capability mapping, the `api: 1` freeze — ADR-0045;
  the *vocabulary* is phase 1, the WASM host functions are phase 3.
- The shadow host contract and the config UI — ADR-0046, phase 1.
- The plugin ABI freeze — ADR-0045, amended by **ADR-0056**: capabilities
  come in three kinds (invocable, subscription, declaration), the host
  primitives (`context.get`, `pref.get`, `pref.set`, `command.register`)
  are part of `api: 1`, the manifest field is `api`, and the freeze lands
  at **M8** — the export plugin is the second, non-UI consumer that
  ADR-0019 requires for a freeze.
- The query language — ADR-0047, phase 3.
- The seat and guest identity — ADR-0038; the *account* and *campaign* are
  phase 1, the *seat* is phase 4.

Still open, and they are product decisions rather than technical ones:
whether the MVP ships the SRD content vault at all (the licence permits
either), and what the demo campaign is.
