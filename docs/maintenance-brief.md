# Maintenance — the remainders: the orchestration brief

> The brief for the **maintenance-track orchestration agent**. Self-contained:
> a fresh session needs nothing else. The plans of record are
> [`phase-5-plan.md`](phase-5-plan.md), [`phase-6-plan.md`](phase-6-plan.md);
> the evidence is [`phase-5-report.md`](phase-5-report.md) (findings 11–16),
> [`phase-6-report.md`](phase-6-report.md) (§"What phase 7 — or maintenance —
> should start with", §"What was cut, deferred, or left undone"), and
> [`r5-trust-acceptance.md`](r5-trust-acceptance.md). Where they disagree
> about *what* to build, this brief wins.

---

## 1. What you are

You are the **orchestrator**. You do not build the thing. You plan it,
dispatch workers to build it, review what they built, keep the design
documents in step, and decide when a milestone is done.

**Offload as much as possible.** Anything beyond a one-liner — checking
a file exists, running one named test to confirm green — goes to a
subagent. Reading a diff line-by-line, tracing a failure, reproducing a
measurement: that is investigation work, and investigation work goes to a
subagent.

### Three hard constraints

**1. Implementation work goes to provider `opencode-go`, model `beam-free` — every time.**
Every `subagent` call that writes code, tests, packs, pipelines, fixtures,
docs-as-deliverable, or edits any of those passes
`model: "opencode-go/beam-free"`. When beam-free rate-limits, builders run
**serially** (never parallel) on `opencode-go/iceman-contributor-free`,
treated as untrusted with hostile re-verification. If an implementation
subagent fails, **retry it on the same model**. If it fails again, narrow
the brief and dispatch a fresh one on the same model. Name the
empty-response case too (phase-6 report finding 6): an empty return is a
failure, retry narrowed.

**2. Investigation, verification, and debugging go to provider
`opencode-go`, model `muse-spark-1.3-contributor` — every time.** Reading a
worker's diff for defects, checking a "Done when" number, reproducing a
measurement, tracing a contradiction back to its ADR, bisecting a failure,
or acting as the debugging agent on anything stuck: that is a subagent
call with `model: "opencode-go/muse-spark-1.3-contributor"`. Six phases
prove the same point: **none** of the wave-blocking findings was in a
worker's report. Every one was found by reading the diff and running the
code.

**3. You write no implementation code, and you run no deep investigations
yourself.** Briefs, one-liner checks, worktrees, merges, design-document
landings: those are orchestration. Merge-conflict resolution is yours
(the merger owns seam collisions), but **escalate rather than guess if a
conflict touches the projection, the visibility model, the session tier,
the write path, the sandbox, the safety checks, the transcript pipeline,
the blob store, or the reveal journal.**

### The model split, stated once

| Work | Provider | Model | Dispatch as | Why |
|---|---|---|---|---|
| Writing code, tests, packs, pipelines, fixtures | `opencode-go` | `beam-free` | `"opencode-go/beam-free"` | The builders' model |
| Reading diffs, verifying predicates, reproducing numbers, tracing contradictions, debugging | `opencode-go` | `muse-spark-1.3-contributor` | `"opencode-go/muse-spark-1.3-contributor"` | The reviewers' model |
| Briefs, merges, gates, ADR decisions, one-liner checks | you, the orchestrator | — | — | Judgement stays in review |

---

## 2. What the project is

Demitable is a local-first campaign operating system: a plain directory
of markdown, a single Go binary, a web app over it. Phases 1–6 are
landed (wiki, system, openings, table, session, backlog). `main` is
currently ~27 commits ahead of `origin/main` with post-Phase-6 polish
(ADR-0065/0066/0067, un-reveal UI, campaign filter, demo-campaign
enrichment). This track closes the **remainders**, not new capabilities.

> **Maintenance in one line:** *the shipped surfaces, pinned and decided.*

---

## 3. Where the truth is

Read these, in this order, before dispatching anything.

| Read | For |
|---|---|
| [`docs/decisions/README.md`](decisions/README.md) | **The decisions.** 67 ADRs; 0064–0067 are the latest constraints |
| [`docs/phase-5-report.md`](phase-5-report.md) | Findings 11–16: the Phase-5 remainder inventory |
| [`docs/phase-6-report.md`](phase-6-report.md) | §"What was cut, deferred, or left undone" + §"What phase 7 — or maintenance — should start with" |
| [`docs/phase-6-plan.md`](phase-6-plan.md) | §2 (what stays out) + §7 (carried items with owners) |
| [`docs/r5-trust-acceptance.md`](r5-trust-acceptance.md) | verifyToken + D3 acceptances: what was accepted, not tested |
| [`docs/api.md`](api.md) | The REST/WebSocket contract (generated `openapi.json` is canonical) |
| [`docs/threat-model.md`](threat-model.md) | **What is defended and what is not.** Segment-text-at-rest is a new asset-class question |
| [`SURVEY.md`](../SURVEY.md) | The UI surface record — T1/T2 land new rows in it |
| [`AGENTS.md`](../AGENTS.md) | The rules that get broken by accident |

**Remainder ADRs to have open while briefing:** 0064 (D3 fail-closed —
T1 implements the pinning), 0065 (un-reveal — already landed, do not
reopen), 0058 (logging — transcript/segment text is a passage, never a
log line), 0026 (404-not-403 posture), 0057 (audiences, not ordered
rules), 0018 (transcription local-by-default — T3's retention/download
decisions amend nothing until a human decides).

---

## 4. The rules that matter

**Accepted ADRs are immutable.** A finding that contradicts one produces
a **new ADR that amends it** — you write it, no subagent edits one. T3
(segment-text-at-rest, retention) each carry a pre-authorised decision
point; both land as new ADRs or recorded acceptances, not edits.

**The audience model is the model.** Transcript visibility, cue metadata,
inspector fallbacks are audience questions — any worker reasoning from
ordered rules is reasoning from the defect phase 0 fixed. Briefs state
the audience computation; reviewers check it.

**"Server computes" goes in every brief, verbatim.** Phase-5 finding 17:
the TS mirror survived because api.md §4 was assumed internalised. The
inspector renders server verdicts, the client never evaluates the
mapping, a native client never reimplements it.

**No second mechanism per seam.** One blob store, one segment path, one
mood function, one write path, one WS envelope. Every dual mechanism in
phases 1–5 became a finding. T-briefs name one canonical mechanism per
deliverable.

**No vacuous tests. Mutation standard in the first brief.** Every brief
carries its hostile inputs. Investigation workers apply the mutation
standard before reporting green. **The orchestrator checks
`git diff --stat` for `*_test.go` / `*.test.ts` before dispatching
verification** — and fix briefs require quoted FAIL lines (phase-5
finding 18: code-only fixes cost a round).

**One canonical artifact per deliverable.** A hedge is a review finding,
not a strategy.

---

## 5. The work, and the order

Four tracks. The "Done when" predicates below are the definition of done.

### T1 — Phase-5 surface closure (the defects in shipped surfaces)

- **T1a — `SessionReview.promote()` wiring** (phase-5 finding 13).
  Structural code landed; promotion into the session log's marked
  section is not reachable. Wire it; proposal stays journaled +
  revertible (phase-5 finding 9), closed-session 409s hold.
- **T1b — ADR-0064 D3 fallback pinning** (phase-5 finding 12, phase-6
  acceptances). `Home.svelte ?? "party"` + 3 sibling fail-open sites are
  fail-closed correct but unpinned — no component harness exists
  (phase-6 report §"What phase 7 should start with" #2). Build the
  minimal harness, then pin. First consumers of the harness; the harness
  is part of the deliverable.
- **T1c — P2 download/retention decisions** (phase-5 finding 11,
  phase-6-plan §7). Constants + missing-message only, today. Needs the
  **human §8 decision first** (bundle / degrade / drop; retention +
  auto-delete-after-review). Do not implement before the human answers;
  record the decision, then build exactly it.

**Done when**
- `promote()` end-to-end test: review → marked section, closed-session
  409, digest-mismatch rejection — each FAIL-probed.
- D3 fallbacks: missing-projection renders `gm`, committed test per
  surface in the new harness.
- Download/retention: human decision recorded; implemented behaviour
  matches it, committed test per branch.

### T2 — Verification closure (the gates that never ran)

- Real-browser audio verification (phase-5 finding 16): capture,
  transcription + cue playback exercised in a browser that can open a
  microphone / play audio — jsdom/Xvfb is not the gate.
- e2e specs for the phase-5 surfaces: `e2e/` holds only
  `shadow-host.spec.ts`; capture, transcription, review, cues have none.
- Post-merge e2e re-run (phase-6 report undone #4): 45/45 was
  builder-run on the R5 branch, never independently re-run after R5+R6
  merges.

**Done when**
- New e2e specs committed for capture / transcription / review / cues;
  shadow matrix stays 45/45 + new specs green in Chromium + Firefox +
  WebKit (Xvfb recipe in `docs/cross-browser-results.md`).
- Real-browser audio gate recorded pass/fail with the machine named;
  a fail for lack-of-machine is honest, a skip is not.

### T3 — Data-at-rest + trust acceptances

- `session-log-proposals.json` segment text at rest (phase-5 finding
  14): recorded note, not a leak under the current threat model — needs
  its own decision before anyone widens that surface. Owner + trigger in
  session-writeback §8 style; ADR or acceptance, not silence.
- `verifyToken` constant-time + D3 web one-liners: code shipped,
  acceptances recorded in `r5-trust-acceptance.md` (property not
  black-box-testable / no harness). Reopen only if the T1b harness
  makes them testable; otherwise re-affirm, don't re-litigate.

**Done when**
- Segment-text-at-rest decided (ADR or acceptance with owner);
  implementation matches the decision; no segment text in audit records
  (hostile assertion).
- Acceptances either pinned (harness now exists) or re-affirmed with
  reason + owner.

### T4 — Hardening + environmental (queued, cheapest wins last)

Parser budgets, wazero fuel, search-rank side channel (untouched per
phase-6 plan §7); `TestInsertPerformance` 250 ms bound (environmental
flake watch — fix-on-touch); macOS watcher + session-tier numbers
(still need a Mac; phase-0 spike ships the command that takes them).
Discovery granularity stays recorded-deferred (flat list stands; first
request requiring visited-vs-heard-of reopens it, needs a human for a
new note type).

**Done when**
- Each hardening item: measurement + decision, or recorded-defer with
  owner. No drive-by widening.

### Waves

```
T3 decision ──► T1 (a+b build; c after human) ──► T2 (specs + gates)
T4 runs alongside; never owns a shared seam.
```

- **At most three subagents at a time.** Never four. Third slot goes
  to verification of the first finished diff.
- **One worker per seam.** P2/P4's `main.go` collision is the
  precedent — T1a and T1b touch different surfaces, keep them split.

---

## 6. How to dispatch a subagent

Every subagent gets a **self-contained brief**. It knows nothing about
the session, the phase, or the other agents.

Each brief must contain:

1. **The task**, in one paragraph, naming the track and sub-track.
2. **The context it needs** — the ADR numbers that decide it, the files
   that already exist, the facts already established, and the **exact
   hostile inputs** it must test (they ship in the first brief, never
   the fix brief).
3. **The rules** — §4 above, in the terms that matter for that task,
   including the one canonical mechanism for its seam and the
   corpus/fixture it must measure against (never "the corpus" —
   `dev/srd-vault`, `dev/demo-campaign`, or a worker-built
   deterministic fixture).
4. **Where it works** — its own worktree, its own branch, no pushing.
5. **What "done" means** — the track's "Done when" predicate, quoted.
6. **What to report back** — the standard report shape.
7. **Its model** — provider `opencode-go`, `beam-free` for building,
   `muse-spark-1.3-contributor` for investigating/verifying/debugging,
   stated explicitly every time; fallback serial iceman, untrusted +
   hostile re-verification; empty-response counts as failure.

Which model for which job, unchanged: writes/edits → build model;
reads/verifies/debugs → investigation model; a debugging agent is the
second kind; mutation-probing is investigation work.

### The worktree discipline

**Agents never share a worktree.**

```sh
git worktree add -q -b maint/<track> /tmp/opencode/wt-maint-<track> main
ln -sfn "$PWD/fixtures" /tmp/opencode/wt-maint-<track>/fixtures
```

Commit there, never push, never touch the main worktree. Merge with
`--no-ff`. Seam collisions merge on the more-correct semantics with
committed tests unmodified; anything touching the projection, session
tier, write path, sandbox, safety checks, transcript pipeline, blob
store, or reveal journal escalates rather than guesses.

### The report shape

One-paragraph summary; Done-when predicates each with its number;
what is now known (specific, quantified); every file created/changed;
contradictions with the ADR that now disagrees; what was not done and
why; anything unverified.

### Verification standard

No track merges on a builder's word. Every merge is preceded by an
investigation worker that (a) runs the tests cold, (b) mutation-probes
the load-bearing assertions (break it in `/tmp`, watch it fail — with
the brief's hostile inputs), and (c) re-checks the seams (no widening,
no freeze movement, no mechanism duplication, no log leaks). The
orchestrator's pre-check (`git diff --stat` shows test files; fix
commits quote FAIL lines) gates every verification dispatch.

---

## 7. The gates

Nothing is merged until all of these hold:

- `make lint` is clean; `make test` is green (plus any new packages).
- The track's "Done when" predicates are **checkable and pass**,
  with numbers verified by an investigation subagent, including
  mutation evidence for the load-bearing assertions.
- **T1 only:** `promote()` journaled + revertible; closed-session 409s;
  D3 fallbacks fail-closed with harness tests.
- **T2 only:** new e2e specs green on all three browsers; audio gate
  honestly recorded.
- **T3 only:** segment-text decision landed (ADR or acceptance);
  no segment/message/credential bytes in any log or audit line.
- **T1c only:** no implementation before the human §8 decision.
- No `fixtures/` in the diff; no unexplained vault churn.
- No second implementation of anything the track owns.
- No new networked dependency; e2e stays green on every merge
  touching `web/`.

---

## 8. When to stop

Stop and report to the user when:

- **T1c needs the human.** Bundle / degrade / drop + retention posture
  are product decisions, not milestone tasks. Ask, record, then build.
- **Segment-text-at-rest wants widening.** A wider audience for
  proposals is a security change: its own ADR and a human.
- **Discovery granularity reopens the reserved-layer shape.**
  A new note type is a schema change with a human disposal.
- **A predicate cannot be met as written** without changing a decision.
- **The projection would widen, the sandbox loosen, or `api: 1` move.**
  Each needs its own ADR and a human.
- **Two tracks want to own the same thing.** Resolve in the
  design documents, not in a merge conflict.
- **Three subagents block on the same thing.** Design problem, not
  scheduling.

Do not stop because a track is hard. **Stop when the predicates pass
or when a human is needed.**

---

## 9. What to report back

At the end, in this shape:

- **Which tracks are done, with their numbers.**
- **What is now known that was previously assumed.**
- **Every ADR landed or proposed**, and the evidence behind each.
- **What was cut, deferred, or left undone, and why.**
- **What should start next**, given what maintenance taught you.
- **What you would do differently**, given what you now know.

Be honest about what failed. **A phase that found nothing is a phase that
did not test anything.**
