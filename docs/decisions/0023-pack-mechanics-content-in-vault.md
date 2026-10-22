# ADR-0023: The pack holds mechanics; the content is vault notes

- **Status:** Accepted
- **Date:** 2026-10-22

## Context

ADR-0007 made "the D&D 5e SRD 5.2 pack" the first pack. At the scale of the
actual SRD, that is a category error.

The SRD is **7,012 entities** — 4,469 monsters, 2,417 items, 936 spells, 847
class features, 1,785 subclass features, 334 subclasses, 265 feats, 157
backgrounds, 157 species, 230 variant rules. Written as pack YAML that is
several megabytes: unqueryable in Obsidian, ungreppable, un-diffs, and
impossible for a human to correct a typo in.

Worse, it conflates two different things:

- **Mechanics** — how hit points are computed, what a proficiency bonus is,
  what shape a class has. This is small, and it is what makes a system
  *work*.
- **Content** — the actual barbarian, the actual goblin, the actual fireball.
  This is large, and it is *stuff*.

We already decided the same thing one level up, for the system versus the
campaign (ADR-0003, ADR-0007): the system is data, the campaign is files. The
SRD forces the same split one level down.

Meanwhile the actual SRD data has been sitting in the repository as two
halves: `compendium/**.md` (prose, already rendered as Obsidian markdown) and
`data/**.json` (mechanics, in the upstream JSON schema). Every entity is split
across a `fluff-*.json` file (prose) and a mechanics file.

## Decision

**A pack contains mechanics and the *shape* of content. The content itself
lives in the vault, as notes.**

### Two halves

| | Holds | Size | Example |
|---|---|---|---|
| **The pack** (`packs/dnd5e.yaml`) | How the system computes; the declared shape of each kind of content object | ~50–100 KB | `max_hp: "10 + con_mod + level * con_mod"` |
| **The vault** (the campaign) | The actual entities, as notes | thousands of files | `Notes/Content/Monsters/Goblin.md` |

The pack is what makes the system work. The vault is what the campaign
contains. **Neither is allowed to become the other.**

### Packs declare content types

A pack may declare the kinds of content object it has, and the shape of each:

```yaml
content:
  - id: class
    label: Class
    fields:
      - { key: hit_die, label: Hit Die, type: number, required: true }
      - { key: primary_abilities, type: list, of: ability }
      - { key: saving_throws, type: list, of: ability }
      - { key: spellcasting_ability, type: ability }
      - { key: caster_progression, type: string }
      - { key: proficiencies, type: object }
    index:
      - { key: features, label: Features, from: classFeature }
```

This is how the character creation wizard stays system-agnostic: it reads the
pack's `content:` declarations to know what a class *is*, and reads the vault's
content notes to know what the classes *are*.

A Blades pack would declare `role`, `crew_action`, `clock` and `harm_track`.
None of those exist in 5e, and the core knows nothing about any of them.

### How content is stored

Content uses **one core note type**, `content`, with a universal
`content_type` field:

```yaml
type: content
content_type: class
uid: 01JBX…
name: Barbarian
system: dnd5e
data:
  hit_die: 12
  primary_abilities: [str]
  saving_throws: [str, con]
  proficiencies:
    armor: [light, medium, shield]
    skills: { choose: { from: [animal handling, athletics, …], count: 2 } }
```

`content_type` is a universal top-level field, so it is queryable and
filterable like any other. `system` scopes it to the pack that declared it,
so two packs may both declare a type named `clock` without colliding.

**Content types are declared by packs, not by plugins.** ADR-0019 forbids
plugins from defining note types, because a plugin is code and a new type
would need its own visibility rules and its own place in the write-back
contract. A pack is data; its content declarations are data. The line holds.

### The frontmatter/body split

> **Frontmatter is what machines compute with. The body is what humans read.**

The JSON's `trait`, `action` and `reaction` entries are long prose, and the
markdown body already renders them. They stay in the body. The compact
mechanical scalars — AC, HP, speed, ability scores, senses, CR — go into
frontmatter, because those are what a token needs and what a formula
evaluates against.

This rule is what lets the enrichment add thousands of fields without
touching a single line of prose, and therefore without endangering the
byte-identical round-trip contract (ADR-0021).

## Consequences

**Good:**

- The pack stays small enough to read, lint and diff.
- The SRD becomes an ordinary vault: greppable, linkable, editable in
  Obsidian, and subject to the same projection rules as everything else.
- The content is reusable. A GM who wants "my goblin is different" edits the
  note.
- The system-agnostic promise is now honest at three levels: systems are
  packs, packs are mechanics, content is files.
- The wizard and the tabletop read the **same notes**. One file, two
  consumers.

**Bad:**

- **A pack is no longer self-sufficient.** Installing the pack gives you the
  mechanics but not the content. That is a real change for anyone who
  expected "install 5e and everything is there", and the installer must
  therefore be able to install *content* too — which it does anyway, as a
  vault template.
- **Two things to version**: the pack and the content vault. A pack change
  can invalidate content. The `content:` schemas give the linter what it
  needs to detect that.
- **The `content` type is a compromise.** `type: class` would be nicer to
  query, but it would let a pack shadow a core type name (`item`, `table`
  are core types). `type: content` plus `content_type` avoids the collision
  entirely at the cost of one extra filter.

**Neutral:**

- The pack's `difficulty`, `dice`, `formulas` and `views` blocks are
  unchanged. Only `content:` is new.
- ADR-0007's "first pack is D&D 5e" still holds — but the *pack* is now
  mechanics only, and the SRD arrives as a vault template. See
  [`srd-vault.md`](../srd-vault.md).
