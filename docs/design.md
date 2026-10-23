# Design Document — Demitable

> Living document. Updated as decisions get made.
> Locked decisions live in `docs/decisions/` as ADRs.
> Last touched: 2026-10-21 — rewritten after ADR-0001…0004.

---

## 1. The one-sentence version

A local-first, Obsidian-compatible campaign operating system: the markdown
vault is the canonical campaign database, and a local web server turns it into
a game — characters, encounters, a 2D table, recorded sessions — while a
sandboxed projection layer keeps the GM's secrets on the GM's machine.

## 2. Why this idea is worth building

The market is split, and nothing spans the split:

| Category | Players | Gap |
|---|---|---|
| Campaign wikis (Kanka, World Anvil, LegendKeeper) | Great authoring | Web, closed, you don't own the data, no live play |
| VTTs (Foundry, Roll20, Owlbear, Alchemy) | Great live play | The *story* lives elsewhere, or in a bolted-on wiki module |
| AI recorders (Tavern Scribe, Tabletop Arc, SessionKeeper) | Audio in → notes out | Batch job after the fact; no live tooling |
| Obsidian | Best-in-class markdown PKM | No game semantics, no dice, no board, no players |

The missing piece: **something that treats a markdown vault as the canonical
campaign database while providing live play tooling on top of it.**

The structural advantage over every rival: the vault is already the thing GMs
keep, already syncs, already versions in git, already works offline, and
already has an editor. We are not asking anyone to migrate their notes. We are
asking them to add a layer on top — and since the app owns the editing surface
(ADR-0006), Obsidian users are not even required to keep using Obsidian.

The differentiator over the AI recorders: they file audio into notes *after*
the session. We file it into a schema *during* the session, and the same
schema drives the table.

The differentiator over the wikis: they hold data you cannot get out. Here
the whole campaign is a directory of files you can read with `cat`.

## 3. Architectural thesis — four rules

These rules are what keep the four phases from becoming four separate products.

### Rule 1 — Markdown is the database

Canonical state lives in files. Structured data lives in YAML frontmatter and
linked files. Any state that can reasonably be expressed in markdown *must* be.
A character is a markdown file. A session log is a markdown file. A system
definition is a YAML file in the vault.

Corollary: **the frontmatter schema is the API.** If a human can hand-write a
note and the app reads it correctly, the schema is right. Every field the app
writes must be a field a human could have written.

> **Refined by ADR-0030.** The rule is about *where truth lives*, not about
> what tools are allowed. Two things live in SQLite — the full-text index and
> the live session state — and both are **derived caches that live outside
> the vault**. The test for anything that wants to go in the database:
>
> > **Can it be rebuilt from the vault, in one pass, without the vault?**
>
> If yes, it is a cache. If no, it belongs in the vault.

### Rule 2 — Live state is a cache, not a source of truth

Session-scoped mutable state (token positions, current HP, initiative order,
fog of war) changes several times per second. Writing that to the vault on
every change is insane, and writing it on a timer creates conflicts.

- **Canonical tier (vault):** notes, lore, character sheets, encounter *plans*,
  packs, session logs. Human-readable, diffable, versioned.
- **Session tier (live):** in-memory + persisted session store. A disposable
  cache of *tonight*.

At session end the session tier is written back as: updated character sheets,
an encounter summary, and a session log note. Lose the session tier, lose an
evening, not the campaign.

> **Refined by ADR-0030.** The session tier is persisted in SQLite outside the
> vault, so it is still a cache — but it now **survives a server crash
> mid-session**, which is what "a cache that vanishes" was always threatening.
> The write-back contract is unchanged.

This "write-back contract" is designed in phase 1, because phase 1 defines the
note schemas that phases 2–4 write into. Retrofitting it is expensive.

### Rule 3 — The game system is data, not code

A **system pack** is a YAML file in the vault describing a ruleset: attributes,
resources, conditions, progression, difficulty model, sheet layout, and
formulas for derived values. Adding D&D 5e, Blades in the Dark, Cairn, or a
homebrew ruleset is *adding a file*. No plugin, no fork, no code.

Packs are shareable text. They live in the vault, next to your notes, and they
diff in git like everything else. See ADR-0003 for the format and the formula
language.

### Rule 4 (added by ADR-0002) — The server is the trust boundary

The vault never leaves the GM's machine. Players receive *projections*:
filtered views assembled per request, per role, per session. A projection can
omit notes, omit properties, or mask values.

This is why the visibility model is a phase-1 schema concern and not a phase-2
feature: it is structural, and structure is expensive to change later.

### Rule 5 (added by ADR-0031) — The server must not require a browser

The only requirement for a native mobile client is that one is *possible*
later, without a major rewrite. That is a design constraint:

> **The server must not require a browser, and the browser client must hold no
> state the server does not know about.**

In practice: the API is the contract and is versioned ([`api.md`](api.md));
authentication accepts a bearer token as well as a cookie; the **server
computes** — formulas, dice, the shared log — so no client re-implements the
pack's expression languages; the pack's sheet layout and content declarations
are declarative, so a client renders them rather than reimplementing them; and
nothing server-side assumes a browser.

No native SDK is implied. The contract is the enabler.

### Rule 6 (added by ADR-0031) — Local by default; the internet is the GM's choice

The server may run 24/7 and be reachable over the internet, with TLS
termination the GM provides. But **no core feature may require the internet**:
the vault, packs, formulas, dice, projection, accounts, the editor, the
browser, the tabletop and the recorder all work with the network down.

The only optional network features are archive import, the MCP endpoint, a
cloud transcription engine, and an advisory "a new version is available"
notice. There is **no automatic update** — the app may notice one, and it must
not install one, because an update that changes the vault format under a
running campaign is not something an app should do to someone.

## 4. Architecture

```
   ┌────────────────────────┐        ┌──────────────────────────┐
   │  Browser — GM          │        │  Browser — Player        │
   │  (cockpit: campaign,   │        │  (character, board,      │
   │   sheets, encounters,   │        │   dice log, discovered   │
   │   session control)     │        │   lore — nothing else)   │
   └───────────┬────────────┘        └────────────┬─────────────┘
               │  HTTP (pages, REST)  +  WebSocket (live state)
   ┌───────────▼────────────────────────────────────▼─────────────┐
   │  Local server (Go)                                           │
   │                                                              │
   │   vault service    read · per-key write · watch · index      │
   │   pack service     load · validate · lint · interpret       │
   │   formula engine   parse · resolve dependencies · evaluate   │
   │   projection svc   visibility filter · role filter · masking │
   │   session manager  live state, initiative, write-back        │
   │   media worker     capture · transcribe (phase 4)            │
   │   static server    the embedded Svelte app                   │
   └───────────┬──────────────────────────────────────────────────┘
               │  file I/O
   ┌───────────▼──────────────────────────────────────────────────┐
   │  Vault (a plain directory of files, on disk)                 │
   │    Notes/  World/  Sessions/  Encounters/  Packs/  Assets/   │
   │    markdown + YAML + attachments                             │
   └──────────────────────────────────────────────────────────────┘
```

The vault is an ordinary directory. It may be an Obsidian vault, but it does
not have to be — the app is the primary editor (ADR-0006) and Obsidian is one
external client of the same files.

### Endpoints

The single HTTP listener serves five things:

| Path | Purpose |
|---|---|
| `/` | The Svelte app |
| `/api/v1/*` | REST — notes, packs, characters, sessions, journal, import |
| `/ws` | WebSocket — live session state (phases 2–4) |
| `/mcp` | MCP server over the vault — off by default, token-gated (ADR-0014) |
| `/asset/*` | Vault attachments — role-scoped, ETag-cached (ADR-0026) |

