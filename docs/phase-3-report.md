# Phase 3 — The Openings: what it found

> Six milestones, fourteen sub-milestones. The phase opened the app's
> boundaries: third parties extend it (O1), programs query it in a fixed
> language over REST and MCP (O2, O3), and its UI isolation holds in every
> claimed browser (O4) with the full surfaces built on top (O5).
>
> It holds itself to the same standard as the two reports before it:
> **a phase that found nothing is a phase that did not test anything.**

---

## The phase in one paragraph

The campaign wiki grew three boundaries that hold. A stranger's WASM
module installs through the import path, reads through a granted
capability, and is refused with error JSON on anything undeclared — with
per-role memory so the GM's state never reaches the player's instance.
One fixed predicate language compiles to SQL and serves both REST and
MCP, with opaque cursors, returned-not-matched counts, and an oracle test
proving a hidden note moves nothing a player can see. The MCP endpoint is
off by default, token-gated, read-only, and says the privacy warning out
loud. The shadow host holds in Chromium and Firefox. The dashboard renders
from a single query inside a measured budget, and every surface that shows
visibility disagrees with the projection at its peril.

The phase found **eleven things**, three of them blocking defects no
worker reported — a SQL-injection/error-leak via `?sort=`, a nil-pointer
crash on dispatch-after-disable, and WASM writes bypassing the journal.
All three were found by reading the diff and running the code.

---

## The six milestones

| | Milestone | The number that says it works |
|---|---|---|
| **O0** | trust-boundary hardening + phase-2 close-out | non-GM fetch of GM-only note → **404, 0 bytes**; Validate pins 14 N3 cases; `requires.app` refusal; partition **3 / 16 / 103.96%** in CI |
| **O1** | WASM tier, full host ABI | per-function grant tests + negatives; GM/player isolation; disable→error JSON, journalled+revertible writes; golden ABI green; `CGO_ENABLED=0` ×5 targets |
| **O2** | fixed predicate language, REST + MCP | 13 grammar shapes + malformed→`ErrParse` (never SQL text); one SQL implementation + equivalence; cursor round-trip + forgery rejection; oracle set+order+count unchanged |
| **O3** | compiled-in MCP plugin | off→unreachable; no/wrong token→401; 4 resources + 6 tools measured; write refused with named error; prompt pinned; protocol `2025-06-18` |
| **O4** | cross-browser harness | **15/15** Chromium, **15/15** Firefox; R2 per-browser helper; WebKit named gap, no ADR needed |
| **O5** | GM dashboard + remainder | 1 request, **0** client vault reads, p95 **80–82 ms** vs **250 ms** budget; disagree-fails per surface; survey complete, nothing dropped |

`make lint` clean, `go test ./...` **23 packages ok, 0 FAIL** on the merged
main. No `fixtures/` tracked (repo-root corpus absent from the index; the
two `*fixtures*` paths are the O4 e2e page and the pre-existing extractor
test). No unexplained `dev/srd-vault` churn. No note content or credential
in any new log line.

---

## The findings

### 1. The sort parameter was a SQL-injection and error-oracle vector

O2 interpolated `?sort=` into `ORDER BY` unvalidated, and REST forwarded
`err.Error()` verbatim — so `Sort: "'"` returned SQLite error text in a
400, and `ORDER BY`-expression control executed. `c.Path` had a whitelist
(`validPath`); `sortField` did not. Now it does, and every
query-construction failure maps to the named `ParseError`. The hostile
test (`'`, `uid') OR ('1'=='1`, `DROP TABLE`) proves the rejection names
the parse error, never SQLite.

This is the phase-0 lesson verbatim: the leak was in the accidental
class — nobody designed `?sort=` to be a query surface, so nobody
whitelisted it.

### 2. Two predicate implementations shipped against a one-implementation predicate

O2 kept the SQL compiler for REST and the in-memory `EvalExpr` for every
other caller, with `search` bypassing both via renamed `FTS/FTSIn` — and
no equivalence test. The Done-when allows two readings ("delete the copy"
or "equivalence test"); the fix took the second: `TestQueryEquivalence…`
runs both over the 1,690-note SRD corpus across 8 predicates plus
sort/desc variants, and dies when either side breaks (proven by mutation
both ways). The in-memory path stays because routing `Vault.List`
through SQL would break the 100 ms filter/sort criterion (20 ms → 250 ms)
— recorded, not hidden.

