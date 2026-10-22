# ADR-0036: The community's stat block and admonition formats are the native formats

- **Status:** Accepted
- **Date:** 2026-10-22
- **Extends:** ADR-0035 (secrets), ADR-0021 (the editor), ADR-0023 (the pack)
- **Addresses:** the support for
  [`obsidian-ttrpg-community/fantasy-statblocks`](https://github.com/obsidian-ttrpg-community/fantasy-statblocks)
  and [`ebullient/obsidian-admonition`](https://github.com/ebullient/obsidian-admonition)

## Context

Two community projects define how TTRPG content is written in Obsidian, and
both are widely used:

| Project | Stars | Licence | Last push |
|---|---|---|---|
| `ebullient/obsidian-admonition` | 1,339 | MIT | 2026-10-19 |
| `obsidian-ttrpg-community/fantasy-statblocks` | 497 | MIT | 2026-01-21 |

**Admonition** defines two forms of a styled block:

- the **callout** form — `> [!tip] Title` with `>`-prefixed body lines;
- the **admonition** form — a fenced code block whose info string names the
  type, with YAML options at the top of the block:

````markdown
```ad-note
title: A tip
collapse: closed
icon: lightbulb
color: 120,120,120
metadata: anything at all

The body.
```
````

The option set is `title`, `collapse`, `icon`, `color` and `metadata` — and
`metadata` is explicitly *arbitrary*. Callout types are extensible the same
way: a custom admonition is defined by a JSON file (`{"type": "...",
"icon": "...", "color": "..."}`) and then works as both an admonition and an
Obsidian callout. **The community already has a supported way to add a type
like `secret`.**

**Fantasy Statblocks** defines the stat block:

````markdown
```statblock
layout: Basic 5e Layout
name: Ancient Black Dragon
ac: 22
hp: 367
stats: [27, 14, 25, 16, 15, 19]
traits: [...]
actions: [...]
```
````

### The finding that settles the question

**Our SRD corpus is already written in these dialects.** Measured across all
1,692 corpus files:

| Construct | Count |
|---|---|
| `ad-statblock` code blocks | **355** |
| Callout types: `quote` (85), `tldr` (24), `note` (9), `example` (2) | 120 |
| Obsidian block references (`^id`) | **204** |

`tldr` and `seealso` are *aliases* in admonition's type table — `tldr` is an
alias of `abstract`. So the corpus uses admonition's callout types *and* its
admonition code-block form, *and* Obsidian's block references.

**So supporting the community's formats is not an integration project. It is
reading what we already have.** The golden corpus is the community's dialect.

## Decision

### The formats are native, not imported

The editor treats Obsidian callouts and `ad-*` / `statblock` code blocks as
first-class. They are not raw code with a plugin's styling bolted on.

### The stat block is the monster format

- The **SRD pipeline preserves** the `ad-statblock` blocks verbatim. It does
  not convert them, because they are already the community's format and
  because the corpus's version is *prose* — a stat block a human reads.
- The **structured** mechanics of a monster come from where ADR-0023 already
  put them: the pack's `data:` on the note, derived from the corpus JSON.
- A user's **own** homebrew monster in the fantasy-statblocks dialect is
  **also** parseable: `statblock` YAML is structured, and the bestiary reads
  it directly.

So there are two sources for a monster's mechanics — our enriched `data:`,
and a `statblock` block in the note — and the bestiary reads both. That is
deliberate: a homebrew monster the GM wrote last night must work without
being re-imported through our pipeline.

### A secret is a block marked GM-only — one concept, several syntaxes

ADR-0033 and ADR-0035 said "a secret is a block the GM can reveal". This ADR
settles **how such a block is written**, and the answer is: *the way the
community already writes blocks.*

| Syntax | Marker | Notes |
|---|---|---|
| Obsidian callout | `> [!secret]` | The callout's type. Admonition's custom-type extension point, so a GM can already define `secret` themselves |
| Admonition code block | ` ```ad-secret ` | Same idea, admonition's other form |
| Any code block, including `statblock` | a `gm: true` option in the block's YAML | Admonition's option model — `gm` sits alongside `title`, `collapse`, `icon`, `color`, `metadata` |
| Generic block | `<!-- secret:begin -->` … `<!-- secret:end -->` | For content that is not a block type at all |

**The vocabulary is configuration, not a hardcoded list.** Which callout types
and which YAML keys mean "secret" is declared by the pack:

```yaml
secrets:
  calloutTypes: [secret, gm-only, hidden]
  codeBlockKeys: [gm, secret]
```

So a pack can add `ad-hidden`, or a GM can define a `secret` admonition
themselves in admonition's own settings, and **our app honours what they
defined** rather than insisting on our spelling. A GM who already uses
admonition does not have to learn our convention, because ours is compatible
with theirs.

### Block identity prefers the block reference

ADR-0035 made block identity a content hash, with the trade that editing a
revealed secret's text re-hides it. **The corpus solves this**: 204 blocks
already carry an Obsidian block reference (`^statblock`).

So the rule becomes:

1. If the block has an explicit Obsidian block reference, that is its id —
   stable, author-controlled, and already how the community writes them.
2. Otherwise, fall back to the content hash, with ADR-0035's trade.

Most real blocks will have an explicit id, because the community's stat
blocks do.

### We render the format; we do not distribute their content

Fantasy Statblocks carries **per-system licence disclosures** — 5e under the
Open Game License, PF2e under the Community Use Policy, Daggerheart under the
DPCGL. Each forbids charging for the content.

Our position is narrow and worth stating: **we are readers of a format, not
distributors of anyone's content.** We do not ship their stat blocks, their
layouts, or any content derived from them. We render the *format* so that a
vault the GM already has keeps working. A GM who wants stat blocks in a
published product must satisfy the terms of the content they use, and we must
not become the party that republishes it.

This is the same position ADR-0025 takes for content distribution, and it
should be applied to formats as well as to content.

## Consequences

**Good:**

- **No new dialect.** `[!secret]` was going to be *our* invention; it is
  actually the community's existing custom-type extension point. We are
  compatible, not novel.
- **The bestiary becomes more capable for free.** A GM's own `statblock` YAML
  is machine-readable, so a homebrew monster works immediately — no import
  step.
- **Block identity is mostly solved**, by the syntax the corpus already uses.
- **The golden corpus is the community's dialect**, so the round-trip test
  tests the thing users actually write.
- **A GM with admonition installed keeps their custom types**, because we
  honour the configured vocabulary rather than hardcoding a list.

**Bad:**

- **We inherit two dialects' complexity.** `ad-statblock` (prose in a code
  block) and `statblock` (structured YAML) are related but not identical,
  and the bestiary has to cope with both.
- **A stat block in the admonition dialect is prose**, so it cannot seed a
  token. Only the `statblock` dialect can. The UI must not pretend otherwise.
- **The `secrets` vocabulary is a pack setting**, so a pack author can get it
  wrong, and a GM's own admonition types live in Obsidian's settings, not in
  ours — meaning we need a way to *read* their custom types if we want to
  honour them exactly. A pragmatic default covers most of it; full fidelity
  is a settings-sync problem we have not solved.
- **Two licence regimes to keep apart** — the SRD's CC-BY-4.0 and the
  community stat blocks' per-system terms. We must not blur them.

**Neutral:**

- The pack's `content:` declarations and the stat block dialect are
  complementary: one is the *schema* a monster conforms to, the other is a
  *rendering format* a note may use.
- Nothing about the pack format changes. A pack describes the shape of a
  monster; the stat block describes how one is written down.
