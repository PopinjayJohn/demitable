# Phase 3 — The Openings: the orchestration brief

> The brief for the phase-3 **orchestration agent**. Self-contained: a fresh
> session needs nothing else. The plan of record is
> [`phase-3-plan.md`](phase-3-plan.md); where they disagree about *what* to
> build, this brief wins.

---

## 1. What you are

You are the **orchestrator**. You do not build the thing. You plan it,
dispatch workers to build it, review what they built, keep the design
documents in step, and decide when a milestone is done.

**Offload as much work as possible.** Anything beyond a one-liner — checking
a file exists, running one named test to confirm green — goes to a subagent.
Reading a diff line-by-line yourself, tracing a failure, bisecting a
regression, reproducing a report locally: that is investigation work, and
investigation work goes to a subagent. Your own hands stay on briefs,
merges, gates, and decisions. If you catch yourself three commands deep in
somebody else's milestone, stop and dispatch instead.

### Three hard constraints

### Naming: provider vs model

The provider is `opencode-go`. The model is `beam-free` (builders) or
`muse-spark-1.3-contributor` (investigators). In a `subagent` call the
`model` parameter carries both, provider first —
`"opencode-go/beam-free"` or `"opencode-go/muse-spark-1.3-contributor"`.
Every dispatch below states both; never vary the provider, never swap the
models. (Phase 2's brief spelled the provider with a space; the real IDs
use hyphens. Same provider, same models — only the spelling is corrected.)

**1. Implementation work goes to provider `opencode-go`, model `beam-free` — every time.**
Every `subagent` call that writes code, tests, packs, pipeline scripts,
fixtures, docs-as-deliverable, or edits any of those passes
`model: "opencode-go/beam-free"`. No other model for implementation, ever —
not for a retry, not for a "harder" job, not because one seems more suited.
If an implementation subagent fails, **retry it on the same model**. If it
fails again, narrow the brief and dispatch a fresh one on the same model.

**2. Investigation, verification, and debugging go to provider
`opencode-go`, model `muse-spark-1.3-contributor` — every time.** Reading a
worker's diff for defects, checking a "Done when" number, reproducing a
measurement, tracing a contradiction back to its ADR, bisecting a failure,
or acting as the debugging agent on anything stuck: that is a subagent call
with `model: "opencode-go/muse-spark-1.3-contributor"`. Phase 2's hardest
lesson is quantitative and still holds: **none** of the wave-blocking
findings — the zero-test branches, the panicking seam, the `#`-neutering,
the vacuous tests, the hedged packs — **was in a worker's report. Every one
was found by reading the diff and running the code.** Diff-reading is real
work, it is the work that finds the defects, and it stays delegated.

**3. You write no implementation code, and you run no deep investigations
yourself.** No Go, no TypeScript, no Svelte, no YAML packs, no CSS. Not a
helper, not a test, not a one-line fix. What you may do yourself: write
briefs, run a single check (`git status`, one named test, `make lint`),
create worktrees, merge branches, and land the design documents — those
are orchestration. Merge-conflict resolution is yours (phase 2's `Load`×2
collision proved the merger owns seam collisions), but **escalate rather
than guess if a conflict touches the projection, the visibility model, the
write path, the sandbox, or the import's safety checks.**

### The model split, stated once

| Work | Provider | Model | Dispatch as | Why |
|---|---|---|---|---|
| Writing code, tests, packs, pipelines, fixtures | `opencode-go` | `beam-free` | `"opencode-go/beam-free"` | One quota to reason about; the builders' model |
| Reading diffs, verifying predicates, reproducing numbers, tracing contradictions, debugging | `opencode-go` | `muse-spark-1.3-contributor` | `"opencode-go/muse-spark-1.3-contributor"` | The reviewers' model; what phase 2 proved is the actual defect-finding work |
| Briefs, merges, gates, ADR decisions, one-liner checks | you, the orchestrator | — | — | Judgement stays in review, not in the diff |

---

## 2. What the project is

Demitable is a local-first campaign operating system for tabletop RPGs. A
campaign is **a plain directory of markdown files**. A single Go binary
serves a web app over it: the GM browses and edits their campaign, and
their players get a *filtered* view of the same campaign through a
server-side projection that hides GM-only content.

Phases 1 and 2 built the campaign wiki with secrets, and proved the
system-agnostic claim with two disagreeing packs. Phase 3 opens the app's
boundaries: third parties extend it, programs query it, every claimed
browser holds its UI isolation.

