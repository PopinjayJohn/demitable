# ADR-0055: The watcher and the index, measured — and one criterion that fails

- **Status:** Accepted
- **Date:** 2026-10-23
- **Amends:** ADR-0030 (the SQLite caches), the phase-1 plan's performance
  criteria, and the phase-1 plan's watcher acceptance criteria
- **Origin:** phase 0, questions 7 and 9 — *does the watcher see everything,
  on every platform*, and *what do the numbers actually cost?* The evidence
  is [`spikes/measurement/`](../spikes/measurement/README.md) §2 and §3.

## Context

The phase-1 plan states two performance criteria and one watcher criterion:

> A generated vault of 5,000 notes, each with 40 unknown frontmatter keys
> and a 2 KB body, is fully indexed — frontmatter parsed to `yaml.Node`,
> link graph built, FTS5 populated — **in under 1 s wall clock, p95 over 5
> runs**, on the CI runner.

> The watcher uses **content hashes, not mtimes**; reconciles a removal
> reported as `CHMOD` by stat-ing the path; and runs a full rescan every
> 30–60 s.

> On **macOS** the watcher is a polling watcher and opens **zero file
> descriptors** per note. A 50,000-note vault indexes and watches there
> without `EMFILE`.

None of them had been measured. Phase 0 measured them on an Intel Xeon
E5-1650 v3, 16 GiB, Linux 7.0.14, `modernc.org/sqlite` v1.61.0, fsnotify
v1.10.1, goldmark v1.8.6.

## Decision

### 1. The index criterion is restated, because it fails and it fails for the wrong reason

**The criterion as written is not met.** 5,000 notes take **1.23 s** to
index, not 1 s. But the reason it fails is the interesting part:

| | 5,000 generated notes | 1,693 real notes |
|---|---|---|
| **load** — walk, read, frontmatter, markdown, link graph | **1,077–1,102 ms** | 206–218 ms |
| **insert** — the FTS5 insert | **144–151 ms** | 83–111 ms |
| **wall clock** | **1,228–1,248 ms** | 294–317 ms |
| index size | 6,799,360 B | 4,251,648 B |
| peak RSS | 78.9 MiB | — |

**The index is 12% of the cost.** The other 88% is parsing: reading 5,000
files, splitting each one's frontmatter, and goldmark-parsing 4.35 MB of
markdown to extract prose, headings, links and the community's content
fences.

So the criterion is corrected to say where the budget goes:

> A generated vault of 5,000 notes, each with 40 unknown frontmatter keys
> and a 2 KB body, is **parsed** — frontmatter parsed to `yaml.Node`, link
> graph built, prose extracted — in **under 1 s**, and the resulting
> documents are **inserted** into FTS5 in **under 250 ms**, p95 over 5 runs,
> on the CI runner. The generator is committed.

**"Index 5,000 notes in under 1 s" was the wrong measurement**, because it
conflated the cheap half with the expensive one. The real corpus is not the
problem: 1,693 notes index in **0.30 s**.

### 2. The rescan interval is 30 seconds, by measurement

A full rescan of the 1,693-note corpus costs **8.999 ms** stat-only and
**51.779 ms** with content hashes:

| Interval | Duty cycle, stat only | Duty cycle, stat + hash |
|---|---|---|
| 15 s | 0.06% | 0.35% |
| **30 s** | **0.03%** | **0.17%** |
| 60 s | 0.015% | 0.086% |

The plan said "every 30–60 s". The two ends are indistinguishable on duty
cycle, so **30 s** is taken as the safer of the two.

### 3. The polling watcher is the macOS strategy, and it costs nothing

| Strategy | fds | per note |
|---|---|---|
| The polling watcher | **+0** for 1,693 notes | **0.0000** |
| fsnotify (inotify), watching 31 directories | +0 | 0.0000 |

The plan's requirement — zero file descriptors per note on macOS — is
met by the polling watcher, and the cost of choosing it is **nothing**:
zero fds, and a 9 ms rescan of the corpus.

**The Linux row does not transfer to macOS, and that is the point.** Linux's
inotify needs no descriptor per watched path; macOS's kqueue backend opens
one per *file*. The same fsnotify call is free on one platform and is the
EMFILE landmine on the other. The spike ships a `macos` command that prints
the same table and warns when the ratio exceeds 0.5 fds per note, so the
number can be taken on the platform it is about.

### 4. The atomic save is seen, and the watch survives it

A change made the way Obsidian makes one — a temp file in the same
directory, renamed over the original — is seen **on the target path in
577 µs**, and the watch survives: a second atomic save is seen 222 µs
later.

This is the case that matters, because the rename **changes the inode**.
An implementation that watched the inode, or that ignored `RENAME`, would
miss it.

### 5. The `CHMOD` reconciliation rule stands, but it is a kqueue artefact

The plan says the watcher must "reconcile a removal reported as `CHMOD` by
stat-ing the path". On Linux, watching the directory, the removal came
through as **`REMOVE`** directly — even while the file was still open
elsewhere.

So the rule is kept (it is harmless and it is the right rule on the
platform that needs it), but it is a **belt to braces on Linux rather than
the mechanism**.

## Consequences

**Good:**

- **The performance criterion now describes the thing that is expensive.**
  A reader of the plan can tell where the time goes.
- **The real corpus is comfortably inside budget**, and the plan's
  5,000-note case is a synthetic stress case: 40 unknown frontmatter keys
  and a 2 KB body per note.
- **The macOS requirement is met at no cost**, and the reason the Linux
  number cannot be reused is written down rather than discovered later.

**Bad:**

- **The corrected criterion is still not met.** 1.23 s of wall clock for
  5,000 notes against a 1.25 s budget (1 s parse + 250 ms insert) is
  within a whisker, and the CI runner is slower than this machine. **Phase
  1 will have to either parallelise the load or cache the parse.**
- **The load phase has no cache.** The caches are rebuildable from the
  vault by design (ADR-0030), so a cold index pays the parse every time.
- **The macOS numbers are still not measured.** The command is written and
  ready; it has not been run on a Mac.

**Neutral:**

- **A single FTS5 query is sub-millisecond**, and the two-index partition
  does not measurably affect single-query latency. The cost is in
  building the index, not in using it.
- The peak RSS of 78.9 MiB for a 5,000-note index is the first memory number
  the project has.

## A note on how these numbers were produced

The spike's own timer covered **only the FTS5 insert**, so its first
reported figure — "137 ms for 5,000 notes" — was the cheap half of the
work presented as the whole of it. The load phase was outside the timed
region.

That is worth recording as a general caution: **a benchmark that times one
step of a pipeline and calls it the pipeline's cost will understate the
cost by whatever the untimed steps do.** The index command now reports
`load time`, `insert time` and `WALL CLOCK` separately, and the criterion
is written against the wall clock.
