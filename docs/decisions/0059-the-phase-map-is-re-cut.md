# ADR-0059: The phase map is re-cut — four phases become five, and 1.5 is abolished

- **Status:** Accepted
- **Date:** 2026-10-24
- **Amends:** the phase map in [`phase-plan.md`](../phase-plan.md), and the
  phase references in ADR-0001, ADR-0003, ADR-0004, ADR-0005, ADR-0006,
  ADR-0007, ADR-0008, ADR-0009, ADR-0010, ADR-0012, ADR-0014, ADR-0016,
  ADR-0018, ADR-0019, ADR-0021, ADR-0024, ADR-0032, ADR-0033, ADR-0041,
  ADR-0048, ADR-0050 and the rest
- **Addresses:** phase 2 had grown too big to be one phase, and phase 1.5
  was a phase that was never really a phase

## Context

Two things had gone wrong with the phase map, and neither was visible from
any single document.

**Phase 2 was too big.** "The System" held the formula engine, the dice
engine, the 5e pack, the SRD content vault, the Blades gate, the creation
wizard, character sheets, the rich editor's block widgets, the WASM plugin
tier, the full host ABI, the query language and the cross-browser harness.
That is a phase and a half. It contains a *rules layer* — how a game system
is expressed and rolled — and a *platform layer* — how other programs extend
and query the app — and they have nothing to do with each other except that
both are "phase 2".

**Phase 1.5 was not a phase.** It held exactly two things: the archive
import endpoint and the MCP plugin. A number between two integers is a
scheduling dodge, and it had one: the phase had no acceptance test of its
own, so "is phase 1.5 done?" had no answer that meant anything.

## Decision

### The map

| | Phase | What it is |
|---|---|---|
| 0 | The Prototype | Done. Knowledge, not code |
| 1 | **The Wiki (MVP)** | A campaign wiki, with secrets |
| 2 | **The System** | The rules and the content: mechanics, packs, content, sheets |
| 3 | **The Openings** | The app's boundaries with other programs: plugins, queries, MCP |
| 4 | **The Table** | Live play |
| 5 | **The Session** | Recording, transcription, music |

**Phase 1.5 is abolished.** Its two items land in phases 2 and 3, where
they have company and where each has an acceptance test.

### What moved

| Was | Now | Why |
|---|---|---|
| Phase 2 — the formula engine | **2** | The rules layer |
| Phase 2 — the dice engine | **2** | The rules layer |
| Phase 2 — the 5e pack, the SRD content vault, the Blades gate | **2** | The content, and the gate that proves the format is not 5e-shaped |
| Phase 2 — the creation wizard, character sheets | **2** | What the rules layer is *for* |
| Phase 2 — the rich editor's block widgets, callouts, stat blocks, the extension registry | **2** | Rendering the content the rules layer produced |
| Phase 1.5 — the archive import endpoint | **2** | Content arriving, which is what phase 2 is about |
| Phase 2 — the WASM plugin tier, the full host ABI | **3** | Other programs extending the app |
| Phase 2 — the query language for `list_by` | **3** | Other programs asking the app questions |
| Phase 1.5 — the MCP plugin | **3** | A program reading the vault, and it needs the query language |
| Phase 2 — the cross-browser harness | **3** | It gates the plugin host's *UI* tier, which is phase 3 |
| Phase 3 — live sessions, seats, session codes, the scratch log | **4** | Unchanged in content |
| Phase 3 — the encounter runner, the tabletop, chat, the shared display | **4** | Unchanged in content |
| Phase 3 — the session write-back contract | **4** | Unchanged in content |
| Phase 4 — recording, transcription, session logs, music | **5** | Unchanged in content |

### The two phases, and their acceptance tests

The old phase 2 was two things wearing one number. Split on that seam:

**Phase 2 — The System.** Everything inside the app that makes it know a
game: formulas, dice, packs, content, sheets, the wizard, the editor's
content support, and the way content arrives.

