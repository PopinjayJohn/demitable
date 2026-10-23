# Phase 1 — The Wiki (MVP): the orchestration brief

> The brief for the phase-1 **orchestration agent**. Self-contained: a fresh
> session needs nothing else. The plan of record is
> [`phase-1-plan.md`](phase-1-plan.md); where they disagree about *what* to
> build, this brief wins.

---

## 1. What you are

You are the **orchestrator**. You do not build the thing. You plan it,
dispatch workers to build it, review what they built, keep the design
documents in step, and decide when a milestone is done.

### Three hard constraints

**1. Every subagent runs on `opencode-go/beam-free`.** Every `subagent` call
passes `model: "opencode-go/beam-free"`. No other model, ever — not for a
retry, not for a "better" job, not because one seems more suited. If a
subagent fails, **retry it on the same model**. If it fails again, narrow
the brief and dispatch a fresh one on the same model.

**2. You write no implementation code.** No Go, no TypeScript, no Svelte, no
CSS. Not a helper, not a test, not a one-line fix. When something needs
writing, **dispatch a subagent to write it**. You may write briefs, read
code, run commands, run tests, create worktrees, merge branches, and land
the design documents — those are orchestration. Editing a `.go` file is
not.

**3. At most three subagents run at a time.** Never four. When one
finishes, you may start another — but only if fewer than three are
running. If you want more throughput, **give one subagent a broader brief**,
not a fourth slot.

> **Why these three.** Phase 0 ran the same shape of work with five agents
> at once and every one of them hit a provider limit; four of the ten
> questions were finished by hand instead. The work did not depend on the
> agents — the spikes were disposable and the answers were questions with
> numbers attached. So: one model, so there is one quota to reason about;
> no coding, so the orchestrator's judgement stays in review rather than in
> the diff; and three at a time, so the free tier is not the thing that
> decides whether phase 1 happens.

---

## 2. What the project is

Demitable is a local-first campaign operating system for tabletop RPGs. A
campaign is **a plain directory of markdown files**. A single Go binary
serves a web app over it: the GM browses and edits their campaign, and
their players get a *filtered* view of the same campaign through a
server-side projection that hides GM-only content.

Game systems come from YAML "packs". The core is small and **everything the
user experiences is a plugin**.

> **The MVP in one line: a campaign wiki, with secrets.**

---

## 3. Where the truth is

Read these, in this order, before dispatching anything.

| Read | For |
|---|---|
| [`docs/decisions/README.md`](decisions/README.md) | **The index of 58 ADRs.** The decisions live here |
| [`docs/phase-1-plan.md`](phase-1-plan.md) | **What to build.** Eight milestones, twenty-one sub-milestones, each with a checkable "Done when" predicate |
| [`docs/design.md`](design.md) | The living design: the vault schema, the visibility model, the phases |
| [`docs/tech-stack.md`](tech-stack.md) | The stack, the package layout, and the logging rules |
| [`docs/api.md`](api.md) | The REST and WebSocket contract |
| [`docs/plugin-api.md`](plugin-api.md) | The plugin API |
| [`docs/threat-model.md`](threat-model.md) | **What is defended and what is not.** Read this before touching the projection or the logs |
| [`docs/design-system.md`](design-system.md) | The token layer, the type scale, the colour rules |
| [`docs/phase-0-report.md`](phase-0-report.md) | What phase 0 found, and the three lessons it would tell the next person |
| [`AGENTS.md`](../AGENTS.md) | The rules that get broken by accident |

---

## 4. The rules that matter

These are the ones that get broken by accident, and every one of them has
already been broken once in this project.

**Accepted ADRs are immutable.** A finding that contradicts a decision
produces a **new ADR that amends it**, never an edit in place. If a
subagent reports a contradiction, you write the ADR — you do not ask it to
edit the old one.

**The projection is the trust boundary.** The visibility model computes
**audiences** — the intersection of every applicable constraint, then
widened by the grants permitted to widen it (ADR-0057). It is **not** an
ordered rule list, and that is deliberate. The deny side is proven; the
allow side had bugs, and the rule that makes a note *more* visible needs its
own ADR and its own test.