### 3. Dispatch-after-disable panicked

O1's `Instance.Close` nilled `i.mod` while `Invoke` dereferenced it
unconditionally and `Dispatch` never checked. Enable → hold handle →
`Disable` → `Dispatch` → nil-pointer panic: a crash on the exact lifecycle
path the predicate covers, in a sandbox whose contract is "error JSON,
never a crash". A `lifeMu`-guarded closed check plus regression test
closed it. Found by the investigator, not the worker — phase-2's hardest
lesson, still true.

### 4. WASM writes bypassed the journal

`hostVaultWrite` went through `Vault.UpdateNote` (`os.WriteFile` direct),
so the predicate's "journalled and revertible" was described, not true.
The fix routes WASM writes through `WriteNoteJournaled` (rollback on
journal failure), and the test asserts the journal entry plus revert.
The in-memory `Store` honestly returns `unsupported` — it has no journal,
so journaling there is impossible, not unimplemented. The distinction is
now in the code, not in a report.

### 5. The SSRF test could not detect a gutted guard

O1's first SSRF test asserted only refusal — and a removed guard still
refuses (the dial fails). A test that passes with the guard deleted is
not a test. The strengthened version injects a `fakeTransport` that would
return 200 if dialed and asserts the guard's error code, so guard-removal
fails loudly. Phase-2's dominant defect shape (tests shaped like the
predicate that assert nothing), caught twice in one phase.

### 6. The ADR-0054 record understated its own maximum