> **Done when** two deliberately differently-shaped game systems both work
> end to end — the 5e pack with the SRD content, and a second system that
> assumes nothing about 5e — and a GM can import an archive into a
> campaign.

**Phase 3 — The Openings.** Everything at the app's boundary with other
programs: plugins that extend it, a query language that lets anything ask
it questions, and MCP, which lets a model read a campaign.

> **Done when** a third party can write a WASM plugin that installs without
> rebuilding the app; a program can query the vault with the fixed
> predicate language over REST or MCP; and the shadow host is verified in
> Firefox and Safari as well as Chromium.

The names are the point. "The System" was true of phase 2 in the sense that
it was *about* game systems — but it was also true that phase 2 contained
the plugin sandbox, which has nothing to do with games. **The Openings**
says what phase 3 actually is: the app stops being closed.

### The dependency edges hold

The re-cut is only valid if nothing needs something that comes later.

- The **tabletop** (phase 4) needs the dice engine and the formula engine
  (phase 2), and the plugin tier if the board is a plugin (phase 3). ✓
- **MCP** (phase 3) needs the query language (phase 3) and the typed layer
  the vault index already produces. ✓
- **Transcription** (phase 5) needs the dice log for correlation
  (ADR-0008) and the session tier (phase 4). ✓
- **Import** (phase 2) needs nothing but the vault reader (phase 1). ✓
- The **pack linter** (phase 2) needs the pack format, not the query
  language. ✓

### Two consequences that are recorded rather than hidden

**Phase 1's M6 criterion changes.** It said "the query language of ADR-0047
is phase 2"; it is now **phase 3**, and phase 1's equality-only filters
stay as they are. Phase 1 is not larger or smaller.

**The cross-browser harness moves from phase 2 to phase 3.** It gates the
shadow host's *UI* tier, which is now phase 3's. Phase 1 ships surface
hosts, so **the cross-browser verification of phase 1's surface hosts is
not yet done** — that is a known gap, and it is why the harness is listed
as a phase-3 item rather than quietly assumed to be covered.

## Consequences

**Good:**

- **Every phase has an acceptance test that means something.** Phase 1.5
  never had one, which is why it did not deserve to be a phase.
- **Phase 2 is a phase again**, and its two halves — mechanics and content
  — are related enough to be one thing.
- **Phase 3 is the platform phase**, and it is where the app's
  extensibility story is finished: the plugin tier, the query language and
  MCP are one idea, not three.
- **The import endpoint stops being a floating phase.** It arrives with the
  content, which is what it is for.

**Bad:**

- **Every phase reference in the design set has moved.** Thirty-odd ADRs
  and a dozen documents. Each ADR that names a phase carries a dated note
  pointing here, so that a stale pointer is visible rather than silent.
- **Phase 3 is new, and new phases are guesses.** It is the phase with the
  least design behind it, because the plugin tier, the query language and
  MCP were each designed as part of something else. It will need a plan of
  its own before phase 2 is finished.
- **The tabletop is now the fourth phase to wait for.** Anyone hoping to
  play at the table is waiting three phases longer than the map used to
  promise — though the work is the same work, in a different order.

**Neutral:**

- **The phase map is still canonical** and still the only place that says
  what is in each phase. The ADRs say *what* was decided; the map says
  *when*.
- Nothing was dropped. The only change is the order and the naming.

## A note on the ADR phase references

The ADRs are immutable in their *decisions*, and several of them name a
phase in passing — "the dice engine is phase 2", "the tabletop is phase 3".
Those sentences were true when they were written and are now false, because
the map moved underneath them.

They are **not** rewritten, because a decision's reasoning does not change
when a schedule does. Each affected ADR carries a dated note instead:

> **Phase references renumbered 2026-10-24.** See ADR-0059 for the mapping.

That is the same treatment every other amendment in this project has got,
and it is the reason the ADR set can still be read as a record of what was
decided and when.
