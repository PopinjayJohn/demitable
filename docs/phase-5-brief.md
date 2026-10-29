# Phase 5 — The Session: the orchestration brief

> The brief for the phase-5 **orchestration agent**. Self-contained: a fresh
> session needs nothing else. The plan of record is
> [`phase-5-plan.md`](phase-5-plan.md); where they disagree about *what* to
> build, this brief wins.

---

## 1. What you are

You are the **orchestrator**. You do not build the thing. You plan it,
dispatch workers to build it, review what they built, keep the design
documents in step, and decide when a milestone is done.

**Offload as much as possible.** Anything beyond a one-liner — checking
a file exists, running one named test to confirm green — goes to a
subagent. Reading a diff line-by-line yourself, tracing a failure,
bisecting a regression, reproducing a measurement: that is investigation
work, and investigation work goes to a subagent.

### Three hard constraints

**1. Implementation work goes to provider `opencode-go`, model `beam-free` — every time.**
Every `subagent` call that writes code, tests, packs, pipelines, fixtures,
docs-as-deliverable, or edits any of those passes
`model: "opencode-go/beam-free"`. Phase 4 proved the fallback works:
when beam-free rate-limits, builders run **serially** (never parallel)
on `opencode-go/iceman-contributor-free`, treated as untrusted with
hostile re-verification. If an implementation subagent fails, **retry it
on the same model**. If it fails again, narrow the brief and dispatch a
fresh one on the same model. (Session note: this session used
`opencode/deepseek-v4.1-flash-free` and the `opencode/beam-free`
personal provider under quota pressure. The default is beam-free. Any
fallback is serial, never parallel, and its output gets the untrusted
treatment in §7.)

**2. Investigation, verification, and debugging go to provider
`opencode-go`, model `muse-spark-1.3-contributor` — every time.** Reading a
worker's diff for defects, checking a "Done when" number, reproducing a
measurement, tracing a contradiction back to its ADR, bisecting a failure,
or acting as the debugging agent on anything stuck: that is a subagent
call with `model: "opencode-go/muse-spark-1.3-contributor"`. Four phases
prove the same point: **none** of the wave-blocking findings was in a
worker's report. Every one was found by reading the diff and running the
code.

**3. You write no implementation code, and you run no deep investigations
yourself.** Briefs, one-liner checks, worktrees, merges, design-document
landings: those are orchestration. Merge-conflict resolution is yours
(the merger owns seam collisions), but **escalate rather than guess if a
conflict touches the projection, the visibility model, the session tier,
the write path, the sandbox, the import's safety checks, or — new in
this phase — the transcript pipeline or the blob store.**

### The model split, stated once

| Work | Provider | Model | Dispatch as | Why |
|---|---|---|---|---|
| Writing code, tests, packs, pipelines, fixtures | `opencode-go` | `beam-free` | `"opencode-go/beam-free"` | The builders' model |
| Reading diffs, verifying predicates, reproducing numbers, tracing contradictions, debugging | `opencode-go` | `muse-spark-1.3-contributor` | `"opencode-go/muse-spark-1.3-contributor"` | The reviewers' model |
| Briefs, merges, gates, ADR decisions, one-liner checks | you, the orchestrator | — | — | Judgement stays in review |

---

## 2. What the project is

Demitable is a local-first campaign operating system: a plain directory
of markdown, a single Go binary, a web app over it. Phases 1–4 built
the campaign wiki with secrets, proved the system-agnostic claim with
two disagreeing packs, opened the boundaries, and put bodies around
the table — sessions, seats, the encounter runner, the tabletop, chat,
the write-back, and the completed webapp. Phase 5 is what the morning
after keeps: **record, transcribe, log, and cue** — plus the contract
conformance the webapp rework left behind.

> **Phase 5 in one line: what the morning after keeps, and what it sounded like.**

---

## 3. Where the truth is

Read these, in this order, before dispatching anything.

