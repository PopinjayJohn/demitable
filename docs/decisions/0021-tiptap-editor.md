# ADR-0021: The editor is a Tiptap plugin, held to a byte-identical round-trip contract

- **Status:** Accepted, amended by ADR-0027 and ADR-0029 (the editor is a
  **compiled-in** plugin mounted in **light DOM** — the app's own content
  surface, not a shadow root and not an iframe)
- **Date:** 2026-10-21

## Context

The editor was going to be CodeMirror 6: a text editor for markdown source.
Tiptap is a ProseMirror-based rich-text editor with a structured document
model, custom nodes and marks, node views, and — since v3.7 (October 2025) —
an official bidirectional markdown extension, `@tiptap/markdown`, with custom
tokenizers and helpers for defining custom syntax.

The reasons to prefer Tiptap are not aesthetic:

- **The document is structured, not a string.** A character sheet with HP,
  conditions, dice results and linked entities wants to be real content with
  real node views, not YAML the user has to hand-edit. ProseMirror's model is
  the right shape for that.
- **Extension model.** Wikilinks, embeds, callouts, comments and block
  references become custom nodes and marks. So can anything a *third-party*
  plugin wants to add, if the editor exposes an extension registry.
- **It is an editor for prose, not for code.** CodeMirror is excellent at
  editing code and awkward at editing prose with embedded structured blocks.
  Accessibility and mobile input are better in a rich-text editor.

The risk is round-trip fidelity. `@tiptap/markdown` is **beta**, and live
issues include escaping bugs that break round-trip stability (tiptap#8519,
October 2026). Every serious markdown WYSIWYG editor mangles something:
whitespace, tight lists, setext headings, reference links, HTML blocks,
backslash escapes. And our design already promised (design doc §5.6) that the
app **preserves what it does not understand** — losing a user's unfamiliar
syntax is a trust failure, not a bug.

## Decision

**The editor is a plugin, built on Tiptap, and it is held to a
byte-identical round-trip contract.**

### It is a compiled-in plugin, mounted in light DOM

The editor ships in the binary and mounts **directly into the note surface**,
in the app's own document — not in a shadow root, and not in an iframe.

This was the alternative to "a plugin in an isolated surface", and it is the
right one for this particular plugin, for reasons that are worth stating
because they generalise:

- **A frame adds friction to editing.** Key handling, focus, IME
  composition, clipboard, and undo history all have to cross the boundary.
  On the most interaction-heavy surface in the app, that friction is the
  difference between an editor that feels native and one that feels like a
  widget.
- **A shadow root is safer than light DOM for styling, and the editor does
  not want that safety.** The editor *is* the app's content surface. It
  should share the app's typography, its focus rings, its selection
  behaviour and its theme, and it must be able to extend the note layout.
- **The editor is first-party.** It belongs in the trusted tier
  (ADR-0020), where `dom.inline` is implicit (ADR-0027).
- **It must work on first run.** A new user should be able to write a note
  without installing an editor first.
- **It needs no isolation from itself.** Isolating the editor from the app
  buys nothing that the capability model does not already give the plugin's
  *data* access.

The trade-off is real: an editor in the binary is harder to replace, and the
Tiptap dependency is baked in rather than swappable. That is acceptable for
a first-party surface, and it is the same trade every app that ships its own
editor makes.

The shadow-root default remains for every other plugin (ADR-0029).

> **Verified.** Mounting the editor in light DOM turns out to be the right
> call for a reason that was not obvious in advance: **Tiptap injects its
> ProseMirror CSS into `document.head`**, which cannot reach a shadow root, so
> an editor in a shadow root renders with `white-space: normal` and a
> ProseMirror warning unless its CSS is re-homed (`injectCSS: false` plus a
> `<style>` in the shadow root, or moving `editor.css`).
>
> CodeMirror 6 — which we use for the source view — has no such problem: it
> detects the shadow root and adopts its theme via `adoptedStyleSheets`,
> leaving `document.head` untouched.
>
> See [`shadow-dom-verification.md`](../shadow-dom-verification.md) §3.
>
> A consequence for the source view: because it is CodeMirror, it works in a
> shadow root with **zero changes**, so it could be a shadow surface if we ever
> wanted it to be.

