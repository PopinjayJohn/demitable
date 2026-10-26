# Phase 2 Plan — The System

> **Goal:** a working local app whose **pack format is complete** — resources,
> formulas, dice and sheets — plus the machinery that interprets it, the two
> packs that prove it is generic, and the way content arrives.
>
> **Phase 2 in one line:** *the rules and the content.*
>
> **Definition of done:** two deliberately differently-shaped game systems
> both work end to end — 5e and Blades — and a GM can import an archive into
> a campaign.
>
> **And one thing it does not claim.** A GM-only note is unreachable by a
> *player*, not by a *plugin*. That is unchanged from phase 1
> (see [`phase-1-plan.md`](phase-1-plan.md) §1 and
> [`threat-model.md`](threat-model.md) §8–§9). A plugin's *logic* is
> sandboxed; a plugin's *UI* runs in the app's document with the app's
> privileges (ADR-0029). The import warning (ADR-0025, ADR-0027) exists
> because imported plugins are executable code — it is a prompt, not a
> security control.

Phase membership and what moved: [`phase-plan.md`](phase-plan.md).

---

## 1. What this phase costs

**Eight milestones, twenty sub-milestones.** Estimates are rough — phase 1
taught that a number without the artifact it was measured on is a wish, so
these are ranges against the phase-1 machine, not commitments.

| Block | Sub-milestones | Full-time | Part-time (≈10 h/week) |
|---|---|---|---|
| N1 — formula engine | 3 | 3–4 wks | 2–3 months |
| N2 — dice engine | 3 | 3–4 wks | 2–3 months |
| N3 — pack format completion + linter | 2 | 2–3 wks | 1–2 months |
| N4 — 5e pack + SRD content vault | 3 | 4–6 wks | 3–4 months |
| N5 — Blades gate | 2 | 2–3 wks | 1–2 months |
| N6 — wizard, sheets, review workflow, spike surfaces | 4 | 6–8 wks | 4–5 months |
| N7 — rich editor content | 2 | 4–6 wks | 3–4 months |
| N8 — archive import | 2 | 3–4 wks | 2–3 months |
| **Phase 2 total** | **21** | **~27–38 weeks** | **~1.5–2 years** |

**The most expensive saving is already banked.** Phase 1 proved the write
path transforms nothing and the projection computes audiences. Phase 2
builds *on* those invariants rather than re-proving them — but every
milestone that touches them must re-run their suites, because phase 1's
report shows the defects live in exactly those seams.

---

## 2. What is in this phase, and what is not

**In:** the formula engine (layered scope, sandbox) · the dice engine
(numeric, pools, opposed, symbol dice, tables) · the completed pack format
(resources as pools/tracks, conditions as flags/scales, progression,
difficulty, views/sheets, content declarations, secrets vocabulary) and
its linter · the 5e mechanics pack · the SRD content vault as an
importable archive · the Blades pack as the format gate · the creation
wizard and character sheets for GM and players with the review workflow ·
the rich editor's block widgets and extension registry · the archive
import endpoint (CLI + REST).

**Out, deliberately:**

| Out | Where |
|---|---|
| The WASM plugin tier, the host ABI | Phase 3 |
| The query language | Phase 3 |
| The MCP plugin | Phase 3 |
| The cross-browser harness | Phase 3 |
| Live sessions, seats, session codes, the scratch log | Phase 4 |
| The encounter runner, the tabletop, chat, the shared display | Phase 4 |
| The session write-back contract | Phase 4 |
| Recording, transcription, session logs, music | Phase 5 |

---

## 3. Milestones

### N1 — The formula engine

**N1a** — the language: lexer, parser, evaluator for the ADR-0003 grammar
(ternary, logic, equality, comparison, additive, multiplicative, unary,
postfix, primary), with the whitelisted builtins only (`min`, `max`,
`floor`, `ceil`, `round`, `abs`, `clamp`, `sum`, `len`, `has`,
`coalesce`, `count`, `lower`, `upper`).
**N1b** — the layered scope (ADR-0042): call scope shadows entity scope
shadows pack scope shadows derived values; derived values resolve lazily,
memoised, with cycle detection.
**N1c** — the sandbox and the linter hook: no assignment, no user-defined
functions, no `__proto__`/`constructor`/`prototype` access; step and depth
limits with a hard timeout; an unresolvable name is a pack error caught
at load time, and shadowing across layers is a lint warning.