The committed suite measured max swing **103.96%**; ADR-0054 §5 recorded
"up to 100.6%" — a 3.36pp gap from a counting-definition difference (max
over all overlapping hits vs the top hit's swing), plus a 3-vs-4 count
depending on whether the empty-player `goblin` query counts. **ADR-0061**
amends §5 with the corrected value and the stated convention. The bounds
(3 / 16 / 105.0) hold.

### 7. `get_links` reimplemented the link index

O3's first `get_links` re-parsed bodies and substring-matched backlinks
instead of reading the indexed `n.Links` — divergent targets (raw display
text, not uids), false-positive-prone backlinks, a private parser that
would rot beside `resolveWikiLinks`. Now index-backed both directions,
pinned by the measured-values test. Residual, honestly documented: the
index genuinely lacks wikilinks (goldmark splits `[[` across text nodes,
so the extractor never sees them) — fixing that is an extractor change,
out of scope, recorded at `mcp.go:637-642`.

### 8. The MCP `list_by` pin is value-level, not path-level

The measured-values test asserts UID-slice equality between the tool and
`Store.Query` on one single-result query — a forked private copy filtered
by type only passes it. Production code uses `ListPage`→Store, but the
test doesn't prove the path. Downgraded the claim rather than overclaiming;
a multi-result adversarial-equivalence assertion is the follow-up.

### 9. Counts and badges disagreed on raw data

O5's `effectiveVis()` silently defaulted on raw data for counts while
`surfaceVisibility()` threw for display — fail-open and fail-closed in one
module. Counts now use the throwing path; the unused fallback is deleted.
Same fix shape as finding 7: one discipline, not two.

### 10. Unknown visibility rendered a nonsense label while denying as `gm`

`Visible()` failed unknown values closed, but the effective-visibility
badge path passed them through verbatim. Now both normalize unknown → `gm`
— strictly narrowing, no expectation table broke.

### 11. The version was never stamped, so `requires.app` always saw `"dev"`

Both import call sites plumbed `AppVersion: version`, but `version`
defaulted to `"dev"` and no `-ldflags -X` ever set it — so every gated
import through a real binary fail-closed. The Makefile `build` and
`release` targets now stamp `-X main.version=$(VERSION)` (`?= 1.0.0`).
Fail-closed was correct; inert-by-default was not, and the distinction
matters for the first real gated archive.

---

## What was measured

| | measured | criterion |
|---|---|---|
| non-GM fetch of GM-only note | **404, 0 body bytes**, per principal (GM/owner/party/anonymous) | 404 + pins |
| Validate, 14 N3 cases via struct entry | **fail naming invariant** | same 14 |
| `requires.app` refusal | **names `requires.app`**, version plumbed + stamped (`1.0.0`) | named refusal |
| partition (SRD vault, CI) | **3 / 16 / 103.96%** vs bounds 3/16/105.0 | recorded bounds |
| grammar shapes over SRD vault | **13 shapes + malformed → `ErrParse`**, no SQL text | ADR-0047 examples |
| oracle (hidden note + rare term) | **set + order + count unchanged** (player 6 uids; GM 10) | unchanged |
| cursor (page, insert, forge, tamper) | **no dup/omission; forgery → `ErrCursor`; Count==len(returned)==3/7** | round-trip |
| MCP posture | **off→404; no/wrong token→401; write→named refusal** | committed |
| MCP surface (demo vault) | **4 resources + 6 tools**, `max_hp:13` live N1, `list_by count:1` | measured values |
| shadow matrix | **Chromium 15/15, Firefox 15/15** (WebKit EGL gap named) | 3 browsers or ADR |
| dashboard (SRD vault, full request set, p95/5) | **80.09 ms** (server probe 82.13) vs **250 ms** budget; **1 request, 0 vault reads** | budget + zero reads |
| full suite | **`make lint` 0; `go test ./...` 23 ok, 0 FAIL** | green |

---

## The ADRs landed

| | |
|---|---|
| **ADR-0061** | The ADR-0054 §5 measurement, corrected — max swing 103.96%, count 3 (amends 0054; evidence: two independent runs + CI suite) |

No other ADR was amended or needed: the R2/Firefox divergence resolved in
the shell per 0029's own trigger (no amendment required), `host_vault_assets`
landed as a pure addition under the `api: 1` freeze, and the golden ABI
module proves the freeze held.

---

## What was cut, deferred, or left undone

- **WebKit column** (O4): environmental EGL gap, not a behavioral red — no
  ADR filed (filing one for an infra gap with zero behavioral evidence
  would be wrong). Needs a GL-capable runner; the harness is unchanged.
- **IME / clipboard / `::part` / R3 Tab-trap / R7 / R10 / CodeMirror-in-shadow**
  (O4): recorded-deferred with owners in `docs/cross-browser-results.md`,
  not silently dropped.
- **MCP write tools** ("make me an NPC"): explicitly out (ADR-0014,
  read-only contract). First request reopens the list, not the milestone.
- **O5 remainder**: CharacterPanel (phase-4 owner), full MarkdownView
  renderer (phase 4), Icon (cut) — in `SURVEY.md`, canonical.
- **Spike `lib/` non-port** (esp. client-side `projection.ts`): deliberately
  not ported — the server owns projection. Recorded in SURVEY.
- **`LoadWasmFromDir` / `Host.LoadPluginsFromVault`** have no `cmd/` caller:
  the loader is tested, the wiring is honestly reported as a gap, not
  claimed as E2E.
- **Wikilink indexing** for `get_links`: extractor change, out of scope,
  documented at the call site.
- **Equivalence pin strength** (finding 8) and **`QueryResult` field
  redundancies**: left additive, follow-ups.
- **macOS watcher numbers**: still need a Mac. Still nobody's in this
  phase. Recorded for the third report running.

---

## What phase 4 should start with

**The table needs the query path O5 now leans on.** The dashboard costs
one full-scan SQL call (~65–80 ms/1,690 notes, no SQL LIMIT — recorded in
`query.go:162-167`); a live table issuing per-token, per-frame, or
per-keystroke queries through the same path will blow any interactivity
budget. Phase 4's first measurement should be the session-tier read/write
cadence against that cost, before the encounter runner assumes a query
shape.

Second, **the projection gets its third adversary.** Phase 2's corpus
covered note visibility; phase 3's oracle covered the query path. Phase 4's
live surfaces — session codes, seats, scratch log, shared display — are a
new principal space (threat-model §9.7, still partial), and the seat-token
rate limiting plus GM-secret rotation phase 1 deferred are still open.

Third, **close the WebKit column before building phase-4 surfaces on the
shadow host.** O4's trigger stays armed; a Safari divergence found after
the tabletop is built costs more than one found now.

---

## What I would do differently

- **Put the hostile test in the first brief, not the fix brief.** The
  sort-injection, the SSRF guard, the oracle — every one arrived as a
  verification finding against code that "passed". A brief that ships the
  exact hostile input with the task gets the test with the code.
- **Say "prove the path, not the value" for equivalence.** The `list_by`
  pin asserts equal outputs on one query; a fork passes it. Equivalence
  needs an adversarial corpus and a path-level assertion, stated up front.
- **Treat "documented as a gap" as a smell after the second use.** The
  wikilink index gap and the unwired loader are both honest — and both
  would have been cheaper as tasks than as documentation. Two honest gaps
  are a pattern, not a coincidence.
- **Same harness lesson as phase 1, quieter this time:** the review found
  what the workers didn't — the panic, the journal bypass, the injection,
  the vacuous tests. None were in a worker's report. The verification
  standard (cold runs + mutation probes + seam re-checks, stated up front
  this time instead of discovered after) is what found them. Keep it.

---

## O5 milestone report (preserved)

> The following is the O5 worker's milestone report, kept as the record
> for the dashboard and surfaces. Phase-level numbers above supersede it
> where they differ.

### Summary

Milestone O5 lands the GM dashboard and the remainder of the `spikes/ui/`
surfaces that fit phase 3, with the projection's consequences carried into
the UI. The dashboard loads against the SRD vault through a single O2
query (`GET /api/v1/notes`) — no client vault reads — and renders inside
a stated budget. Landed surfaces (dashboard, note editor, player view,
visibility badge/inspector) all carry the disagree-fails discipline; the
character panel and full markdown construct renderer are deferred to
phase 4 with reasons recorded in `SURVEY.md`.

### Survey checklist

See `SURVEY.md` (canonical). Landed: GmDashboard, NoteEditor, PlayerView,
VisibilityBadge, VisibilityInspector. Deferred: CharacterPanel (phase 4
owner), MarkdownView (partially landed; full renderer phase 4), Icon (cut).

### Predicates

- Dashboard render budget: **250 ms** (stated). Measured full request set
  against the SRD vault (1,690 notes, GM principal, 5 runs) — a *manual*
  measurement (server-side probe, synthetic 1690-note vault), not a CI
  assertion:
  80.09, 72.23, 65.22, 65.27, 66.64 ms; p95 (max of 5) = **80.09 ms**.
  Budget met with margin. The landed vitest suite enforces the
  single-request mechanism and budget plumbing; the 80 ms figure is manual.
  The client test (`web/src/lib/dashboard.test.ts`) fails when the budget is
  lowered below the measured time.
- Zero client vault reads: asserted by intercepting the request set in
  `loadDashboard` (`assertZeroVaultReads`); the test fails if a per-note
  (`/api/v1/notes/:uid`) or render request is made. Result: **0 vault
  reads** in the dashboard load path; the only request is `/api/v1/notes`.
- Disagree-fails: `surfaceVisibility()` requires a server-projected
  `effective_visibility` and throws on raw data; `assertVisibilityAgreement()`
  throws on mismatch. Covered per surface showing visibility (dashboard,
  player view) by `web/src/lib/dashboard.test.ts`.
- Go: `make lint` + `make test` green. No `fixtures/` in the diff.

### What is now known

- A single full-scan O2 query (~65-80 ms over 1,690 SRD notes) is cheap
  enough to be the dashboard's only request; an N+1 storm is unnecessary
  and would blow any reasonable budget.
- Exposing `effective_visibility` + `defaulted` on `NoteView` lets client
  surfaces display the projection without recomputing it.

### Files changed

- `internal/vault/view.go`, `internal/vault/visible.go` — add
  `EffectiveVisibility`/`Defaulted` to `NoteView` and populate them.
- `web/src/lib/api.ts` — `NoteSummary` carries projected visibility.
- `web/src/lib/dashboard.ts` — dashboard data layer, budget, vault-read
  guard, projection discipline helpers.
- `web/src/lib/dashboard.test.ts` — budget/vault-read/disagree-fails tests.
- `web/src/surfaces/GmDashboard.svelte`, `web/src/surfaces/PlayerView.svelte`.
- `web/src/App.svelte` — surface switching.
- `SURVEY.md`, `docs/phase-3-report.md`.

### Contradictions

None found.

### Not done + why

- Character panel and full markdown construct renderer deferred to phase 4
  (reasons in `SURVEY.md`).

### Unverified

- Cross-browser rendering of the new surfaces: **Unverified**. The O4
  harness exists, but running it against the new surfaces is a follow-up,
  not this milestone.
