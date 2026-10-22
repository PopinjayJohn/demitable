# ADR-0022: The dice engine supports symbol dice, via tables of faces

- **Status:** Accepted
- **Date:** 2026-10-21

## Context

ADR-0008 made the app the dice roller and the pack the definition of how
rolling works. The dice model sketched there was numeric: `NdM`, keep
highest, exploding, thresholds, opposed pools.

That is not enough. A large part of the symbolic-dice appeal is dice whose
faces carry **symbols rather than numbers**:

- **Rune dice** — Elder Futhark, a set of six or eight glyphs.
- **Tarot and oracle cards** — arcana, suits, numbered and unnumbered.
- **Coins with custom faces** — heads and tails as symbols, or a coin that
  lands on one of six guild crests.
- **Elemental and alignment dice**, heraldry, zodiac.
- **Icon dice** for map and location selection — roll a d6 of region icons to
  pick where the party's rumour comes from.
- **Narrative dice** — a die whose faces are prompts, not numbers.

Two related cases need the same machinery:

- **Random tables** — "roll for a rumour", treasure tables, wild magic
  surges, the 5e *Divine Roller*. These are a table of outcomes with no dice
  at all.
- **Mixed dice** — a d6 where five faces are 1–5 and one is a critical with a
  symbol.

All of these are the same object: **a named set of faces**, and a roll that
picks from it.

## Decision

**A die's faces carry payloads, not just numbers. Tables are the unified
concept.**

### The face model

```yaml
- id: fehu
  label: Fehu
  symbol: "ᚠ"        # a glyph, or a reference to an asset
  asset: null        # optional: Assets/Tables/futhark/fehu.svg
  value: null        # optional: a number for arithmetic
  rank: 1            # optional: an ordering for comparison
  tags: [success]    # optional: success | failure | wild | critical
```

- `value` is **optional**. Pure symbol dice have no numbers at all.
- `rank` is **optional**. A face without a rank is unordered, and a die of
  unordered faces cannot be compared — the engine must say so rather than
  invent an order.
- `symbol` is a unicode glyph or an asset reference. Assets live in the vault
  and are served by the core, so players receive them with the table.

### Tables are notes

A table is a first-class vault object — `type: table` — holding its faces in
`data.faces`. It uses the existing note schema with no special casing, so it
is browsable, linkable, searchable and projectable like everything else.

Packs may also ship tables for system-specific sets. A vault table shadows a
pack table of the same id.

This unification means a "random table" and a "symbol die" are the same
concept with different dice, which is exactly what they are.

### Dice definitions

```yaml
dice:
  sets:
    - { key: d6,  sides: 6, numeric: true }
    - { key: rune, sides: 6, table: elder-futhark, numeric: false }
    - { key: omen, sides: 8, table: elder-futhark, numeric: false, ranked: true }
```

`numeric: false` means the roll produces a *symbol*, not a total.
`ranked: true` asserts the table's faces carry a usable ordering.

### Result types

| Type | Produced by | Carries |
|---|---|---|
| `numeric` | A numeric die or pool | A total |
| `symbol` | A table-backed die | The face — glyph, label, asset |
| `count` | A pool | The number of successes |
| `named` | A table lookup | The face's label |

A roll's result type is derived from its definition, not guessed at.

### Counting in pools

Successes come from either **tagged faces** (`tags: [success]`) or a **rank
threshold** (`rank >= 3`). A table can mix both. If neither is defined and a
count is requested, the engine errors rather than defaulting.

### Comparisons

Comparing dice requires ranks. An unordered die in a comparison is an error
with a clear message — "this table has no ordering; add `rank` to its faces or
use a numeric set" — not a silent fallback to declaration order.

## Consequences

**Good:**

- One mechanism covers numeric dice, symbol dice, random tables, mixed dice
  and oracle cards. The engine does not grow a branch per flavour.
- The 5e pack gets wild magic surges, the Divine Roller, and "roll for a
  rumour" tables from the same machinery as a d20.
- Story-first games — the ones where a rune is the result — are supported
  properly rather than as a bolt-on.
- Tables being ordinary notes means the GM can write them in Obsidian, link
  them into a session log, and let the projection layer handle visibility.

**Bad:**

- **Faces are richer, so the face model is a schema change.** The dice engine
  must handle "no value", "no rank" and "no tags" gracefully everywhere —
  including in the log, the sheet and the UI.
- **Assets need delivering.** Glyphs are free; image faces need the core's
  asset endpoint to serve them to players, with the projection applying.
- **Comparisons are stricter.** Code that assumed every die is numeric has to
  handle the unordered case.
- **The "die pool counts successes" concept arrives early**, and it is the
  part of the dice engine most likely to need revision once Blades is
  implemented as the second pack.

**Neutral:**

- A symbol die with a `value` on each face can still produce a numeric total.
  Numeric and symbolic are not exclusive, and mixed dice need no special
  case.
