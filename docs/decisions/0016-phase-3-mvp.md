# ADR-0016: The phase 3 MVP is tokens, fog and initiative — no map editing

- **Status:** Accepted
- **Date:** 2026-10-21

## Context

Phase 3 is the 2D tabletop. This is the phase most likely to consume the
entire project, because "a VTT" is not a feature list — it is an application
with a feature list, and the well-known ones (Foundry, Roll20) have had a
decade and a team each to accumulate theirs.

The temptation is to aim for parity, and parity is how a part-time project
becomes a five-year project that never ships.

## Decision

**The first playable tabletop does tokens, fog, initiative and dice. It does
not edit maps.**

In scope:

- Grid and hex layouts, with snap and configurable cell size.
- Image map as background, with pan and zoom.
- Tokens: portrait, name, size, HP, conditions, player/GM ownership, a
  visibility flag for the GM's hidden tokens.
- Fog of war: a reveal brush, a GM-only layer, and per-token vision.
- Initiative tracker, with the pack's initiative workflow.
- Dice log, shared and streamed.
- Measurement and distance.
- Player view: pan, zoom, and their own character's sheet beside the board.

Explicitly **out of scope** for the first playable tabletop:

| Excluded | Why |
|---|---|
| Drawing walls, doors, regions | Map authoring is a second application. Foundry's scene editor is a project of its own |
| Dynamic lighting and shadowcasting | Expensive to implement well and to render smoothly, and the payoff only appears in systems with detailed vision rules |
| Line of sight and vision rules | Same reason. A grid-and-range approximation is enough for most play |
| In-map scripting | Unbounded scope |
| 3D, isometric, anything | The constraint that makes this tractable |

The GM brings a map image. If they want walls, they bring a map image that
already has walls drawn, or they use the map tool they already have.

## Consequences

**Good:**

- The phase is a real tabletop on a realistic schedule, and it exercises every
  part of the architecture that matters: the live session hub, the projection
  layer, the pack-defined dice workflows, and the write-back contract.
- Tokens and fog are what players actually notice. Walls are what map nerds
  notice.
- Everything excluded is additive later. Adding a scene editor to a working
  tabletop is a new feature; adding it to a half-built one is a rewrite.

**Bad:**

- **The tabletop will be visibly less capable than Foundry**, and users will
  compare. The honest framing is that this is a tabletop for people who want
  their campaign in files, not a Foundry replacement.
- Games that rely on precise line-of-sight will not work. For most systems
  and most tables, grid range and "the GM says you can't see it" is enough.
- Vision rules that vary per system (darkvision, blindsight, truesight) will
  need pack support eventually, and the pack format does not have it yet.
  That is a phase-2 pack-format addition, not a phase-3 blocker.

**Neutral:**

- The fog-of-war implementation should not assume a grid-only world, so that
  hex and free-form modes stay possible later.
