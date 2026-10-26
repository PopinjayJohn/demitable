# The published schema — Demitable

This document is the contract between a campaign vault and the app. It lists
every field the app **understands**, and it says plainly which fields the
app merely **carries**.

A GM's own `status: alive` is carried, not understood: the app leaves it
alone, and a pack's filters cannot filter by it. The reserved layer is the
only part of the frontmatter the app reads.

## Reserved fields

Twelve names, bare and top-level, simple-typed, so that Obsidian's Bases
can query them. Anything else in a note's frontmatter is the GM's own.

| Field | Type | Visibility | Vocabulary / notes |
|---|---|---|---|
| `type` | string | note's | core note type (see below) |
| `uid` | string | note's | stable identity, never changes |
| `name` | string | note's | display name |
| `system` | string | note's | pack id that interprets this note |
| `campaign` | link | note's | a wikilink to the campaign note |
| `kind` | string | note's | `pc`, `npc`, `creature` |
| `player` | string | note's | the player account that owns the note |
| `workflow` | string | note's | `draft`, `submitted`, `active`, `retired` |
| `visibility` | string | note's | `public`, `party`, `owner`, `private`, `discovered`, `gm` |
| `grants` | list | owner | additional principals who may see the note |
| `revealed` | list | note's | journalled reveal targets |
| `discovered` | list | owner | flat list of note references |

`campaign:` holds a **link** to the campaign note, written as a wikilink:

```yaml
campaign: "[[The Salt Debt]]"
```

A campaign note's own `campaign:` is the same link, self-referential.

`discovered` is a **flat list** of note references, not records:

```yaml
discovered:
  - "[[The Drowned Quarter]]"
  - "[[Cormorant Stair]]"
```

The reserved layer cannot carry the `how` or `at` of a discovery; if that
granularity is needed it becomes a separate note type, not a nested field.

### Visibility per reserved field

The reserved layer declares a visibility per field, as a pack's content
types already do:

| Field | Visibility |
|---|---|
| `discovered` | `owner` — the note's `player`, and the GM |
| `grants` | `owner` — the note's `player`, and the GM |
| every other reserved field | the note's visibility |

A note with no `player` falls back to the note's visibility (a GM-created
note has nobody else to hide from).

### Core note types

`character`, `location`, `faction`, `item`, `quest`, `session`,
`encounter`, `event`, `table`, `content`, `handout`, `message`, `journal`,
`document`, `campaign`, `home`.

`campaign` and `document` are note types: a campaign is a first-class note
with its own `members` and `packs`; a document is a thing that is simply
there — a notice, a letter, a wanted poster, a ledger page.

### Visibility vocabulary

`public`, `party`, `owner`, `private`, `discovered`, `gm`. An unknown value
fails closed and is treated as `gm`.

## What the app understands vs. what it carries

**Understood (read and acted on):** the twelve reserved fields above, plus
the fields a pack declares for its content types (the example pack
declares `species`, `occupation`, `hit_points`, `secrets`, `stage`,
`giver`).

**Carried (preserved, never read):** everything else. `status`, `date`,
`faction`, `location`, `portrait`, `map`, `tags`, `aliases`, and any other
GM-owned field. The app leaves them alone; they survive open → edit →
save byte-for-byte.

## GM-owned fields

There is no reserved `date`. A session log's date and a timeline entry's
date are the GM's own `date:`. **A GM-owned `date` must be a quoted
string.** An unquoted `2026-03-14` is a YAML timestamp, which the
extractor reports as an issue.

The same care applies to any date-shaped GM field. Booleans are fine in
GM-owned fields; the app only reports a boolean issue for a reserved
field whose declared type is not boolean.

## Starter Dataview queries

These queries are correct for the schema above. They are shown as source
and never executed by the app.

```dataview
TABLE name, type, visibility, workflow FROM "" WHERE type = "character"
```

```dataview
TABLE name, uid, campaign FROM "" WHERE type = "campaign"
```

```dataview
LIST FROM "" WHERE type = "location" AND visibility = "discovered"
```

```dataview
TABLE name, discovered FROM "" WHERE type = "character"
```

```dataview
TABLE name, hit_points FROM "" WHERE type = "character"
```

```dataview
TABLE name FROM "" WHERE type = "document"
```

## Starter Bases

Obsidian Bases (`.base` files) are not shipped in this release. The
format could not be verified against Obsidian's own parser, and shipping a
plausible-looking file that Obsidian would reject would be worse than
shipping none. The Dataview queries above are the starter views.
EOF