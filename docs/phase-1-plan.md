# Phase 1 Plan — The Wiki

> **Goal:** a working local app whose **minimal core** opens a campaign vault,
> browses it as a typed campaign database, edits notes (prose *and*
> structure), interprets a system pack, and serves a role-filtered view to
> players — with nothing but a markdown directory on disk, and every feature
> above the core delivered as a plugin.
>
> **Definition of done:** a complete character can be created, edited,
> formula-evaluated and re-read without any of it needing to be hand-fixing;
> a GM-only note is provably unreachable by a player; and the core runs with
> every plugin disabled.

---

## 1. What this phase costs, honestly

**Thirteen milestones, thirty-seven sub-milestones.** The milestones are
navigation; the sub-milestones are the actual work, and they are what the
estimates below refer to.

| | Full-time | Part-time (≈10 h/week) |
|---|---|---|
| M0–M1 — core, vault service, indexes, journal, assets, store, watcher | 8–10 weeks | 5–6 months |
| M2–M4 — formula engine, dice engine, packs, the 5e pack | 8–10 weeks | 5–6 months |
| M5 — SRD pipeline, corpus, suites | 4–6 weeks | 3 months |
| M6 — plugin host, surfaces, WASM runtime, API contract | 10–12 weeks | 6–7 months |
| M7 — editor, corpus, cross-browser harness | 8–12 weeks | 5–7 months |
| M8 — accounts, codes, projection | 6–8 weeks | 4 months |
| M9 — browser and search plugins | 6–8 weeks | 4 months |
| M10 — scaffolder, demo campaign, published schema | 8–10 weeks | 5 months |
| M11–M12 — export plugin, the Blades gate | 5–6 weeks | 3 months |
| **Phase 1 total** | **~70–90 weeks** | **~3.5–4.5 years** |

**All four phases are a multi-year effort.** Stating that here is deliberate:
the largest credibility risk in this document set was a plan that read like a
sprint list and was not one.

Two things have changed the estimate downward since the first draft. The
editor is CodeMirror rather than Tiptap, which removes the ProseMirror
integration, the node-level serializer and the lossless-splice work. And the
Blades gate moved from last to immediately after the packs, so a format
problem surfaces in week six rather than month nine.

**If the part-time figure is unacceptable, the lever is scope, not effort.**
The two candidates to cut are the SRD pipeline (M5) and the plugin host's
WASM tier (M6c). Both are named in §2.

## 2. The phase boundary

**In phase 1:** the vault service, the formula engine, the dice engine, the
pack format and the 5e pack, the SRD content vault, the plugin host, the
editor, accounts and the projection layer, the browser and search plugins,
the scaffolder, the demo campaign, the export plugin, and the Blades gate.

**Explicitly out, and why:**

