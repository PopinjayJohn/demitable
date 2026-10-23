# ADR-0044: The core schema expresses segmented tracks and severity scales

- **Status:** Accepted
- **Date:** 2026-10-23
- **Extends:** ADR-0003 (the pack format) and ADR-0012 (the Blades gate)
- **Addresses:** a blocking open decision — the Blades gate requires clocks
  and harm tracks, and the core schema had no shape for either

## Context

ADR-0012's gate says a Blades pack must prove that:

> Clocks work as a resource kind with **size, progress and segmentation**.
> Harm works as a **condition track**, not a numeric pool.

And it says the gate firing means the format was not generic enough and phase 2
waits.

The core schema did not have those shapes. `data.resources` were scalar pools
with a max, and `data.conditions` were flags. So the gate **would** have
fired — and the design gave no hint that a core schema change was the
expected outcome.

That is a design smell: a gate that cannot pass without an unspecified
change is not a gate, it is a surprise.

## Decision

**A resource may be a pool or a track. A condition may be a flag or a scale.
The core knows both and assumes neither.**

```yaml
data:
  resources:
    hp:                       # a pool
      kind: pool
      value: 23
      max: 38
    clock_harlan:             # a track
      kind: track
      size: 6
      segments: 2
      filled: 2
  conditions:
    - key: poisoned           # a flag
      kind: flag
    - key: harm               # a scale
      kind: scale
      steps: [0, 1, 2, 3]
      value: 1
```

| Shape | Used by | Carries |
|---|---|---|
| **pool** | 5e hit points, mana, exhaustion | A value and a max |
| **track** | Blades clocks, progress clocks | Size, marks filled, optional segments |
| **flag** | 5e conditions | Presence |
| **scale** | Blades harm, Flesh wounds | An ordered set of steps and a current value |

The pack declares which shape each of its resources uses. The core's
rendering, the encounter runner and the VTT all handle both.

### The invariants still hold

This is the part that matters. The schema invariants in ADR-0007 are of the
form *"a pack need not define X"* — and we have **added a shape**, not
**required one**:

- A pack need not define any resources (invariant 1) — still true.
- A pack need not define any conditions (invariant 5) — still true.
- A pack that defines only pools, like 5e, is unaffected.

The gate was never "every resource must be a track". It was "the format must
be able to express a track". It now can.

## Consequences

**Good:**

- **The Blades gate can actually pass**, on the schema, rather than firing
  and forcing a panic.
- **One concept covers four shapes**, and a pack picks per resource.
- **The VTT and the encounter runner gain what they needed anyway** — a
  clock is a thing you mark, not a number you increment.

**Bad:**

- **The core schema is bigger.** A resource is now a union, not a number, so
  every consumer of `data.resources` handles a shape it might not expect.
- **A pack author can pick the wrong shape**, and "clock as a pool with a max
  of 6" is a valid but unhelpful encoding. The linter can check for a track
  that has no segments where the pack expects them.

**Neutral:**

- The vocabulary — pool, track, flag, scale — is our own, but it is four
  words describing four genuinely different mechanics, not a re-spelling of
  something the community already has.