The API is **client-agnostic** and the contract is written down in
[`api.md`](api.md): a native mobile app could be built against it. The
projection is server-side, so a client never holds data it is not allowed to
see, and every endpoint accepts either a session cookie or a bearer token.

### Implementation shape (ADR-0005)

```
demitable/
  cmd/demitable/      entrypoint: config, open vault, start server, open browser
  internal/
    vault/            index · frontmatter · write semantics · watcher · journal
    pack/             loading · validation · lint
    formula/          expression language: lexer, parser, evaluator
    dice/             dice engine: faces, tables, pools, opposed rolls
    entity/           note model · universal fields · visibility
    projection/       role and visibility filtering — the trust boundary
    accounts/         local accounts, join codes, sessions
    plugin/           host: registry · capabilities · WASM runtime · assets
    server/           routes · REST · WebSocket · embedded static assets
  plugins/            first-party plugins, compiled in
    editor/           CodeMirror editor plugin
    browser/          note browser, typed views, dashboards
    search/           literal + ranked search
    mcp/              MCP server over the vault
    diceui/           dice roller interface
  packs/              built-in system packs → embedded
  web/                Svelte shell → web/dist
```

`go build` produces one static binary containing the API, the WebSocket, the
packs, the built frontend and every first-party plugin. Running it is
`demitable --vault <dir>`. No runtime dependencies, no `npm` on the user's
machine, no container.

### The core and the plugins (ADR-0019)

The application is a **minimal core plus plugins**. The core is what must
never break and must never be trusted blindly; everything the user experiences
is a plugin.

**Core:**

| Area | Why it cannot be a plugin |
|---|---|
| Vault service — read, write, watch, index, journal | Owns the user's data |
| Pack format and its runtimes (formula and dice engines) | The core's own data contract |
| Accounts, roles, projection | **The trust boundary** |
| Plugin host | Self-evidently |
| HTTP/WS server, asset serving | Self-evidently |
| UI shell | Something must exist for a plugin to render into |

**Plugins:** the editor · the note browser and typed views · dashboards ·
search · character sheets · the creation wizard · the encounter planner and
runner · the dice interface · the tabletop · session recording · music · the
MCP server.

The governing rule: **the core never hands a plugin the vault.** It hands the
plugin a *scoped view* — a capability object exposing exactly the operations
the plugin's manifest declares, filtered by the caller's role. This is the
projection principle of ADR-0002 applied one level down.

The plugin API is **not designed up front**. The editor is written as the
first plugin against a provisional API; the API grows to fit it; a second,
non-UI plugin proves it is not accidentally editor-shaped; only then is it
frozen at `api: 1`.

### Plugin tiers (ADR-0020)

The tier describes how a plugin's **server-side logic** is delivered. Any
plugin may contribute browser UI, which runs client-side in the app's
document.

| Tier | What it is | Trusted with | Runtime install |
|---|---|---|---|
| **Compiled-in** | Go source linked into the binary at build time | Everything — it *is* the program | No |
| **WASM** | `logic.wasm` run by **wazero**, pure Go, no cgo | Only declared capabilities, role-scoped | Yes |
| **Vault CSS** | `Plugins/<id>/style.css` in the vault | Nothing; inert | Yes, by dropping a folder in |

Go's `plugin` package is explicitly not used: it is platform-limited,
toolchain-coupled, and it breaks both cross-compilation and the
single-binary promise.

A WASM plugin uses an Extism-shaped ABI — one exported entry point taking
and returning JSON, plus host functions (`vault.read`, `vault.write`,
`vault.query`, `http.fetch`, `log`, `now`) granted only if the manifest
declares them. **A plugin never gets raw filesystem access to the vault**, so
the core can always enforce role-scoped visibility. Network access is absent
by default: WASI preview 1 has no sockets.

### Plugin surfaces (ADR-0029)

| Kind | Mechanism | Who uses it |
|---|---|---|
| **Shadow** (default) | A `<div>` host with an attached shadow root | Almost every plugin |
| **Light DOM** (opt-in) | Mounted into a named host the app provides | The editor, the table |
| **Frame** (opt-in) | A separate origin, `sandbox`ed | Embedding a third-party page or untrusted HTML |

Shadow DOM's sharp edges, which the design accounts for: inherited properties
and custom properties cross the boundary but element styles do not;
`event.target` retargets to the host, so handlers use
`event.composedPath()[0]`; and `adoptedStyleSheets` is the modern way to
inject styles into a shadow root.

### The plugin API surface

```yaml
id: demitable.editor
name: Editor
version: 1.0.0
api: 1
capabilities: [vault.read, vault.write]
provides:
  - { id: editor, kind: service }
  - { id: editor.surface, kind: ui, title: Editor, entry: ui/index.html, mount: note.body }
consumes:
  - { service: dice, version: ">=1" }
```

- **Services** — the core resolves a service by id and version range. The dice
  engine, the formula engine and the pack registry are core services that
  plugins consume.
- **Mount points** — `note.body`, `note.sidebar`, `sidebar.left`,
  `sidebar.right`, `panel.bottom`, `command`. Plugins declare where they
  appear; they also contribute commands, shortcuts and context-menu entries.
- **The browser bridge** — a versioned, role-scoped `postMessage` API
  (`call`, `on`, `emit`). A player-scoped plugin instance never receives
  GM-scoped bridge methods.
- **Capabilities** — granted per plugin, per role, and enforced by the core
  rather than by convention.

### Assets

Maps, portraits, token images and handouts are vault attachments, and they
are subject to the same visibility rules as the notes that reference them
(ADR-0026):

> **An attachment is visible to a player if a note visible to that player
> references it, or if it is explicitly marked public.**

The core serves them at `/asset/<path>`, filtered by the projection, with
content-hash ETags so clients cache them. Assets the caller cannot see
return **404, not 403** — a 403 confirms the path exists, which is itself a
leak.

Assets referenced by no note are GM-only by default. Portraits follow their
character's visibility automatically.

### Plugin UI

Plugin surfaces are **shadow roots** in the app's document (ADR-0029), talking
over a role-scoped bridge (ADR-0020). A plugin may opt into **light DOM** —
mounting into the app's own document — under an explicit `dom.inline`
capability (ADR-0027).

> **A plugin is trusted at the privilege level of whoever opened it.** The
> capability model constrains *data reach* and gives role-scoped instances; it
> is not a defence against a hostile plugin. A sandboxed plugin's *logic* is
> confined; its *UI* is not.

The privilege model, stated once and correctly:

> **All plugin UI runs in the app's document, and therefore has the app's
> privileges. A plugin's *logic* is sandboxed; a plugin's *UI* is trusted.**

A same-origin iframe was never a security boundary — it isolated *styles and
layout*, while breaking keyboard shortcuts, IME, clipboard and drag-and-drop
across the boundary. Shadow DOM keeps the style isolation and none of the
cost, which is why the bestiary can be dragged onto the map.

Theming crosses the boundary by inheritance: custom properties on `:root`
reach every shadow surface, while the app's element styles do not.

### The vault lifecycle

A vault is a directory. It gets into your hands in one of three ways:

| Route | What it is |
|---|---|
| **Empty** | `demitable init` — the scaffolder creates folders, a `Home.md` and one template per note type |
| **Imported** | `demitable import <url>` — download an archive and merge it in |
| **An existing directory** | `demitable --vault <dir>` — you already have notes; the app opens them |

