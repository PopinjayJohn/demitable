## The corpus

| | |
|---|---|
| Markdown files | 1693 |
| With a frontmatter block | 1692 |
| Without a frontmatter block | 1 |
| Blocks that failed to parse | 0 |
| Block line endings | LF×1692 |
| Block indentation |   ×1692 |

## Reported issues

None. Every block parsed cleanly.


## Top-level keys

| Key | Files | Value types | Writing style | Example |
|---|---|---|---|---|
| `cssclasses` | 1692 | !!seq×1692 | plain×1692 |  |
| `obsidianUIMode` | 1692 | !!str×1692 | plain×1692 | `preview` |
| `aliases` | 1663 | !!seq×1663 | plain×1663 |  |
| `tags` | 1662 | !!seq×1662 | plain×1662 |  |

## The surgical contract

One field of each file was patched with a different value, and every
other line of the block compared byte for byte. This is what ADR-0021
promises and what the app depends on.

- every other byte unchanged: **1692**
- changed: **0**
- the patch was refused: **0**

## Re-rendering fidelity

Every field of every file was written back with the value it already
holds, and the result compared with the original bytes. This is a
stronger property than the surgical contract, and it is *not*
expected to hold: a Value does not carry the original node's quoting
style, so a re-rendered block normalises its quoting. That is
acceptable only because the app re-renders the one field it owns
and splices the rest.

- reproduced exactly: **30**
- normalised: **1662**
- the write was refused: **0**

First difference:

```
fixtures/compendium/backgrounds/acolyte-xphb.md:
wrote
obsidianUIMode: preview
cssclasses:
- json5e-background
tags:
- background
- compendium/src/5e/xphb
aliases:
- Acolyte

wanted
obsidianUIMode: preview
cssclasses:
- json5e-background
tags:
- background
- compendium/src/5e/xphb
aliases:
- "Acolyte"

```

## The reserved field set

Each of the twelve reserved fields is written with a representative
value, and the result checked for type-correct YAML and for a clean
round trip.

- `type` (string)
- `uid` (string)
- `name` (string)
- `system` (string)
- `campaign` (string)
- `kind` (string) — one of pc, npc, creature
- `player` (string)
- `workflow` (string) — one of draft, submitted, active, retired
- `visibility` (string) — one of public, party, owner, private, gm
- `grants` (list, a list)
- `revealed` (list, a list)
- `discovered` (list, a list)