**Done when**
- The pack examples evaluate: `max_hp` from `attributes.con` and `level`,
  and `attack_mod` from `attributes.str_mod`, `proficiency` and
  `weapon.bonus` supplied in the call scope — with the call scope winning
  where a name exists in more than one layer.
- A formula that references another formula needs no declaration order;
  a cyclic pair (`a` needs `b`, `b` needs `a`) is reported as a pack
  error naming both formulas, not a hung evaluation.
- A pack whose formula names a binding no declared context provides
  (e.g. a missing `level`) fails lint with the formula, the name, and
  the scope that was expected to supply it.
- A pack that defines `level` in the pack scope while an entity carries
  `data.level` lints with a shadowing warning naming both.
- An adversarial formula (deep nesting, long chain) is stopped by the
  step/depth limits or the timeout — measured, with the limits stated —
  and never by a stack overflow or a hung request.
- No formula can assign, define a function, or reach `__proto__`,
  `constructor` or `prototype`; each attempt is a parse-time rejection,
  one test per case.

### N2 — The dice engine

**N2a** — the expression language (ADR-0008): `NdM`, flat modifiers,
keep-highest/lowest (`4d6kh3`, `2d20kl1`), exploding (`6d6>=5`), Fudge
(`4dF`), wild percentile, success thresholds, opposed pools, `direction:
high | low`, `degreesOfSuccess`, named workflows (`against`, `bonus`,
`onSuccess`).
**N2b** — symbol dice and tables (ADR-0022): faces with payloads
(`id`, `label`, `symbol`, optional `asset`, optional `value`, optional
`rank`, optional `tags`); tables are ordinary `type: table` notes with
faces in `data.faces`; a vault table shadows a pack table of the same
id; result types (`numeric`, `symbol`, `count`, `named`) derived from
the definition.
**N2c** — the roll path and the log: the server rolls
(`POST /api/v1/roll`, `shared: true | false`), every roll logged with
full provenance (roller, workflow, expression, individual results,
modifier sources, target, outcome, timestamp) into the session note.

**Done when**
- A 5e-shaped `1d20+5` workflow, a Blades-shaped `{rating}d6` vs
  `{opposition}d6` pool, and a roll-under (`direction: low`) workflow
  each resolve to the documented outcome kind — total, success count,
  and inverted success respectively.
- Counting successes works from tagged faces and from a rank threshold;
  a count requested with neither defined is an error naming the table,
  not a default.
- An unordered die (faces without `rank`) in a comparison is an error
  saying the table has no ordering — never a silent fallback to
  declaration order.
- A vault `type: table` note shadows the pack table of the same id in
  every roll that names it.
- Every roll in a 100-roll sequence carries roller, workflow,
  expression, individual results, modifier sources, target, outcome and
  timestamp; `shared: false` rolls never appear in a non-GM log view.

### N3 — The pack format, completed, and its linter

**N3a** — the completed declarations: `attributes`, `resources` (pools
*and* tracks — `size`, `segments`, `filled` — ADR-0044), `conditions`
(flags *and* scales — ordered `steps` plus a value — ADR-0044),
`progression`, `difficulty` in the pack's own terms, `dice:`,
`formulas:`, `content:` with `fields`/`choices`/`index`,
`views.sheet`, and the `secrets:` vocabulary
(`calloutTypes`, `codeBlockKeys` — ADR-0036).
**N3b** — the linter: enforces the seven schema invariants of ADR-0007
(a pack need not define resources, numeric attributes, levels,
attributes at all, or conditions; must express PC, NPC and creature;
difficulty modelled by the pack), plus shadowing warnings (ADR-0042)
and wrong-shape hints (a clock encoded as a pool with `max: 6` warns).

Phase-1 packs keep loading unchanged — every N3 block is additive, so a
pack without `dice:` simply has no roll workflows.

**Done when**
- Each of the seven invariants has a negative pack that fails lint
  (named invariant, named field) and a positive pack that passes — 14
  cases, all committed.
- A 5e-shaped pack with levels, pools and flags and a Blades-shaped
  pack with no levels, no HP-like resource, tracks and scales both lint
  clean.
