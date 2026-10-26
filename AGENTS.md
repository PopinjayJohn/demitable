# AGENTS.md

> **Demitable** — a local-first campaign operating system for tabletop RPGs.
> A campaign is a plain directory of markdown files. A single Go binary serves
> a web app over it: the GM browses and edits their campaign, and their
> players get a *filtered* view of the same campaign through a server-side
> projection that hides GM-only content.
>
> This file is the brief for an agent working in this repository. It says
> where the truth is, what the rules are, and what "done" means. **Read it
> before changing anything.**

---

## 1. Where the truth is

Read in this order. Everything else is detail.

| Read | For |
|---|---|
| [`docs/decisions/README.md`](docs/decisions/README.md) | **The index of 59 ADRs.** The decisions live here, and the ADR index says what each one decided |
| [`docs/phase-1-plan.md`](docs/phase-1-plan.md) | **What to build.** The MVP's milestones, each with a checkable "Done when" predicate |
| [`docs/design.md`](docs/design.md) | The living design: the vault schema, the visibility model, the phases |
| [`docs/tech-stack.md`](docs/tech-stack.md) | The stack, the package layout, and the logging rules |
| [`docs/api.md`](docs/api.md) | The REST and WebSocket contract |
| [`docs/plugin-api.md`](docs/plugin-api.md) | The plugin API |
| [`docs/threat-model.md`](docs/threat-model.md) | **What is defended and what is not.** Read this before touching the projection or the logs |
| [`docs/design-system.md`](docs/design-system.md) | The token layer, the type scale, the colour rules |
| [`docs/phase-0-report.md`](docs/phase-0-report.md) | What phase 0 found, and the three lessons it would tell the next person |
| [`docs/phase-1-report.md`](docs/phase-1-report.md) | What phase 1 found, and the fourteen findings it would tell the next person |
| [`docs/phase-1-brief.md`](docs/phase-1-brief.md) | The brief for the phase-1 orchestration agent |

**The phase map** — [`docs/phase-plan.md`](docs/phase-plan.md) — says what
each phase contains. Phase 1 is the MVP; phase 0 is done.

---

## 2. The rules that matter

These are the ones that get broken by accident.

### Accepted ADRs are immutable

**A finding that contradicts a decision produces a *new* ADR that amends
it — never an edit in place.** ADR-0050 amends ADR-0033; ADR-0056 amends
ADR-0045; ADR-0058 amends ADR-0031. An amendment to an accepted ADR is a
finding the project failed to have.

### `spikes/` is disposable. Two other directories are not.

| Path | What it is |
|---|---|
| `spikes/` | **Disposable.** Phase 0's spikes. Their *answers* are in the ADRs; their code is thrown away. Do not build on them, and do not delete them without reading what they settled |
| `dev/demo-campaign/` | **A deliverable.** Phase 1's demo campaign, 21 real notes, written to test the schema. It is the schema's test, and it is the yardstick for M5c and M7c |
| `internal/vault/frontmatter/` | **Production code, promoted.** The extractor. Load-bearing: ADR-0021 and ADR-0049's round-trip contract depends on it. 56 tests, including one that runs against the real corpus |
| `fixtures/` | **Transient and gitignored.** The SRD corpus. It is a build input, not a dependency. Never commit it. It may be absent |

### The projection is the trust boundary

The visibility model computes **audiences** — the intersection of every
applicable constraint, then widened by the grants that are permitted to
widen it (ADR-0057). It is **not** an ordered rule list, and that is
deliberate: "the first rule that applies wins" meant some rules never ran,
and the rules that lost were the narrowing ones.

**The deny side is proven; the allow side had bugs.** Read ADR-0057 before
changing the resolver, and run the projection's property test afterwards.

### A log line names a file and a construct, never a passage

```go
// right
slog.ErrorContext(ctx, "failed to parse", "path", path, "construct", "folded scalar")

// wrong — a leak
slog.ErrorContext(ctx, "failed to parse: "+string(content))
```

