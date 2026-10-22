# Decision index

Every locked decision, in the order it was made. Each is immutable; changing
one means writing a new ADR that supersedes it.

| # | Decision | Status |
|---|---|---|
| [0001](0001-not-a-plugin.md) | Not an Obsidian plugin — a local web application | Accepted |
| [0002](0002-vault-is-gm-only.md) | The vault is GM-only; players access through server projections | Accepted |
| [0003](0003-system-packs.md) | Game systems are declarative YAML packs with a sandboxed formula language | Accepted |
| [0004](0004-phase-1-scope.md) | Phase 1 ships the schema plus one complete system pack, end to end | Accepted |
| [0005](0005-stack.md) | Go server + Svelte SPA, shipped as a single binary | Accepted, amended by 0019–0021 |
| [0006](0006-app-owns-prose.md) | The web app owns prose editing — Obsidian becomes one client, not the host | Accepted, amended by 0021 |
| [0007](0007-first-pack-dnd5e.md) | The first pack is D&D 5e (SRD 5.2), with schema invariants | Accepted |
| [0008](0008-dice-in-the-pack.md) | Dice mechanics live in the pack; the app is the roller | Accepted |
| [0009](0009-player-accounts.md) | Player accounts, character ownership, and a character review workflow | Accepted |
| [0010](0010-network-model.md) | LAN-first networking, designed to survive a VPN without changes | Accepted |
| [0011](0011-pack-distribution.md) | Packs are local files only — no registry, no signing | Accepted |
| [0012](0012-second-pack-blades.md) | The second pack is Blades in the Dark, as a format proof | Accepted |
| [0013](0013-search.md) | Search is literal for names and links, ranked for body text | Accepted |
| [0014](0014-mcp-in-phase-1.md) | An MCP server over the vault ships in phase 1 | Accepted |
| [0015](0015-write-journal.md) | The server keeps a write journal with revert | Accepted |
| [0016](0016-phase-3-mvp.md) | The phase 3 MVP is tokens, fog and initiative — no map editing | Accepted |
| [0017](0017-between-sessions-access.md) | Between sessions, players get read-only campaign access and their own sheet | Accepted |
| [0018](0018-transcription.md) | Transcription is pluggable, with a local engine as the default | Accepted |
| [0019](0019-core-and-plugins.md) | The app is a minimal core; every feature ships as a plugin | Accepted |
| [0020](0020-plugin-tiers.md) | Three plugin tiers — compiled-in, WASM, and vault CSS | Accepted |
| [0021](0021-editor.md) | The editor is CodeMirror 6 with block widgets, held to a byte-identical round-trip contract | Accepted, amended 2026-10-22 |
| [0022](0022-symbol-dice.md) | The dice engine supports symbol dice, via tables of faces | Accepted |
| [0023](0023-pack-mechanics-content-in-vault.md) | The pack holds mechanics; the content is vault notes | Accepted |
| [0024](0024-transient-source.md) | The SRD source is transient; the enriched output is a development artefact | Accepted |
| [0025](0025-archive-import.md) | A vault can be populated by importing an archive from a URL | Accepted |
| [0026](0026-asset-delivery.md) | Maps and assets are served by the core, role-scoped, and cached by clients | Accepted |
| [0027](0027-in-document-components.md) | Light-DOM plugin surfaces are opt-in, under an explicit capability | Accepted, amended by 0029 |
| [0029](0029-shadow-dom-isolation.md) | Plugin UI is isolated by shadow DOM, not by a frame | Accepted |
| [0030](0030-sqlite-caches.md) | SQLite holds the derived caches — the search index and live session state | Accepted |
| [0031](0031-internet-exposure.md) | The server may be internet-exposed, behind the GM's own TLS termination | Accepted |
| [0032](0032-scope-discipline.md) | Fifteen things we will not build | Accepted |

## The shape of the decisions

They cluster into six themes:

**Shape of the product** — 0001, 0005, 0006. A local web app in Go and
Svelte, shipped as one binary, that owns the editing surface and treats
Obsidian as a peer client of the file format rather than the host.

**The architecture** — 0019, 0020, 0027, 0029. The app is a minimal core
with every feature delivered as a plugin, in one of three tiers: compiled-in
Go, sandboxed WASM on wazero, or vault CSS. The core never hands a plugin
the vault. Plugin UI is isolated by a **shadow root** — a frame was never a
security boundary, only a style and layout one — and mounting in light DOM
is an explicit, per-plugin grant. **Verified empirically**, including native
drag-and-drop across the boundary
([`shadow-dom-verification.md`](../shadow-dom-verification.md)).