| Read | For |
|---|---|
| [`docs/decisions/README.md`](decisions/README.md) | **The decisions.** 63 ADRs; phase-4 report §"ADRs landed" for the latest |
| [`docs/phase-5-plan.md`](phase-5-plan.md) | **What to build.** Five milestones, twelve sub-milestones |
| [`docs/phase-plan.md`](phase-plan.md) | Phase membership; phase 5's recording scope + what moved |
| [`docs/design.md`](design.md) | The living design (vault schema, session tier, risk table) |
| [`docs/session-writeback.md`](session-writeback.md) | **The §8 open questions.** P3 implements the marked-section default |
| [`docs/api.md`](api.md) | The REST/WebSocket contract (P0a's staleness list; §10 backlog stays backlog) |
| [`SURVEY.md`](../SURVEY.md) | The UI surface record — O5-era, P0c refreshes it |
| [`docs/threat-model.md`](threat-model.md) | **What is defended and what is not.** Audio blobs are a new asset class; §9.7-style partial modelling is expected, stated plainly |
| [`docs/phase-4-report.md`](phase-4-report.md) | What phase 4 found: nineteen findings, the model/quota lesson, every carried item |
| [`docs/phase-1-report.md`](phase-1-report.md) | The standing lessons (rate limiter that never engaged, tests that counted) |
| [`AGENTS.md`](../AGENTS.md) | The rules that get broken by accident |

**Phase-5 ADRs to have open while briefing:** 0018/0031 (transcription
pluggable-local, exposure posture), 0002 (trust boundary), 0014 (MCP
read-only — transcript/segment surfaces never assume writes), 0015
(journal — chat notes, reveal events, log proposals all journal),
0030 (derived caches outside the vault — the blob store rule),
0026 (404-not-403 serving posture for the audio endpoints), 0057/0061
(audiences, partition bounds), 0050 (reserved layer, flat discovery),
0039 (partial apply — P3's proposal inherits the preview discipline),
0017 (between-sessions access rides existing notes), 0058 (logging).

---

## 4. The rules that matter

**Accepted ADRs are immutable.** A finding that contradicts one produces
a **new ADR that amends it** — you write it, no subagent edits one. P0
carries two pre-authorised decision points (guest authorship rule,
ADR-0052 amendment); both land as new ADRs, not edits, with the
worker-gathered evidence.

**The audience model is the model.** Seat, account, and display
resolution all compute audiences (ADR-0057). Transcript visibility,
cue metadata, and the authorship rule are audience questions — any
worker reasoning from ordered rules or "deny overrides every grant"
is reasoning from the defect phase 0 fixed once. Briefs state the
audience computation; reviewers check it.

**No second mechanism per seam.** One blob store, one segment path,
one mood function, one preview pipeline, one write path, one fog
function, one seat resolver. Every dual mechanism in phases 1–4
became a finding. P-briefs name one canonical mechanism per
deliverable.

**The transcript is the hottest surface the logging rule has ever met.**
Speech-to-text of a live table is names, secrets spoken aloud, and
out-of-character asides. A log line names a file and a construct,
never a passage (ADR-0058) — and segment text, message bodies, and
audio metadata are passages. New code paths get hand-review to the
investigation model for log content specifically, not just as part
of the seam sweep.

**Local by default, never silently remote.** Fresh campaigns resolve
the local engine with custom config present (tested). Enabling
anything else states the leaves-the-machine warning verbatim at the
point of enabling (tested, exact-string). No networked dependency
enters the build; the sidecar is a pinned download, not a bundle.

**No vacuous tests.** Every brief carries its hostile inputs (player
fetch of audio URLs, unreviewed-transcript reads, implicit-engine
selection, music-path fetch with the network down, tampered mood
inputs, forged seat on capture/reveal/cue). Investigation workers
apply the mutation standard before reporting green. A test that
cannot fail is not a test. **And the orchestrator checks
`git diff --stat` for `*_test.go` / `*.test.ts` before dispatching
verification** — a testless branch never reaches a reviewer (phase-4
finding 13, seven times).

**One canonical artifact per deliverable.** A hedge is a review
finding, not a strategy.

**Three directories are still not disposable.** `internal/vault/`,
`dev/demo-campaign/`, `dev/srd-vault/` are committed corpora and CI
fixtures. `spikes/` is disposable; `fixtures/` is transient and
gitignored. P2's fixture audio is built by the worker (synthesized,
deterministic) and committed as a test input — that is a fixture
**of** the suite, not the transient corpus, and the distinction is
stated in the brief.

---

## 5. The work, and the order

Five milestones, twelve sub-milestones. The plan has each one's
"Done when" predicate; those predicates are the definition of done.

### Where phase 5 already starts

Phases 1–4 are landed and pushed, plus the user's webapp rework
(Viewer, split explorer, campaign Home, in-body reveals, docked
stat box, journal drawers) and the WebKit-green matrix close-out.
**None of it is rebuilt.** P0 inventories the reworked surfaces
against the design before phase 5 builds on them. The session tier,
the journal, the projection, and the write-back are the floor P1–P4
stand on. The seam map extends by two entries: the **blob store**
owns bytes (session-scoped, outside the vault), the **transcript
pipeline** owns segments-into-prose. The vault still owns
durability; the projection still owns who sees everything,
including transcripts, cues, and review state.

### The waves

Dependencies: P0 → P1 → P2 → P3; P2 + P4 together; P0b early.

| Wave | Milestones | Why together |
|---|---|---|
| 1 | **P0** | The floor: contract truth, carryover closure, SURVEY truth |
| 2 | **P1** | Capture + blob store; everything downstream needs audio shaped this way |
| 3 | **P2**, **P4** | Transcript pipeline and cue surface; different surfaces, one shared rule (local files, no network, never around the projection) |
| 4 | **P3** | The log proposal; needs the real transcript pipeline + lexicon loop to close against |

**At most three subagents at a time.** Never four. A wave with two
milestones and three slots means the third slot goes to verification:
dispatch an investigation subagent to read the first finished diff
while the second milestone is still building.

### Two milestones that get extra attention

**P1 — the first new asset class since phase 1.** Audio blobs are
untranscribed speech: the most intimate bytes the product has ever
held. The 404-hostile tests plus the no-bytes-to-players discipline
get the investigation model's hostile re-run, not the builder's word.

**P2 — the trust-and-distribution crux.** Implicit-engine selection
and the sidecar story are the two decisions an auditor will read
first. The exact-string disclosure tests and the fresh-campaign-
defaults-local test are load-bearing; the download story (pinned,
checksumed, missing-with-message) is verified, not described.

---

## 6. How to dispatch a subagent

Every subagent gets a **self-contained brief**. It knows nothing about
the session, the phase, or the other agents.

Each brief must contain:

1. **The task**, in one paragraph, naming the milestone and sub-milestone.
2. **The context it needs** — the ADR numbers that decide it, the files
   that already exist, the facts already established, and the **exact
   hostile inputs** it must test (phase-4 finding 1: they ship in the
   first brief, not the fix brief).
3. **The rules** — §4 above, in the terms that matter for that task,
   including the one canonical mechanism for its seam and the
   corpus/fixture it must measure against (phase-4 finding: name
   `dev/srd-vault` or the worker-built fixture, never "the corpus").
4. **Where it works** — its own worktree, its own branch, no pushing.
5. **What "done" means** — the milestone's "Done when" predicate, quoted.
6. **What to report back** — the standard report shape.
7. **Its model** — provider `opencode-go`, `beam-free` for building,
   `muse-spark-1.3-contributor` for investigating/verifying/debugging,
   stated explicitly every time; fallback per §1 (serial iceman,
   untrusted + hostile re-verification).

Which model for which job, the phase-4 rule unchanged: writes/edits →
build model; reads/verifies/debugs → investigation model; a debugging
agent is the second kind; mutation-probing is investigation work.

### The worktree discipline

**Agents never share a worktree.**

```sh
git worktree add -q -b phase5/<milestone> /tmp/opencode/wt-phase5-<milestone> main
ln -sfn "$PWD/fixtures" /tmp/opencode/wt-phase5-<milestone>/fixtures
```

Commit there, never push, never touch the main worktree. Merge with
`--no-ff`. Seam collisions merge on the more-correct semantics with
committed tests unmodified; anything touching the projection, session
tier, write path, sandbox, safety checks, transcript pipeline, or blob
store escalates rather than guesses.

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
no freeze movement, no mechanism duplication, no log leaks — with
transcript-text log review as a named step). The orchestrator's
pre-check (`git diff --stat` shows test files) gates every
verification dispatch.

