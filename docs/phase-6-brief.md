# Phase 6 — The Backlog: the orchestration brief

> The brief for the phase-6 **orchestration agent**. Self-contained: a fresh
> session needs nothing else. The plan of record is
> [`phase-6-plan.md`](phase-6-plan.md); where they disagree about *what* to
> build, this brief wins.

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
`model: "opencode-go/beam-free"`. Phase 5 proved the fallback works: when
beam-free rate-limits, builders run **serially** (never parallel) on
`opencode-go/iceman-contributor-free`, treated as untrusted with
hostile re-verification. If an implementation subagent fails, **retry it
on the same model**. If it fails again, narrow the brief and dispatch a
fresh one on the same model.

**2. Investigation, verification, and debugging go to provider
`opencode-go`, model `muse-spark-1.3-contributor` — every time.** Reading a
worker's diff for defects, checking a "Done when" number, reproducing a
measurement, tracing a contradiction back to its ADR, bisecting a failure,
or acting as the debugging agent on anything stuck: that is a subagent
call with `model: "opencode-go/muse-spark-1.3-contributor"`. Five phases
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
of markdown, a single Go binary, a web app over it. Phases 1–4 built the
campaign wiki, proved the system-agnostic claim, opened the boundaries,
and put bodies around the table. Phase 5 recorded the session —
capture, transcription, logs, cues. **Phase 6 closes the contract:** the
rows in api.md §10 marked backlog, cut, or not-yet, plus the trust
remainders with hostile tests still missing.

> **Phase 6 in one line: what the docs already claimed.**

---

## 3. Where the truth is

Read these, in this order, before dispatching anything.

| Read | For |
|---|---|
| [`docs/decisions/README.md`](decisions/README.md) | **The decisions.** 64 ADRs; 0064 redefined the inspector's language |
| [`docs/phase-6-plan.md`](phase-6-plan.md) | **What to build.** Six milestones, twelve sub-milestones |
| [`docs/phase-plan.md`](phase-plan.md) | Phase membership; now names phase 6 |
| [`docs/phase-5-report.md`](phase-5-report.md) | What phase 5 found: nineteen findings, every carried item |
| [`docs/api.md`](api.md) | The REST/WebSocket contract (§10 is the backlog inventory) |
| [`docs/session-writeback.md`](session-writeback.md) | **The §8 discovery question.** R6 answers it |
| [`docs/threat-model.md`](threat-model.md) | **What is defended and what is not.** Reveal revocation is a new asset-class question |
| [`docs/phase-4-report.md`](phase-4-report.md) | F5 cuts (state/patch re-emission), §14 seams, the model/quota lesson |
| [`docs/phase-1-report.md`](phase-1-report.md) | The standing lessons (tests that count, noise-band bounds) |
| [`SURVEY.md`](../SURVEY.md) | The UI surface record — R1b/R2 land new rows in it |
| [`AGENTS.md`](../AGENTS.md) | The rules that get broken by accident |

