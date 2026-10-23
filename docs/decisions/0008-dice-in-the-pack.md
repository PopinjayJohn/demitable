# ADR-0008: Dice mechanics live in the pack; the app is the roller

- **Status:** Accepted
- **Date:** 2026-10-21

## Context

Phase 2 includes an **encounter planner and runner**. A runner that cannot
resolve a roll is a checklist with extra steps. But dice are the most
system-specific mechanic there is, and a naive implementation assumes a d20
family:

- **D20 family (5e):** `1d20 + modifier` vs a target number. Advantage = roll
  2d20, keep the higher. Natural 20 is a critical. Higher is better.
- **Dice pools (Blades, Many Dice):** roll `rating` dice, count successes
  against a target. Higher is better, but the arithmetic is different.
- **Roll-under (Mörk Borg, The One-Ring):** *lower* is better. Successes are
  counted below a threshold.
- **Opposed pools:** two pools, higher count wins, with escalation and
  position.
- **Degrees of success (Savage Worlds, GURPS):** pass, good, great, amazing.

A single hard-coded roller would encode one of these and quietly break the
others — precisely the failure mode ADR-0003 was written to prevent.

## Decision

**The app rolls. The pack defines how.**

### The dice expression language

A pack-level `dice:` block describes the notation, the resolution rules, and
the named roll workflows. The app implements a single expression parser
covering the constructs the pack can request:

| Construct | Example | Meaning |
|---|---|---|
| Basic roll | `1d20`, `2d6` | N dice of M sides |
| Flat modifier | `1d20+5`, `2d6-1` | Added to the result |
| Keep highest / lowest | `4d6kh3`, `2d20kl1` | Dice pools, advantage, disadvantage |
| Exploding | `6d6>=5`, `6d6<=2` | Extra rolls on a threshold |
| Fudge dice | `4dF` | Each die is −1, 0 or +1 |
| Wild die | `1d%` with wild `9,0` | Percentile with wild faces |
| Success threshold | roll `>= X` | Result compared to a target |
| Opposed | two pools | Higher count wins |

The expression language is separate from the formula language of ADR-0003,
though they share the same lexer foundations and the same safety rules.

### Pack structure

```yaml
dice:
  default: "1d20"

  advantage:
    mode: keep-highest      # keep-highest | keep-lowest
    dice: 2
    sides: 20

  critical:
    when: natural-max       # natural-max | natural-min | none
    effect: "1d6"           # extra dice on a critical

  direction: high           # high | low — roll under systems invert this

  degreesOfSuccess: []      # e.g. [pass, good, great, amazing]

  workflows:
    - key: attack
      label: Attack Roll
      dice: "1d20"
      against: resources.ac
      bonus: attack_bonus
      onSuccess: damage
    - key: save
      label: Saving Throw
      dice: "1d20"
      against: dc
      bonus: save_bonus
    - key: check
      label: Ability Check
      dice: "1d20"
      against: dc
      bonus: check_bonus
```

A dice-pool system declares different workflows entirely:

```yaml
dice:
  direction: high
  workflows:
    - key: pool
      label: Roll
      dice: "{rating}d6"
      vs: "{opposition}d6"
      outcome: highest-count
```

### Dice log

Every roll is recorded with its full provenance: who rolled it, which
workflow, the expression, the individual die results, the modifiers and their
sources, the target, the outcome, and the timestamp. This log is a campaign
asset — it is written into the session note, is searchable, and is what the
phase-4 transcript can be correlated against ("that was the roll that killed
him").

## Consequences

**Good:**

- The encounter runner becomes real: shared rolls, streamed to every player's
  screen, with the GM's view showing the actual numbers and the player's view
  showing the outcome (and only the outcome, if the pack says so).
- The dice log is a genuine differentiator. It is the thing that makes a
  campaign's history queryable, and it is the natural join point between the
  VTT and the phase-4 recorder.
- Systems that roll low, use pools, or use degrees of success are supported
  without special-casing the core.

**Bad:**

- **A second expression language** to design, implement, document and make
  safe. This is a real cost — the pack format now has two languages in it.
- The pack author must express resolution rules declaratively. Some systems
  have dice interactions (escalation, push-your-luck, combo dice) that may not
  fit. The escape hatch is pack-defined *workflows* rather than raw code, so
  the ceiling is low and the fix is "add a workflow kind to the pack format",
  which is a format change but not a code change for existing packs.
- Degrees of success and opposed rolls add meaningful complexity to the
  runner's UI. Realistically, phase 2 should ship one workflow shape and
  expand it.

**Neutral:**

- The dice engine does not need to be in phase 1. It is phase 2 work
  > **Amended twice.**
  >
  > *2026-10-23.* The **engine** was moved to phase 1 (M3), because the
  > Blades format gate (ADR-0012) requires a dice-pool roll to prove the
  > format is generic. The **runner** — the interface, the shared log, the
  > roll workflows — was phase 2.
  >
  > *2026-10-23, later.* Phase 1 was cut to an MVP, and **the whole dice
  > engine moved to phase 2**, along with the 5e pack and the Blades gate.
  > Phase 1 has no dice at all. See [`phase-plan.md`](../phase-plan.md)., and the
  pack format can gain its `dice:` block in phase 2 without invalidating
  phase-1 packs — a pack without a `dice:` block simply has no roll workflows.
