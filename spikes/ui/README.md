# Demitable — UI spike

## Questions answered

**Q3 — What does this application look like?** A disposable prototype of four
clickable surfaces sharing one token layer (CSS custom properties):

- **GM's campaign browser and dashboard** (`src/surfaces/GmDashboard.svelte`)
- **Note editor** — source pane, live preview, frontmatter panel
  (`src/surfaces/NoteEditor.svelte`)
- **Character panel** — a form rendered from the pack's declared fields
  (`src/surfaces/CharacterPanel.svelte`)
- **Player's view** (`src/surfaces/PlayerView.svelte`)

All four are clickable and share `src/lib/tokens.css`. The GM's view and the
player's view are designed together in one artefact: the same campaign,
rendered through different projections, so the projection's consequences show up
in the interface — a stat block is simply absent from a player's list rather
than hidden, a secret block is stripped from the body rather than made
invisible, and a masked property reads "Unknown to Wren" instead of leaking its
value.

The note editor and character panel are included in full; the overrun fallback
was not needed.

**Q4 — What do a callout, a stat block and a wikilink actually look like?**
Rendered from the corpus's own content, not a paraphrase:

- `ad-statblock` blocks — prose inside a code fence with a `title:` option,
  token image, labelled fields, the six-column ability table, and `## Traits` /
  `## Actions` sections (355 in the corpus, 41 distinct layouts)
- Callouts with fold markers — `[!quote]`, `[!tldr]`, `[!note]`, `[!example]`
  (120 blocks; fold markers on 2)
- `^blockid` references — 614 occurrences, 204 distinct ids
- Tables — 582; 358 of them the six-column stat-block grid
- Task lists — 0; ordered lists — 0; wikilinks `[[…]]` — 0 (the corpus links
  with relative `.md` paths and `#anchor` instead)
- A side-by-side rendering of a GM-only stat block (Adult Red Dragon,
  `visibility: gm`) beside a party-visible one (Guard, `visibility: party`),
  drawn from real corpus excerpts at `src/lib/corpus/` (see `tools/census.mjs`
  and `tools/extract-corpus.mjs`).

ADR-0036's dialect support is **sufficient for this corpus**: every construct
that exists in it has a rendering, decided and drawn here.

## How to run

```
cd spikes/ui
npm ci            # pins TS 5.9.x (TS 7 breaks svelte-check)
npm run dev       # Vite dev server
npm run build     # production build
npm run check     # svelte-check with TypeScript 5.9.x
```

Open `http://localhost:5173` (or whatever Vite prints). The topbar lets you
switch the surface, render as GM / Bram / Wren / the shared display, toggle a
colour proof mode (greyscale, flat-hue, protanopia, deuteranopia,
tritanopia), and switch light/dark themes.

## What changed

The prototype is a *reading* of ADR-0033, ADR-0034, ADR-0035, ADR-0036,
ADR-0040, ADR-0043 and ADR-0046 executed in code, so its findings are:

1. **Visibility must be shown as effective, not declared.** Every note badge
   in the dashboard and player list shows the resolved visibility
   (`party`, `gm`, `owner`, `private`) plus whether it was defaulted
   (ADR-0033 §6), and the inspector below a note cites the exact rule chain
   (scope → absolute deny → workflow gate → base level → grant → default).

2. **A GM-only stat block beside a party-visible one is the projection's
   interface.** The Adult Red Dragon (`visibility: gm`) is absent from every
   non-GM surface — no stub, no placeholder. The Guard (`visibility: party`)
   renders fully for the party. The two stat blocks are drawn side by side in
   the walk-through screenshots at `walkthrough/02-gm-only-statblock.png` and
   `walkthrough/04-party-note-with-secret.png`.

3. **Secrets are stripped, not hidden.** A `> [!secret]` callout inside a
   party-visible note (The Hollow Hill) renders for the party with the secret
   text removed; the text never reaches a non-GM client.

4. **ADR-0034's colour-independence rule survives greyscale.** The greyscale
   proof screenshot (`walkthrough/10-proof-greyscale.png`) shows every state
   still separated: visibility badges use a glyph, a word and a border *pattern*
   (dotted/dashed/repeated) in addition to hue; secrets use a dashed rule plus
   a hatch fill; selection uses an inset rule. The flat-hue proof collapses
   every semantic colour to one grey, and the only remaining differences are
   lightness, shape, label and border pattern — which is exactly what the rule
   requires.

5. **The corpus is 1,692 files, not "about 1,700".** It contains 355
   `ad-statblock` blocks, 120 callouts, 614 block references across 204
   distinct ids, 582 tables, and **no wikilinks at all** — links are relative
   `.md` paths with `#anchor`s. Task lists and ordered lists do not exist in
   the corpus.

## What this spike defers

Nothing within Q3's scope was deferred. The note editor and character panel
are built and clickable, not dropped to phase 1.

## ADR proposals

`adr-proposal.md` proposes two new ADRs in the house format, based on what the
prototype showed:

- **Proposed ADR (Q3):** the projection's consequences belong in the UI — a
  visibility badge with four channels (glyph, word, border pattern, hue) and a
  "why this role sees it" inspector. Nothing is distinguished by hue alone.
- **Proposed ADR (Q4):** the renderer renders constructs, not documents — raw
  HTML is sanitised and the corpus's malformed `class"level"` attributes are
  repaired, not silently dropped; dangling links render as present-but-not-
  followable; block references render as `^id` anchors.

The ADRs are proposed only; they are not landed here. `docs/decisions/` is
untouched.

## What's disposable

Everything under `spikes/ui/` except the two answer documents and the census
tool is disposable: the Svelte surfaces, the token layer, the demo campaign,
the markdown renderer. The *answers* (README, design-system, ADR proposals,
the walk-through screenshots and transcript) are the deliverable. The corpus
in `fixtures/` is gitignored and never committed.