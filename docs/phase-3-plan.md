# Phase 3 Plan — The Openings

> **Goal:** a program whose boundaries with other programs are real — third
> parties extend it without rebuilding it, programs query it in a fixed
> language over REST or MCP, and its UI isolation holds in every browser it
> claims.
>
> **Phase 3 in one line:** *what extends it, what asks it, what reads it.*
>
> **Definition of done:** a third party can write a WASM plugin that installs
> without rebuilding the app; a program can query the vault with the fixed
> predicate language over REST or MCP; and the shadow host is verified in
> Firefox and Safari as well as Chromium.
>
> **And one thing it does not claim.** A plugin's UI is still trusted, not
> sandboxed (ADR-0029) — the WASM tier sandboxes *logic*, and the import
> warning says the UI part in plain words. A more isolated UI is an
> out-of-process tier, which ADR-0020 records and this phase deliberately
> does not build.

Phase membership and what moved: [`phase-plan.md`](phase-plan.md).

---

## 1. What this phase costs

**Six milestones, fourteen sub-milestones.** Estimates are rough — phase 2
taught that a number without the artifact it was measured on is a wish, so
these are ranges against the phase-2 machine, not commitments.

| Block | Sub-milestones | Full-time | Part-time (≈10 h/week) |
|---|---|---|---|
| O0 — trust-boundary hardening, phase-2 close-out | 2 | 1–2 wks | 2–4 weeks |
| O1 — WASM plugin tier, full host ABI | 3 | 3–4 wks | 2–3 months |
| O2 — query language, shared by REST and MCP | 2 | 2–3 wks | 1–2 months |
| O3 — MCP plugin | 2 | 2–3 wks | 1–2 months |
| O4 — cross-browser harness | 2 | 1–2 wks | 1 month |
| O5 — full UI surfaces (GM dashboard + remainder) | 3 | 3–4 wks | 2–3 months |
| **Phase 3 total** | **14** | **~12–18 weeks** | **~7–11 months** |

**The cheapest saving is already banked.** Phase 2 proved the pack format
against two disagreeing systems, the projection against two pack-declared
corpora, and the import against seven hostile archives. Phase 3 builds *on*
those suites rather than re-proving them — but every milestone that touches
the projection, the write path, or the sandbox re-runs them, because both
earlier reports show the defects live in exactly those seams.

---

## 2. What is in this phase, and what is not

**In:** the O0 hardening (item-endpoint principal checks, `Validate(*Pack)`
invariant gap, `requires.app` enforcement, the ADR-0054 re-measurement) ·
the WASM tier on wazero with the full host ABI and the golden freeze test ·
the fixed predicate language compiling to SQL, with opaque cursors and the
two-index partition measured · the MCP plugin (token-gated, off by default,
with the privacy prompt) · the Playwright Firefox+Safari shadow-host matrix ·
the GM dashboard and the remainder of `spikes/ui/`, over the query language.

**Out, deliberately:**

| Out | Where |
|---|---|
| Live sessions, seats, session codes, the scratch log | Phase 4 |
| The encounter runner, the tabletop, chat, the shared display | Phase 4 |
| The session write-back contract | Phase 4 |
| Recording, transcription, session logs, music | Phase 5 |
| An out-of-process plugin tier | Recorded in ADR-0020, deliberately unbuilt |
| MCP write tools ("make me an NPC") | After phase 3 — read-only is the phase-3 contract (ADR-0014) |
| A native-client SDK | Not implied by any contract (api.md §9a) |

---

## 3. Milestones

### O0 — Trust-boundary hardening and phase-2 close-out

**O0a** — principal checks on item endpoints: `GET /api/v1/notes/:uid` and
`GET /api/v1/notes/:uid/render` resolve against the requester (GM sees raw;
a non-GM receives the projection; hidden notes 404). The pre-existing hole
phase 2 found twice and left explicitly out of scope.
**O0b** — close-out: `Validate(*Pack)` enforces the invariants and dice-ref
checks on every entry path (the N3 nit); the import enforces
`requires.app` with a named refusal (the N8 dead-metadata gap); the
ADR-0054 measurement re-runs against the SRD vault with the recorded bounds
as a failing suite (phase-2 plan §6 item 3 — N6 never proved it).

**Done when**
- A non-GM fetch of a GM-only note by uid returns 404 with zero body bytes;
  a party-visible note returns the projected body, never raw; a committed
  test per principal (GM, owner, party, anonymous) pins each, and the
  pre-existing raw-behavior tests are updated, not deleted.