| Out of phase 1 | Why |
|---|---|
| The character-creation **wizard** | Its input contract is settled in phase 1 (the pack's `content:` declarations), but the interface is phase 2. The *accounts and review workflow* it depends on are in phase 1 |
| The dice **runner** | The dice *engine* is phase 1 (M3), because the Blades gate needs a dice-pool roll. The interface, the shared log and the workflows are phase 2 |
| The encounter planner and runner | Phase 2 |
| The 2D tabletop, the player client | Phase 3 |
| Chat, the shared display mode | Phase 3 |
| Recording, transcription, music | Phase 4 |
| The archive import endpoint | Phase 1.5. Content reaches users by unzipping a downloaded archive, which needs no server feature |
| The MCP plugin | Phase 1.5. It is a good plugin; it is not one to write *first* |

The dice engine and the dice runner are now distinguished in ADR-0004 and
ADR-0008, which previously disagreed with the plan about which was phase 1.

---

## 3. Milestones

Each milestone lists its sub-milestones and a checkable predicate. "Done
when" is a thing a script can assert, not a thing a reader has to agree with.

### M0 — Skeleton and build pipeline

**M0a** — Go module, `cmd/demitable`, the server serving `web/dist` via
`go:embed`, the Svelte shell, and the config surface.

**Done when**
- `go build ./cmd/demitable` produces a binary; started with `--vault <tmp>
  --port 0 --no-browser`, it serves `GET /` containing the built bundle's
  hash and `GET /api/health` with 200, **from one process, with no `node` on
  `PATH`**.
- The exposure configuration surface exists with safe defaults: `--expose`,
  `--public-url`, `--trust-proxy`, `--allow-http-import`, `--no-import`
  (ADR-0031, ADR-0025).
- `CGO_ENABLED=0` builds succeed for `linux/amd64`, `linux/arm64`,
  `darwin/amd64`, `darwin/arm64`, `windows/amd64`, and each binary is under
  12 MB.
- Graceful shutdown on `SIGTERM`: in-flight writes finish, the SQLite WAL is
  checkpointed, and the process exits 0.

### M1 — Vault service, indexes, store, watcher

**M1a — the vault** · read, index, watch, frontmatter.
**M1b — the write journal** · append-only, content-addressed blobs, revert.
**M1c — the asset endpoint** · role-scoped, ETag-cached.
**M1d — the store** · SQLite, FTS5, the `notes` table.
**M1e — the watcher** · debounce, hash-compare, rescan backstop.

**Done when**
- A generated vault of 5,000 notes, each with 40 unknown frontmatter keys and
  a 2 KB body, is fully indexed — frontmatter parsed to `yaml.Node`, link
  graph built, FTS5 populated — **in under 1 s wall clock, p95 over 5 runs**,
  on the CI runner. The generator is committed.
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
  30–60 s.
- On **macOS** the watcher is a polling watcher and opens **zero file
  descriptors** per note. A 50,000-note vault indexes and watches there
  without `EMFILE`.
- A vault is treated as untrusted input: a note larger than 8 MB, a
  frontmatter block larger than 1 MB, a frontmatter deeper than 32 levels and
  a frontmatter with more than 10,000 keys are each rejected with a clear
  error, not a crash.
- A YAML alias bomb is refused: `node.Decode` is never called on untrusted
  frontmatter, and the tree walk is bounded.

### M2 — The formula engine

**M2a** — lexer, Pratt parser, evaluator, dependency resolution.
**M2b** — the sandbox.

**Done when**
- Formulas are evaluated against the layered scope of ADR-0042: call scope
  shadows entity scope shadows pack scope, with derived values resolved
  lazily, memoised, and cycle-detected.
- The pack's own examples evaluate: `max_hp: "10 + attributes.con + level *
  con_mod"` and `attack_mod: "attributes.str_mod + proficiency +
  weapon.bonus"`, with `level` and `proficiency` from the pack scope and
  `weapon.bonus` from the call scope.
- A formula referencing an unknown identifier is a **linter failure at pack
  load**, not a runtime error at the table.
- The malicious suite passes: each case in `formulas_test.go` asserts both
  that evaluation returns an error **and** that the error message contains
  the formula name and the byte offset of the offending token.
  `constructor.constructor("return 1")()`, `a.__proto__`, and a
  self-referencing formula are all refused. **1,000 nested `[`** is refused
  with a depth-limit error, not a stack overflow.
- A formula that loops or explodes is cut off by a **step budget and a
  wall-clock timeout**, both enforced by the host.
- A pack that shadows a name between layers produces a **linter warning**.

### M3 — The dice engine

**M3a** — notation, numeric dice.
**M3b** — tables, faces, symbol dice.
**M3c** — pools, opposed rolls, the log.

**Done when**
- `4d6kh3` keeps the highest three; `2d20kl1` keeps the lowest; `1d20+5`
  adds five; `6d6>=5` explodes; `4dF` yields −1, 0 or +1.
- A table `mixed-d6` with faces `1..5` (value = the number) and one face
  `critical` (value = 6, `tags: [critical]`) rolled 1,000 times produces a
  total between 6 and 11 inclusive, **exactly one face tagged `critical` per
  roll**, and one log entry per roll carrying the six face ids, the total,
  the roller and the timestamp.
- A symbolic die produces a **symbol result, not a number**, and a die whose
  faces have no rank **cannot be compared** — the error names the fix.
- A pool counts successes from tagged faces or a rank threshold, and reports
  an error if neither is defined.
- A core schema with `kind: track` and `kind: scale` resources and
  conditions (ADR-0044) round-trips through the engine: a track's `filled`
  count is bounded by its `size` and its `segments`.

### M4 — Packs: model, loader, validator, linter

**M4a** — the loader, the validator, the linter.
**M4b** — the 5e pack.

**Done when**
- `demitable pack lint packs/dnd5e.yaml` is clean.
- A deliberately broken pack — an unknown formula target, a cycle, a missing
  label, a shadowed name — produces a precise, actionable error naming the
  field and the line.
- **The 5e pack evaluates a level-5 fighter correctly.** The fixture
  `fixtures/testdata/fighter-l5.yaml` (STR 12, DEX 14, CON 16, WIS 10, CHA
  8, level 5, chain mail and shield, proficient in perception) with the pack's
  `hit_die: 10` and `max_hp: "10 + attributes.con + (level - 1) * con_mod"`
  yields `str_mod` +1, `proficiency` +3, `max_hp` 38, `ac` 18,
  `passive_perception` 13. **The fixture and the expected values are
  committed.**
- The pack's `content:` declarations carry a `visibility` default per type
  (ADR-0033, ADR-0036): `spell` and the character-creation content is
  `party`; `monster` and `item` are `gm`.
- The linter checks the schema invariants of ADR-0007, and reports the
  **track and scale shapes** of ADR-0044 as available rather than required.

### M5 — The SRD pipeline and the content vault

**M5a** — acquisition and `srdcheck`.
**M5b** — the enricher.
**M5c** — the corpus, validation and link-checker suites.

**Done when**
- `srdcheck` verifies the corpus is version 2.28.1, that the expected file
  inventory is present, that the recorded checksums of the index and a sample
  of entity files match, and that there are **no unexpected top-level
  entries**. A failure stops the build with a clear message.
- `srdcheck` also verifies the **upstream provenance**: the repository URL,
  the commit hash, the licence file's verbatim text and its SPDX identifier.
  **If those cannot be pinned, the build fails** (ADR-0024).
- The build is **deterministic**: two runs on the same corpus produce
  byte-identical output. Every emitted collection is sorted; no timestamp or
  hash-map ordering reaches a file.
- The output contains **exactly 1,692 files**, of which **658 are monsters and
  658 are spells** — asserted numerically, not by eyeball (ADR-0024).
- **No non-SRD entity appears.** The filter is `srdKeys` membership, not a
  source tag: the source-tag rule alone admits 2,628 keys `srdKeys` excludes.
  A run that emits a key outside `srdKeys` fails the build.
- The 355 `ad-statblock` blocks and the 204 `^statblock` block references are
  **preserved verbatim** (ADR-0036).
- Every wikilink in the output resolves.
- The corpus is **retained outside git** as a pinned, checksummed copy. The
  working-tree copy is gitignored and removable; the pinned copy is not.
- The two `phb`-sourced list pages are verified against the SRD and excluded
  with a recorded reason.

### M6 — The plugin host

**M6a** — registry, capabilities, services, the `api: 1` freeze.
**M6b** — surface hosts, the bridge, the shell.
**M6c** — the WASM runtime.
**M6d** — the API contract.

**Done when**
- A plugin can register a service, a mount point and a command, and be
  called through it.
- A plugin declaring only `vault.read` **cannot** write, and **cannot** read
  a note hidden from the calling role.
- The capability and host-function vocabularies are the same list, and the
  manifest validator rejects a capability with no host function.
- The **golden ABI test** — a wasm module importing every host function —
  builds at compile time, so a changed signature fails CI.
- The host is configured with `WithCloseOnContextDone(true)`, a per-call
  context deadline the host owns, and `WithMemoryLimitPages` at 256 pages.
  **An infinite-loop guest is terminated within 200 ms** by the deadline
  (measured: without `closeOnContextDone`, `Runtime.Close()` returns while the
  call keeps spinning forever).
- A guest importing `wasi_thread_start` **fails to instantiate**.
- A guest calling `sock_send` receives an error, and the ADR-0020 claim that
  wazero does not implement sockets is corrected to the truth: it does, and
  safety is a matter of configuration, not absence.
- A plugin with `dom.inline` mounts into a **light-DOM host**; one without
  mounts into a **shadow-root host** the app provides (ADR-0026). The app can
  style the shadow surface with `::part()`; the plugin cannot seal the root
  away from the app.
- The core renders a plugin's declarative `config` block into a settings
  form, and a plugin's own config surface takes precedence.
- The shell boots with **every plugin disabled** and still opens a vault.

### M7 — The editor plugin

**M7a** — the buffer, the raw block, the corpus.
**M7b** — write-side verification, the frontmatter panel.
**M7c** — the cross-browser harness.
**M7d** — the source view, the extension registry.

**Done when**
- **Every corpus fixture round-trips byte-identically.** The fixtures include
  tight and loose lists, setext headings, reference links, aligned tables,
  HTML blocks, autolinks, task lists, footnotes, emoji, zero-width
  characters, CRLF, a BOM, no trailing newline, a file that is nothing but a
  Dataview block, the 355 `ad-statblock` blocks, callouts with fold markers,
  and the 204 `^blockid` references.
- **The write path uses no parser.** Opening a note and saving it unchanged
  leaves the file byte-identical — asserted by `git diff --exit-code` over the
  corpus.
- A note containing an unknown Dataview block is preserved exactly through
  open → edit elsewhere → save.
- A lossy edit is **rejected with an explanation**, not silently applied.
- The write-side check re-parses the serialised output and compares it to
  the editor's document; a mismatch **blocks the write** and shows a diff.
- The frontmatter panel edits the reserved set of ADR-0040 and emits
  **type-aware YAML**: booleans as `true`/`false` and never as `yes`/`no`,
  wikilinks in properties always quoted, property names matched
  case-insensitively, and the value's YAML *type* matching
  `.obsidian/types.json` where the user has registered one.
- The **cross-browser harness is committed to the repository** and runs
  against Chromium, Firefox and WebKit in CI, asserting all ten rules of
  [`shadow-dom-verification.md`](shadow-dom-verification.md) §4 in each
  engine. **CI fails on any engine that does not pass.**
- The harness does not use `ShadowRoot.getSelection()`, which does not exist
  outside Chromium; the selection is read from the active shadow root via
  `composedPath()`.
- A plugin can contribute a block widget through the editor's extension
  registry, and it round-trips.

### M8 — Accounts, codes, and the projection

**M8a** — accounts, the two code spaces, tokens.
**M8b** — the projection.
**M8c** — the property test and the exposure checklist.

**Done when**
- A wrong passphrase, a **used** campaign code, an **expired** campaign
  code, a used session code and an expired session code are each rejected,
  with distinct messages.
- **The two code spaces are disjoint**: a campaign code presented to the
  session endpoint is refused, and a session code presented to the account
  endpoint is refused. Their lengths differ — eight against six.
- The projection resolves against a **principal** — `gm`, `account`, `seat`
  or `anonymous` — not a role string. A guest seat is a principal, and it can
  hold a character.
- A seat's bound character is **implicitly visible to that seat**, and a seat
  naming a character the caller may not play is rejected.
- A campaign note's `party` resolves against **its own `members` list**, and
  a member id that does not exist is tolerated rather than fatal.
- **The property test:** for every note in the demo campaign, every declared
  property, and every **non-GM** principal, the serialised projection
  contains no field whose visibility is `gm`, and no `owner` field for a
  caller who does not own the note. **The GM principal is asserted
  separately** — it *does* contain every `gm` field.
- A note in `workflow: draft` is visible to its owner and the GM **and to
  nobody else**, regardless of its `visibility` and its grants.
- A **display principal** — the shared screen for the party — resolves to
  **party-visible content only**. It never sees `owner`-scoped, granted or
  character-discovered content, and it is **not** the union of what the party
  members can see, because that union would leak one player's private note to
  the rest (ADR-0048).
- A note with `visibility: private` is visible to its `player` and to nobody
  else, not even the GM.
- The count returned by a query reflects what was **returned**, not what
  matched, so it is not an oracle for what is hidden.
- An FTS query from a player runs against the **player index**, so a term
  appearing only in a GM-only note cannot change the ranking of their
  results.
- An audit log records authentication events, and — under `--expose` —
  reads of GM-only notes.

### M9 — The browser and search plugins

**M9a** — typed views and filters.
**M9b** — sheet rendering.
**M9c** — the search interface.

**Done when**
- A 5,000-note vault: every view filters and sorts in under 100 ms.
- A character's sheet renders **exactly** the values
  `GET /api/v1/characters/<uid>?expand=1` returns, and every value also
  present in the note's frontmatter is rendered identically to the frontmatter
  value. The set of rendered fields equals the pack's `views.sheet`.
- The query language of ADR-0047 is the **same** language in REST and in
  MCP, and it compiles to SQL.
- The two parsers agree: a conformance suite parses the corpus with the Go
  parser and the browser parser and compares the extracted plain text and
  links. **A disagreement fails CI** (ADR-0043).
- The literal index resolves a name, alias or wikilink target exactly, and
  the ranked index returns results in BM25 order.

### M10 — Scaffolding, the demo campaign, the published schema

**M10a** — the scaffolder and the templates.
**M10b** — the demo campaign.
**M10c** — the example campaign.
**M10d** — the published schema and starter Bases.

**Done when**
- `demitable init /tmp/x` produces a vault containing one note of every type
  in §5.1 of the design document, each with a `uid`, a `type` and resolvable
  relative links, and it opens with **zero errors logged**. A scripted test
  performs the init, the open, and the open of one note of each type.
- Opening and saving every note in the example campaign without modifying it
  leaves the vault **byte-identical** — `git diff --exit-code` is empty.
- The demo campaign contains **both** a visited and an unvisited location, a
  GM-only note, a character in `draft`, a played encounter using an SRD
  monster, a session log, a `table` note and an asset — and the projection
  test over it passes in both directions.
- The published `schema.md` lists every reserved field, its type and its
  vocabulary, and the starter Bases and Dataview queries work against a
  freshly scaffolded vault.

### M11 — The export plugin

**M11a** — the transport and the resources.
**M11b** — the tools.

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

### M12 — The Blades pack: the format gate

**Done when** — all six proofs of ADR-0012 hold:

1. A pack with no `levels` block loads and lints clean.
2. A pack with no HP-like resource expresses a character fine.
3. A dice-pool workflow rolls correctly: rating dice against opposition
   dice, highest count wins, with position and escalation.
4. Clocks work as a **track** — size, progress, segmentation (ADR-0044).
5. Harm works as a **scale**, not a numeric pool.
6. The difficulty model is opposition dice and clock pressure, with no XP
   budget anywhere in the core.

**If any of these need a change to the *core* rather than the pack, the pack
format was not generic enough and phase 2 waits.** M12 runs immediately after
M4, before the editor and before the schema is frozen by the 5e pack.

---

## 4. Sequencing

```
M0 ──► M1 ──┬──► M7 (editor) ─────────────► M10 ──► M11 (export, api:1 frozen)
            └──► M2 ──► M3 ──► M4 ──┬──► M12 (Blades gate)
                        │              │
                        │              └──► M5 (SRD pipeline) ──► M7
                        │
                        └──► M6 (plugin host) ──► M7
                                      │
M8 (accounts, projection) ◄────────────┘
    │
M9 (browser + search) ◄──┘
```

- **M6 precedes M7.** The plugin host exists so the editor can be written
  against it, and the editor is what shapes the API.
- **M5 precedes M7.** The corpus *is* the editor's golden corpus. Running M5
  before M7 turns the editor milestone from "build an editor and hope" into
  "build an editor against 1,692 files that already break it".
- **M12 runs immediately after M4.** A gate that fires after the thing it
  gates is not a gate.
- **M8 and M9 need M1's index** and M4's pack visibility declarations.

**The critical path is M0 → M1 → M2 → M3 → M4 → M6 → M7 → M10 → M11**, with
M5 and M12 as gates that must clear before the path continues.

---

## 5. Phase 1.5 — what was deferred, and why

| Deferred | Why | ADR |
|---|---|---|
| **The archive import endpoint** | Security-sensitive, network-facing, off by default, and its only phase-1 user would have been us publishing an archive a community member now hosts. Content reaches users by unzipping a downloaded archive, which needs no server feature | 0025 |
| **The MCP plugin** | Drags an external protocol into phase 1 for a Medium-risk, off-by-default feature. Any second non-UI plugin proves the API is not editor-shaped — the export plugin does that | 0014 |

---

## 6. The exposure checklist, and who owns it

The app may be internet-exposed (ADR-0031). Before it is declared safe to
expose, each item must hold, **and the milestone that owns it must be
complete**. This list is checkable rather than aspirational, because
"internet-exposed" is the deployment where every one of them matters.

| Check | Owner |
|---|---|
| The import refuses a URL resolving to a private, loopback or link-local address, and refuses to reconnect to an address other than the one it validated | M10 |
| The import requires https and is off unless explicitly enabled | M10 |
| Authentication is rate-limited and backed off on every auth endpoint | M8 |
| **A session code is short-lived, revocable, and accepted only by the session endpoint** | M8 |
| **A campaign code is single-use, expires, and accepted only by the account endpoint** | M8 |
| **A guest seat sees its bound character, the live table, and nothing else** | M8 |
| **A seat token grants nothing outside its session** | M8 |
| The GM authenticates with a credential, not by being local | M8 |
| Absolute URLs come from the configured public base URL, never the `Host` header | M6 |
| Forwarded headers are trusted only from a configured proxy address | M6 |
| The `Secure` cookie flag follows the configured public scheme | M8 |
| CORS is off by default; the API is same-origin | M6 |
| The MCP endpoint is off by default, and logs a warning if enabled | M11 |
| The app starts and works with the network interface down, and with DNS unavailable | M0 |
| There is no automatic update path | M0 |

---

## 7. Open items

- **The formula language's context model** — ADR-0042. Settled.
- **The Go markdown parser** — ADR-0043. Settled: `goldmark`, with a
  cross-parser conformance suite.
- **The Blades clock and harm representation** — ADR-0044. Settled: pools
  and tracks, flags and scales.
- **The WASM ABI, the capability mapping and the `api: 1` freeze** —
  ADR-0045. Settled.
- **The shadow host contract and the config UI** — ADR-0046. Settled.
- **The `list_by` query language and the cursor format** — ADR-0047.
  Settled.
- **The seat and guest identity** — ADR-0038. Settled.
- **Still open, and they are product decisions rather than technical ones:**
  the scratch log, chat, the shared display mode, the personal player
  journal, and the campaign lifecycle. Each is cheaper as a schema decision
  now than as a feature later. See `docs/research-findings.md`.
