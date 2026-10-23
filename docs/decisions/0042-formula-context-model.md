# ADR-0042: Formulas are evaluated against an explicit, layered scope

- **Status:** Accepted
- **Date:** 2026-10-23
- **Extends:** ADR-0003 (the formula language)
- **Addresses:** a blocking open decision — the pack examples reference
  `level` and `weapon.bonus`, neither of which is entity data

## Context

ADR-0003 says "Identifiers resolve against the entity's data and against other
formulas". But the pack format's own examples need more than that:

```yaml
formulas:
  max_hp: "10 + attributes.con + level * con_mod"
  attack_mod: "attributes.str_mod + proficiency + weapon.bonus"
```

`level` is not in the note's `data:` — it comes from the pack's progression.
`weapon.bonus` is not entity data at all; it describes *the situation being
rolled*. And `proficiency` is pack-declared.

So one language was being used across three different kinds of data, with no
rule saying which wins when a name appears in more than one.

## Decision

**A formula is evaluated against a scope, which is a stack of named bindings.
The stack shadows outward.**

| Layer | What it holds | Example |
|---|---|---|
| 1. **Call scope** | The situation being resolved | `weapon.bonus`, `target.ac`, `round` |
| 2. **Entity scope** | The note's `data:` | `attributes.str`, `resources.hp`, `level` |
| 3. **Pack scope** | Pack-declared constants and progression | `proficiency`, `spell_dc_base`, `hit_die` |
| 4. **Derived values** | Other formulas | `con_mod`, `max_hp` |

A name is resolved in the **innermost layer that has it**. The call scope
shadows the entity scope, which shadows the pack scope.

**Derived values resolve lazily**, memoised, with cycle detection. A formula
that references another formula does not need the other to be declared first.

**An unresolvable name is a pack error, caught by the linter at load time** —
not a runtime surprise for a GM mid-session. The linter evaluates every
formula in every declared context the pack provides, so a missing `level` or a
missing `proficiency` is a lint failure.

## Consequences

**Good:**

- **One language, three kinds of data, one resolution rule.** The pack
  examples now work as written.
- **The situation is a parameter, not a special case.** A roll against
  `weapon.bonus` and a sheet's `max_hp` are the same mechanism.
- **Errors surface at load, not at the table.** The linter is the thing that
  makes a pack safe to trust.

**Bad:**

- **Shadowing is a trap.** A pack that defines `level` in the pack scope
  shadows an entity's `data.level` silently. The linter should report
  shadowing as a warning.
- **The call scope is a contract between the pack and the caller.** A formula
  that needs `weapon.bonus` only works where the caller supplies it. The
  pack should declare the names it expects in a call scope, so the linter can
  check that a workflow provides them.

**Neutral:**

- Derived values can be referenced by bare name (`con_mod`) or by a path
  (`formulas.con_mod`). Bare is the ergonomic form; the path is the explicit
  one.