- `Validate(*Pack)` on a hand-built pack with no content types, no
  difficulty model, or a dangling dice ref fails naming the invariant —
  the same 14 N3 cases, through the struct entry point.
- An archive whose manifest requires a newer app is refused naming
  `requires.app`; a committed test pins it.
- The ADR-0054 measurement (top-result changes, max position shift, max
  score swing vs the SRD vault, GM vs player index) runs in CI with the
  recorded bounds, and exceeding any bound fails the suite.

### O1 — The WASM plugin tier and the full host ABI

**O1a** — the runtime: wazero, pure Go, no cgo; the Extism-shaped entry
point (`demitable_call` taking and returning JSON); instantiation per
(role, session) with separate memory and no shared state across roles.
**O1b** — the host functions, one vocabulary (ADR-0045, as amended by
ADR-0056): `host_vault_read`, `host_vault_write`, `host_vault_query`,
`host_vault_assets` (the capability phase-1 finding 13 proved missing),
`host_http_fetch` (granted-only), `host_log`; an undeclared capability
call returns error JSON, never a crash; no WASI filesystem preopens; no
sockets.
**O1c** — the lifecycle: install at runtime through the N8 archive path
with the ADR-0027 warning; disable without restart (surfaces unmount,
instances shut down, writes stay in the journal and revertible);
`api: 1` additive-only — the golden ABI module (imports every host
function, built at compile time) fails CI if any signature moves.

**Done when**
- A third-party WASM module built outside this repository (any
  Extism-PDK or raw `wat`) installs via the import path, reads through a
  granted `vault.read`, and is refused with error JSON on an undeclared
  `vault.write` — committed tests, one per host function, plus the
  negative.
- Two instances of one plugin (GM + player) share no observable state: a
  committed test writes GM-only state and proves the player instance
  cannot read it (the side-channel the per-role rule exists to close).
- Disabling a plugin mid-session unmounts its surfaces and shuts its
  instances; its prior writes remain journalled and revertible — asserted,
  not described.
- The golden ABI module instantiates green; `CGO_ENABLED=0` matrix still
  clean; no new runtime dependency; `api: 1` diff shows additions only
  (new host functions), never a changed signature.

### O2 — The query language, shared by REST and MCP

**O2a** — the language (ADR-0047): the fixed grammar (`==`, `!=`, `>`,
`>=`, `<`, `<=`, `contains`, `in`, `and`, `or`, `not`, parentheses) over
the universal fields plus `data.<path>`; one implementation compiling to
SQL over the SQLite `notes` cache; the two pre-existing `SearchFTSIn`
spellings (phase-1 finding 5) deleted in favour of it, with callers
migrated.
**O2b** — the guarantees: opaque signed cursors (not offsets; tampered
cursors rejected); counts reflect returned-not-matched (a GM-only match
never shifts a player-visible count — the oracle test); the two-index
partition measured per ADR-0054 against the SRD vault, bounds committed.

**Done when**
- Every ADR-0047 example query parses and returns the documented rows
  against the committed SRD vault; a malformed query fails with a named
  parse error, never with SQL text.
- `GET /api/v1/notes` filters and MCP `list_by` call the same compiled
  implementation — asserted by deleting one call site's private copy (there
  is only one) or by an equivalence test over a shared corpus.
- A cursor from page one fetches page two after an intervening insert
  without duplication or omission; a forged cursor is rejected; neither
  leaks position information (no total-matched anywhere a hidden note
  could shift it).
- The oracle test: adding a GM-only note containing a player's rare term
  changes neither the player's result set, order, nor count.

### O3 — The MCP plugin

**O3a** — the surface (ADR-0014): `/mcp` on streamable HTTP; the four
resources (`note`, `character` with computed formulas via the N1 engine,
`pack`, `campaign`) and the six read-only tools (`search_notes`,
`get_character`, `list_by` via O2, `get_links`, `resolve_reference`,
`evaluate_formula`); read-only — a write attempt is refused, with a
committed test.
**O3b** — the posture: off by default (absent flag/config means no route);
token-gated (no token refused, wrong token refused, GM-token-equivalent
scope stated); never binds publicly by default; enabling prints the
privacy prompt (local-by-default, never silently remote — the campaign
leaves the machine when a remote model is connected); MCP protocol
version pinned and recorded.