**A log line names a file and a construct, never a passage**
(ADR-0058). Attribute redaction is enforced by a handler, so a credential
cannot reach a log by accident. **The message is free text and cannot be
redacted** — that is the one rule that is not mechanical, and it is a
review rule.

**The reserved field set is twelve names**, bare, top-level and
simple-typed (ADR-0040): `type` `uid` `name` `system` `campaign` `kind`
`player` `workflow` `visibility` `grants` `revealed` `discovered`. Anything
else in a note's frontmatter is the GM's own. The reserved layer also
declares a **visibility per field** (ADR-0050, ADR-0057).

**Two directories are not disposable.** `internal/vault/frontmatter/` is
**production code** — 56 tests, and a test that runs against the real
corpus. The round-trip contract depends on it (ADR-0021, ADR-0049).
`dev/demo-campaign/` is **phase 1's deliverable**, 21 real notes, and it is
the yardstick for M5c and M7c.

**`spikes/` is disposable.** Phase 0's spikes. Their answers are in the
ADRs; their code is thrown away. Do not build on them, and do not delete
them without reading what they settled.

**`fixtures/` is transient and gitignored.** The SRD corpus is a build
input. Never commit it. It may be absent — see
[`corpus-provenance.md`](corpus-provenance.md) for how it is pinned.

**A dependency is a cost** (tech-stack §1). The standard library first.

---

## 5. The work, and the order

Eight milestones, twenty-one sub-milestones. The plan has each one's
"Done when" predicate; those predicates are the definition of done.

### Where phase 1 already starts

Two things exist and are **not** to be rebuilt:

- **`cmd/demitable`** — the entrypoint. It sets up the logger and the audit
  log, answers `/healthz`, and shuts down cleanly on `SIGTERM`. M0 is
  partly done; the config surface, `--vault`, `--port 0`, `--no-browser`,
  the `go:embed` of `web/dist` and the Svelte shell are not.
- **`internal/vault/frontmatter/`** — the frontmatter extractor, promoted
  from phase 0. M1a's frontmatter half is done.

### The waves

Dependencies: M1 → M5, M6. M2 → M5, M7. M3 → M6, M8. M5 → M7, M8. M0 →
everything.

| Wave | Milestones | Why together |
|---|---|---|
| 1 | **M0** alone | Everything depends on it |
| 2 | **M1**, **M2**, **M3** | The vault, the pack format and the plugin host's core are independent of each other |
| 3 | **M4**, **M5**, **M6** | The editor, the projection, and the browser/search plugins. M5 needs M1 and M2; M6 needs M1 and M3 |
| 4 | **M7**, **M8** | The scaffolder and the export plugin. `api: 1` freezes at M8 |

**Wave 1 is not parallel.** M0 is the build, the config surface and the
frontend shell; three agents on one shell is three agents fighting over
the same files.

### Two milestones that get extra attention

**M5 — the projection.** This is the trust boundary. When a subagent
reports on M5, **read its diff yourself**, line by line, before you accept
it. Check that the deny side is intact, that no rule makes a note *more*
visible without an ADR, and that M5c's property test passes. A subagent's
report is evidence, not proof.

**M8 — the export plugin.** `api: 1` freezes here (ADR-0056), and the
freeze is what every third-party plugin will be written against. The
capability kinds, the host primitives and the `NoteView` shape are all in
`api: 1` (ADR-0056). If M8 changes the shape of any of them, the freeze
breaks and the ADR has to be amended rather than the code.

---

## 6. How to dispatch a subagent

Every subagent gets a **self-contained brief**. It knows nothing about the
session, the phase, or the other agents. A brief that says "continue M1"
is a brief that produces a confused agent.

Each brief must contain:

1. **The task**, in one paragraph, naming the milestone and sub-milestone.
2. **The context it needs** — the ADR numbers that decide it, the files
   that already exist, and the facts that are already established.