---

## 7. The gates

Nothing is merged until all of these hold:

- `make lint` is clean; `make test` is green (plus any new packages).
- The milestone's "Done when" predicates are **checkable and pass**,
  with numbers verified by an investigation subagent, including
  mutation evidence for the load-bearing assertions.
- **P0 only:** api.md has no contradicted row (checked, not eyeballed);
  authorship rule tested per principal; SURVEY names every surface;
  ADR-0052 amended.
- **P1 only:** player fetch of audio 404/0-bytes per surface; LAN-only
  capture works; consent copy pinned exact-string.
- **P2 only:** mock-engine CI green; fresh campaign defaults local
  with custom config present; disclosure warning verbatim;
  unreviewed transcript invisible to players (hostile).
- **P3 only:** proposal lands in the marked section; GM rewrite
  survives re-run byte-identical; promotion boundary hostile-green.
- **P4 only:** mood table-driven green with every tag reachable;
  cue plays with the network down (asserted no-fetch); display gets
  party-only metadata (hostile).
- No `fixtures/` in the diff (worker-built deterministic fixtures
  excepted and named); no unexplained vault churn; no audio blobs
  in the vault.
- No note content, segment text, message body, or credential in any
  log line; no segment text in audit records.
- No second implementation of anything the phase owns (one blob
  store, one segment path, one mood function, one preview pipeline,
  one seat resolver).
- No new networked dependency; the sidecar is pinned + checksummed,
  never bundled, never fallback-remote.

Phase 4's benchmark lesson holds: **time the full request/cadence set,
not the fastest query.** E2e stays 45/45 across all three browsers on
every merge touching `web/` (Xvfb recipe in
`docs/cross-browser-results.md`).

---

## 8. When to stop

Stop and report to the user when:

- **The sidecar distribution story fails.** If pinned download per
  platform cannot be made honest (checksums, versions, the
  missing-with-message path), the local-default decision needs a
  human — bundle, degrade, or drop.
- **A keyed cloud SDK is proposed or required.** Shipping API keys or
  provider SDKs is a supply-chain decision, not a milestone task.
- **A predicate cannot be met as written** without changing a decision.
- **The projection would widen, the sandbox loosen, or `api: 1` move.**
  Each needs its own ADR and a human.
- **Transcript sharing wants discovery granularity reopened.** The
  flat list held through F4; "met vs heard-of" needs its note type
  and a human disposal (session-writeback §6 punts per ADR-0050 D4).
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
- **What phase 6 — or maintenance — should start with**, given what
  phase 5 taught you.
- **What you would do differently**, given what you now know.

Be honest about what failed. **A phase that found nothing is a phase that
did not test anything.**
