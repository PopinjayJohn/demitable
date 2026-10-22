# ADR-0007: The first system pack is D&D 5e (SRD 5.2), with schema invariants to prevent 5e-shaped assumptions

- **Status:** Accepted, amended by ADR-0023 — the pack holds *mechanics* only;
  the SRD *content* ships as an enriched vault template, not inside the pack.
  See [`srd-vault.md`](../srd-vault.md).
- **Date:** 2026-10-21

## Context

The first pack decides what the phase-1 schema is tested against. Two
candidate goals:

- **Audience.** D&D 5e is by far the most-played system. A working 5e pack is
  immediately useful to a large number of people, and SRD 5.2.1 is
  CC-BY-4.0, so it can be redistributed with attribution.
- **Proof.** A pack for an unusual system (Blades in the Dark, Cairn, Fate)
  stress-tests the pack format far harder, because it has no equivalent of
  ability scores, hit points, or levels.

Doing 5e first is the pragmatic choice, but it carries a specific danger:
**the schema gets shaped around 5e and then every other system has to fight
it.**

## Decision

**The first pack is D&D 5e (SRD 5.2.1).**

To prevent the schema from inheriting 5e's shape, the following are adopted as
**schema invariants** — properties the note model and pack format must satisfy
regardless of the system:

1. A pack need not define any **resources**. (Cairn has no HP by default.)
2. A pack need not define **numeric attributes**. (Fate's aspects are strings.)
3. A pack need not define **levels**. (Many systems are level-less.)
4. A pack need not define **attributes at all**. (A system may use a single
   "rating" pool.)
5. A pack need not define **conditions**.
6. Every pack must be able to express a **player character**, an **NPC**, and
   a **creature** — the three entity shapes the campaign tools need.
7. Difficulty is modelled by the **pack**, in the pack's own terms. The app
   must not assume an XP budget.

These are enforced by a **pack linter** in phase 1: a pack that assumes a
resource exists where the schema provides none, or that hard-codes
level-based maths in the wrong layer, fails lint.

A second, deliberately differently-shaped pack is required before phase 2 is
considered validated. Its system is chosen to maximise structural difference
from 5e, not similarity.

## Consequences

**Good:**

- The first pack exercises the real thing: classes, species, backgrounds,
  ability scores, skills, proficiencies, saves, AC, hit dice, spell slots,
  conditions, and a monster list. If the pack format handles all of that, it
  is not a toy.
- SRD 5.2.1 is CC-BY-4.0, so the pack can be shipped and shared with
  attribution rather than a licensing blocker.
- The pack is immediately useful to the largest possible audience, which
  matters for a project hoping to attract contributors.

**Bad:**

- 5e's vocabulary will leak into the schema if the invariants are not
  enforced. The linter is not optional; it is the mechanism that keeps the
  system's central promise honest.
- A large ruleset invites scope creep in phase 2 — full spellcasting, every
  subclass, every feat. The pack should model the *shape* of 5e, and content
  depth should be scoped explicitly.

**Neutral:**

- SRD 5.2.1's specific content (classes, species, backgrounds, monsters) is
  data, not code, and belongs in the pack as data.
- **Amended by ADR-0023:** at 7,012 entities, "the pack" and "the content"
  must be separated. The pack holds the mechanics and the `content:`
  declarations; the entities themselves live in the vault as enriched notes,
  and arrive as a vault template. The invariants in this ADR still hold —
  they now apply to the pack *and* to the content schemas it declares.
- **The third pack is Savage Worlds** (see ADR-0012 for the second, Blades).
  It is chosen for what it exercises that the first two do not: **dice types
  beyond d20** (d4/d6/d8/d10/d12, so the dice engine needs a real die set
  rather than "d20 with a different number"), **exploding dice**, and
  **degrees of success** (pass/good/great/amazing). Blades covers dice pools,
  no-levels and clocks; between the two, the format has been asked for dice
  variety, opposed resolution, severity tracks and graduated outcomes.

  A third pack for a system the maintainers actually play should follow
  before the project is public-facing — that is a separate pack, a fourth,
  and its own decision.
