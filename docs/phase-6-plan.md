# Phase 6 Plan — The Backlog

> **Goal:** close the contract — every surface the app promises but does not
> keep, every backlog row with a design pass still owed, every small trust
> remainder with a hostile test still missing.
>
> **Phase 6 in one line:** *what the docs already claimed.*
>
> **Definition of done:** un-reveal exists with stated semantics; reveals can
> target players; the inspector explains from a server source; campaign fields
> are machine-readable under a decided audience; the live contract
> (`session.state`/`patch`, `kind:created`, join-code minting) is emitted and
> documented; the OpenAPI document is generated, not sketched; and the trust
> remainders each carry a hostile test or a recorded acceptance. Nothing in
> this phase widens the visibility model without its own ADR.
>
> **And one thing it does not claim.** New player-facing capabilities
> (research gaps #6–#17), a native SDK, the transcription download story, or
> release engineering. Those are scoped in §2 and stay out.

Phase membership and what moved: [`phase-plan.md`](phase-plan.md), which now
names phase 6. Prior-phase remainders feeding this phase:
[`phase-5-report.md`](phase-5-report.md) (findings 11–16, carryovers),
[`phase-4-report.md`](phase-4-report.md) (§10 cuts, §14 seams),
[`api.md`](api.md) §10 (the backlog), [`session-writeback.md`](session-writeback.md)
§8 (discovery granularity).

---

## 1. What this phase costs

**Six milestones, eleven sub-milestones.** Estimates are ranges against the
phase-5 machine (serial fallback builders cost roughly double the rounds —
phase-4 report §17, phase-5 report §19).

| Block | Sub-milestones | Full-time | Part-time (≈10 h/week) |
|---|---|---|---|
| R0 — landing (report, triage) | 1 | done this session | done this session |
| R1 — reveals, both directions | 2 | 1–2 wks | 2–4 weeks |
| R2 — inspector chain source | 1 | 3–5 days | 1–2 weeks |
| R3 — campaign fields | 1 | 3–5 days | 1–2 weeks |
| R4 — session live contract + OpenAPI | 4 | 1–2 wks | 2–4 weeks |
| R5 — trust remainders | 2 | 1 wk | 2–3 weeks |
| R6 — open decisions disposal | 1 | 2–3 days | 1 week |
| **Phase 6 total** | **12** | **~4–7 weeks** | **~2–4 months** |

---

## 2. What is in this phase, and what is not

**In:** the un-reveal design pass + endpoint (with its ADR); per-player
reveal UI on the existing contract; the inspector's server-side
constraint-verdict source (ADR-0064 D1, implemented); campaign-declared
fields under a decided audience (with its ADR); `session.state` /
`session.patch` server re-emission; `kind: "created"` for notes; join-code
minting; a generated OpenAPI document; the small trust batch
(`/display/state` binding, cue/mood cross-session posture, constant-time
token compare, journal segment redaction, transcript-log hand-review,
ADR-0064 D3 fallback fixes); disposal of the open decisions (discovery
granularity, `requires.app`, dashboard third entity, offline behaviour).

**Out, deliberately:**

| Out | Where |
|---|---|
| Native-client SDK (Swift/Kotlin) | Separate product decision (api.md §10); the contract is the enabler and it ships in R4 |
| Transcription download story (pinned per-platform fetch) | Maintenance track — phase-5 report finding 11; needs the §8 human decision (bundle/degrade/drop) first |
| Per-campaign retention + auto-delete-after-review | Maintenance track — needs the retention decision first |
| Backup/restore affordance, release pipeline/signing | Release track, not the contract track |
| Search-rank side channel, parser budgets, wazero fuel | Hardening track — each needs its own measurement + decision, none is a contract row |
| Gaps #6–#17 (player surface, combat depth, staples, GM sinks) | New capabilities, not backlog; first request reopens the list, not the milestones |
| MCP writes, out-of-process plugin tier, Tauri, live captions | Stay out (ADRs 0014/0020, ADR-0018 stretch status) |

---

## 3. Milestones

### R0 — Landing (done this session)

The phase-5 report is written (`phase-5-report.md`), the ten overtaken
issues are closed with landing pointers (#2, #3, #4, #5, #11, #18, #23,
#36, #37, #42), SURVEY names all 25 surfaces/components, and api.md
documents all shipped P1–P4 routes. Recorded here so the phase starts
clean.

### R1 — Reveals, both directions

**R1a — un-reveal.** api.md §10 states the hard part honestly: reveals are
monotonic and journalled, a grant may have propagated, clients hold the
text. This sub-milestone is a design pass first (its own ADR): the revocation
semantic — per-audience or per-block, what the journal records — then the
endpoint. The documented limit is part of the deliverable: already-broadcast
text cannot be un-sent, and the semantic says what un-reveal *does* guarantee
(future fetches, re-renders, replays).

**R1b — per-player reveal UI.** The contract already takes
`{ players: [...] }` plus the display target; the webapp only exposes
"reveal to the table". UI work on an existing contract: a targeted-reveal
picker with the same preview discipline as every write.

**Done when**
- Un-reveal conceals per the decided semantic — committed test per audience
  (concealed audience refetches clean; un-concealed audience unaffected;
  journal records the revocation).
- The webapp reveals to named players — committed test per audience shape
  (one, several, display, table).

### R2 — Inspector chain source

ADR-0064 D1 redefined the inspector as constraint-verdicts, not a rule
chain; `VisibilityInspector` still renders `decisions=[]`. Build the server
source: a role-scoped endpoint (or projection-attached field) returning the
applicable constraints (C1–C8) and widenings (W1–W3) with each verdict for
the asking principal — and **no GM-only rule text to non-GM callers**
(api.md §10 states the leak constraint). The UI renders from it.

**Done when**
- GM sees the full verdict list for a note — committed test.
- Non-GM sees only its own verdicts, zero GM-only text — committed hostile
  test (assert absence in raw bytes, not just rendered output).

### R3 — Campaign fields

The wiki Home view needs `members`, `packs`, `home` machine-readable.
Non-reserved fields are never serialised today, so this is a
visibility-model change gated by its own ADR: what audience may see
`members` (a list of account ids)? Then the implementation, then the Home
view uses it instead of raw-file reads and the `type: home` convention.

**Done when**
- The ADR is landed with the decided audience; the fields are served to
  exactly that audience — committed hostile test per principal; no widening
  of any other field.

### R4 — Session live contract + OpenAPI

Four contract rows, all owed: **`session.state` / `session.patch` server
re-emission** (cut in F5; phase-4 report §10: the live table needs exactly
these — server emits, existing client stubs consume; one envelope, no new
mechanism); **`kind: "created"` emitted for notes** (characters only today);
**join-code minting** (`POST /players/:id/join-code`, single-use, 24 h,
same posture as the creation flow); **OpenAPI document generated from
handlers** (the endpoint list stops being a sketch — generated, so it
cannot drift).

**Done when**
- State/patch emission resumes and the live table converges without refetch
  — committed test (emit → client state matches REST).
- Note creation emits `kind: "created"` — committed test.
- Minted code joins exactly once, expires in 24 h — committed test
  (reuse → 401).
- The OpenAPI document generates from the handler table and fails CI on
  drift (add-route-without-doc breaks the build).

### R5 — Trust remainders

The small batch, each with a hostile test or a recorded acceptance:
`/display/state` session-binding check (phase-4 report §14, narrowing);
cue/mood cross-session posture harmonised (phase-5 finding 15 — decide
which posture is canonical, implement, pin); `verifyToken` constant-time
compare; `session-log-proposals.json` segment redaction (or recorded
acceptance of GM-local segment text at rest); the P1–P3 transcript-log
hand-review against ADR-0058 (phase-5 plan §9.7 — audit task, evidence is a
signed-off checklist, not code); ADR-0064 D3 fallback fixes (the four
fail-open surfaces; `Home.svelte ?? "party"` still live).

**Done when**
- Each item carries its hostile test (or acceptance record with the reason
  and the owner). Display binds to its session — hostile cross-session
  test. No widening anywhere in the batch.

### R6 — Open decisions disposal

Five decisions, each small, each needing a human only where stated:
discovery granularity (**human**: `visited`/`heard-of` note type per
ADR-0050 D4, or keep flat — session-writeback §8's question is answered
either way and the answer lands in the doc); `requires.app` range language
(decide the syntax); dashboard third entity (decide if a surface needs it,
else close); offline behaviour for a native client (decide and record in
api.md §10); formula-WASM-to-client (re-defer explicitly with the forcing
reason, or fund it — not silent).

**Done when**
- Every item is decided (ADR where it changes a decision, doc line where
  it records one) and the deciding evidence is cited. No open item survives
  without an owner and a milestone.

---

## 4. Sequencing

```
R0 (landing, done) ──┬──► R1 (reveals) ──► R5 (trust batch closes last)
                     ├──► R2 (inspector) ─┤
                     ├──► R3 (campaign fields)
                     └──► R4 (live contract + OpenAPI)
R6 (decisions) runs alongside; discovery + offline answers land before R3/R4 close.
```

- **R1–R4 are parallelisable** (disjoint seams: reveal journal, inspector
  source, campaign serialisation, WS emission). At most three subagents;
  the third slot goes to verification of the first finished diff.
- **R5 closes last**: it re-sweeps every surface R1–R4 touched.
- **R6 runs alongside** but its answers bound R3 (campaign audience) and R4
  (offline posture) — those close only after their decision lands.

---

## 5. The exposure checklist, and who owns it

Phase 5's checklist stands. New and newly-due rows:

| Check | Owner |
|---|---|
| Un-reveal conceals per semantic; already-broadcast limits stated | **R1** |
| Inspector source leaks no GM-only text to non-GM | **R2** |
| Campaign fields served to exactly the decided audience | **R3** |
| State/patch emission projection-filtered; WS parity (emit or cut) | **R4** |
| Join-code mint single-use + expiry; OpenAPI generated-not-sketched | **R4** |
| Display bound to session; cue/mood posture harmonised; no widening | **R5** |
| Segment text redacted-or-accepted in journals; log hand-review signed | **R5** |

---

## 6. What phase 5 taught this phase

1. **"Server computes" goes in every brief.** The TS mirror survived because
   api.md §4 was assumed internalised (phase-5 finding 17). R2/R4 briefs
   state it verbatim.
2. **Fix briefs demand FAIL-line evidence.** Code-only fixes cost a round
   (finding 18). Every R-brief carries its hostile inputs in the first
   brief, and every fix brief requires quoted FAIL lines.
3. **One worker per seam.** P1/P3 combined worked; the seam is the unit of
   parallelism (§4), not the milestone.
4. **"No test files in the diff" still bounces.** Seven times in phase 4,
   twice more in phase 5 (P3's code-only fix). The orchestrator's
   `git diff --stat` pre-check gates every verification dispatch.

---

## 7. Open items (carried, with owners)

- P2 download story — maintenance track; needs the §8 human decision first.
- Per-campaign retention + auto-delete — maintenance track; needs the
  retention decision first.
- Backup/restore affordance, release pipeline — release track.
- Search-rank side channel, parser budgets, wazero fuel — hardening track.
- Gaps #6–#17 — not backlog; first request reopens the list, not the
  milestones.
- `TestInsertPerformance` 250 ms bound — environmental flake watch; untouched
  unless it bites.
- macOS watcher + session-tier numbers — still need a Mac.
