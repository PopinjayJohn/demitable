# ADR-0021: The editor is CodeMirror 6 with block widgets, held to a byte-identical round-trip contract

- **Status:** Accepted, **amended 2026-10-22** — the editor was originally

  Tiptap/ProseMirror. Measurement showed the byte-identical contract cannot
  be met by re-serializing markdown, by any library, so the editor is now
  CodeMirror 6. See [`docs/editor-architecture.md`](../editor-architecture.md)
  for the full evidence. **Amended again 2026-10-23**: the phase-1 cut moves
  the *rich* layer — block widgets, the extension registry, the cross-browser
  harness — to **phase 2**. Phase 1 ships CodeMirror source editing with a
  live preview, where the buffer *is* the file.
- **Date:** 2026-10-21

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

The editor must round-trip a user's markdown **byte-identically**. Not
semantically — *byte-identically*. The user's file is their campaign, and a
tool that reformats it on save is a tool that will eventually lose something.

This ADR originally chose **Tiptap 3 / ProseMirror**, on the reasoning that
"the document is structured, not a string" — which is true, and was the wrong
question. The contract is about *bytes*, and ProseMirror's model is a tree.

### The measurement that changed the decision

`@tiptap/markdown` v3.32.0, 46 hand-picked fixtures: **1 byte-identical
result** (the empty document).

| Implementation | Byte-identical |
|---|---|
| `@tiptap/markdown` | 1/46 |
| `commonmark` (JS) → HTML | 1/29 |
| `markdown-it` → HTML | 1/29 |
| `marked` → HTML | 1/29 |
| `remark` + `remark-gfm` (mdast → markdown) | 8/21 hard subset |

This is not a maturity gap. **CommonMark's parse is lossy by design** — the
spec's "Ambiguous constructs" mean several source texts map to one AST, so a
serializer can only pick *a* canonical form, never recover the original bytes.
No reference implementation round-trips CommonMark byte-identically; `cmark`'s
own round-trip test asserts *semantic* equivalence, not byte identity.

What `@tiptap/markdown` loses silently: **entire tables** (`| a | b |` →
`""`), **images** (→ alt text), HTML blocks, checkbox state, setext headings,
reference links, emphasis delimiter style, code-block style, hard-break style,
autolinks, and **backslash escapes — which are not idempotent**, so `\# x`
becomes a heading on the second save. Entity references double-escape, so
**repeated saves progressively corrupt text**.

The only architecture that satisfies the contract is to **keep the original
bytes and splice, never re-serialize**. `remark`/`micromark` retain byte
offsets on every AST node, so a minimal splice is computable.

### The argument for a rich document, tested

"CodeMirror cannot do structured content" is false, and we have a working
example of the technique in the very editor we are targeting: **Obsidian is
CodeMirror 6 with block widgets**, not ProseMirror. It renders embeds, tables,
callouts and properties inside a byte-exact buffer.

And working through our own features, **nothing in phase 1 needs a
ProseMirror tree**:

| Feature | Needs a rich document? |
|---|---|
| Notes, lore, session logs | No — prose |
| **Character sheet** | **No.** The pack's `views.sheet` is a declarative field list. That is a *form*, not a document |
| Encounter plan | Mostly no — a form plus a list |
| Inline stat block, dice result | A block widget |
| **The tabletop** | **No** — a separate plugin surface, not part of a note document |

The surface that most obviously wanted a rich document — the board — was never
a note document. And the surface that is most obviously structured — the
sheet — is already a form by design.

## Decision

**The editor is CodeMirror 6. The buffer is the source, so the byte-identical
contract holds by construction. Structured content is rendered as block
widgets.**

### How structured content works

- **Block widgets** — `Decoration.replace` substitutes a rendered widget for a
  block, and `Decoration.widget` for an inline one. This is Obsidian's model,
  and it is the right one for us: the widget is a live view over the note's
  `data:`, and the text underneath stays exactly as the user wrote it.