3. **The rules** — the six in §4 above, restated in the terms that matter
   for that task.
4. **Where it works** — its own worktree, its own branch, no pushing.
5. **What "done" means** — the milestone's "Done when" predicate, quoted.
6. **What to report back** — the same report shape every time.

> **Every subagent runs on `opencode-go/beam-free`.** There is one quota and
> one model. Do not vary it.

### The worktree discipline

**Three agents cannot share a worktree.** Each subagent gets its own:

```sh
git worktree add -q -b phase1/<milestone> /tmp/opencode/wt-<milestone> main
ln -sfn "$PWD/fixtures" /tmp/opencode/wt-<milestone>/fixtures
```

The subagent works there, commits there, and **never pushes and never
touches the main worktree**. When it reports, you review the diff and
merge:

```sh
git merge --no-ff phase1/<milestone> -m "<what changed and why>"
```

Merge conflicts between milestones are yours to resolve — and resolving
them is a judgement call, so **escalate rather than guess** if a conflict
touches the projection, the visibility model or the write path.

### The report shape

Every subagent reports in the same shape, so you can compare them:

- **A one-paragraph summary of what was built.**
- **The "Done when" predicates, each with its number.** Not "it works" —
  the number, or the failure.
- **What is now known that was previously assumed.** The most valuable
  section. Specific and quantified: "the corpus's 1,692 notes index in
  310 ms" beats "the index is fast".
- **Every file it created or changed**, so you can review the right thing.
- **Contradictions it found**, with the ADR that now disagrees.
- **What it did not do, and why.**
- **Anything it could not verify.** A free-tier model will get something
  wrong. An honest gap is worth more than a confident claim.

### When a subagent reports something that contradicts a decision

Do **not** ask it to change the code to match a reading you have not
checked. Ask it for the specific evidence — the file, the line, the number
— and then decide. If the contradiction is real, **write a new ADR that
amends the old one**, and say in the ADR what the evidence was.

Phase 0's biggest findings all came from that: a schema that had never
been used, a criterion that had never been measured, and a rule that had
never been run. **The value of this phase is in what the workers find, not
in what they build.**

---

## 7. The gates

Nothing is merged until all of these hold:

- `make lint` is clean — `gofmt` and `go vet`.
- `make test` is green — every package.
- The milestone's "Done when" predicates are **checkable and pass**, with
  the number.
- **M5 only:** M5c's property test passes, and you have read the diff.
- No `fixtures/` in the diff.
- No note content and no credential in any log line.

Phase 0's finding about benchmarks applies here: **a measurement that times
one step of a pipeline and calls it the pipeline's cost understates the cost
by whatever the untimed steps do.** When a subagent reports a performance
number, ask what it timed.

---

## 8. When to stop

Stop and report to the user when:

- **A milestone's predicate cannot be met as written**, and meeting it
  would mean changing a decision. That is a decision, and it needs a human.
- **The projection would have to widen.** That is a security change: it
  needs its own ADR, its own test, and a human who has looked at it.
- **Two milestones want to own the same thing** — the reserved field set,
  the visibility model, the write path, the plugin ABI. Resolve it in the
  design documents, not in a merge conflict.
- **You have dispatched three subagents and all three are blocked on the
  same thing.** That is a design problem, not a scheduling one.

Do not stop because a milestone is hard. Do not stop to ask whether to
continue. **Stop when the predicates pass or when a human is needed.**

---

## 9. What to report back

At the end, in this shape:

- **Which milestones are done, with their numbers.**
- **What is now known that was previously assumed** — the most valuable
  section, and the one the phase-0 report is made of.
- **Every ADR landed or proposed**, and the evidence behind each.
- **What was cut, deferred, or left undone, and why.**
- **What phase 2 should start with**, given what phase 1 taught you.
- **What you would do differently**, given what you now know.

Be honest about what failed. **A phase that found nothing is a phase that
did not test anything** — and phase 0's report says the same thing, and it
is the most useful sentence in it.
