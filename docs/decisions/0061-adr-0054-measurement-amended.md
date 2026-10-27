# ADR-0061: The ADR-0054 §5 measurement, corrected — the maximum swing is 103.96%, and the count is 3

- **Status:** Proposed — drafted during phase 3's O0 follow-up for the
  orchestrator to land; it is not accepted until it is

- **Date:** 2026-10-26
- **Amends:** ADR-0054 §5 (the two-index partition is load-bearing)
- **Addresses:** the verification finding that the committed ADR-0054 suite
  and the ADR's recorded numbers disagree by 3.36 percentage points, and that
  the ADR's "3 of 8" is not the count its own table shows

> **Origin:** phase 3, O0b — the ADR-0054 measurement re-runs in CI with the
> recorded bounds as a failing suite. The suite is
> [`internal/store/adr0054_measurement_test.go`](../../internal/store/adr0054_measurement_test.go);
> it measures over the SRD corpus (1,693 notes), the same eight GM queries and
> the same BM25 weights `(10, 5, 8, 2, 1)` as `spikes/measurement/search`.

## Context

ADR-0054 §5 records, over the corpus with ADR-0033's pack defaults applied
(1,169 GM-only notes, 524 party-visible):

- the top result changes in **3 of 8** queries;
- a note's rank moves by **up to 16 positions**;
- a note's BM25 score changes by **up to 100.6%** — the Darkvision spell
  scores −3.84 in the GM index and −7.70 in the player index.

The committed suite reproduces **3 / 16 / 103.96%**. Two of those numbers are
the same as the ADR's, and the third is not; the suite's count and the ADR's
count also differ if the ADR's table is read literally. The gap is not a
corpus or tokenisation change: the suite reproduces the Darkvision spell's
−3.8382 → −7.7002 exactly. Both differences are in **what the number counts**.

### 1. 100.6% is the top hit's swing, not the maximum

The Darkvision spell is the top hit for "darkvision" in both indexes. Its
score moves from −3.8382 to −7.7002, a change of **100.62%**, and that is the
figure ADR-0054 §5 records. But it is not the largest swing in that query.
Computing the swing over **every player hit that also appears in the GM top
20** — which is what "a note's score changes by up to" means — gives a larger
one:

| Query | Note | GM rank | Player rank | GM BM25 | Player BM25 | Swing |
|---|---|---|---|---|---|---|
| darkvision | Darkvision (spell) | 1 | 1 | −3.8382 | −7.7002 | 100.62% |
| darkvision | Elf (race) | 11 | 8 | −2.4570 | −5.0113 | **103.96%** |

Ranks are 1-based here; the suite stores them 0-based.

The ADR's sentence names the spell and says "up to 100.6%". The spell's number
is correct; the "up to" is not. 103.96% is the maximum, and the bound of 105.0%
was evidently chosen for it — the recorded prose just never caught up.

### 2. "3 of 8" counts changed top results, not lost ones

The suite's `topChanges` increments only when **both** indexes return a top hit
and the two UIDs differ:

```go
if gmTop != "" && plTop != "" && gmTop != plTop {
    topChanges++
}
```

That excludes "goblin": the GM index returns **13 hits** and the player index
**0** (every goblin in the SRD is a monster, and every monster is `gm`). Its
top result has not changed; it has vanished. Counting a vanished top hit as a
change would make it **4 of 8**, which is what ADR-0054's table reads like: the
table marks "goblin" as "yes, changed" while the prose says 3. Both numbers
are defensible; one has to be stated.

The same convention question applies to `maxShift`, which is why this ADR
names it too: the suite takes the largest displacement among player hits in
the **top 10** that also appear in the GM **top 20**, and gets 16 (the Wizard
note for "magic missile"). It does not measure displacement below rank 10.

## Decision

**Amends ADR-0054 §5.**

1. **The maximum score swing is 103.96%**, not 100.6%. It is the Elf race note
   for "darkvision", moving from GM rank 11 to player rank 8, score −2.4570 to
   −5.0113. The Darkvision spell's **100.6% is retained as the illustrative
   top-ranked swing**, not as the maximum; ADR-0054 §5's sentence is corrected
   to say both.
2. **`topChanges` counts only queries where both indexes return a top hit and
   the two differ: 3 of 8.** A query whose player index has no hit at all is
   *not* a changed top result under this convention, and "goblin" (13 GM hits,
   0 player hits) is that query. Under the other convention the figure is 4 of
   8. The suite asserts the first and its bound stays 3.
3. **`maxShift` is 16 positions**, measured over player hits in the top 10 that
   also appear in the GM top 20 (the Wizard note, "magic missile"), and the
   bound stays 16.
4. **The corpus split is unchanged and confirmed:** 1,693 notes, 1,169
   GM-only, 524 party-visible, with ADR-0033's defaults. The suite and the
   spike agree on every number that is not defined differently above.
5. **The bounds the suite asserts are 3 / 16 / 105.0%.** The 105.0% bound keeps
   1.04 percentage points of headroom over the measured 103.96%; a future
   preprocessing change that widens the gap is a measurement to make, not a
   bound to raise silently.

## Consequences

**Good:**

- The ADR and the suite now state the same numbers, and the suite's comment
  records the convention so the next reader does not have to reconstruct it
  from a `&&` in the code.
- The gap was found by making the measurement executable, which is the point
  of O0b: a recorded number that no suite checks had already drifted from what
  the same corpus produces.

**Bad:**

- The ADR's prose and its table disagreed for two phases, and nobody noticed
  because both were prose. The table's "yes" for goblin is now explicitly the
  other convention.
- A bound with 1.04 points of headroom is a tight bound. Adobe's FTS5 build
  and SQLite versions can shift BM25 in the fourth decimal; the suite compares
  percentage points, not scores, so this is a bound on the corpus and the
  preprocessing, not on the engine.

**Neutral:**

- No code under `internal/` changes except the test's name and comments. The
  measurement, the queries and the weights are unchanged from the spike.

## What this amends, and what it does not

It amends **ADR-0054 §5 only**. §1 (headings in the literal index), §2 (the
column-per-field FTS5 table), §3 (community content fences), §4 (the search
box's language), §6 (the predicate operator set) and the decision that the
two-index partition is required are untouched. The partition remains
load-bearing; this ADR only makes its evidence say what the suite measures.
