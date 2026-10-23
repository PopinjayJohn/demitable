# Demitable

**Two make a full table.**

A TTRPG **system-agnostic, Obsidian-compatible** campaign operating system.

Your campaign lives as plain markdown in a plain directory. A local server
turns that directory into a game — character sheets, encounter planning, a 2D
table, recorded sessions — and a projection layer keeps the GM's secrets on
the GM's machine.

> **Status:** the design is done and phase 1 has not started. This repo
> holds the design, the decisions, the phase-0 prototype's findings, and the
> first production code.
>
> Read [`docs/design.md`](docs/design.md) for the living design document,
> [`docs/phase-1-plan.md`](docs/phase-1-plan.md) for the build plan,
> [`docs/decisions/`](docs/decisions/) for why it is shaped this way, and
> [`docs/phase-0-report.md`](docs/phase-0-report.md) for what the prototype
> phase found.

## Getting started

```
make build     the binary -> dist/demitable
make test      the Go suite
make lint      gofmt and go vet
make run       the server on :8080
make help      every target
```

**Working in this repository — human or agent — read
[`AGENTS.md`](AGENTS.md) first.** It says where the truth is, which rules
get broken by accident, and what "done" means: the milestones in
[`docs/phase-1-plan.md`](docs/phase-1-plan.md) are written as *checkable
predicates*, so a change is finished when the number is right.

## The shape of it

```
   Browser — GM                 Browser — Player
   campaign, sheets,            character, board,
   encounters, sessions         dice log, discovered lore
           │                             │
           └── HTTP + WebSocket ─────────┘
                          │
                 ┌────────▼─────────┐
                 │  Core (Go)       │  vault · packs · formulas
                 │  single binary   │  dice · projection · plugin host
                 └────────┬─────────┘
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
        ┌──────────┐ ┌─────────┐ ┌──────────┐
        │ editor   │ │ browser │ │ tabletop │
        │ (editor) │ │ search  │ │ stt, dice│   … all plugins
        └──────────┘ └─────────┘ └──────────┘
                          │
                 ┌────────▼─────────┐
                 │  A vault         │  markdown · YAML · assets
                 │  (a directory)   │
                 └──────────────────┘
```

Six ideas hold it together:

1. **Markdown is the database.** The frontmatter schema is the API. Anything
   you can write by hand, the app can read.
2. **Live state is a cache, not a source of truth.** Tonight's token positions
   are disposable. The vault is the record.
3. **The game system is data, not code.** A system pack is a YAML file. Adding
   D&D 5e, Blades in the Dark, or a homebrew ruleset means adding a file.
4. **The pack holds mechanics; the vault holds content.** A pack says what a
   class *is*. The vault holds the barbarian. The SRD — 7,000 entities — is a
   vault template, not a pack.
5. **The server is the trust boundary.** The vault never leaves the GM's
   machine; players get filtered projections of it.
6. **The app is a core plus plugins.** Everything you experience is a plugin —
   the editor, the browser, the tabletop. The core never hands a plugin the
   vault.
7. **Plugin UI is isolated by a shadow root, not a frame.** A frame was never
   a security boundary — only a style and layout one — and it broke keyboard,
   IME, clipboard and drag-and-drop. So a monster can be dragged from the
   bestiary onto the map.
8. **Obsidian is one client, not the host.** The app edits the same
   Obsidian-compatible files Obsidian does.

## Phases

| Phase | Name | Deliverable |
|---|---|---|
| 0 | The Prototype | Six weeks. Ten questions answered; the frontmatter extractor written; the interface designed |
| 1 | The Wiki (MVP) | Core UI/UX, the vault, the user system, markdown. A campaign wiki, with secrets |
| 2 | The System | Pack format completed, formula and dice engines, the 5e pack and SRD content, the Blades gate, the creation wizard, character sheets, the rich editor, archive import |
| 3 | The Openings | The WASM plugin tier, the query language, the MCP plugin, the cross-browser harness |
| 4 | The Table | Live sessions, the encounter runner, the 2D tabletop, chat, the shared display |
| 5 | The Session | Speech-to-text recording, session logs, mood-driven music |