Attribute redaction is enforced by a handler (ADR-0058), so a credential
cannot reach a log by accident. **The message is free text and cannot be
redacted** — that is the one part of the rule that is not mechanical.

### The reserved field set is twelve names

`type` · `uid` · `name` · `system` · `campaign` · `kind` · `player` ·
`workflow` · `visibility` · `grants` · `revealed` · `discovered`.

Bare, top-level, and **simple-typed**, because Obsidian's Bases cannot
query nested properties (ADR-0040). Anything else in a note's frontmatter
is the GM's own, and the app leaves it alone.

The reserved layer also declares a **visibility per field**: `discovered`
and `grants` are owner-scoped; everything else inherits the note's
(ADR-0050, ADR-0057).

---

## 3. Commands

```
make build     the full pipeline -> dist/demitable
make test      go test ./...
make lint      gofmt, go vet
make fmt       gofmt -w
make run       the server on :8080
make corpus    the golden-corpus suites (needs fixtures/)
make check     fmt + vet + test, what CI runs
make help      every target
```

`make test` and `make lint` must be green before a change is finished.
There is no frontend yet — `web/` does not exist — so the Svelte targets
are not wired up.

---

## 4. Conventions

**Go.** Standard library first; **a dependency is a cost** (tech-stack §1).
Pure Go, no cgo, one static binary, cross-compiled for five platforms.
Errors are wrapped with `%w` and a package prefix. `gofmt` is not
optional.

**Tests.** `go test`. Table-driven where it helps. A test that asserts a
contract is worth more than a test that asserts an implementation — the
corpus and projection suites assert *properties*, and the extractor's
suite asserts that patching one field leaves every other byte alone.

**Comments.** Explain *why*, and what the alternative was. A comment that
restates the code is noise; a comment that records a measurement or a
rejected alternative is the thing a reader will need in six months.

**Commits.** One per milestone or sub-milestone. The message says what
changed and why, in the imperative. Do not push without being asked.

**Prose.** The same standard as the code. The design documents are read
more often than they are written, and a vague sentence in an ADR is a bug
that survives for years.

---

## 5. What "done" means

**A milestone is done when its "Done when" predicate in
[`docs/phase-1-plan.md`](docs/phase-1-plan.md) is checkable and passes.**
The predicates are written to be checkable — "in under 1 s wall clock, p95
over 5 runs", "1,692 of 1,692 files", "zero errors logged" — so a change is
finished when the number is right, not when the code looks finished.

Two things are also done:

- **The ADR is landed, or the proposal is written.** If a finding changes
  a decision, the change lands as a new ADR that amends the old one. An
  unlanded proposal is not a decision.
- **The finding is written down.** If a measurement changed a belief, the
  belief changes in a document, with the number attached.

---

## 6. What not to do

- **Do not edit an accepted ADR in place.** Write a new one that amends it.
- **Do not commit `fixtures/`.** It is a build input.
- **Do not put note content or a credential in a log line.**
- **Do not build on `spikes/`.** It is disposable.
- **Do not add a dependency without saying why in the ADR.**
- **Do not widen the visibility model.** A rule that makes a note *more*
  visible is a security change and needs its own ADR and its own test.
- **Do not push** unless you have been asked to.

---

## 7. If you are the first agent here

Phase 1's plan has **eight milestones**, each with a checkable predicate.
M1a is the vault: read, index, watch, frontmatter. The frontmatter half
of it **already exists** — `internal/vault/frontmatter/` — so M1a is mostly
wiring, and the interesting part is the watcher, the index and the journal.

Start with the **extractor's neighbours**, not the extractor: the vault
reader, the index, and the watcher. The extractor is done and pinned.

The three lessons from phase 0 are the most useful thing to carry:

1. **A schema you have never used is a schema you have not tested.**
2. **The expensive part is rarely the part you measured.**
3. **The leaks are in the accidental class** — in the things nobody designed
   to be a security decision.