### The contract

For every file in the golden corpus:

```
parse(markdown) → document → serialize(document) == markdown   (byte-identical)
```

This is the editor plugin's acceptance test, and it is a hard gate.

### Three mechanisms make the contract achievable

**1. The golden corpus.** A directory of deliberately awkward markdown
fixtures: tight and loose lists, setext headings, reference links, tables
with alignment, nested blockquotes, HTML blocks, footnotes, math, task lists,
wikilinks with aliases and headings, embeds, block references, callouts of
every kind, `%%comments%%`, Dataview and Bases code blocks, frontmatter with
awkward values, CRLF, BOM, no trailing newline, emoji, zero-width
characters, and a file that is nothing but a Dataview block.

Every fixture must round-trip byte-identically. New syntax support arrives
with a fixture.

**2. The raw-node escape hatch.** Any block the parser cannot represent
becomes a `raw` node holding its source verbatim, serialized back untouched.
This is how the preservation rule of §5.6 survives a WYSIWYG model: unknown
syntax is not dropped, it is held.

**3. Lossless-first editing.** An edit that cannot be expressed losslessly
is **rejected with an explanation**, not silently applied. A WYSIWYG editor
that quietly reformats a user's file is worse than one that refuses.

### Write-side verification

The vault writer does not trust the editor's output. Before writing, it
re-parses the serialized markdown and compares the resulting document to the
one the editor produced. A mismatch blocks the write and shows a diff. The
app cannot corrupt a vault with a round-trip bug.

### Editor extension registry

The editor plugin exposes a registry so other plugins can add syntax: a
spell-slot block, an inline dice result, a character chip. This is what makes
the editor a *platform* rather than a feature, and it is why the editor
belongs in a plugin tier rather than in the core.

### Mount points

The core offers named mount points — `note.body`, `note.sidebar`,
`sidebar.left`, `sidebar.right`, `panel.bottom`, `command` — and plugins
declare where they appear. A plugin can also contribute commands, keyboard
shortcuts, and context-menu entries.

## Consequences

**Good:**

- Structured content is first-class: sheets, dice results and linked entities
  are real nodes with real renderings.
- Third-party plugins can extend the editor, which is what makes an
  editor plugin worth having in a platform architecture.
- A hard, automatable acceptance test replaces "it looks right in the
  preview".
- The raw-node escape hatch means the beta risk of `@tiptap/markdown` is
  bounded: unsupported syntax degrades to preserved text, not to data loss.

**Bad:**

- **The golden corpus is a large, ongoing piece of work.** Every markdown
  construct somebody uses has to be in it, and it grows forever.
- **Beta dependency risk.** `@tiptap/markdown` is explicitly "an early
  release… subject to change". The core must not depend on its internals, so
  the editor plugin must be able to swap it out.
- **The editor is baked into the binary.** Tiptap and the markdown extension
  are build-time dependencies, not swappable at runtime. A different editor,
  or a patched markdown extension, means a new release.
- **A WYSIWYG editor changes user expectations.** People accustomed to a
  source editor will expect to see and edit raw markdown. A source view has
  to exist alongside the rich view.

**Neutral:**

- Because the editor is a plugin, it can be replaced. A different editor, or
  an editor specialised for a particular system, becomes a plugin rather than
  a fork — though replacing *this* one now means forking the binary, because
  it is compiled in.
- The vault's canonical format remains markdown. ProseMirror's JSON is an
  in-memory representation, never written to disk.
- The editor being in-document means it shares the app's DOM, so the
  capability model does not constrain it — which is correct, because it *is*
  the app's content surface.
