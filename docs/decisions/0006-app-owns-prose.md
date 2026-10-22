# ADR-0006: The web app owns prose editing — Obsidian becomes one client, not the host

- **Status:** Accepted, amended in part by ADR-0021 (the editor is Tiptap, and
  is a plugin rather than a core feature). The *decision* stands; only the
  editor technology and its packaging changed.
- **Date:** 2026-10-21

## Context

ADR-0001 removed the Obsidian plugin, leaving two ways to split editing
between the app and Obsidian:

- The app edits structured data only; Obsidian edits prose.
- The app edits everything, including prose.

The second was chosen. It has consequences well beyond a missing feature.

## Decision

**The web app is a complete wiki editor.** Notes are opened, edited and saved
in the app with a proper markdown editor: CodeMirror 6, live preview,
wikilink autocomplete, and Obsidian-compatible syntax.

Obsidian remains fully supported as an *external editor* over the same files —
the app writes standard Obsidian-compatible markdown and reads anything
Obsidian writes back.

## Consequences

**Good — and the biggest one:**

- **Obsidian is no longer a dependency.** The app works standalone for users
  who have never heard of Obsidian, and Obsidian becomes one of several
  clients of our file format. "Obsidian-compatible" becomes a compatibility
  promise rather than a platform requirement, which roughly doubles the
  addressable audience and removes the single hardest distribution constraint
  in the project.
- The app can offer integrated views that a separate editor cannot: clicking a
  wikilink in prose opens a live character sheet; the encounter runner updates
  the note it is embedded in; the session recorder writes into the log while
  you read it.
- One coherent product with one keyboard model, one search, one navigation.
  Users do not have to learn two tools and remember which one owns which file.

**Bad:**

- **We are now building a markdown editor**, and a good one. (Amended: the
  editor is a **Tiptap plugin** — see ADR-0021 — and its round-trip fidelity
  is held to a byte-identical golden-corpus contract rather than to
  inspection.) The editor is the floor of the product, and building it after
  phase 2 would mean retrofitting it into every feature.
- **The concurrent-edit surface has doubled.** Both Obsidian and the app are
  full editors of the same file. Our write semantics (design doc §5.5) reduce
  the damage but do not eliminate it.
- **The app must handle Obsidian's syntax faithfully** — wikilinks, embeds,
  callouts, frontmatter, block references, tags. "Obsidian-compatible" now
  means the parser has to agree with Obsidian on real files, not just round-trip
  its own output. This is a long tail of small incompatibilities.

### Mitigations for the concurrent-edit surface

1. **Per-key writes for structured edits** (design doc §5.5). Editing a
   character's HP never rewrites the body.
2. **Marked body sections.** The app writes generated content only inside
   `<!-- sp:begin-* -->` markers, so hand-written prose in the same file is
   never at risk.
3. **Watch and warn, never lock.** The filesystem watcher detects external
   changes and shows a "changed on disk — reload?" banner, mirroring
   Obsidian's own behaviour. No file locks: they break Obsidian.
4. **Optimistic concurrency on save.** The client sends the file version it
   based its edit on; the server rejects the save if the file changed
   underneath. No silent clobbering.
5. **The app is the primary editor.** Documented as the intended workflow, with
   Obsidian as the power-user path for prose. Users are told to close a note
   in one editor before editing it in the other.

### The long tail

Obsidian-flavoured markdown that the app must read and write:

| Syntax | Handling |
|---|---|
| `[[wikilink]]`, `[[link\|alias]]` | Full support, with autocomplete and hover preview |
| `![[embed]]` | Render as embeds; the app resolves them from the vault index |
| `[[note#^blockid]]` | Block references resolve and are linkable |
| `#tags` and `tags:` | Both respected; the app's tag browser covers both |
| `> [!callout]` | Rendered in preview |
| Frontmatter | Parsed, edited in a structured form panel, preserved verbatim on round-trip |
| `%%comments%%` | Preserved; hidden in preview |
| Dataview / Bases code blocks | Left untouched, shown as source, never executed |

The rule: **the app preserves what it does not understand.** Anything it cannot
parse is preserved byte-for-byte rather than dropped. Losing a user's
unknown-to-us syntax is the worst failure this editor can have.

## Note

This ADR makes phase 1 noticeably larger. That is accepted deliberately: the
editor is the product's floor, and building it after phase 2 would mean
retrofitting it into every feature.