**Trust and access** — 0002, 0009, 0010, 0014, 0017, 0026, 0031. The vault never
leaves the GM's machine. Players reach the campaign through server-side
projections, authenticated by local accounts, over a LAN or the internet, with
an opt-in MCP endpoint as a separate, token-gated read surface — and with
read-only access to their own view of the world between sessions. Attachments
follow the same rules as the notes that reference them. The server may be
internet-exposed behind the GM's own TLS termination, which makes the import
endpoint's SSRF defence mandatory and the backup story essential.

**The data model** — 0003, 0007, 0012, 0015, 0022, 0023, 0024, 0030. Systems
are YAML packs with sandboxed formula languages; the first pack is 5e, the
second is Blades, and schema invariants stop the format inheriting either
one's shape. The pack holds mechanics and the shape of content; the content
itself is vault notes, built from a transient source corpus by a
deterministic pipeline. Dice carry payloads, not just numbers. Every
app-initiated write is journalled and revertible. SQLite holds the two
**derived caches** — the FTS5 search index and the live session state — both
outside the vault, because they can be rebuilt from it.

**Distribution** — 0011, 0024, 0025. There is no registry we operate. Packs
ship in the binary; content arrives as an archive imported from a URL. The
SRD corpus is a development input, never shipped. Because imported archives
can carry code, the formula sandbox is load-bearing rather than merely
prudent, and an import warns before installing plugins.

**Craft** — 0008, 0013, 0016, 0018, 0021. Dice are pack-defined mechanics the
app actually rolls. Search is literal for names and ranked for prose. The
tabletop does tokens, fog and initiative, and does not edit maps.
Transcription is pluggable with a local default. The editor is CodeMirror 6
with block widgets, held to byte-identical round-trip — a change forced by
measurement, since no markdown library can re-serialize byte-identically.

**Scope** — 0004, 0032. Phase 1 proves the schema with one pack end to end,
and the whole thing ends with a real campaign. Fifteen things are explicitly
out of bounds, so that a proposal to add one has to argue against a recorded
decision rather than against nothing.

## The three-layer split

The decisions stack into a consistent shape, and it is worth stating plainly
because it is the spine of the whole design:

| Layer | What | Where | Decided in |
|---|---|---|---|
| **The application** | A minimal core plus plugins | the binary | 0019, 0020 |
| **The system** | Mechanics and the shape of content | `packs/*.yaml` | 0003, 0023 |
| **The content** | The actual entities — classes, monsters, spells | vault notes | 0023 |

Each layer is data or plugins rather than code, and each is replaceable
without touching the layer above it. A new ruleset is a pack. A new monster
is a note. A new feature is a plugin.

## Research findings

Three research passes are indexed as issues, with the full reports in
`docs/research/`:

| Report | Index | Covers |
|---|---|---|
| Player and GM needs | [#18](https://github.com/PopinjayJohn/demitable/issues/18) | 16 gaps in the player and GM experience |
| Implementation risks | [#36](https://github.com/PopinjayJohn/demitable/issues/36) | The stack, measured |
| Adversarial review | [#37](https://github.com/PopinjayJohn/demitable/issues/37) | Contradictions, undecided decisions, untestable criteria |
| Shadow-DOM harness | [#32](https://github.com/PopinjayJohn/demitable/issues/32) | The empirical basis for ADR-0029 |

## A note on the plugin trust model

Three decisions interact, and they are worth reading together because the
naive reading gets it wrong:

- **0020** — a WASM plugin's *logic* is sandboxed: no network, no filesystem,
  capabilities only.
- **0029** — a plugin's *UI* is not sandboxed. It runs in the app's document,
  so it has the app's privileges. What it gets is **style and layout
  isolation** — a shadow root — not security.
- **0027** — `dom.inline` opts a plugin into the app's own styles and
  layout. It is a *placement* grant, not a privilege grant.

The split, stated once:

> **A plugin's logic is sandboxed. A plugin's UI is trusted.**

A same-origin iframe was never a security boundary either — it isolated
styles and layout while breaking keyboard, IME, clipboard and drag-and-drop
across the boundary. Dropping it bought back native interaction, which is
what a virtual tabletop needs.

## A note on privacy

Three decisions touch it, and they should be read together:

- **0002** — the vault is GM-only; players get projections.
- **0014** — the MCP endpoint is off by default and token-gated, and
  connecting a model means the campaign leaves the machine.
- **0018** — transcription is local by default and never implicitly remote.

The project's stance is the same in all three: **local by default, and never
silently remote.** Anything that sends data outward is opt-in, says so at the
moment of being enabled, and is easy to turn off.
