# ADR-0049: What a frontmatter write must preserve, measured against the corpus

- **Status:** Accepted
- **Date:** 2026-10-23
- **Extends:** ADR-0021 (the editor and the round-trip contract), ADR-0040
  (the frontmatter field namespace), ADR-0043 (markdown processing)
- **Addresses:** phase 0's first question — *what does a real frontmatter
  block contain, and what must the writer preserve byte-for-byte?*

## Context

ADR-0021 promises that a frontmatter write is a **surgical edit**: the bytes
of a key's value are located in the original text and only those bytes are
replaced, so comments, key order, quoting style, indentation and line endings
survive untouched. ADR-0043 says the same about markdown.

Both promises had never been run against a real corpus. Phase 0 built the
extractor and pointed it at the 1,692 Obsidian-format markdown files in the
SRD corpus.

The corpus turned out to be **cleaner than the design assumed**, and the
cleanliness is itself the most important finding here.

### What the corpus actually contains

Every one of the 1,692 frontmatter blocks was parsed. All 1,692 parsed
cleanly: **zero failures, zero reported issues.**

| | |
|---|---|
| Markdown files | 1,693 |
| With a frontmatter block | 1,692 |
| Without one | 1 |
| Blocks that failed to parse | **0** |
| Issues reported | **0** |
| Distinct top-level keys | **4** |

The four keys are `obsidianUIMode`, `cssclasses`, `aliases` and `tags` — all
Obsidian-native, all of them the community's own. **Not one of our twelve
reserved fields appears in the corpus.** There is no `type`, no `uid`, no
`system`, no `campaign`, no `kind`, no `player`, no `workflow`, no
`visibility`, no `grants`, no `revealed`, no `discovered`.

The value shapes are just two: a plain scalar (`obsidianUIMode: preview`) and
a block sequence whose items are plain (9,071 of them) or double-quoted
(2,435). All 1,692 files are LF. There is not one comment, block scalar,
anchor, alias, merge key, flow collection, nested mapping, wikilink in a
property, quoted key, tab, or CRLF line ending in the entire corpus.

### What that means for the plan

Phase 0's plan assumed the corpus would expose the extractor's edge cases.
**It would not have.** Every construct the brief worried about — `!!merge`
keys, anchors, folded scalars, CRLF, duplicate keys, non-string keys,
Obsidian's vault-wide property types — is absent from the test data.

So the extractor needed a second corpus of its own, and the package now
carries twelve fixtures in `internal/vault/frontmatter/testdata/`, one per
construct the SRD corpus lacks. The corpus proves the parser handles real
files; the fixtures prove it handles hard ones. **A schema you have never
used is a schema you have not tested** — and the same is true of a parser.

## Decision

### 1. The surgical contract holds, and it is measured

One field of each of the 1,692 files was patched with a different value, and
every other line of the block was compared byte for byte.

> **1,692 of 1,692 files: every other byte of the block unchanged.**

No file was changed beyond the patched value. No patch was refused.

This is the property ADR-0021 promises, and it is now a measurement rather
than an intention.

### 2. Re-rendering is lossy, and that is fine — but only because the app splices

The stricter property — write every field back with the value it already
holds, and get the same bytes — **does not hold**: 30 files reproduce
exactly, 1,662 are normalised.

