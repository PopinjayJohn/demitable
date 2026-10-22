# Choosing an editor: Tiptap or CodeMirror 6

> The evidence behind the decision, so it can be made on facts rather than
> preference.
>
> Trigger: ADR-0021 chose Tiptap *because* of the byte-identical round-trip
> contract, and measurement ([#20](https://github.com/PopinjayJohn/demitable/issues/20))
> shows that contract cannot be met by re-serializing markdown — by any
> library.
>
> Related: [`docs/research/implementation-risks.md`](research/implementation-risks.md) §2.1.

---

## 1. The finding that forces the question

`@tiptap/markdown` v3.32.0, 46 hand-picked fixtures: **1 byte-identical
result** (the empty document).

| Implementation | Byte-identical |
|---|---|
| `@tiptap/markdown` | 1/46 |
| `commonmark` (JS) → HTML | 1/29 |
| `markdown-it` → HTML | 1/29 |
| `marked` → HTML | 1/29 |
| `remark` + `remark-gfm` (mdast → markdown) | 8/21 hard subset, 20/29 easier |

**This is not a maturity gap.** CommonMark's parse is lossy *by design*: the
spec's "Ambiguous constructs" mean several source texts map to one AST, so a
serializer can only pick *a* canonical form.

What `@tiptap/markdown` silently loses:

| Input | Output | |
|---|---|---|
| `\| a \| b \| c \|` + delimiter row | `""` | **the entire table** |
| `![alt](img.png)` | `alt` | **the image** |
| `<div>…</div>` | text content | **the HTML block** |
| `- [ ] todo` | `- todo` | checkbox state |
| `\# not a heading` | `# not a heading` | **the escape — and it is not idempotent** |
| `AT&amp;T &copy;` | `AT&amp;T &amp;copy;` | **double-escaping; repeated saves corrupt text** |
| `Heading\n=======` | `# Heading` | setext → ATX |
| `[text][ref]` + definition | `[text](url)` | reference style |
| `_italic_` | `*italic*` | delimiter style |
| indented code | fenced code | code style |
| `line one\` | `line one  ` | hard-break style |
| `<https://x>` | `[https://x](https://x)` | autolink |
| inline HTML | markdown equivalent | HTML |

Live Tiptap issues in the same area: `tiptap#8154` (a table cell's `|` is not
re-escaped on serialize, **corrupting the table on round-trip**), `#8152` (a
U+001F cell separator leaks into the output), `#8136` (soft line breaks become
literal `\n`), `#8116` (an HTML comment inside a heading **crashes** with a
`RangeError`).

**The conclusion that matters:** the contract is satisfiable only by **keeping
the original bytes and splicing**, never by re-serializing. That is an
architecture fact, not a library preference. `remark`/`micromark` retain
`position` (line/column/**offset**) on every AST node — verified — so a
minimal splice is computable from a changed block's offset range.

---

## 2. The argument for Tiptap, and its real weight

ADR-0021's case was: "the document is structured, not a string". Concretely,
Tiptap offers:

1. **A document schema.** A typed tree that cannot be malformed. You cannot
   nest a table in a table, or put a heading inside a list item illegally.
2. **Node views as a first-class extension model.** A plugin can add a
   structured inline element with a custom renderer.
3. **Content-aware transactions.** An edit can be structural — "wrap this
   selection in a callout" — rather than textual.
4. **A large extension ecosystem** — tables, mentions, character counts, AI
   integration, and so on.
5. **Collaborative editing** via Yjs (though CodeMirror has
   `y-codemirror.next`).

### The argument is weaker than it looks, for one specific reason

**Obsidian — the format we promise compatibility with — is CodeMirror 6 with
block widgets, not ProseMirror.** Obsidian's Live Preview renders embeds,
tables, callouts, properties and block references inside a CodeMirror buffer
that stays byte-exact. It is the proof that CodeMirror can carry structured
content in a lossless buffer.

So "CodeMirror cannot do structured content" is false, and we have a working
example of the technique in the very editor we are targeting.

### What actually needs a ProseMirror tree?

Working through our own features:

| Feature | Does it need a rich document? |
|---|---|
| Notes, lore, session logs, world entries | No — prose |
| **Character sheet** | No. The pack's `views.sheet` is already a **declarative field list**. That is a *form*, not a document. The sheet is rendered from the pack; it is not a ProseMirror document. |
| **Encounter plan** | Mostly no — a form plus a list, with prose notes attached |
| **Inline stat block, dice result, embedded creature** | A **block widget** — `Decoration.replace` in CodeMirror |
| **The tabletop** | **No** — it is a separate plugin surface, not part of a note document at all |

So the surface that most obviously wants a rich document — the board — was
never a note document. And the surface that is most obviously structured — the
character sheet — is already a form by design.

**The honest conclusion: nothing in phase 1 requires a ProseMirror tree.**

---

## 3. The comparison

| | **CodeMirror 6** | **Tiptap 3 / ProseMirror** |
|---|---|---|
| Source of truth | The buffer. Byte-identity is free. | The JSON tree. Byte-identity is impossible via `@tiptap/markdown`. |
| Structured content | Block widgets via `Decoration.replace`; proven by Obsidian | Node views; a real schema |
| Editing a structured field | Text underneath; no "click a row to edit" | Native; the model is the field |
| Text editing ergonomics | Excellent. Multi-cursor, folding, LSP, lint, completion | Good, but heavier; ProseMirror is not a text editor first |
| Very large documents | Fine | **ProseMirror does not virtualize.** A 50k-word note is visibly slow to render, scroll and map. |
| Invalid states | Possible | Impossible by schema |
| Ecosystem | LSP, lint, autocomplete, formatting, markdown plugins | Tiptap extension ecosystem, Yjs collaboration |
| Bundle | Small | Larger |
| Dependency risk | Stable, boring | `@tiptap/markdown` is **beta** with live round-trip bugs |

---

## 4. The options

### A. CodeMirror 6 only, with block widgets for structured content

Prose in CodeMirror; structured content rendered as block widgets, exactly as
Obsidian does it. The character sheet is a form panel rendered from the pack.

- **Pro:** the byte-identical contract is satisfiable **by construction** —
  the buffer is the source. One editor, one selection model, one undo stack.
  The behaviour matches our compatibility target. The beta dependency
  disappears. Large notes are fine.
- **Con:** editing structured content means editing the text underneath it.
  The document schema does not exist, so a malformed structure is possible.
- **Cost:** low.

### B. Per-pack editor selection — both editors ship

The **pack** declares, per note type, whether the note is a rich document or
prose. `editor: structured | prose`. A 5e pack says a character sheet is
structured; a session log is prose. A Blades pack says its clocks are
structured.

- **Pro:** each editor does what it is good at, and the choice is made
  *per system* rather than globally — which fits a system-agnostic product.
  Tiptap keeps a real home. The byte-identity contract applies to prose notes
  and is relaxed only where the pack explicitly opts out.
- **Con:** two editors ship, two code paths, and a user switching between a
  prose note and a structured note moves between editing models. A plugin
  author must know which they are in.
- **Cost:** high.

### C. The hybrid, chosen by hand

CodeMirror for prose notes, Tiptap for structured ones, decided by the user
rather than the pack.

- **Pro:** simple rule, no pack authoring burden.
- **Con:** the user has to know which is which; and a pack's content type
  could demand a structured editor for a note the user opens in prose.
- **Cost:** high.

### D. Keep Tiptap alone, splice at the node level

Every ProseMirror node carries its source span; serialization splices rather
than regenerates.

- **Pro:** one editor, and the contract holds exactly as ADR-0021 words it.
- **Con:** deep ProseMirror work — a markdown parser *and* a WYSIWYG editor.
  The most expensive option by a wide margin, and it makes the schema
  (which is the app's whole product) a ProseMirror schema.
- **Cost:** very high.

---

## 5. What we would be giving up in A

Being clear about it, because it is the substance of the trade:

- **A document schema.** Nothing prevents a malformed note. But note that
  malformed *notes* are already possible — they are markdown files, and Obsidian
  does not validate them either.
- **Structural editing.** "Wrap this in a callout" becomes a text operation.
  However, the editor plugin can offer such commands itself; it does not need
  a ProseMirror model to do it.
- **A home for Tiptap-shaped extensions.** Not lost — a plugin can still
  ship a Tiptap surface for its own structured content, mounted into the
  editor's extension registry. It just is not the *default* prose editor.

---

## 6. The measurement that settles the rest

The research also produced a fact worth weighing: **ProseMirror does not
virtualize.** Comfortable editing is roughly ≤10k words; beyond that a note
is visibly slow to render, scroll and map, and `doc.descendants` over a large
document is O(n) per transaction.

Our corpus is 1,692 files of note-sized prose, so this is not a phase-1
problem. But a campaign vault accumulates: a five-year campaign with a long
session log and a heavily-annotated lore entry can exceed that, and a
source view that always works is a better failure mode than a rich editor
that stops rendering.

If Tiptap is used at all, the design should cap the rich editor and fall
back to the source view beyond the cap.

---

## 7. What would have to be true

For **B** (per-pack selection) to be worth its cost:

- A pack author genuinely needs a rich document for some note types, and
  that need cannot be met by a form plus widgets. The character sheet is
  already a form; the encounter plan is mostly a form. **We have not found
  the case.**
- The complexity of two editing models is acceptable to users, who are
  already learning an unusual app.

For **A** to be sufficient:

- Users are content with structured content being a rendered block rather
  than an editable tree. **Obsidian's millions of users suggest yes** — and
  Obsidian is the format we are promising compatibility with.

Our recommendation is **A**, with Tiptap retained as an *option* — a plugin
can ship a Tiptap surface, and the editor's extension registry can host it —
rather than as the default prose editor.

But this is a judgement about the product's character, and it is the user's
call. What the measurement settles is that **the byte-identity contract and
Tiptap-as-the-prose-editor are mutually exclusive**, and something has to
give.