### Deployment

The server may run 24/7 on the GM's machine and be reachable over the
internet. It speaks plain HTTP and sits behind TLS termination the GM
provides (ADR-0031):

```
        internet
            │  https, GM's certificate
            ▼
   ┌──────────────────┐
   │  Caddy / nginx / │   ← the GM's responsibility:
   │  a tunnel        │      certificate, renewal, firewall, backups
   └────────┬─────────┘
            │  http://127.0.0.1:8080
            ▼
   ┌──────────────────┐
   │  demitable       │   ← plain HTTP, 24/7, on the GM's machine
   │  the vault       │
   └──────────────────┘
```

`--expose` turns on the defaults that only matter when the server is reachable
from outside: bind to all interfaces, always-secure cookies, trusted forwarded
headers only from a configured proxy, a **required** public base URL for
generated links, stricter auth rate limits, mandatory GM authentication, and
an audit log of reads of GM-only notes.

The import endpoint defends against SSRF — private address ranges refused
after resolution, DNS rebinding prevented by connecting to the resolved
address, and size, time and redirect caps — because a server that fetches
arbitrary URLs from the internet is a request-forgery primitive.

### One server, one vault

A server process serves **exactly one vault**, and that vault holds any number
of campaigns (ADR-0037, ADR-0041). The vault is the unit of everything —
backup, sync, sharing, and the trust boundary — and the accounts file lives
inside it, so it travels with the campaign.

The URL carries the **campaign**, not the vault. A GM with several campaigns
runs several processes; a reverse proxy can give the players one URL.

### Distribution: an archive at a URL

ADR-0011 ruled out an operated package registry, because packs contain
formulas and a registry we run is a supply-chain surface of our own making.
But ruling out a *hosted index we operate* is not ruling out *distribution*.

The mechanism that satisfies the constraints is an **archive at a URL**
(ADR-0025). The URL can be a release asset, a website, an object store — the
distribution point is the content author's choice, and the mechanism is a
plain file fetch. Licence terms travel with the content, in a manifest beside
the files.

```
demitable import https://example.org/dnd5e-srd-content.zip
```

- The archive carries an optional `demitable-import.json` manifest with its
  id, version, required packs, licence and attribution.
- **Extraction is the security-sensitive part.** The archive is unpacked to a
  temporary directory, the whole tree is validated, and only then is it moved
  into the vault. Path traversal, absolute paths and symlinks are refused;
  entry count, total size and compression ratio are capped.
- **`.demitable/` and `.git/` are always rejected and reported.** An imported
  `players.yaml` would silently grant an outsider's accounts access to the
  vault. Player accounts do not travel with content — that is deliberate.
- **Plugin bundles are installed, but the import warns first.** A plugin is
  executable code. The warning names each bundle, its version and its
  capabilities, and says plainly that a plugin runs with the permissions of
  whoever opens it. Interactive imports require confirmation; non-interactive
  ones skip plugins unless told otherwise.

There is still **no registry we operate** — no index, no signing keys, no
revocation, no supply chain of our own. Discovery is a README, a forum post,
a link.

## 5. The vault schema

### 5.1 Note types

| Type | Purpose | Lives in |
|---|---|---|
| `character` | PC or NPC | `Notes/Characters/` |
| `location` | Place | `Notes/World/Locations/` |
| `faction` | Organisation | `Notes/World/Factions/` |
| `item` | Relic, gear, artefact | `Notes/World/Items/` |
| `quest` | Objective chain | `Notes/Quests/` |
| `session` | Session log | `Sessions/` |
| `encounter` | Planned or run encounter | `Encounters/` |
| `event` | Timeline event | `Notes/World/Timeline/` |
| `table` | A set of faces — symbol dice, random tables, oracle decks | `Tables/` |
| `content` | A pack-declared content object — a class, a species, a spell, a monster | `Notes/Content/` |
| `handout` | A prop revealed to the table, with a journalled reveal event | `Notes/Handouts/` |
| `home` | Campaign dashboard / MOC | `Notes/Home.md` |

Folders are a **human convention**, not a machine contract. The machine reads
`type:` from frontmatter. This means a user can reorganise folders without
breaking the app, and a note can live anywhere.

### 5.2 The frontmatter contract

Two layers, always:

**Universal top-level fields** — the same across every system, so they can be
sorted, filtered and grouped by any tool (including Obsidian's Bases, whose
property system does not support nested objects):

```yaml
type: character      # note type
uid: 01JBX8K2M9Q      # stable identity, never changes
name: Ilia Vane
system: dnd5e        # which pack interprets this entity
campaign: stormdark  # which campaign this belongs to; absent = personal
kind: pc             # pc | npc | creature
player: p_8f2a       # the player account that owns this note
workflow: active     # draft | submitted | active | retired
visibility: party    # public | party | owner | gm
grants: [p_91bc]     # additional parties who may see this note
faction: "[[Saltmarsh Cartel]]"
location: "[[The Drowned Quarter]]"
portrait: Assets/Portraits/ilia-vane.png
visibility: party    # public | party | owner | gm  (see 5.4)
tags: [npc, rival, harbor]
```

For `type: content`, `content_type` names the pack-declared kind — `class`,
`species`, `spell`, `monster` — and `system` scopes it to the pack that
declared it (ADR-0023).

`player` and `workflow` exist because players may create and own their own
characters (ADR-0009). `player` is what `visibility: owner` resolves against;
`workflow` is the app-level review gate that a player-authored note passes
through before the table sees it. Neither is pack-specific — they describe who
may speak for a thing, not how the thing works.

**There is no generic `status` field.** A pack declares the fields its own
content types need, in its `content:` declarations, and a GM's own `status:
alive` is left alone (ADR-0040).

The reserved set is twelve names, all bare and top-level, so that Obsidian's
Bases can query them and the community's existing views work unmodified
(ADR-0040).

**System-specific data** — nested, interpreted by the pack, and the only place
a system is allowed to be weird:

```yaml
data:
  attributes:
    str: 10
    dex: 14
    con: 12
    int: 8
    wis: 15
    cha: 11
  resources:
    hp: 23
    ac: 15
    speed: 30
  conditions: []
  skills:
    - key: stealth
      label: Stealth
      value: 4