The reason is specific and worth knowing: a `Value` does not carry the
original node's quoting style, so a re-rendered block writes `aliases:
- Acolyte` where the corpus had `aliases: - "Acolyte"`.

This is acceptable **only** because of how the app writes: it re-renders the
one field it owns and splices every other byte. If the app ever began to
re-serialise a whole block, this property would have to be fixed first — by
carrying the node's style into `Value`.

### 3. The writer quotes a string when either real parser would not read it back as a string

Go's `yaml.v3` and `js-yaml` 4.3.2 — the parser Obsidian uses — were run
against 98 candidate scalars and compared.

They agree on the great majority. `true`/`false` are booleans; `null`, `~`,
`Null` and `NULL` are nulls; numbers are numbers; `2026-10-23` is a
timestamp; and **`yes`, `no`, `on`, `off`, `y` and `n` are strings in both**.
Both follow the YAML 1.2 core schema.

They disagree in seven places. Five do not affect the decision, because both
call the value a non-string: `1E3` and `1e3` (a float to Go, an int to
js-yaml) and `[[Note]]`, `[br]` and `{br}` (a collection to both).

Two do:

- **`2026-13-45`** — js-yaml resolves *any* `YYYY-MM-DD` shape to a
  timestamp, even an out-of-range one, where yaml.v3 leaves it a string. A
  date-shaped value is therefore always quoted.
- **`1_000`** — yaml.v3 reads the underscore as a digit separator, so it is
  an integer, where js-yaml leaves it a string. yaml.v3's own check already
  catches this one.

So the rule is: **quote a string if yaml.v3 would not read it back as a
string, or if it has the shape of a date.**

### 4. The YAML 1.1 boolean words are deliberately *not* quoted

YAML 1.1 readers — Python's PyYAML among them — read `yes`, `no`, `on`,
`off`, `y` and `n` as booleans. The app does not quote them, because:

- every tool in the vault's toolchain is not a YAML 1.1 reader — Obsidian,
  Bases and Dataview all read YAML with js-yaml, and the app reads it with
  yaml.v3, and all of them agree these are strings;
- the app never writes a boolean as anything but `true` or `false`, so the
  only exposure is a *string* value that happens to be `yes`;
- quoting them would make the app's output diverge from the ecosystem's
  idiom for a reader that does not exist here.

This was considered, measured and rejected. It is recorded because it is the
kind of decision that looks like an oversight later.

### 5. A list is written in the block's own style, and never inlined

Two rules, both learned the hard way:

- **A list or a map is never written on its key's line.** `key: - item` is
  not YAML. The first version of the writer did exactly that for a
  one-item list, and produced a file the parser then refused. A list is
  written in block style, on its own lines.
- **The block's own sequence style is preserved.** YAML allows a block
  sequence to sit at the same column as its key, and that compact style is
  what all 1,692 corpus files use:

  ```yaml
  tags:
  - npc
  - rival
  ```

  A writer that put the items one step in would change every line of every
  list it touched — exactly what a surgical write must not do. So the
  writer detects the block's sequence style and reproduces it. Only a block
  with no sequence at all falls back to a default, and the default is the
  compact style, because that is the only style measured in the corpus.

### 6. Flow collections are read but never written

The corpus contains none, and Obsidian's property editor does not write
them. The reader handles them — a flow sequence is a list, a flow mapping is
a map — but the writer always writes block style.

### 7. The writer's output for the reserved field set

The twelve reserved fields are written as:

| Field | Written as |
|---|---|
| `type`, `uid`, `name`, `system`, `campaign`, `kind`, `player`, `workflow`, `visibility` | a plain string, quoted only if a parser would not read it back as a string |
| `grants`, `revealed`, `discovered` | a block sequence, in the block's own style |

Booleans are `true` and `false`, never `yes` and `no`. A wikilink is always
quoted, because `[[Note]]` unquoted is a nested flow sequence and not a
string — the trap ADR-0040 exists for, and the corpus's own `aliases:` list
shows the community quoting for the same reason.

## Consequences

**Good:**

- **The round-trip contract is now a measurement.** 1,692 of 1,692.
- **The writer's rules are derived from the corpus and from the two parsers
  that matter**, not from a general idea of YAML.
- **The corpus's narrowness is documented**, so the next person does not
  assume it exercises the schema. It does not — not one reserved field
  appears in it.
- **The extractor's edge cases have a corpus of their own**, in `testdata/`.
- **The quoting rule is a decision with a measurement behind it**, including
  the case that was considered and rejected.

**Bad:**

- **The corpus cannot validate our schema.** Every fixture for the reserved
  fields is synthetic, and synthetic fixtures are only as good as the person
  who wrote them. The demo campaign is the next real test.
- **Re-rendering a whole block normalises quoting.** Acceptable today,
  dangerous if the writer's shape ever changes.
- **The compact-sequence default is the corpus's style, not Obsidian's
  confirmed style.** It is the only style measured. Phase 1 should check
  what Obsidian's property editor actually writes, and this ADR amended if
  it differs.

**Neutral:**

- The extractor's issue list is a *report*, not an error list. A value whose
  YAML type is not a string — a boolean, a date — is reported, because the
  type matters to a reader that treats everything as text. The caller
  decides which reports are a problem.

## A note on how this was found

Three of the six rules above were found by writing a test and reading the
failure, not by reading the design:

- the sequence style, because the corpus has 11,506 list items and no two
  styles to learn from;
- the inlined list, because a one-item list is the only case where the
  rendered body is a single line;
- the quoting rule, because the two parsers disagree in seven places and
  only a measurement finds all of them.

The rule that was *not* found — a `for` loop with no post statement, in the
function that converted a column to a byte offset — made every span the
writer computed wrong. It was found because the first version of the
extractor was never run.