- A pack declaring `formula:` (singular), an anchor/alias, a merge key,
  a duplicate key, or an unquoted date is rejected with a precise
  message — the phase-1 profile checks still hold around the new
  blocks.
- The example pack from phase 1 extended with one resource, one
  formula and one `dice:` workflow still validates, and its character
  still round-trips through the editor.

### N4 — The 5e pack and the SRD content vault

**N4a** — the mechanics pack (`packs/dnd5e.yaml`): attributes, resources
(hp, ac, speed, spell slots), formulas (`con_mod`, `max_hp`,
`attack_bonus`, save DCs), conditions, progression (levels 1–20),
difficulty (XP budget), `dice:` workflows (attack, save, check), content
declarations (class, species, background, feat, spell, monster), sheet
views. Small (~50–100 KB), readable, linted.
**N4b** — the pipeline: transient `fixtures/` (gitignored, manually
acquired 2.28.1, verified by `tools/srdcheck`) → deterministic
enrichment → committed `dev/srd-vault/`. Uids derived from the source
path (never random), every emitted collection sorted, no timestamps in
the comparison, frontmatter for machines / body for humans (ADR-0023).
**N4c** — the published archive: the enriched vault published as an
importable archive with `demitable-import.json` (id, version, requires,
licence CC-BY-4.0, attribution), reached via `demitable import <url>`
(ADR-0025). **Not bundled in the binary.**

**Done when**
- Every SRD entity in the `xphb`/`xdmg`/`xmm` sources has a note:
  `type: content`, `content_type` set, `system: dnd5e`, mechanics in
  frontmatter, prose in the body — and **zero** non-SRD entities ship
  (the emitted notes carry their own `source` markers, so this is
  checkable without the corpus).
- Every content note validates against the pack's `content:`
  declarations, and every wikilink in the vault resolves — both suites
  run in CI against the committed vault.
- The pipeline reproduces the committed vault **byte-identically**
  (maintainer check, run locally — CI cannot obtain the corpus,
  ADR-0024). A divergence fails the check with the first differing
  file.
- The archive installs via N8's import into an empty vault and opens
  with zero errors logged.
- The binary carries the pack but not the vault: the five
  cross-compiled sizes are reported, and no content note is embedded.

### N5 — The Blades gate

The second pack, deliberately differently-shaped (ADR-0012). It encodes
structure and mechanics only — no rulebook prose — and must pass
**without a core change**: if any proof below needs the core to move,
the format was not generic enough and the phase waits.

**Done when**
- A Blades pack with **no `levels` block** loads and lints clean.
- A Blades character with **no HP-like resource** (harm as a `scale`
  condition track, clocks as `track` resources with size, progress and
  segmentation) validates and renders.
- A dice-pool workflow rolls `rating` dice vs `opposition` dice,
  highest count wins, with position and escalation — 100 seeded rolls
  all resolve to the documented outcome kind.
- The difficulty model is opposition dice and clock pressure, **not**
  an XP budget — and the encounter planner consumes it with no
  5e-specific branch (asserted by deleting the 5e pack from the test
  fixture and re-running: still green).
- Faction/crew entities are first-class notes the pack declares and
  the browser lists — the entity model is not only about characters.
- The Blades pack's `secrets:` vocabulary (its own callout types) is
  honoured by the projection's block stripping.

### N6 — The creation wizard, the character sheets, the review workflow

**N6a** — the wizard: reads the pack's `content:` declarations to know
what a class/species/background *is*, and the vault's content notes to
know what the classes *are* — so the same wizard drives 5e and Blades
with no per-system branch. Previews compute server-side
(`POST /api/v1/compute`, `GET /api/v1/notes/:uid?expand=1` — the
client never evaluates a formula).
**N6b** — the sheets: rendered from the pack's `views.sheet` — a form,
not a document — with computed values from the server; the rendered
field set equals the pack's declaration for that content type.
**N6c** — the review workflow (ADR-0009, as amended): `draft` →
`submitted` → `active`, with `retired` **not** a visibility gate
(ADR-0050 D6); `draft`/`submitted` visible to author and GM only;
the GM creates-and-approves in one step; accounts, join codes and
`player` ownership from phase 1 are the substrate.
**N6d** — the spike surfaces phase 2 needs (from `spikes/ui/`,
ADR-0052): the player's view, the visibility badge, and the
inspector. The badge shows the note's *effective* visibility; the
inspector beneath an opened note lists the rule chain that produced
it, citing sections. The spike's GM dashboard is **not** built here —
it is a phase 3 target (see §7).