**Phase-6 ADRs to have open while briefing:** 0035 (secret reveals —
R1 amends the monotonicity), 0064 (constraint-verdicts — R2 implements
D1), 0057/0061 (audiences, partition bounds), 0050 (reserved layer),
0039 (partial apply — R1's revocation inherits the journal discipline),
0015 (journal — reveal/revocation events), 0058 (logging), 0026
(404-not-403 posture for any new endpoint), 0033 (campaign scope —
R3 amends the serialisation, not the model).

---

## 4. The rules that matter

**Accepted ADRs are immutable.** A finding that contradicts one produces
a **new ADR that amends it** — you write it, no subagent edits one. R1
(un-reveal) and R3 (campaign fields) each carry a pre-authorised decision
point; both land as new ADRs, not edits, with worker-gathered evidence.

**The audience model is the model.** Un-reveal scoping, inspector
verdicts, and campaign-field serving are audience questions — any worker
reasoning from ordered rules is reasoning from the defect phase 0 fixed.
Briefs state the audience computation; reviewers check it.

**"Server computes" goes in every brief, verbatim.** Phase 5 finding 17:
the TS mirror survived because api.md §4 was assumed internalised. R2
and R4 briefs state it as a rule, not a reference — the inspector renders
server verdicts, the client never evaluates the mapping, a native client
never reimplements it.

**No second mechanism per seam.** One reveal journal, one inspector
source, one campaign-field path, one WS envelope, one write path. Every
dual mechanism in phases 1–5 became a finding. R-briefs name one
canonical mechanism per deliverable.

**No vacuous tests. Mutation standard in the first brief.** Every brief
carries its hostile inputs (revoked-audience refetch, non-GM inspector
bytes, out-of-audience field reads, single-use join-code reuse,
wrong-session display). Investigation workers apply the mutation standard
before reporting green. **The orchestrator checks `git diff --stat` for
`*_test.go` / `*.test.ts` before dispatching verification** — and fix
briefs require quoted FAIL lines (phase-5 finding 18: code-only fixes
cost a round).

**One canonical artifact per deliverable.** A hedge is a review finding,
not a strategy.

**Three directories are still not disposable.** `internal/vault/`,
`dev/demo-campaign/`, `dev/srd-vault/` are committed corpora and CI
fixtures. `spikes/` is disposable; `fixtures/` is transient and
gitignored.

---

## 5. The work, and the order

Six milestones, twelve sub-milestones. The plan has each one's
"Done when" predicate; those predicates are the definition of done.

### Where phase 6 already starts

Phases 1–5 are landed and pushed. R0 (phase-5 report, issue triage,
SURVEY/api catch-up) is done this session. The reveal journal, the
projection, the WS envelope, and the write-back are the floor R1–R4
stand on. The seam map gains one entry: the **inspector source** owns
constraint verdicts (server-computed, role-scoped). The vault still owns
durability; the projection still owns who sees everything.

### The waves

Dependencies: R6 decisions bound R3 (campaign audience) and R4 (offline
posture); R5 re-sweeps everything last.

| Wave | Milestones | Why together |
|---|---|---|
| 1 | **R1**, **R2** | Disjoint seams (reveal journal vs inspector source); both need design-first briefing |
| 2 | **R3**, **R4** | Disjoint seams (campaign serialisation vs WS emission); R6 answers must have landed first |
| 3 | **R5**, **R6** | R5 re-sweeps all touched surfaces; R6 disposals close the phase |

**At most three subagents at a time.** Never four. A wave with two
milestones and three slots means the third slot goes to verification:
dispatch an investigation subagent to read the first finished diff
while the second milestone is still building.

### Two milestones that get extra attention

**R1 — revocation is a security semantic, not an inverse.** Un-reveal
touches the monotonicity the whole reveal design rests on. The ADR comes
before the endpoint, and the documented limit (already-broadcast text)
is part of the deliverable, not a footnote. The investigation model's
hostile re-run covers revoked-audience refetch plus the propagation
boundary.

**R4 — the emission the table converges on.** State/patch re-emission
re-opens the F5 cut on the live path. WS parity (emit or cut) and
projection-filtered emission get the investigation model's hostile
re-run, not the builder's word.

---

## 6. How to dispatch a subagent

Every subagent gets a **self-contained brief**. It knows nothing about
the session, the phase, or the other agents.

Each brief must contain:

1. **The task**, in one paragraph, naming the milestone and sub-milestone.
2. **The context it needs** — the ADR numbers that decide it, the files
   that already exist, the facts already established, and the **exact
   hostile inputs** it must test (phase-5 finding: they ship in the
   first brief, never the fix brief).
3. **The rules** — §4 above, in the terms that matter for that task,
   including the one canonical mechanism for its seam and the
   corpus/fixture it must measure against (never "the corpus" —
   `dev/srd-vault` or a worker-built deterministic fixture).
4. **Where it works** — its own worktree, its own branch, no pushing.
5. **What "done" means** — the milestone's "Done when" predicate, quoted.
6. **What to report back** — the standard report shape.
7. **Its model** — provider `opencode-go`, `beam-free` for building,
   `muse-spark-1.3-contributor` for investigating/verifying/debugging,
   stated explicitly every time; fallback per §1 (serial iceman,
   untrusted + hostile re-verification).

Which model for which job, the phase-5 rule unchanged: writes/edits →
build model; reads/verifies/debugs → investigation model; a debugging
agent is the second kind; mutation-probing is investigation work.

### The worktree discipline

**Agents never share a worktree.**

```sh
git worktree add -q -b phase6/<milestone> /tmp/opencode/wt-phase6-<milestone> main
ln -sfn "$PWD/fixtures" /tmp/opencode/wt-phase6-<milestone>/fixtures
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

No milestone merges on a builder's word. Every merge is preceded by an
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
- The milestone's "Done when" predicates are **checkable and pass**,
  with numbers verified by an investigation subagent, including
  mutation evidence for the load-bearing assertions.
- **R1 only:** revocation semantic in a landed ADR; revoked-audience
  refetch clean (hostile); propagation limits stated, not implied.
- **R2 only:** non-GM inspector bytes contain zero GM-only text
  (hostile, raw-bytes assertion).
- **R3 only:** fields served to exactly the ADR-decided audience
  (hostile per principal).
- **R4 only:** state/patch emission converges without refetch; OpenAPI
  generates from handlers (drift fails CI); minted code single-use.
- **R5 only:** display bound to session (hostile cross-session test);
  every batch item pinned or acceptance-recorded.
- **R6 only:** every item decided with cited evidence; no ownerless
  open item survives.
- No `fixtures/` in the diff; no unexplained vault churn.
- No note content, segment text, message body, or credential in any
  log line; no segment text in audit records.
- No second implementation of anything the phase owns (one reveal
  journal, one inspector source, one field path, one WS envelope).
- No new networked dependency; e2e stays 45/45 across all three
  browsers on every merge touching `web/` (Xvfb recipe in
  `docs/cross-browser-results.md`).

---

## 8. When to stop

Stop and report to the user when:

- **Un-reveal's semantic breaks monotonicity assumptions** the journal
  or write-back depends on — revocation vs revert needs a human.
- **Campaign fields want a wider audience than the ADR allows.**
  Widening is a security change: its own ADR and a human.
- **Discovery granularity reopens the reserved-layer shape.**
  A new note type is a schema change with a human disposal.
- **A predicate cannot be met as written** without changing a decision.
- **The projection would widen, the sandbox loosen, or `api: 1` move.**
  Each needs its own ADR and a human.
- **Two milestones want to own the same thing.** Resolve in the
  design documents, not in a merge conflict.
- **Three subagents block on the same thing.** Design problem, not
  scheduling.

Do not stop because a milestone is hard. **Stop when the predicates pass
or when a human is needed.**

---

## 9. What to report back

At the end, in this shape:

- **Which milestones are done, with their numbers.**
- **What is now known that was previously assumed.**
- **Every ADR landed or proposed**, and the evidence behind each.
- **What was cut, deferred, or left undone, and why.**
- **What phase 7 — or maintenance — should start with**, given what
  phase 6 taught you.
- **What you would do differently**, given what you now know.

Be honest about what failed. **A phase that found nothing is a phase that
did not test anything.**