The canonical map, including what moved and why, is
[`docs/phase-plan.md`](docs/phase-plan.md).

## Plugins

Plugins come in three tiers:

| Tier | What it is | Trusted with | Installable at runtime |
|---|---|---|---|
| **Compiled-in** | Go source linked into the binary | Everything | No |
| **WASM** | `logic.wasm` on wazero — pure Go, sandboxed | Only declared capabilities, role-scoped | Yes |
| **Vault CSS** | `Plugins/<id>/style.css` | Nothing; inert | Yes, by dropping a folder in |

Any plugin may contribute browser UI, which runs in a **shadow root** in the
app's document and talks to the core over a role-scoped bridge. A frame is
reserved for embedding a third-party page.

## Layout

```
AGENTS.md                 the brief for an agent working in this repository
Makefile                  the tasks: build, test, lint, run, release
cmd/demitable/            the entrypoint
internal/                 the Go packages
dev/demo-campaign/        phase 1's demo campaign — 21 real notes
spikes/                   phase 0's spikes — disposable, their answers are in the ADRs
docs/design.md            the living design document
docs/phase-plan.md        the canonical phase map, and what moved
docs/phase-0-plan.md      phase 0: the ten questions
docs/phase-0-brief.md     phase 0: the brief for the phase-0 agent
docs/phase-0-report.md    what phase 0 found, and what changed
docs/corpus-provenance.md the corpus's provenance, and why it is pinned by hash
docs/threat-model.md      what is defended, and what is explicitly not
docs/design-system.md     the token layer, the type scale, the colour rules
docs/phase-1-plan.md      phase 1: milestones and acceptance criteria
docs/tech-stack.md        the complete stack, with reasons
docs/api.md               the API contract — what a client may rely on
docs/operations.md        running it 24/7, exposed, and safely
docs/plugin-api.md        the plugin API — provisional by design
docs/session-writeback.md the write-back contract and visibility rules
docs/srd-vault.md         the SRD content vault: pipeline and validation
docs/research-findings.md  every research finding, and where it lands
docs/research/             the three research reports, in full
docs/shadow-dom-verification.md
                          what was actually tested about plugin isolation
docs/decisions/           architecture decision records, one per decision

packs/                    system packs — mechanics and content shapes (ships)
dev/                      development artefacts: the SRD vault and the demo
                          campaign. Never shipped
tools/                    build-time tools, not shipped in the binary
fixtures/                 transient SRD source corpus — gitignored, deleted
                          after the build
```

## Getting a vault

```
demitable init my-campaign                      # an empty campaign
demitable import https://example.org/content.zip  # merge in an archive
demitable --vault ~/vaults/my-campaign         # open one you already have
```

Content is distributed as **archives at a URL**, not from a registry we
operate. The archive carries its own licence and attribution in an optional
manifest, and the URL is wherever the author wants it.

Two things an import will not do:

- **`.demitable/` is never imported.** It holds accounts and passphrase
  hashes. An imported `players.yaml` would grant an outsider's accounts
  access to your vault. Those entries are rejected and reported.
- **Plugins are installed with a warning.** A plugin is executable code, and
  it runs with the permissions of whoever opens it — for the GM, that means
  the whole vault. The warning names each bundle and its capabilities;
  interactive imports ask first, and non-interactive ones skip plugins
  unless told otherwise.

Unpacking is the security-sensitive part: the archive is unpacked to a temp
directory, the whole tree is validated — path traversal, absolute paths,
symlinks and zip bombs all refused — and only then moved into the vault.

## Working agreement

Decisions are recorded as ADRs in `docs/decisions/` so the reasoning survives
and can be revisited. An ADR is immutable; changing a decision means writing a
new one that supersedes it. Open questions live at the end of
`docs/design.md`.

## Naming

The project is **Demitable** — a demi-table is half a table, and two of them
make a full table, which is exactly what a party of players is. The name also
plays on *demiplane*, where campaign things are half-hidden until you need
them. The binary is `demitable`; the vault's app-private folder is
`.demitable/`.