**Done when**
- A player creates a character through the wizard for **each** of the
  two packs with no per-system code path — asserted by the wizard
  reading only `content:` declarations plus content notes.
- The state machine holds: `draft` and `submitted` visible to author
  and GM and nobody else regardless of `visibility` or grants; a GM
  edit of any state is allowed; a player edit of `submitted` is
  refused; `retired` keeps the visibility it had.
- A sheet renders exactly the values
  `GET /api/v1/characters/<uid>?expand=1` returns, and every value
  also present in frontmatter matches byte-for-byte; the field set
  equals the pack declaration — for one 5e and one Blades character
  each.
- Every list endpoint resolves against a **second, non-GM principal**:
  an account sees its own `draft` plus party-visible notes and nothing
  else — the seam is `principalFromRequest`, and the test that would
  have caught phase 1's invisible-journal bug (a `private` note with
  no `player` visible to nobody) now fails if any scaffolded or
  wizard-created note is invisible to every principal.
- The projection's property test from phase 1 (M5c) passes unchanged
  over a **pack-declared corpus**: at least one owner-scoped and one
  gm-scoped resource per pack, so the test cannot pass vacuously —
  plus the demo campaign's expectation tables, still green in both
  directions.
- A non-GM account opening the player's view sees only what the
  projection allows — asserted note-for-note against the demo
  campaign's expectation tables, not by eyeballing the UI.
- Every opened note carries the visibility badge showing its
  *effective* visibility, and the inspector lists the rule chain that
  produced it. A badge that disagrees with the projection's answer
  for the same note and principal fails the suite: the badge reads
  the projection, it does not reimplement it.

### N7 — The rich editor's content support

