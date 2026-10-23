# ADR-0060: The binary has no size limit — the invariant is one file with no runtime dependencies

- **Status:** Accepted
- **Date:** 2026-10-24
- **Amends:** the M0 "Done when" criterion in
  [`phase-1-plan.md`](../phase-1-plan.md), and the binary-size gate in
  `.github/workflows/ci.yml`
- **Addresses:** a criterion that would have failed on legitimate phase-1
  work, for a reason that had nothing to do with the work

## Context

Phase 1's plan required the cross-compiled binary to be **under 12 MB**,
and CI enforced it as a hard gate: a build over 12,582,912 bytes failed the
job.

The number has two problems.

**It is stale, and it was never a decision.** It descends from a phase-0
research measurement of **7.6 MB** for a binary that did **not** yet embed
the frontend. Nothing in the ADR set asserts a size limit — ADR-0005
promises *"single binary, no cgo"*, and ADR-0030 records that *"the cost is
binary size: a transpiled SQLite adds a few megabytes"*. One is a shape and
one is a cost. The cap was a plan-level number that hardened into a gate
without ever being argued for.

**It is about to fire for the wrong reason.** Measured on M0's actual
binary, with the frontend embedded:

| | Bytes | |
|---|---:|---|
| M0's binary, `linux/amd64` | 11,288,736 | **10.77 MiB** |
| the cap | 12,582,912 | 12.00 MiB |
| **headroom** | **1,294,176** | **1.23 MiB** |

For scale: the transpiled SQLite costs **3.84 MiB** of that, and the
embedded frontend bundle **28,672 bytes**. The rest is the Go runtime and
`net/http`.

M1 has not yet been written, and phase 1 still owes: the **vault service**,
the **plugin host**, the **editor** (CodeMirror 6 plus a markdown parser),
the **embedded packs**, and the FTS5 index. The editor alone is a
front-end dependency several times the size of the entire current bundle.

So the cap would be breached within the phase, by ordinary progress. And a
cap that fires on legitimate work is worse than no cap: the person whose
build breaks it will look for the cheapest way to get under the line, and
the cheapest ways are all bad — drop the embedded frontend, drop SQLite,
drop the packs embed, or link dynamically and break the one-file promise.
**The criterion would have made the architecture worse.**

This is phase 0's second lesson, arriving on schedule: *the expensive part
is rarely the part you measured*. What was measured was a 7.6 MB binary.
What the gate governs is a 25 MB one.

## Decision

**The binary has no size limit. There is no maximum size, and no build
fails because it is too large.**

The invariant ADR-0005 actually promised is kept, and it is the one that
matters:

> **One file. No cgo. No runtime dependencies.**

That is checkable, and CI checks it:

- `CGO_ENABLED=0` builds succeed for `linux/amd64`, `linux/arm64`,
  `darwin/amd64`, `darwin/arm64`, `windows/amd64`;
- the produced binary is **statically linked** — no dynamic library
  dependencies, nothing for the user to install.

**Size is still reported, and is still measured.** CI prints the byte count
of every target, and the release matrix still records it. A regression that
inflates the binary is *visible* in the build log and in the diff. It is
simply not a **gate**.

The distinction is the whole point: a measurement you watch is evidence; a
threshold you enforce is a constraint, and this one was wrong.

### Why not a much larger limit instead?

A 50 MB or 100 MB limit would have the same defect with more room to
breathe. Any fixed threshold eventually either fails on legitimate work or
sits so far above the real size that it catches nothing. The failure mode
is not "the limit was set too low" — it is "a size limit is the wrong
shape of constraint for an artifact whose size is driven by features nobody
has written yet".

## Consequences

**Good:**

- **The build stops being a hostage.** Nothing about the editor, the packs,
  the plugin host or FTS5 has to be traded against a byte count.
- **The real invariant is enforced.** "Statically linked, no runtime
  dependencies" is what ADR-0005 promised, and it is now checked directly
  rather than inferred from a number.
- **The size stays visible.** CI still prints it, so an accidental 100 MB
  embed shows up in the log and in review — as a thing a human notices,
  not as a thing a broken build hides.
- **The criterion survives the phase.** It will still be true when the
  editor, the packs and the plugin host are in.

**Bad:**

- **Nothing stops a runaway dependency.** A transitive dependency that
  adds 50 MB would merge without a red build. The mitigation is review and
  the reported number, not a gate — and the number is reported.
- **One fewer automated trip-wire.** The build is now one check less
  strict.

**Neutral:**

- **ADR-0030's "a few megabytes" is still true**, and now has the exact
  figure behind it: **3.84 MiB** for `modernc.org/sqlite` on
  `linux/amd64`.
- **The research measurement is not wrong**, it is *about a different
  binary*. 7.6 MB was the size before the frontend was embedded.

## A note on what was changed

The plan's M0 criterion and the CI gate are amended together with this
decision, because a criterion and the check that enforces it are one
thing. Nothing else in the plan's M0 criterion changes.