**Done when**
- With the endpoint off, `/mcp` is unreachable (connection refused or
  404 — committed test, not config description); with it on and no token,
  every call is refused; with a token, all four resources and six tools
  answer against the committed vault with measured values.
- `evaluate_formula` evaluates through the N1 engine with the ADR-0042
  scope (a committed test with a call-scope binding proves it is not a
  reimplementation).
- A write-shaped call is refused with a named error; the refusal is
  committed, not documented.
- The enable path emits the privacy prompt text and journals the
  enablement; the prompt text is pinned by a test.

### O4 — The cross-browser harness

**O4a** — the matrix: Playwright over Chromium, Firefox and Safari
(WebKit) running the ADR-0029 verification suite (keyboard, IME,
clipboard, native drag-and-drop both directions with `isTrusted` events,
`getSelection` retargeting, `adoptedStyleSheets`, `::part`, dialog
focus restore, CodeMirror-in-shadow-root).
**O4b** — the verdict: every divergence is either fixed in the shell or
escalated as an ADR amending ADR-0029 (the decision says to revisit if
browsers differ — that trigger is this milestone's purpose); rule 2
(selection path) works in all three or is reimplemented per browser with
per-browser tests.

**Done when**
- The matrix runs in CI (or, where browsers cannot run in CI, the exact
  command plus the recorded results table, with the gap named) — green in
  three browsers, or red with an ADR number attached.
- Any ADR-0029 rule that does not hold in Firefox or Safari has a
  per-browser implementation with a per-browser test, or the milestone
  stops and the amendment is written — "works in Chromium" is not the
  predicate.

### O5 — The full UI surfaces

**O5a** — the survey: every surface in `spikes/ui/` (ADR-0052) is either
landed or recorded-deferred with a reason; the survey itself is
committed (a checklist in the milestone report, not a second spike).
**O5b** — the GM dashboard: campaign health over live content (counts,
members, recent sessions, content coverage) rendered exclusively through
O2 queries — no direct vault reads from any client surface; no N+1
query storm (dashboard render budget stated and measured against the SRD
vault — exceeding it fails the suite).
**O5c** — the remainder: whatever the survey lands besides the
dashboard, each with the N6d treatment (badge-grade effective-visibility
discipline: every new surface reads the projection, never reimplements
it).

**Done when**
- The dashboard loads against the committed SRD vault with zero client
  vault reads (asserted by intercepting the client's request set, not by
  reading its source) inside the stated render budget.
- Every landed surface carries the disagree-fails discipline where it
  shows visibility: a surface contradicting the projection for the same
  note and principal fails the suite.
- The deferred list names each un-landed spike surface and its reason
  (phase 4 owner, or cut); nothing is silently dropped.

---

## 4. Sequencing

```
O0 (hardening) ──► O2 (query language) ──┬──► O3 (MCP plugin)
                                         └──► O5 (dashboard + surfaces)
O1 (WASM tier) ──► O3 (a WASM MCP transport is NOT implied — O3 is a
                        compiled-in plugin; O1 proves third parties work)
O4 (browsers) independent, early (a revisit reopens shadow-host code
                        O5 builds on)
```

- **O0 first.** Hardening before new surfaces: O2–O5 all serve projections
  to non-GM callers, and the item-endpoint hole sits on that exact path.
- **O1 and O4 run alongside O0** — three independent surfaces, three slots.
  O1 touches the sandbox; O4 touches the shell; neither touches the other.
- **O2 needs O0** (the oracle/count guarantees assume the hardened
  endpoints) and unblocks O3 and O5.
- **O3 needs O1 and O2** — the host proves third-party code runs, the
  language proves `list_by` is one implementation. O3 itself stays
  compiled-in: a WASM-distributed MCP is not implied.
- **O5 needs O2 and O4** — queries for data, browsers for surfaces.

**The critical path is O0 → O2 → O3/O5**, with O1 and O4 alongside. O4 is
scheduled early enough that a revisit still leaves room for the
shadow-host fix.

---

## 5. The exposure checklist, and who owns it

Phase 2's checklist stands (phase-2 plan §5); the import rows are closed by
N8's seven hostile archives. New rows for the surfaces this phase opens:

| Check | Owner |
|---|---|
| Authentication is rate-limited and backed off on every auth endpoint | Phase 1 (M5) — re-verified by O0's per-principal item tests |
| **The session-code endpoint is rate-limited and counts attempts** | Phase 4 |
| **The GM secret is rotatable, and rotation invalidates outstanding sessions** | Phase 1 (M5) |
| **Every read of a GM-only note is written to the audit log** | Phase 1 (M5) — extended by O0 (item endpoints now read-checked) |
| **A guest seat sees its bound character, the live table, and nothing else** | Phase 4 |
| **A seat token grants nothing outside its session** | Phase 4 |
| Absolute URLs come from the configured public base URL, never the `Host` header | Phase 1 (M3) |
| Forwarded headers are trusted only from a configured proxy address | Phase 1 (M3) |
| The `Secure` cookie flag follows the configured public scheme | Phase 1 (M5) |
| CORS is off by default; the API is same-origin | Phase 1 (M3) |
| **A player's FTS query is filtered before BM25/IDF ranking** | **O2** (measured vs the SRD vault, bounds committed) |
| **Markdown is sanitised before the editor renders it** | Phase 1 (M4) — re-verified by O4 (three browsers, one sanitiser) |
| The MCP endpoint is off by default, token-gated, and logs a warning if enabled | **O3** (plus the privacy prompt, pinned) |
| **The import refuses a private address and refuses to reconnect elsewhere** | Phase 2 (N8) — closed |
| **The import requires https and is off unless explicitly enabled** | Phase 2 (N8) — closed |
| **The formula sandbox is paranoid: step/depth limits, hard timeout, no UDFs, no prototype access — including for imported packs** | Phase 2 (N1) — re-run by O1 (WASM callers use the same engine) |
| **The dice engine has the same limits, and a roll never blocks the server** | Phase 2 (N2) |
| **Imported plugin bundles warn (capabilities + `dom.inline` paragraph) and never install silently** | Phase 2 (N8) — **exercised for real by O1** (a WASM bundle installs through it) |
| **A WASM plugin's undeclared capability call returns error JSON, never a crash; per-role instances share no state** | **O1** |
| **The MCP token buys the GM's full view and nothing else; a missing token buys nothing** | **O3** |
| **O2 counts never reveal hidden matches; cursors never leak position** | **O2** |
| The app starts and works with the network interface down, and with DNS unavailable | Phase 1 (M0) — re-verified: everything except the import works offline |
| There is no automatic update path | Phase 1 (M0) |

---

## 6. What phase 2 taught this phase

Four findings arrive as requirements, not advice:

1. **A test that cannot fail is not a test** (phase-2 report, all three
   waves). Every performance predicate in this phase states its budget and
   fails when the budget is missed; every security predicate names the
   hostile input it refuses. Mutation-probing — break it in `/tmp`, watch
   the suite fail — is the verification standard that worked, and
   investigation workers apply it before reporting green.
2. **The merger owns the seam collisions.** N6 and N7 each added a `Load`
   with different semantics and the merge collided; the union (ID-scan
   with stem-alias, `ListIDs` returning parsed IDs) was a judgement call.
   This phase's briefs settle loader ownership up front: the pack package
   owns loading, and only one milestone per wave extends it.
3. **Hedging is not engineering.** N5's three duplicate pack files bought
   nothing and cost a trim round. One canonical artifact per deliverable —
   a hedge is a review finding, not a strategy.
4. **The projection gets its second adversary.** Phase 2's corpus covers
   note visibility and block stripping. Phase 3's adversary is the
   *query* path: counts, cursors, and rankings are new oracles, and O2's
   oracle test plus the re-measured partition are the missing adversary.

And the standing lessons: measure the criterion's subject, not a proxy;
read the diff — workers' reports are evidence, not proof.

---

## 7. Open items

- The macOS watcher numbers phase 1 could not take — still needing a Mac,
  still owned by nobody in this phase, recorded here so they are not
  forgotten twice.
- The discovery granularity phase 0 left behind (`visited` vs `heard of`
  as one flat fact) — still open, still not this phase's to close.
- Whether `requires.app` needs version-range semantics or exact-match —
  O0 enforces refusal; the range language is undecided.
- Whether the dashboard needs a third entity shape beyond the N5 answer —
  O5 decides, and the answer is a pack declaration, not a core change.
- What the demo campaign becomes once dashboards query it — O5 decides
  whether it gains a dashboard wing or stays the projection's yardstick.
- The out-of-process plugin tier (ADR-0020 neutral) and MCP write tools
  (ADR-0014) stay explicitly out; the first request for either reopens
  this list, not the milestones.