**N7a** — block widgets and the registry: CodeMirror 6
`Decoration.replace`/`Decoration.widget` over the untouched buffer;
a plugin extension registry for structured blocks (a spell-slot block,
a stat line, a clock widget); the buffer is still the source, so the
byte-identical round-trip contract holds by construction.
**N7b** — the community formats: `ad-statblock` preserved verbatim
(prose a human reads), `statblock` YAML machine-readable for the
bestiary (ADR-0036); Obsidian callouts rendered; secrets
(`[!secret]`, `ad-secret`, `gm: true` options, marker blocks — per the
pack's `secrets:` vocabulary) stripped server-side for non-GM roles
with GM reveal controls (reveal to table / to a player), broadcast
live over the WebSocket with client refetch (ADR-0035).

**Done when**
- Every corpus fixture from phase 1 still round-trips byte-identically
  — the rich layer adds widgets, never a re-serialisation.
- A note with a `statblock` monster, an `ad-statblock` block, a
  `[!secret]` callout, an `ad-secret` block and a `gm: true` code
  option renders all five distinctly for the GM; the player projection
  of the same note contains **none** of the five texts — asserted by
  string search over the serialised projection, not by DOM inspection.
- A GM reveal (`secret.reveal`) reaches a connected player client as
  `secret.revealed` and the refetched note contains the block; a
  secret inside a GM-only note stays invisible however often it is
  revealed (the reveal implies note visibility per ADR-0035 — the
  test asserts the implied grant, not the leak).
- A third-party widget registered through the extension registry
  renders in the editor and writes back only through the buffer —
  `git diff` after open-and-save with no edits is empty.
- The projection's block stripping is covered by the N6 property test
  corpus: no `gm`-vocabulary block text appears in any non-GM
  projection.

### N8 — The archive import

**N8a** — the transport: `demitable import <url>` (CLI) and
`POST /api/v1/import` with `GET /api/v1/import/report` (REST). Flags:
`--vault`, `--new`, `--force`/`--skip`, `--include-plugins`,
`--no-plugins`, `--allow-http-import` (https-only by default).
**N8b** — the safety: extract to a temp dir, validate the whole tree,
then move. Path traversal, absolute paths and symlinks refused; entry
count, total size and compression ratio capped; `.demitable/` and
`.git/` rejected **and reported**; checksums verified before
extraction; private address ranges refused after resolution with
reconnect-to-resolved-address (DNS rebinding defence); size, time and
redirect caps; every import journalled and audit-logged. Plugin
bundles install only with the ADR-0027 warning — interactive requires
an explicit yes, non-interactive skips unless `--include-plugins`.

**Done when**
- The published SRD archive (N4) imports into an empty vault end to
  end: every note present, every link resolving, zero errors logged —
  this is the phase's third predicate, measured, not asserted.
- A hostile archive per case is refused with a named error and leaves
  the vault untouched: zip slip (`../`), absolute path, symlink,
  10,000-entry bomb, 100:1 compression ratio, `.demitable/players.yaml`
  smuggled inside, `.git/config` smuggled inside — seven archives,
  seven refusals, vault `git diff` empty after each.
- An import over HTTPS succeeds; the same URL over plain HTTP is
  refused unless `--allow-http-import` / the configured flag is set.
- A URL resolving first to a public address then to `127.0.0.1` is
  refused (rebinding test, no live network needed — a local resolver
  stub).
- Plugin bundles: interactive import prints the per-bundle capability
  list with the `dom.inline` paragraph and waits; non-interactive
  without `--include-plugins` skips them and says so in the report;
  `--no-plugins` refuses them outright.
- The exposure checklist's import rows hold (see §5): SSRF defence,
  https-only default, off-unless-enabled — each mapped to the test
  that proves it.

---

## 4. Sequencing

```
N1 (formula) ──┐
               ├──► N3 (pack format + linter) ──► N4 (5e + SRD) ──► N8 (import)
N2 (dice) ─────┘                                          │
                       N5 (Blades gate) ◄─────────────────┘
                             │
N3 ──► N6 (wizard + sheets + review) ──► N7 (rich editor)
```

- **N1 and N2 are independent** — the two languages share lexer
  foundations and safety rules but neither imports the other. One
  worker each, or one worker for both if slots are short.
- **N3 needs N1 and N2** — the pack declares formulas and dice, so the
  linter must be able to evaluate the former and name the latter's
  workflows.
- **N4 needs N3** — the 5e pack is the first real customer of the
  completed format.
- **N5 needs N3 and N4** — the gate is "N4's sibling passes without a
  core change", so N4's shape is the baseline it differs from. N5 must
  not move the core; if it must, the format failed and N3 reopens.
- **N6 needs N3, N4 and N5** — the wizard reads declarations (N3) and
  content notes (N4), and must drive both packs (N5) with no branch.
  N6d's player view needs the second principal, which is also N6.
- **N7 needs N3** (the `secrets:` vocabulary) and runs alongside N6 —
  the editor and the wizard touch different surfaces.
- **N8 needs N4** (the archive it installs) and the N3 linter (the
  `requires` check). It closes the phase.

**The critical path is N1/N2 → N3 → N4 → N8**, with N5, N6 and N7
alongside. N5 is the gate that can send N3 back; schedule it early
enough that a failure still leaves room for the format fix.

---

## 5. The exposure checklist, and who owns it

Phase 1's checklist stands; the import rows were phase 2's, and now
they are owned. New rows for the compute and roll endpoints are added,
because a server that evaluates expressions from packs — including
imported ones — has a sandbox as a security boundary.

| Check | Owner |
|---|---|
| Authentication is rate-limited and backed off on every auth endpoint | Phase 1 (M5) — re-verified by N6's second principal |
| **The session-code endpoint is rate-limited and counts attempts** | Phase 4 |
| **The GM secret is rotatable, and rotation invalidates outstanding sessions** | Phase 1 (M5) |
| **Every read of a GM-only note is written to the audit log** | Phase 1 (M5) — extended by N7's block stripping: a stripped block is not a read |
| **A guest seat sees its bound character, the live table, and nothing else** | Phase 4 |
| **A seat token grants nothing outside its session** | Phase 4 |
| Absolute URLs come from the configured public base URL, never the `Host` header | Phase 1 (M3) |
| Forwarded headers are trusted only from a configured proxy address | Phase 1 (M3) |
| The `Secure` cookie flag follows the configured public scheme | Phase 1 (M5) |
| CORS is off by default; the API is same-origin | Phase 1 (M3) |
| **A player's FTS query is filtered before BM25/IDF ranking** | Phase 1 (M1d/M6) — re-measured by N6 against the SRD vault, not the 21-note demo |
| **Markdown is sanitised before the editor renders it** | Phase 1 (M4) — re-verified by N7's widgets |
| The MCP endpoint is off by default, token-gated, and logs a warning if enabled | Phase 3 |
| **The import refuses a private address and refuses to reconnect elsewhere** | **N8** |
| **The import requires https and is off unless explicitly enabled** | **N8** |
| **The formula sandbox is paranoid: step/depth limits, hard timeout, no UDFs, no prototype access — including for imported packs** | **N1** (ADR-0025 tightened ADR-0011's trust) |
| **The dice engine has the same limits, and a roll never blocks the server** | **N2** |
| **Imported plugin bundles warn (capabilities + `dom.inline` paragraph) and never install silently** | **N8** |
| The app starts and works with the network interface down, and with DNS unavailable | Phase 1 (M0) — re-verified: everything except the import works offline |
| There is no automatic update path | Phase 1 (M0) |

---

## 6. What phase 1 taught this phase

Four findings from the phase-1 report arrive as requirements, not advice:

1. **The pack format's completion comes first** (report §"What phase 2
   should start with"). Phase 1's format carries content types, typed
   fields and a visibility default. N3 adds resources, formulas, dice
   and sheets — and the schema invariants in design §6.1 are enforced
   against two disagreeing packs, not asserted. That enforcement is
   N5.
2. **The projection gets its first real adversary.** The phase-1
   property test is strong on 21 demo notes and 36 synthetic ones, and
   the 1,693-note corpus carries none of the twelve reserved fields —
   so the corpus cannot exercise the model. N6's pack-declared corpus
   (owner-scoped and gm-scoped resources in both packs) is the missing
   adversary, and every N6/N7 projection claim runs against it.
3. **The search ranking is re-measured against a real audience.** The
   two-index partition was measured with 1,169 synthetic GM-only
   notes; the phase-1 index has 21 notes and one GM-only one. N6
   re-runs the ADR-0054 measurement against the SRD vault — and a
   top-result change, a position shift, or a score swing beyond the
   recorded bounds fails the suite.
4. **The second principal is real from N6 on.** Every list endpoint in
   phase 1 resolves against the GM. N6's accounts make the second
   principal real, and from that milestone every list endpoint is a
   place where the projection is applied to somebody other than the
   GM. The invisible-journal test (phase 1, finding 11) is the
   regression that proves it.

And the process lessons, carried forward: a test that cannot fail is
not a test (every perf predicate fails when the perf is missed);
measure the criterion's subject, not a proxy (replacement timing is
not insertion timing); read the diff — the workers' reports are
evidence, not proof.

---

## 7. Open items

- The formula language's exact builtin set beyond ADR-0003's fourteen —
  N1 settles it, and the linter is where the decision lands.
- Whether `/compute` needs rate limits beyond auth — N6 measures a
  worst-case preview storm (one keystroke per 100 ms per player) and
  decides.
- Whether the Blades pack needs a third entity shape beyond character
  and crew — N5 decides, and the answer is a pack declaration, not a
  core change.
- The `.base` starter story phase 1 deferred — N4's published schema
  either produces a faithful one or continues to decline to.
- What the demo campaign becomes once two packs exist — N6 decides
  whether it gains a Blades wing or stays 5e with the Blades corpus
  alongside it.
- **The full spike design is a phase 3 target, not phase 2 scope.**
  Phase 2 takes what it needs from `spikes/ui/` — the player's view,
  the visibility badge and the inspector (N6d), the sheets (N6b), the
  rich editor content (N7). The GM dashboard and whatever else
  remains land with phase 3 (see
  [`phase-plan.md`](phase-plan.md) §Phase 3), because a dashboard
  over live content wants the query language that phase 3 builds
  (ADR-0047).
- The macOS watcher numbers phase 1 could not take — still needing a
  Mac, still owned by nobody in this phase, recorded here so they are
  not forgotten.
