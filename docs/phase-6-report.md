# Phase 6 — The Backlog: what it found

> Six milestones, twelve sub-milestones, all landed. The phase closed the
> contract: un-reveal with stated semantics, per-player reveals, the
> inspector's server-side source, machine-readable campaign fields, the live
> session contract re-emitted, a generated OpenAPI document, and the trust
> batch with hostile tests or recorded acceptances.
>
> It holds itself to the same standard as the five reports before it:
> **a phase that found nothing is a phase that did not test anything.**

---

## The phase in one paragraph

A GM can now conceal a revealed secret again — per audience, journalled,
with the already-broadcast limit stated rather than implied; reveal to named
players instead of only the table; ask any note why a principal may see it
from a server endpoint that leaks zero GM-only bytes; serve campaign members,
packs, and home machine-readably under a decided audience; run the live table
on server-emitted state and patch frames with join codes minted single-use;
and generate the OpenAPI document from the handler table so drift breaks the
build. The docs now describe the shipped behavior instead of the backlog.

The phase found **nine things**, three of them blocking defects no worker
reported — a `target:"table"` widening introduced by the R1 refactor itself,
an R4 delivery with working code and zero committed tests for two of four
predicates, and a docs-truth gap (SURVEY + api.md asserting "no server
source" for shipped behavior) of the exact class that failed the phase-5
gate. All three were found by reading the diff and running the code. None
was in a worker's report.

---

## The six milestones

| | Milestone | The number that says it works |
|---|---|---|
| **R1** | un-reveal + per-player reveal UI | 9/9 hostile tests (8 + pin); revoked refetch raw-bytes clean |
| **R2** | inspector constraint-verdicts source | 3/3 tests; non-GM bytes zero GM-only text (raw-bytes) |
| **R3** | campaign fields under decided audience | 7/7 per-principal tests; no other field widened |
| **R4** | live contract + OpenAPI | 4 predicates; 187 generated paths; drift breaks build |
| **R5** | trust remainders | display/cue hostile 401s; constant-time + D3 accepted; e2e 45/45 |
| **R6** | open decisions disposal | 5/5 decided with cited evidence; SURVEY/api catch-up flipped |

`make lint` clean; `go test ./...` all packages ok, 0 FAIL. No `fixtures/`
in the phase diff. No note content, segment text, message body, or
credential in any new log line. Two ADRs landed by the orchestrator
(0065, 0066); no worker edited `docs/decisions/`.

---

## The findings

### 1. The R1 refactor widened `target:"table"` from display-only to everyone

The hostile verification caught what the builder's own suite could not: the
`toRevealAudience` refactor mapped map-audience `"table"` to `All=true`
while the old inline parse mapped it to `Display=true`. A GM automation
sending `target:table` expecting display-only would have exposed the block
to all players — a widening with no ADR and no test. Fixed by restoring
`Display` for the map-target position plus a committed pin test
(`TestR1PinTargetTableDisplayOnly`), FAIL-probed both ways. The adjacent
narrowing (unknown bare strings no longer default to all) was kept
deliberately. Lesson, recurring: refactors of audience parsing are
security changes even when they look mechanical.

### 2. R4's code worked and its predicates were unverified

The first R4 delivery proved convergence, redaction, single-use, and drift
correctness under probe — and committed tests for none of emission
convergence, code redaction, or the notes `kind:created` half, while the
OpenAPI generator silently omitted ~35 loop-registered routes and the test
checked only two of three committed copies. The fix round added the four
behavior tests (each FAIL-probed), extended the collector (56 → 187 paths),
and collapsed to the two enforced copies. A predicate proved by probe and
described in a report is not a predicate — phase-1 finding 14, twice more
in this phase (R4, R5's verifyToken).

### 3. Working code with no test is the phase's most common shape

R4's missing tests, R5's `verifyToken` constant-time change with no
pinning test, the D3 web one-liners with no component harness, R1's
targeted picker with server-only coverage. The standard held: hostile test
or acceptance record with reason and owner — verifyToken and the D3
fallbacks closed as recorded acceptances in `docs/r5-trust-acceptance.md`,
not as tests that cannot bite. An acceptance is honest; a silent gap is
not.

### 4. The drift test caught a real cross-milestone skew

R3's campaign routes landed after R4 generated `openapi.json`, and
`TestOpenAPIGeneratedFromHandlers` failed on main — the generator working
as designed. R5's regeneration (with R3's routes present) closed it. This
is the mechanism justifying itself: generated-not-sketched means the doc
cannot go stale silently, and it did not.

### 5. The docs gate failed narrowly, again on truth-vs-code

R6's disposals were all decided, but SURVEY.md still called the inspector
deferred-data and api.md §10 still said "no endpoint" / "needs its own
ADR" for three shipped behaviors — the phase-5 finding-10 class. Fixed
docs-only before merging. The brief anticipated R1b/R2 SURVEY rows; the
builders never wrote them. Lesson: doc rows for a milestone belong in the
milestone's own diff, checked by the verifier, not in a later sweep.

### 6. Beam-free needed three attempts on one prompt class

The R4 fix-list prompts failed twice at render time on the builder model
and twice more returned empty; the work exists (tests were written) but
the reporting did not happen, and an investigation-model worker had to
adjudicate uncommitted work. Narrower prompts eventually succeeded. The
fallback discipline (serial, untrusted, hostilely re-verified) was never
triggered because the failure was not quota — the brief's fallback names
only rate-limits. Next phase should name the empty-response case too.

### 7. The session-tier vs frontmatter question resolved itself

ADR-0035 described a two-layer reveal store; the code ships one layer
(frontmatter + journal, synchronous). ADR-0065 records the one layer as
canonical rather than reintroducing the tier. One mechanism per seam,
decided by what ships.

### 8. The audience model needed no defense this phase

No worker reasoned from ordered rules; `visible.go` disavows them inline.
The threat-model §3 ordered list and design §5.4 phrasing remain as
history per ADR-0064; the code never followed them.

### 9. Display-vs-mood asymmetry harmonised to the stricter posture

Cue GETs were 200-stripped cross-session while mood GETs were 401. R5
picked 401 for both (narrowing, no ADR) and pinned it. The leaked-body
FAIL probe (`cues` with src paths in a 200 to a foreign seat) shows why
the stricter side won.

---

## What was measured

| | measured | criterion |
|---|---|---|
| R1 hostile | 9/9 PASS, revoked refetch raw-bytes clean | mutation-probed |
| R2 hostile | 3/3 PASS, non-GM zero GM-only bytes | mutation-probed (4 guards) |
| R3 hostile | 7/7 PASS per principal + all-routes probe | mutation-probed (3 probes) |
| R4 hostile | 4 predicates + 187 generated paths | mutation-probed (4 guards) |
| R5 hostile | display/cue 401s pinned; e2e 45/45 | code green, 2 acceptances |
| R6 | 5/5 decided + SURVEY/api flipped | docs-only |
| cold suite | `make lint` exit 0; `go test ./...` all ok, 0 FAIL | green |
| e2e shadow matrix | 45/45 (builder-run, R5 branch) | green, not independently re-run post-merge |

---

## The ADRs landed

| | |
|---|---|
| **ADR-0065** | Un-reveal revokes a revealed audience, it does not erase the broadcast (amends ADR-0035; also supersedes its two-layer durability sentence and records the per-id-audience fix) — evidence: R1 9/9 hostile suite, each FAIL-probed |
| **ADR-0066** | Campaign-declared fields inherit the note's audience (amends ADR-0033 serialisation, not the model) — evidence: R3 7/7 hostile suite, raw-bytes, mutation-probed |

---

## What was cut, deferred, or left undone

- **Discovery granularity** — recorded-deferred (flat list stands, no new
  note type). Owner: first request requiring the distinction; needs a human
  if the note type is ever funded (schema change).
- **verifyToken constant-time** — code shipped, acceptance recorded
  (property not black-box-testable). Owner: maintenance track.
- **D3 web fallbacks** — fail-closed correct, unpinned (no component-test
  harness exists). Owner: maintenance track; first harness reopens.
- **e2e post-merge** — 45/45 builder-run on R5 branch, not independently
  re-run after the R5+R6 merges (web deltas: two one-line fallback flips
  plus committed acceptance doc). Lowest-risk gap in the phase.
- **P2 download story, retention, backup, release, hardening seams,
  gaps #6–#17** — unchanged tracks per the phase-6 plan; none reopened.
- **No pushes.** All merges `--no-ff`, per rule.

## Wave/merge record (for the log)

R1 → merged `e05735a` · ADR-0065/0066 → `c9a6c2c` · R2 → merged `edb78ac` ·
R3 → merged `786359b` · R4 → merged `506753f` · R5 → merged `0198053` ·
R6 → merged `0d71060`. R1 fix `a6ef0bf` (pre-merge); R4 fix `d89be9d`
(pre-merge); R5 fix `c6a01c6` (pre-merge); R6 docs fix `cc23c54`
(pre-merge). No pushes (per rule — push on request).

---

## What phase 7 — or maintenance — should start with

1. The deferred discovery question (owner + trigger recorded in
   session-writeback §8), if any play moment needs heard-of distinct
   from visited.
2. A component-test harness for the web surfaces — the D3 fallbacks are
   the named first consumers.
3. The e2e post-merge run, to close the one unverified gate.
4. The hardening track (parser budgets, wazero fuel, search-rank side
   channel) — untouched by this phase, still queued.

## What I would do differently

- Put the SURVEY/api row updates in each milestone's own brief as
  deliverables, with the verifier checking them — the sweep caught what
  the milestone should have carried.
- Name the builder-model empty-response case in the fallback discipline,
  not just rate-limits.
- Split R5 in the first brief (code batch vs audit/docs batch) instead of
  discovering the seam mid-wave; one worker per seam, already the rule,
  applied one wave earlier.