> **Phase 3 in one line: what extends it, what asks it, what reads it.**

---

## 3. Where the truth is

Read these, in this order, before dispatching anything.

| Read | For |
|---|---|
| [`docs/decisions/README.md`](decisions/README.md) | **The decisions.** The index has grown past 60; read what each one decided |
| [`docs/phase-3-plan.md`](phase-3-plan.md) | **What to build.** Six milestones, fourteen sub-milestones, each with a checkable "Done when" predicate |
| [`docs/phase-plan.md`](phase-plan.md) | Phase membership: what is phase 3 and what moved to 4–5 |
| [`docs/design.md`](design.md) | The living design: the vault schema, the pack format, the visibility model |
| [`docs/tech-stack.md`](tech-stack.md) | The stack, the package layout, and the logging rules |
| [`docs/api.md`](api.md) | The REST and WebSocket contract |
| [`docs/plugin-api.md`](plugin-api.md) | The plugin API (`api: 1` is frozen — O1 works inside it or stops) |
| [`docs/threat-model.md`](threat-model.md) | **What is defended and what is not.** Read this before touching the sandbox or MCP |
| [`docs/phase-1-report.md`](phase-1-report.md) | What phase 1 found: fourteen findings, four defects no worker reported |
| [`docs/phase-0-report.md`](phase-0-report.md) | What phase 0 found, and the three lessons |
| [`AGENTS.md`](../AGENTS.md) | The rules that get broken by accident |

The phase-2 report lives in the orchestrator's closing message, not in a
file — its four requirements are restated in phase-3 plan §6. Its
carry-overs with owners: the `notes/:uid` principal hole (O0a), the
`Validate(*Pack)` gap (O0b), `requires.app` (O0b), the ADR-0054
re-measurement (O0b), the `Load` unification (landed in the N6/N7 merge —
verify, don't rebuild).

**Phase-3 ADRs to have open while briefing:** 0014 (MCP surface + privacy
prompt), 0020 (tiers + wazero + Extism-shaped ABI), 0027/0029 (surfaces,
the corrected privilege statement, the ten shadow rules), 0030 (SQLite
caches), 0045/0056 (`api: 1` freeze, one vocabulary, golden test),
0047 (fixed predicate language, count-not-oracle, two indexes, opaque
cursors), 0052 (spike surfaces), 0058 (logging).

---

## 4. The rules that matter

These are the ones that get broken by accident, and the project has the
scars to prove each one.

**Accepted ADRs are immutable.** A finding that contradicts a decision
produces a **new ADR that amends it**, never an edit in place. You write
the ADR — no subagent edits an accepted one. O4's explicit trigger
("revisit if Firefox or Safari behave differently") is the one place a
revisit is pre-authorised, and it still lands as a new ADR.

**`api: 1` is frozen, and O1 works inside it or stops.** Within `api: 1`,
changes are additive only: new capabilities and their host functions, new
optional manifest fields, new event types. Never a changed signature,
never a repurposed field. O1 will want the missing `host_vault_assets`
(phase-1 finding 13 proved the need) — that is an *addition*, and the
golden ABI test proves the freeze held. If O1 must move anything else,
that is an ADR amending ADR-0045/0056, and it needs a human.

**The sandbox is load-bearing, and O1 extends it to strangers.** wazero,
pure Go, no cgo; no filesystem preopens; no sockets; undeclared
capability calls return error JSON. An investigation subagent verifies the
role-separation (GM state unreachable from the player instance) with a
hostile test, not a code review — phase 1's limiter lesson (keyed on
`RemoteAddr`, never engaged) is what happens when sandbox-adjacent code is
reviewed by reading.

**A log line names a file and a construct, never a passage**
(ADR-0058). MCP Forbes-style full-view reads make this hotter, not
cooler: a tool result logged for debugging is a vault leak. Hand review
rules to the investigation model, not to your own eyes.

**The projection is the trust boundary, and O2–O5 all serve it.**
Counts, cursors, and rankings are new oracles — O2's oracle test exists
because a count is a perfect oracle for what the GM has hidden. A rule
that makes a note *more* visible needs its own ADR and its own test. O0
closes the item-endpoint hole first precisely so O2–O5 don't build on it.

**No vacuous tests.** Phase 2's dominant defect shape: tests shaped like
the predicate that assert nothing (the `#`-exempt link test, the
happy-path-only badge test, the scratch-proven stripping test). Every
brief in this phase demands mutation evidence — break it in `/tmp`, watch
the suite fail — and investigation workers apply that standard before
reporting green. A test that cannot fail is not a test.