- **The character sheet is a form panel**, rendered from the pack's
  `views.sheet` — not a document. The pack already declares the layout, so
  the sheet, the print view and any future client all render from one
  declaration.
- **The tabletop is a plugin surface**, not part of a note document.

### Tiptap is no longer the prose editor

Tiptap is not a dependency of the editor plugin. A plugin may still ship a
Tiptap-based surface — a rich character-sheet document, say — mounted through
the editor's extension registry, and the plugin API supports it. But it is
not the prose editor, and it does not carry the byte-identical contract.

### The contract, retained

`parse → serialize` must be **byte-identical** for every file in the golden
corpus. This is now achievable rather than aspirational, because the editor
does not re-serialize: the buffer is the source, and saving writes the buffer
back.

Three mechanisms make it hold:

1. **The golden corpus.** A directory of deliberately awkward markdown
   fixtures — tight and loose lists, setext headings, reference links, aligned
   tables, HTML blocks, autolinks, task lists, footnotes, emoji,
   zero-width characters, CRLF, BOM, no trailing newline, and a file that is
   nothing but a Dataview block. Every one must round-trip byte-identically.
   New syntax support arrives with a fixture.
2. **The raw-node escape hatch.** Any block the editor cannot represent
   becomes an opaque block holding its source verbatim, written back untouched.
   A CodeMirror plugin that cannot handle a construct simply does not match
   it.
3. **Lossless-first editing.** An edit that cannot be expressed losslessly is
   **rejected with an explanation**, not silently applied. In practice this
   means a plugin that cannot parse a block it matched must decline to match
   it.

### Write-side verification

The vault writer does not trust the editor's output. Before writing, it
re-parses the serialized markdown and compares the result to the editor's
document. A mismatch blocks the write and shows a diff. **The app cannot
corrupt a vault with a round-trip bug.**

### Frontmatter is patched, never re-encoded

The contract covers the frontmatter block too, and it is achievable because
we never re-encode the block. A frontmatter write is a **surgical edit**: the
key's span is located in the original text and only that value is replaced.
Comments, key order, quoting style, indentation, CRLF and everything else
survive.

This is what M1's per-key merge already implies, and it is why a
`yaml.Node` re-encode — which *would* destroy folded scalars, normalise CRLF,
drop the `---` marker and add a `!!merge` tag — is not part of the write
path. The rule is: **walk the `yaml.Node` tree to find the key; splice the
replacement into the original text; never re-serialise the block.**

## Consequences

**Good:**

- **The contract is satisfiable by construction**, not by hoping a library
  round-trips. The beta dependency on `@tiptap/markdown` disappears.
- **One editor, one selection model, one undo stack.**
- **Behaviour matches our compatibility target.** Obsidian solves the same
  problem the same way.
- **Large notes stay fast.** ProseMirror does not virtualise; CodeMirror does
  not have to.
- **The extension model is still open.** Plugins contribute block widgets and
  inline widgets, which is where third-party structured content lives.
- **The character sheet is a form**, rendered from one declaration, so the
  print view and any future client render it for free.

**Bad:**

- **No document schema.** Nothing prevents a malformed structure. But
  malformed *notes* are already possible — they are markdown files, and
  Obsidian does not validate them either.
- **Structural editing becomes a text operation.** "Wrap this in a callout" is
  a text edit. The editor plugin can offer such commands itself; it does not
  need a ProseMirror model to do it.
- **A widget is a view, not a model.** Editing a value inside a widget means
  editing the text underneath, or round-tripping to a form panel. For the
  character sheet that is fine, because the sheet *is* a form panel.
- **CodeMirror 6 has no schema for document-level structure**, so "the
  document is a tree" is no longer something the editor can offer.

**Neutral:**

- The editor plugin is much smaller than the one this ADR originally
  described, which is most of why the phase-1 restructuring becomes possible.
- The golden corpus is still required, and still the acceptance test. It now
  tests the editor rather than a parser we do not control.