```

Nested data keeps the universal fields clean and lets a pack define whatever
shape it needs. Dataview handles it well; Bases sees only the universal layer.
Both work, which is the right trade-off.

### 5.3 Identity and renaming

Filenames change when humans rename things. Wikilinks survive that in Obsidian,
but our tooling needs identity that does not move. Every note gets a `uid` on
creation — a short ULID. Never reused, never changed, machine-only.

Links are stored as wikilinks (`[[Ilia Vane]]`) so Obsidian resolves them.
Resolution goes through the vault index, which maps both filenames and aliases
to uids, and can therefore tell us "the GM renamed this note, follow it".

### 5.4 The visibility model

Settled in [ADR-0033](decisions/0033-campaign-and-grants.md). A note's
visibility is resolved in this order; the first rule that applies wins.

**1. Scope.** A note with `campaign: X` is visible only to members of X, plus
the GM. A note with no campaign is visible to the GM, and to anyone
explicitly granted. One vault can hold any number of campaigns, and
`visibility: party` means **"all players in this note's campaign"** — not "in
this session". That distinction is the leak the old model had.

A **campaign is a note** (ADR-0037), and it is what defines its members. So
for a campaign note, `party` resolves against **its own `members` list** —
one rule, and "party" finally means something.

**2. The absolute deny.** `visibility: gm` — on a note, or on a property of
one — is **never** serialised into a non-GM response. No grant overrides it.
This is what makes the model predictable, and it is why a *revealable secret*
is a different field rather than a note-level `gm`.

**3. The workflow gate.** A note whose `workflow` is `draft` or `submitted` is
visible **only to its owner and the GM**, regardless of its visibility and
regardless of any grant. A grant-suppressing deny.

**4. The base level.**

| `visibility` | Who |
|---|---|
| `public` | Anyone, authenticated or not — a link is enough |
| `party` | All members of the note's campaign |
| `owner` | The note's `owner`, and the GM |
| `gm` | The GM only. Never serialised to a non-GM role |

**5. Grants.** `grants: [p_8f2a, p_91bc]` — *additional* parties who may
see the note, on top of the base level. A grant never overrides rules 2 or 3.

**6. The default**, when `visibility` is absent:

- a note **with** a campaign defaults to `party`;
- a note **without** one defaults to `gm`.

So a note inside a campaign is shared with the table unless it says otherwise,
and a note outside one is private unless it says otherwise. The UI must show
the *effective* visibility on every note.

**7. Property level.** A property may declare its own visibility, in the
pack, and **inherits the note's** otherwise. `owner` on a property resolves
against the note's `player`; **if the note has no player, it falls back to the
note's visibility.** Without that rule the shipped 5e pack hid every NPC's hit
points from players. `gm` on a property is absolute.

**8. Masking.** `display:` overrides the rendered value per audience, applied
**last** — after grants and denies have decided whether a value is present.

**Reveals.** A **handout** is a note whose visibility is mutable at runtime;
revealing it is a **journalled event** (ADR-0015), and it can target
everyone, one player, or the shared display.

A **secret** is any **block marked GM-only**: an Obsidian callout of a
secret type (`> [!secret]`), an admonition code block (` ```ad-secret `), a
`gm: true` option on any code block including a `statblock`, or an explicit
marker block. The vocabulary is **pack configuration**, so a GM who already
uses admonition's custom types keeps them (ADR-0036).

A secret is a **deferred grant** rather than a deny, and it can be promoted in
place. Two things follow from that distinction: the absolute deny of rule 2 is
untouched, and a secret is giveable later.

**The secret's text is stripped server-side.** It is never serialised into a
non-GM response, so a player who inspects the network, the DOM, or the
WebSocket frames sees nothing. The reveal itself is a **live broadcast** over
the WebSocket, not a file write — the table sees it as it happens, rather than
after a watcher round trip. See [ADR-0035](decisions/0035-secret-callouts.md).

**Content visibility is declared by the pack**, per content type, so the SRD
ships with the right defaults — `spell` is `party`, `monster` is `gm` — rather
than with an accident.

**Why this needs a UI.** Six rules in order is more than one lookup. The
app needs a **"why can this role see this note?" inspector**, or the
complexity will bite in the wrong places.

### 5.5 Write semantics

The app and Obsidian can both edit the same file. Three rules keep this safe:

1. **Per-key frontmatter writes for structured edits.** When the app changes a
   character's hit points, it never rewrites the whole frontmatter block. It
   reads the current file, applies the changed keys, writes back. Two editors
   touching different fields do not conflict.
2. **Marked body sections for generated content.** The app writes only inside
   markers, so hand-written prose in the same file is never at risk:

   ```markdown
   ## Session Log
   <!-- sp:begin-session-log -->
   …app-generated content…
   <!-- sp:end-session-log -->
   ```
3. **Optimistic concurrency on full-file saves.** When the app saves prose it
   sends the version it based the edit on. If the file changed underneath, the
   save is rejected rather than silently clobbering. Combined with the
   filesystem watcher, this turns "both editors" from a corruption risk into a
   prompt.

No file locks, ever — they break Obsidian.

### 5.6 Obsidian-flavoured markdown

The app reads and writes what Obsidian writes (ADR-0006), which means agreeing
with Obsidian's syntax, not just round-tripping our own output. The editor is
a CodeMirror plugin (ADR-0021), so the document is a **text buffer with
structured blocks rendered as widgets** — the text on disk is the source, and
the block widgets are views over the note's `data:`. Nothing but the text is
ever written to the vault.

| Syntax | Handling |
|---|---|
| `[[wikilink]]`, `[[link\|alias]]` | Full support, autocomplete, hover preview |
| `![[embed]]` | Rendered; resolved from the vault index |
| `[[note#^blockid]]` | Block references resolve and are linkable |
| `#tags` and `tags:` | Both respected; one tag browser covers both |
| `> [!callout]` | Rendered in preview |
| `> [!secret]` | A **secret callout** — GM-only until revealed, and the reveal is broadcast live (ADR-0035) |
| ` ```ad-<type> ` | Admonition code block, with YAML options at the top of the block (ADR-0036) |
| ` ```statblock ` | A community stat block, in structured YAML — read by the bestiary (ADR-0036) |
| `^blockid` | An Obsidian block reference. The preferred stable id for a block (ADR-0036) |
| Frontmatter | Parsed, edited in a form panel, **preserved verbatim on round-trip** |
| `%%comments%%` | Preserved, hidden in preview |
| Dataview / Bases blocks | Left untouched, shown as source, never executed |

**The governing rule: the app preserves what it does not understand.**
Anything it cannot parse is kept byte-for-byte rather than dropped. Silently
losing a user's unfamiliar syntax is the worst failure this editor can have,
and it is a trust failure, not a bug.

Three mechanisms make that rule hold under a WYSIWYG model (ADR-0021):

1. **The raw-node escape hatch.** Any block the parser cannot represent
   becomes a `raw` node holding its source verbatim, serialized back
   untouched.