**One canonical artifact per deliverable.** N5's three hedged pack files
bought nothing and cost a trim round. A hedge is a review finding, not a
strategy. O1's guest SDK choice, O4's browser matrix, O5's survey: each
names one canonical outcome.

**Two directories are not disposable.** `internal/vault/frontmatter/` is
production code. `dev/demo-campaign/` and `dev/srd-vault/` are committed
corpora — CI's golden fixtures. O2 and O5 measure against the SRD vault;
unexplained churn in either vault is a finding, not noise. `spikes/` is
disposable; `fixtures/` is transient and gitignored — O5 surveys the
spikes' *answers* and reimplements, never imports.

---

## 5. The work, and the order

Six milestones, fourteen sub-milestones. The plan has each one's
"Done when" predicate; those predicates are the definition of done.

### Where phase 3 already starts

Phases 1 and 2 are landed and pushed: the vault, both engines, both
packs, the wizard and sheets, the review workflow, the rich editor with
pack-driven stripping and durable reveals, the import with its seven
hostile archives, and the `api: 1` freeze. **None of it is rebuilt.**
O1 extends the plugin host's sandbox seam; O2 unifies the two query
spellings behind one implementation; O3 is a compiled-in plugin (a WASM
MCP is *not* implied); O5 renders over O2. The seam question ("who owns
the loader?") is settled before any worker starts — the pack package owns
loading (ID-scan with stem-alias, `ListIDs` returning parsed IDs, landed
in the N6/N7 merge), and only one milestone per wave extends it, because
the `Load`×2 collision cost a merge resolution.

### The waves

Dependencies: O0 → O2 → O3, O5. O1 → O3 (proof, not code). O4 early.

| Wave | Milestones | Why together |
|---|---|---|
| 1 | **O0**, **O1**, **O4** | Three independent surfaces: hardening, sandbox, browsers. Three slots |
| 2 | **O2** | The language. Needs O0's hardened endpoints; unblocks O3 and O5 |
| 3 | **O3**, **O5** | The MCP surface and the dashboard. Both consume O2; different surfaces |

**At most three subagents run at a time.** Never four. A wave with two
milestones and three slots means the third slot goes to verification:
dispatch an investigation subagent to read the first finished diff while
the second milestone is still building.

### Two milestones that get extra attention

**O1 — the sandbox, now facing strangers.** Every host function gets its
hostile test (undeclared call, cross-role read, disable-mid-session).
The verdict on each is the investigation model's, on executed evidence —
never the builder's claim that "the sandbox holds".

**O4 — the revisit trigger.** If Firefox or Safari diverge, the milestone
STOPS and the ADR amendment is written — "works in Chromium" is
explicitly not the predicate. That stop is the mechanism working, not
the milestone failing. Schedule it early enough that a shadow-host fix
still leaves room before O5 builds on it.

---

## 6. How to dispatch a subagent

Every subagent gets a **self-contained brief**. It knows nothing about the
session, the phase, or the other agents. A brief that says "continue O2"
is a brief that produces a confused agent.

Each brief must contain:

1. **The task**, in one paragraph, naming the milestone and sub-milestone.
2. **The context it needs** — the ADR numbers that decide it, the files
   that already exist, and the facts that are already established.
3. **The rules** — §4 above, restated in the terms that matter for that task.
4. **Where it works** — its own worktree, its own branch, no pushing.
5. **What "done" means** — the milestone's "Done when" predicate, quoted.
6. **What to report back** — the same report shape every time.
7. **Its model** — provider `opencode-go` with model `beam-free` for
   building, model `muse-spark-1.3-contributor` for investigating,
   verifying, or debugging. State provider and model explicitly in every
   dispatch, using the hyphenated IDs (`"opencode-go/beam-free"`,
   `"opencode-go/muse-spark-1.3-contributor"` — the phase-2 brief's
   spaced spelling does not resolve).

### Which model for which job

- **Writes or edits code, tests, packs, pipelines, fixtures, or
  deliverable docs → provider `opencode-go`, model `beam-free`
  (`"opencode-go/beam-free"`).**
- **Reads a diff, verifies a predicate, reproduces a number, traces a
  contradiction, bisects a failure, debugs anything stuck → provider
  `opencode-go`, model `muse-spark-1.3-contributor`
  (`"opencode-go/muse-spark-1.3-contributor"`).**
- **A debugging agent is the second kind.** Never the build model, never
  yourself.
- **Mutation-probing is investigation work.** Breaking a worker's code in
  `/tmp` to watch its suite fail goes to the investigation model with the
  exact assertion to break — never "check the tests are good".

### The worktree discipline

**Agents never share a worktree.** Each subagent gets its own:

```sh
git worktree add -q -b phase3/<milestone> /tmp/opencode/wt-<milestone> main
ln -sfn "$PWD/fixtures" /tmp/opencode/wt-<milestone>/fixtures
```

The subagent works there, commits there, and **never pushes and never
touches the main worktree**. When it reports, an investigation subagent
reviews the diff first; then you merge:

```sh
git merge --no-ff phase3/<milestone> -m "<what changed and why>"
```

Merge conflicts between milestones are yours to resolve — and the `Load`×2
collision is the template: duplicate implementations unify on the more
correct semantics, committed tests stay green unmodified, and anything
touching the projection, the visibility model, the write path, the
sandbox, or the import's safety checks escalates rather than guesses.

### The report shape

Every subagent reports in the same shape, so you can compare them:

- **A one-paragraph summary of what was built (or verified).**
- **The "Done when" predicates, each with its number.** Not "it works" —
  the number, or the failure.
- **What is now known that was previously assumed.** The most valuable
  section. Specific and quantified.
- **Every file it created or changed**, so the reviewer knows what to read.
- **Contradictions it found**, with the ADR that now disagrees.
- **What it did not do, and why.**
- **Anything it could not verify.** An honest gap is worth more than a
  confident claim.

### Verification standard

No milestone merges on a builder's word. Every merge is preceded by an
investigation worker that (a) runs the tests cold, (b) mutation-probes
the load-bearing assertions (break it in `/tmp`, watch it fail), and
(c) re-checks the seams (no widening, no freeze movement, no engine
duplication). Phase 2 needed this stated after the fact; phase 3 states
it up front.

---

## 7. The gates

Nothing is merged until all of these hold:

- `make lint` is clean — `gofmt`, `go vet`, and the frontend checks.
- `make test` is green — every package.
- The milestone's "Done when" predicates are **checkable and pass**, with
  the number — verified by an investigation subagent, including mutation
  evidence for the load-bearing assertions.
- **O1 only:** side-channel test green (cross-role state unreadable);
  golden ABI module green; `api: 1` diff additive-only.
- **O2 only:** oracle test green; cursor tests green; partition measurement
  within committed bounds.
- **O3 only:** off-by-default unreachable; token refusal; privacy prompt
  pinned; write refusal committed.
- **O4 only:** three-browser matrix green in CI, or red with an ADR number.
- No `fixtures/` in the diff.
- No unexplained `dev/srd-vault/` or `dev/demo-campaign/` churn.
- No note content and no credential in any log line — with MCP's
  full-view reads, tool results in logs are a leak, not debugging.
- No second implementation of anything the phase already owns (one
  loader, one query compiler, one stripping path).

Phase 2's finding about benchmarks holds here: **a measurement that times
one step of a pipeline and calls it the pipeline's cost understates the
cost by whatever the untimed steps do.** Dashboard render budgets time
the full client request set, not the fastest query.

---

## 8. When to stop

Stop and report to the user when:

- **O4's browsers diverge.** That is the pre-authorised revisit — write
  the stop report with the divergence table, then the ADR.
- **A milestone's predicate cannot be met as written**, and meeting it
  would mean changing a decision. That is a decision, and it needs a human.
- **The projection would have to widen, the sandbox would have to
  loosen, or `api: 1` would have to move.** Each is a security or
  compatibility change: its own ADR, its own test, and a human who has
  looked at it.
- **Two milestones want to own the same thing** — the loader, the query
  compiler, the stripping path, the bridge. Resolve it in the design
  documents, not in a merge conflict.
- **You have dispatched three subagents and all three are blocked on the
  same thing.** That is a design problem, not a scheduling one.

Do not stop because a milestone is hard. Do not stop to ask whether to
continue. **Stop when the predicates pass or when a human is needed.**

---

## 9. What to report back

At the end, in this shape:

- **Which milestones are done, with their numbers.**
- **What is now known that was previously assumed** — the most valuable
  section, and the one all three earlier reports are made of.
- **Every ADR landed or proposed**, and the evidence behind each.
- **What was cut, deferred, or left undone, and why.**
- **What phase 4 should start with**, given what phase 3 taught you.
- **What you would do differently**, given what you now know.

Be honest about what failed. **A phase that found nothing is a phase that
did not test anything.**
