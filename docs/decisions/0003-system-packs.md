# ADR-0003: Game systems are declarative YAML packs with a sandboxed formula language

- **Status:** Accepted

- **Date:** 2026-10-21

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

"System-agnostic" is the project's headline claim and the one most likely to
be tested by someone playing a system we did not think of.

Three ways to support many systems:

- **A generic core vocabulary.** One schema every system maps onto: named
  abilities, generic health, generic conditions. Maximum uniformity, maximum
  compromise. Fate's aspects, Blades' clocks and tokens, and GURPS' reaction
  dice all get flattened into something unrecognisable.
- **Code adapters per system.** Highest fidelity. But shipping code for every
  ruleset is unsustainable, homebrew requires a fork, and it makes "the system
  is data" false.
- **Declarative packs.** A pack is a YAML file describing the system, plus
  formulas for the parts that need computation.

## Decision

A **system pack** is a YAML file in the vault (`Packs/<id>.yaml`) describing:

- `attributes` — the numeric abilities the system uses, with labels, ranges
  and defaults. Named by the system, not by us: `str`/`dex` for one, `edge`/
  `wits`/`hull` for another.
- `resources` — tracked pools with a kind (hit points, mana, grit, sanity),
  min/max, and a formula for the maximum.
- `formulas` — derived values (modifiers, totals, difficulty budgets) in a
  small sandboxed expression language.
- `conditions` — status effects the system recognises.
- `progression` — levels / ranks / XP and how they advance.
- `difficulty` — the system's own difficulty model (XP budget, die-pool
  opposition, clocks-and-tokens difficulty). Different systems genuinely
  disagree here, and the pack must be able to disagree.
- `views` — how the character sheet and encounter tools should lay out the
  system's fields.

Adding a system = adding a file. No code, no fork, no plugin.

### The formula language

A deliberately small, safe expression language — not JavaScript, not `eval`.

```
expression  := ternary
ternary     := logic_or ('?' expression ':' expression)?
logic_or    := logic_and ('||' logic_and)*
logic_and   := equality ('&&' equality)*
equality    := comparison (('==' | '!=') comparison)*
comparison  := additive (('<' | '<=' | '>' | '>=') additive)*
additive    := multiplicative (('+' | '-') multiplicative)*
multiplicative := unary (('*' | '/' | '%') unary)*
unary       := '-' unary | 'not' unary | postfix
postfix     := primary ('.' name | '(' args ')' | '[' expr ']')*
primary     := number | string | true | false | null
             | name | '(' expression ')' | '[' list ']' | '{' map '}'
```

- Identifiers resolve against the entity's data (`attributes.str`,
  `resources.hp`) and against other formulas, resolved by dependency order.
- Whitelisted builtins only: `min`, `max`, `floor`, `ceil`, `round`, `abs`,
  `clamp`, `sum`, `len`, `has`, `coalesce`, `count`, `lower`, `upper`.
- No assignment, no user-defined functions, no property access to
  `__proto__` / `constructor` / `prototype`, depth- and size-limited.
- Cycles are detected and reported as pack errors, not runtime explosions.

Because the language is small and side-effect free, a pack can be shared
between strangers with the same trust level as a markdown file.

## Consequences

**Good:**

- Adding and sharing systems is a text-file operation, in keeping with the
  rest of the project's philosophy.
- The pack is where "system-agnostic" is actually implemented rather than
  asserted, and it is inspectable by the user — a GM can read the pack and
  know exactly what the app does with their character.
- Non-D20 systems (dice pools, clocks, tokens, aspects) fit, because the pack
  can define resources and difficulty models that have no 5e equivalent.

**Bad:**

- The formula language is a thing we must design, implement, document and
  support. Every new pack author has to learn it.
- Expressiveness limits what a pack can do. A pack cannot compute a bespoke
  algorithm; it can only compute expressions. Complex systems may need
  extensions, and the language will have to grow.
- Formulas are strings, so pack errors surface at runtime rather than compile
  time. Mitigation: a pack validator plus a "test this pack" harness in phase 1.

**Neutral:**

- The language is close in spirit to Obsidian Bases' formula syntax, so
  concepts transfer, but it is independent of it.
