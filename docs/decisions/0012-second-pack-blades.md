# ADR-0012: The second pack is Blades in the Dark, as a format proof

- **Status:** Accepted, amended by the phase-1 cut — the Blades gate is

  **phase 2**'s gate, not phase 1's, because the 5e pack and the formula
  and dice engines moved there with it. See
  [`phase-plan.md`](../phase-plan.md).
- **Date:** 2026-10-21

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

ADR-0007 made the D&D 5e pack the first one and required a second,
deliberately differently-shaped pack before phase 2 counts as validated. The
second pack is the actual test of the project's central claim. A pack for
another d20-family system would prove nothing.

## Decision

**The second pack is Blades in the Dark.**

It is the right proof because it breaks every assumption 5e bakes in, in one
system:

| Dimension | D&D 5e | Blades in the Dark | Invariant exercised |
|---|---|---|---|
| Dice | 1d20 + modifier | Dice pool, `rating` dice vs `opposition` dice | The dice engine must support pools, not just NdM+K |
| Progress | Levels 1–20 | XP, but advancement is play-driven, not level-driven | Levels are not required |
| Health | Hit points and hit dice | **Harm**, a clock-like track with severity | Resources are not required to be HP-shaped |
| Resistance | Damage types and resistances | Position, resistance, armour as a die | The "defence" concept is not required to be a number |
| Resources | HP, spell slots, exhaustion | Clocks, tokens, heat, crew position | New resource *kinds* must be expressible |
| Advancement | Level, class, subclass | Role, crew action, advancement in-fiction | Progression is not required to be numeric |
| Progress tracking | XP threshold | Progress clocks, per crew member | Faction/crew entities are first-class |
| Scale | Personal | Crew and faction as units of play | The entity model is not only about characters |

### What the Blades pack must prove

1. A pack with **no `levels` block** loads and lints clean.
2. A pack with **no HP-like resource** can still express a character.
3. A **dice-pool workflow** rolls correctly: `rating` dice against
   `opposition` dice, highest count wins, with position and escalation.
4. **Clocks** work as a resource kind with a size, a progress track and
   segmentation.
5. **Harm** works as a condition track rather than a numeric pool.
6. The **difficulty model** is opposition dice and clock pressure, not an XP
   budget — and the encounter planner uses it without special-casing.

If any of these require a change to the *core* rather than the pack, the pack
format was not generic enough and phase 2 waits.

### Prose and licensing

Blades in the Dark is a copyrighted ruleset. The pack encodes **structure and
mechanics** — ratings, roles, clock shapes, harm severity, the dice model — and
does not reproduce rulebook text. Where a label needs a rulebook term, the
term itself is used as a game term (which is what a system pack does), but no
prose is copied. This is the same posture as the 5e pack: SRD 5.2.1 is
CC-BY-4.0 and is redistributed with attribution; Blades content is not
redistributed at all, only the mechanics are.

## Consequences

**Good:**

- The claim in the README becomes something demonstrated rather than asserted.
- The dice engine gets its second, harder shape while it is still young.
- The pack format gains the resource kinds and entity shapes that a real
  second system demands, before phase 2 builds on top of it.

**Bad:**

- Blades is a bigger modelling job than 5e in some ways, and its mechanics
  are unusual enough that the pack will feel bespoke. That is the point, but
  it is work.
- Two packs means two sets of edge cases in the formula and dice engines.
  This is a feature wearing a disguise, but it will feel like a cost at the
  time.

**Neutral:**

- The pack may not be usable by most users. That is acceptable for a
  validation artefact; a third pack, written for a system the maintainers
  actually play, should follow before the project is public-facing.