2. **The buffer is the source.** The editor is CodeMirror 6 (ADR-0021) and
   does not re-serialize — saving writes the buffer back. The contract is
   satisfiable *by construction*, not by hoping a library round-trips. (It
   does not: measured, no markdown library re-serializes byte-identically,
   because CommonMark's parse is lossy by design.)
3. **The golden corpus.** A directory of deliberately awkward markdown
   fixtures. Every one must round-trip **byte-identically**. New syntax
   support arrives with a fixture.
4. **The raw-block escape hatch.** Any block the editor cannot represent
   becomes an opaque block holding its source verbatim, written back
   untouched. A plugin that cannot parse a block it matched simply declines
   to match it.
5. **Lossless-first editing.** An edit that cannot be expressed losslessly is
   rejected with an explanation, not silently applied.
6. **Frontmatter is patched, never re-encoded.** A frontmatter write is a
   surgical edit: the key's span is located in the original text and only
   that value is replaced. Comments, key order, quoting, indentation and
   CRLF all survive. The block is never re-serialised, because a
   `yaml.Node` re-encode would destroy folded scalars, normalise CRLF, drop
   the `---` marker and add a `!!merge` tag.

And on the write side, the vault writer does not trust the editor: it
re-parses the serialized output and compares it to what the editor produced. A
mismatch blocks the write and shows a diff. **The app cannot corrupt a vault
with a round-trip bug.**

### 5.7 Scaffolding

A new campaign is created by the app: folders, a `Home.md` dashboard, starter
templates for each note type, one example character, and a chosen system pack
installed. The scaffolder is also a template engine — every note type has a
template with prompts, used by "new note" and by Templater-style tooling.

### 5.8 App-private storage

The app keeps its own state in one hidden folder at the vault root, so it
travels with the campaign and is covered by the user's existing backup and
sync:

```
.demitable/
  players.yaml        player accounts and ownership (ADR-0009)
  journal/            append-only write journal, one file per day (ADR-0015)
  blobs/              content-addressed prior versions, for revert
  config.yaml         app settings: which packs are installed, defaults
```

`.demitable/` is **excluded from the note index, from both search indexes, and
from the MCP resource list.** A file the app cannot see is a file the app
cannot corrupt. It is a dot-folder so Obsidian's file browser and Bases treat
it as ordinary non-note content. It is also **never importable** — an archive
containing `.demitable/` entries is rejected (ADR-0025), because an imported
`players.yaml` would grant an outsider's accounts access to the vault.

> ### Two directories, similar names
>
> - `.demitable/` — **inside the vault.** Travels with the campaign, covered by
>   the user's backup and sync. Holds accounts, the journal, and app config.
>   Never importable.
> - `~/.local/share/demitable/` — **outside the vault.** The XDG app-data
>   directory. Holds the SQLite caches: the FTS5 search index and live session
>   state (ADR-0030). Never synced, never backed up, rebuildable from the
>   vault, and **not** in the vault precisely so a sync client can never try
>   to merge a binary database.
>
> They are different things, and the resemblance is a hazard. If you are ever
> unsure which one a path refers to, the rule is simple: **if it is inside
> the vault it is `.demitable/`; if it is an absolute path under
> `~/.local/share` it is app data.**

Everything else in the vault is user content, readable and writable by the
user, and readable by the app with no exceptions.

### 5.9 Search

Two mechanisms behind one search box (ADR-0013):

- **Literal index** — case-insensitive exact and prefix matching over file
  name, `name`, `aliases`, and wikilink targets. Prefix-anchored with a
  substring fallback. This is what runs on every keystroke and fills the
  results first, because it returns the note you meant.
- **Ranked index** — SQLite FTS5 over tokenised body text, with title and
  alias boost (ADR-0030). This surfaces the note you did not know you were
  looking for. The body is preprocessed before indexing: frontmatter
  stripped, markdown removed, wikilinks resolved to plain text, code blocks
  and comments skipped.

The two live in different places, for a reason. The **literal index is in
memory** — it is exact lookup over names and links, cheap to rebuild, and
needed live by the projection layer. The **ranked index is SQLite FTS5**,
persisted outside the vault, because it is the one index expensive enough to
be worth not rebuilding on every launch, and because FTS5 gives a correct
tuned BM25, prefix and phrase queries, and snippet generation for free.

Both are maintained incrementally by the watcher. Both are filtered by role
**before** results reach a player, so search is part of the security boundary
rather than a way around it.

## 6. The pack format

Full spec in ADR-0003. Sketch:
```yaml
id: dnd5e
name: D&D 5th Edition (SRD 5.2)
version: 1.0.0

attributes:
  - { key: str, label: Strength,  type: number, default: 10, min: 1, max: 30 }
  - { key: dex, label: Dexterity, type: number, default: 10, min: 1, max: 30 }
  # …

resources:
  - { key: hp, label: Hit Points, kind: health, default: 1,
      maxFormula: max_hp, visibility: owner }
  - { key: ac, label: Armour Class, kind: defence, default: 10 }

formulas:
  con_mod: "floor((attributes.con - 10) / 2)"
  max_hp: "10 + attributes.con + level * con_mod"
  attack_mod: "attributes.str_mod + proficiency + weapon.bonus"

conditions:
  - { key: prone, label: Prone }
  - { key: poisoned, label: Poisoned }

progression:
  levels: { max: 20, label: Level }

difficulty:
  model: xp-budget
  budgetPerLevel: 4
  thresholds:
    trivial: 0.5
    easy: 1.0
    medium: 1.5
    hard: 2.0
    deadly: 3.0

views:
  sheet:
    - { type: attribute, key: attributes.str, label: STR }
    - { type: resource,  key: resources.hp,   label: HP }
    - { type: formula,   key: max_hp,         label: Max HP }
```

The pack defines not just the data shape but the **sheet layout** and the
**difficulty model**. A 5e pack budgets an encounter by XP; a Blades pack
budgets it by opposition dice and clock pressure; a dice-pool pack budgets it
by opposed pools. Each pack disagrees, correctly, because the systems disagree.

### 6.1 Schema invariants
Because the first pack is 5e (ADR-0007), the schema carries an explicit list
of properties it must *not* assume. These are enforced by a pack linter, and
they are the mechanism that keeps "system-agnostic" from being a slogan:

1. A pack need not define any **resources**.
2. A pack need not define **numeric attributes** (Fate's aspects are strings).
3. A pack need not define **levels**.
4. A pack need not define **attributes at all**.
5. A pack need not define **conditions**.
6. Every pack must express a **player character**, an **NPC**, and a
   **creature**.
7. Difficulty is modelled **by the pack**, in the pack's own terms. The app
   never assumes an XP budget.

A second, deliberately differently-shaped pack is required before phase 2 is
considered validated.

### 6.2 Dice

The app is the dice roller; the pack defines how (ADR-0008). A pack's `dice:`
block declares the notation, the resolution rules, and the named roll
workflows:

```yaml
dice:
  default: "1d20"
  advantage: { mode: keep-highest, dice: 2, sides: 20 }
  critical: { when: natural-max, effect: "1d6" }
  direction: high                     # high | low — roll-under systems invert
  degreesOfSuccess: []                # [pass, good, great, amazing]
  workflows:
    - { key: attack, label: Attack Roll, dice: "1d20",
        against: resources.ac, bonus: attack_bonus }
    - { key: save,   label: Saving Throw, dice: "1d20",
        against: dc, bonus: save_bonus }
```

The dice expression language supports `NdM`, flat modifiers, keep-highest and
keep-lowest (`4d6kh3`, `2d20kl1`), exploding dice (`6d6>=5`), Fudge dice
(`4dF`), wild percentile, success thresholds, and opposed pools. It is a
second expression language alongside the formula language, sharing the same
lexer foundations and the same safety rules.

A dice-pool system declares entirely different workflows and still fits:

```yaml
dice:
  direction: high
  workflows:
    - { key: pool, label: Roll, dice: "{rating}d6", vs: "{opposition}d6",
        outcome: highest-count }
```

Every roll is logged with full provenance — roller, workflow, expression,
individual die results, modifier sources, target, outcome, timestamp — and
that log is written into the session note. It is both a campaign asset and the
natural join point between the tabletop and the phase-4 recorder.

### 6.3 Symbol dice and tables

Not every die has numbers on it (ADR-0022). A die's faces carry **payloads**,
and the unified concept is a **table**: a named set of faces that a roll picks
from.

```yaml
- id: fehu
  label: Fehu
  symbol: "ᚠ"
  asset: null          # optional: Assets/Tables/futhark/fehu.svg
  value: null          # optional: a number for arithmetic
  rank: 1              # optional: an ordering for comparison
  tags: [success]      # optional: success | failure | wild | critical
```

- `value` is optional — pure symbol dice have no numbers.
- `rank` is optional — an unordered die **cannot be compared**, and the engine
  says so rather than inventing an order.
- A table can mix numeric and symbolic faces, which is how a "critical d6"
  works with no special case.

```yaml
dice:
  sets:
    - { key: d6,  sides: 6, numeric: true }
    - { key: rune, sides: 6, table: elder-futhark, numeric: false }
    - { key: omen, sides: 8, table: elder-futhark, numeric: false, ranked: true }
```

**Tables are ordinary notes** — `type: table`, faces in `data.faces` — so a
GM can write one in Obsidian, link it into a session log, and have the
projection layer handle its visibility like any other content. Packs may ship
tables too; a vault table shadows a pack table of the same id.

Result types are derived from the definition, not guessed: `numeric` (a
total), `symbol` (the face), `count` (successes in a pool, from tagged faces
or a rank threshold), `named` (a table lookup).

This one mechanism covers numeric dice, rune dice, tarot, oracle cards,
icon dice, the 5e *Divine Roller*, wild magic surges, and "roll for a
rumour" tables.

### 6.4 Content declarations

A pack declares the kinds of content object it has, and the shape of each
(ADR-0023). This is what lets the character creation wizard stay
system-agnostic: it reads the declarations to know what a class *is*, and
reads the vault's content notes to know what the classes *are*.

```yaml
content:
  - id: class
    label: Class
    fields:
      - { key: hit_die, label: Hit Die, type: number, required: true }
      - { key: primary_abilities, type: list, of: ability }
      - { key: saving_throws, type: list, of: ability }
      - { key: spellcasting_ability, type: ability }
      - { key: caster_progression, type: string }
      - { key: proficiencies, type: object }
    choices:
      - { key: skills, label: Skills, from: proficiencies.skills }
    index:
      - { key: features, label: Features, from: classFeature }
```

**The pack holds mechanics; the vault holds content.** The SRD is 7,012
entities — that is a vault template, not a pack. The pack declares that a
`class` exists and what shape it has; the vault contains the barbarian.

The rule that makes the split work: **frontmatter is what machines compute
with; the body is what humans read.** A monster's stat-block prose stays in
the body; its AC, HP, ability scores and CR go into frontmatter.

Content is stored with **one core note type**, `content`, plus a universal
`content_type` field, so a pack may declare a type named `clock` without
colliding with a core type:

```yaml
type: content
content_type: monster
uid: 01JBX…
name: Goblin
system: dnd5e
data:
  ac: 15
  hp: "7 (2d6)"
  abilities: { str: 8, dex: 14, con: 10, int: 10, wis: 8, cha: 8 }
  cr: "1/4"
```

**Content types are declared by packs, not by plugins.** ADR-0019 forbids
plugins from defining note types, because a plugin is code and a new type
would need its own visibility rules and its own place in the write-back
contract. A pack is data; its content declarations are data. The line holds.

See [`srd-vault.md`](srd-vault.md) for the pipeline that produces the 5e
content vault.

## 7. The phases in detail

### Phase 1 — The Wiki *(ADR-0004: schema + one pack, end to end)*

Deliverables, in the order the milestones build them:

| | |
|---|---|
| The schema (§5) | note types, the frontmatter contract, the visibility model, write semantics |
| The **core services** | the formula engine, the dice engine, the pack loader and linter |
| The **5e pack** | mechanics and content declarations, plus the SRD content vault (ADR-0023) |
| The **plugin host** | registry, capabilities, tiers, surface hosts, the bridge (ADR-0019, ADR-0020, ADR-0027, ADR-0029) |
| The **API contract** | `/api/v1`, the compute endpoints, the WebSocket protocol ([`api.md`](api.md)) |
| The **accounts and projection** | local accounts, join codes, the trust boundary (ADR-0009) |
| The **editor plugin** | CodeMirror 6, block widgets, the golden corpus, byte-identical round-trip (ADR-0021) |
| The **browser and search plugins** | typed views, dashboards, the two search indexes |
| The **scaffolder** | `demitable init`, the templates, the demo campaign (ADR-0025) |
| The **campaign export plugin** | the second, non-UI plugin — where `api: 1` freezes (ADR-0019) |
| A **worked example campaign** | the acceptance test, exercising the projection layer |
| The **Blades pack** | the format gate, run before the schema is frozen (ADR-0012) |

Deferred to **phase 1.5**: the archive import endpoint (ADR-0025) and the
MCP plugin (ADR-0014). The reasons are in
[`phase-1-plan.md`](phase-1-plan.md) §"Phase 1.5".

Plus the **Blades pack** as the format-validation gate (ADR-0012).

The full milestone breakdown, with acceptance criteria, is in
[`phase-1-plan.md`](phase-1-plan.md).

The hard problems, in order:

- **The plugin API** (ADR-0019). The highest-risk artifact in the project,
  because everything depends on it and changing it breaks everything. The
  method that manages this risk is to *not design it up front*: write the
  editor as the first plugin against a provisional API, then a non-UI plugin
  against the result, and freeze it at `api: 1` only then.
- **The editor plugin** (ADR-0021). CodeMirror, the Obsidian syntax
  extensions, the block-widget model, the raw-block escape hatch, and the
  golden corpus that makes byte-identical round-trip an automatable gate
  rather than a hope.
- **Writing to the vault without losing the user's work** (§5.5). Everything
  downstream depends on getting this right.
- **The formula engine.** Small language, safe evaluator, dependency-ordered
  resolution, cycle detection, and a validator that reports pack errors
  before the pack is ever used.
- **The vault index.** Parse frontmatter, build uid/alias/link indexes, keep
  them fresh via the watcher. This is also what the projection layer queries.
- **Visibility filtering.** Correctness here is a security property: a leak of
  a GM-only note is a failure of the product, not a bug.
- **The write journal** (ADR-0015), which must cover every write path the app
  has — including the ones added later.

Out of scope for phase 1: dice rolling UI, encounters, the board, audio,
player-facing character creation.

### Phase 2 — Campaign Manager

**Character creation wizard, used by the GM and by players.** Driven entirely
by the pack: steps, choices and point-allocation rules come from the pack's
YAML. The wizard *writes a character note*. It does not live in a database.
Output is frontmatter a sheet renders, plus a human-readable sheet section in
the note body.

Two paths through the same wizard (ADR-0009):

- **GM** — creates a character and approves it in one step. Also creates NPCs,
  which use a lighter path: a statline plus a couple of traits, because the
  NPC side is where a GM actually feels the volume.
- **Player** — drafts a character in `draft`, visible only to them and the GM,
  then submits it. The GM reviews and approves; on approval it becomes
  `active` and visible to the party per its own `visibility`. A rejected
  character returns to `draft` with feedback.

**Character sheets.** Two representations on purpose:
- `data:` frontmatter — structured, queryable, machine-written.
- A rendered sheet in the note body — human-editable, holds freeform notes,
  survives without the app.

The app provides a live editor view; the note body is the durable fallback.

**Encounter planner.** Pick a location → pull candidate NPCs and factions from
the vault → build the group → budget it against the pack's difficulty model →
produce an `encounter` note with initiative order, tactics, and map reference.

**Encounter runner.** Live initiative tracker, HP and conditions, a shared
**dice log** using the pack's workflows (§6.2), and on close, write-back to
the participants' character notes via §5.5.

The hard part of phase 2 is the **difficulty model**, which is entirely
system-specific — and it is exactly what the pack's `difficulty` block exists
to solve. The second hard part is the dice engine, which must serve
d20-family, dice-pool and roll-under systems from one implementation.

### Phase 3 — The Table (2D VTT)

A live board in the player web client, plus an encounter plan in the vault.
The two are the same data: the encounter note is the plan; the server holds the
live state.

**MVP, and no more** (ADR-0016):
grid and hex snap · image map background · tokens with portrait, HP and
conditions · fog of war (reveal + GM-only layer) · initiative sidebar · dice log
· measurement · player view with pan and zoom.

Deliberately excluded: **map editing** (walls, doors, regions), dynamic
lighting, line-of-sight rules, 3D anything, in-map scripting. The GM brings a
map image; map authoring is a second application layered on the first, and
Foundry's scene editor is a project of its own. "2D" is the constraint that
makes this tractable.

Maps live as attachments in the vault (`Assets/Maps/`) referenced from the
encounter note, so a campaign with 200 maps still syncs as files.

The player client is a **PWA** — players open a URL on their phone, join with
a session token, see the board. Nobody installs anything. This is the single
biggest adoption lever in the project.

### Phase 4 — The Session

**Recording.** Desktop capture (system audio + mic) → a timestamped transcript.

**Transcription is pluggable, with a local engine as the default**
(ADR-0018): `whisper.cpp` spawned as a sidecar and driven over JSON on stdio,
with `faster-whisper` and cloud providers as configurable alternatives. Local
by default, and it must be: a session contains unrehearsed player speech and
the GM's unreleased secrets. Player consent is a design requirement, not a
checkbox in settings.

**Live mode is a stretch goal, not the baseline.** On real game audio —
overlapping speech, dice, table talk, accents — a local model is not
dependable enough to caption live. The baseline is record, transcribe after,
review, log.

**Speaker attribution.** Manual tagging in a review UI (click a name on a
segment). Manual is more accurate than automatic diarization and always
available. Auto-diarization later, optional.

**Session log.** Transcript → structured summary (scenes, NPCs introduced,
decisions, loot, loose threads) → written into the session note via the marked
section from §5.5. An LLM does this, using whatever model the user brings
(local Ollama or a hosted key). This is where AI earns its place: not
generating content, but *filing* it into a schema.

**Music from mood.** The mood signal comes from transcript pace, volume and
sentiment; explicit GM cue markers; and optionally a tension state the
encounter runner tracks. It maps onto a **music library** of local files with a
metadata sidecar: mood tags, intensity, duration, loop points. Auto-queue with
crossfade.

**Assistive by default.** The system suggests a cue; the GM taps to accept.
Full auto is a setting people will want and will immediately complain about. A
table that fades to the wrong track mid-dialogue is worse than no music.

### Between sessions

Players are not locked out of the app between sessions (ADR-0017). A player can
log in and see their character, fully editable by them, plus a read-only view
of the campaign filtered by the same projection rules used during a session —
the locations they have visited, the NPCs they have met, the quests they are
party to, and anything marked `visibility: party`. Nothing else.

This makes the projection layer do real work continuously rather than only
under pressure at game time, so its bugs surface early. It also gives the
session write-back a second consumer, which keeps that contract honest.

## 8. Risks

| Risk | Severity | Mitigation |
|---|---|---|
| Scope explosion — phase 3 alone is a multi-month product | High | Strict MVP list; each phase ships something usable |
| **The plugin API is wrong and everything depends on it** | High | Not designed up front: two real plugins shape it before it is frozen; `api: 1` with a deprecation window for breaking changes |
| "System-agnostic" fails on weird systems (Fate, Blades, GURPS) | High | Schema invariants (§6.1) enforced by a linter; a deliberately different second pack is required before phase 2 is validated |
| Concurrent edits between the app and Obsidian | High | Per-key writes, marked body sections, optimistic concurrency on save, watcher-based invalidation, no locks (§5.5) |
| Visibility leak — a GM secret reaches a player | High | Projection layer is the single exit path; treat as a security boundary and test it as one |
| **A plugin leaks GM-only data** through its own scope | High | **Accepted, not mitigated.** A plugin is trusted at the privilege level of whoever opened it (ADR-0019, ADR-0027). A sandboxed plugin's *logic* is capability-scoped and role-scoped; its *UI* is not sandboxed, and a compiled-in plugin is the program. The import warning says so plainly |
| **Two expression languages** (formulas + dice) is more surface than one | High | Shared lexer foundations, one safety model, both sandboxed; packs that need neither simply omit the block |
| Player-facing wizard pulls phase-2 auth and review work forward | High | Accounts and ownership are now phase-1 scope (ADR-0009); the review workflow is app-level, not pack-level, so it does not complicate the pack format |
| **WYSIWYG markdown mangles files** | High | Byte-identical golden corpus as a hard gate; raw-node escape hatch; re-parse verification on write (ADR-0021) |
| Building a markdown editor is a big, dull chunk of phase 1 | Medium | CodeMirror rather than a hand-rolled editor; the corpus test keeps the long tail honest; Obsidian remains a working fallback |
| Frontmatter round-trip corrupts user files | Medium | Parse to `yaml.Node`, patch surgically, never re-serialise the whole block |
| Roll-under and dice-pool systems fall outside the dice engine's first cut | Medium | Pack declares workflows; if a system cannot be expressed, the format gains a workflow kind rather than the engine gaining a special case |
| STT quality on real game audio (overlap, accents, dice clatter) | Medium | Assistive mode; local models; player-side recording as a backup source |
| Music licensing | Medium | Local files only; the user supplies their library; no streaming service |
| Plaintext transport on a shared network | Medium | LAN-first with a stated limitation; VPN guidance for remote play; session join codes |
| **The server is internet-exposed**, so the import endpoint is an SSRF primitive | **High** | Private ranges refused after DNS resolution, rebinding prevented by connecting to the resolved address, https-only, size/time/redirect caps, and the import off by default (ADR-0031) |
| **A compromised host means the whole vault** | High | Not mitigable in software. The vault is plain files, so the backup is `git` or a file copy and is independent of the app. The app cannot lose the user's data because it does not own it |
| The GM becomes a sysadmin | Medium | `--expose` makes the safe configuration the easy one; deployment recipes for Caddy, Tailscale and Cloudflare Tunnel |
| Sessions accumulate on a 24/7 server | Low | The session store (ADR-0030) needs retention and cleanup |
| No automatic updates | Low | A deliberate trade: an update must not change a vault format under a running campaign. The app may *notice* a release |
| Pack ecosystem may not form with no registry | Low | Accepted trade for zero supply-chain surface (ADR-0011); discovery via the existing file-sharing channels |
| Journal storage growth inside the vault | Low | Blobs deduplicate by content hash; 90-day retention with reference-aware GC |
| MCP endpoint widens the read surface | Medium | Off by default, token-gated, and the privacy trade-off is stated at the point of enabling it (ADR-0014) |
| Two search indexes drift out of sync with the vault | Medium | Both are updated by the same watcher event; a mismatch test asserts they agree on note existence |
| Shipping whisper.cpp is a distribution problem | Medium | It is a per-platform native build; downloaded or placed on first run, and phase 4 degrades gracefully without it |
| Local transcription quality on game audio | Medium | Assistive mode; a campaign-name lexicon for the recogniser; the baseline is post-session, not live |
| The tabletop is visibly less capable than Foundry | Medium | Stated plainly in positioning; everything excluded is additive later (ADR-0016) |
| The between-sessions view depends on session write-back quality | Medium | "Visited" and "met" must be real recorded concepts, not guesses (ADR-0017) |
| Plugin UI in iframes is awkward for the editor | **Resolved** | The editor is a compiled-in plugin mounted **in-document** (ADR-0021, ADR-0027), so it never pays the iframe cost. The iframe model stands for everything else |
| Local server onboarding friction | Medium | One static binary, one flag; consider a Tauri wrapper later for a native window |

## 9. Open questions

**Closed.** The four pre-phase-1 decisions and the six blocking decisions
are all settled:

| | |
|---|---|
| A campaign is a first-class note | ADR-0037 |
| Two credentials in disjoint code spaces; principals, not roles | ADR-0038 |
| The write-back is a partial apply with per-key conflict reporting | ADR-0039 |
| The frontmatter fields are bare, with a tightened reserved set | ADR-0040 |
| One server serves one vault; the vault holds many campaigns | ADR-0041 |
| The formula language's layered scope | ADR-0042 |
| Three markdown readers, and a parser-free write path | ADR-0043 |
| Pools and tracks, flags and scales, in the core schema | ADR-0044 |
| The plugin ABI, the capability mapping, the `api: 1` freeze | ADR-0045 |
| The shadow host contract and the config UI | ADR-0046 |
| One query language for `list_by`, shared by REST and MCP | ADR-0047 |

**Still open, and they are product decisions rather than technical ones.**
Each is cheaper as a schema decision now than as a feature later:

1. **The scratch log** — an in-session capture tier, so the GM can catch
   "the Duke's steward was named Harlan" at minute 40 rather than at the end.
2. **Chat** — a table channel, GM→player, player→GM, and an OOC marker.
3. **The shared display mode** — one screen for the party, with the GM
   pointing it at things. This changes the projection's *shape*, so it is a
   phase-1 decision whether or not the feature is phase 3.
4. **The personal player journal** — the player's own prose, which players
   ask for by name.
5. **The campaign lifecycle** — handover to a new GM, retirement, archival.
6. **Whether the SRD vault stays a development artefact** or is published as
   an importable archive for users. The licence permits either.

## 10. Change log

- **2026-10-21** — Initial draft: thesis, phases, layers, risks.
- **2026-10-21** — Rewritten after ADR-0001…0004: dropped the plugin architecture
  for a local web app; added the server trust boundary and projection layer;
  added the visibility model, write semantics and pack format as first-class
  schema concerns.
- **2026-10-21** — After ADR-0005…0007: Go + Svelte single binary; the app
  owns prose editing, making Obsidian one client rather than the host; first
  pack is D&D 5e SRD 5.2, with schema invariants and a linter to stop the
  schema inheriting 5e's shape.
- **2026-10-21** — After ADR-0008…0011: dice are pack-defined and rolled by
  the app; player accounts with a character review workflow, which pulls
  identity work into phase 1; LAN-first networking with no TLS in v1; packs
  are local files only, with no registry.
- **2026-10-21** — After ADR-0012…0015: Blades in the Dark as the format
  proof; dual literal-and-ranked search; an opt-in, token-gated MCP server in
  phase 1; and a write journal with revert for every app-initiated write.
- **2026-10-21** — After ADR-0016…0018: the phase 3 MVP excludes map editing;
  transcription is pluggable with a local default and live captioning is a
  stretch goal; players get read-only campaign access between sessions.
  Project named **Demitable**; identifiers renamed from `sp-ng` to
  `demitable`.
- **2026-10-21** — After ADR-0019…0022: the app is split into a **minimal
  core with every feature as a plugin**; three plugin tiers (compiled-in Go,
  WASM on wazero, vault CSS); the editor becomes a **CodeMirror** plugin held to
  a byte-identical round-trip contract with a golden corpus; and the dice
  engine gains **symbol dice** via tables of faces, where tables are ordinary
  notes.
- **2026-10-21** — Added [`plugin-api.md`](plugin-api.md) (a deliberately
  provisional API surface so M5 has something to build against) and
  [`session-writeback.md`](session-writeback.md) (the write-back contract,
  now that it has two consumers). The visibility model was formalised: grants
  are additive, `gm` is an absolute deny.
- **2026-10-22** — After ADR-0023…0025: the pack holds mechanics and the
  shape of content, while the content itself is vault notes; the SRD source
  corpus is transient and its output a development artefact that never ships;
  and **content is distributed by importing an archive from a URL**, with
  `.demitable/` never importable, plugin bundles installed only behind a
  warning, and the formula sandbox tightened to paranoid because imported
  packs are untrusted input.
- **2026-10-22** — After ADR-0026 and ADR-0027: **attachments are served by
  the core and filtered by the same projection as the notes that reference
  them**, 404 rather than 403 for what a player may not see; and a plugin may
  contribute **in-document web components** under an explicit `dom.inline`
  capability, which runs with the app's own privileges. The SRD content is
  now **published as an importable archive** rather than bundled.
- **2026-10-22** — Round 2 and 3 decisions: **one session note per
  session**, always; **the world is the GM's**, so the review workflow covers
  characters only; class features stay as sections in their class note, and
  spell lists are **derived at read time**; a **demo campaign** sits beside
  the SRD content to exercise the projection layer. The **editor is a
  compiled-in plugin mounted in-document**; the `when` condition reuses the
  formula language; the **WASM ABI is Extism's shape** implemented on wazero
  rather than via the Extism SDK; and the **core renders plugin config**.
- **2026-10-22** — After **ADR-0029**: plugin UI is isolated by a **shadow
  root**, not a sandboxed iframe. A frame was never a security boundary — it
  isolated styles and layout while breaking keyboard, IME, clipboard and
  drag-and-drop across the boundary. The privilege model is now stated
  correctly: all plugin UI has the app's privileges; a plugin's *logic* is
  sandboxed, its *UI* is trusted. **Verified empirically** in Chromium — see
  [`shadow-dom-verification.md`](shadow-dom-verification.md) — including
  native drag-and-drop across the boundary in both directions. The full stack
  is specified in [`tech-stack.md`](tech-stack.md), and TypeScript is pinned
  to 5.9.x because `typescript@7` alone breaks `svelte-check`.
- **2026-10-22** — After **ADR-0031**: the server may be **internet-exposed**,
  behind TLS termination the GM provides, and may run 24/7. The architecture
  does not change — the projection still holds and the vault is still local
  files — but the attack surface does, so `--expose` tightens the defaults
  and the **import endpoint must defend against SSRF**. Two new rules: the
  server must not require a browser, and local use stays the default.
- **2026-10-23** — The four pre-phase-1 decisions. **A campaign is a
  first-class note** (ADR-0037). **Two credentials in disjoint code spaces,
  and principals instead of roles** (ADR-0038) — a campaign code and a
  session code, accepted by different endpoints, and the role enum replaced
  by a principal so a guest seat can hold a character. **The write-back is
  a partial apply with per-key conflict reporting** (ADR-0039), with
  journal-based recovery and a preview that runs the identical resolution
  as the apply. **The frontmatter fields are bare, with a tightened
  reserved set** (ADR-0040) — `owner` becomes `player` and the generic
  `status` is gone, so a GM's own `status: alive` is left alone.
- **2026-10-23** — **One server serves one vault, and that vault holds many
  campaigns** (ADR-0041). The vault is the unit of everything — backup, sync,
  sharing, and the trust boundary — and the accounts file lives inside it, so
  it travels with the campaign. The URL carries the campaign, not the vault.
- **2026-10-23** — The plan was **rewritten**: thirteen milestones became
  thirty-seven named sub-milestones, every "Done when" is a checkable
  predicate with a fixture or a threshold, the sequencing diagram was
  corrected, and **the timeline is now stated honestly** — ~70–90 weeks
  full-time, or ~3.5–4.5 years part-time, for phase 1 alone. The phase
  boundary was fixed across three documents that disagreed about it
  (ADR-0004, ADR-0008, and the plan), and the six blocking decisions were
  settled as ADR-0042…ADR-0047.
