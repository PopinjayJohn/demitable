# The Plugin API — provisional

> **Provisional.** ADR-0019 says the plugin API is *not* designed up front:
> it grows to fit the editor (first plugin) and is then proven by a
> non-UI plugin (MCP). This document exists so milestone M5 can be built
> against something, and so the ambiguities are written down rather than
> discovered. It is explicitly **not** frozen. `api: 1` is not declared here.

---

## 1. The shape of it

A plugin is a bundle with up to four parts:

| Part | Where it runs | Notes |
|---|---|---|
| Server logic | The core process | Compiled-in Go, or sandboxed WASM |
| UI | The browser, in a shadow root — or in light DOM under `dom.inline` | Optional |
| Assets | Served by the core | Optional |
| CSS | Served by the core | Also available standalone as a vault CSS plugin |

A plugin with UI and no server logic is valid and common. A plugin with
server logic and no UI is valid too — the MCP plugin is exactly that.

### The bundle

```
demitable.editor/
  plugin.yaml        manifest
  logic.wasm         optional, sandboxed server logic
  ui/index.html      optional, iframe entry point
  ui/app.js
  ui/style.css
  assets/…
```

A directory or a zip. Nothing is fetched over the network (ADR-0011).

---

## 2. The manifest

```yaml
id: demitable.editor          # unique, reverse-DNS-ish
name: Editor
version: 1.0.0                # the plugin's own semver
api: 1                        # the plugin API major it targets
description: Structured markdown editing for campaign notes.
author: …
homepage: …

capabilities:
  - id: vault.read
  - id: vault.write
    scope:
      paths: ["Notes/Characters/*"]
      types: [character]

provides:
  - id: editor
    kind: service
    version: 1.0.0
  - id: editor.surface
    kind: ui
    title: Editor
    entry: ui/index.html
    mount: note.body
    icon: pencil
    order: 10
    when: "always"            # or a condition expression

consumes:
  - service: dice
    version: ">=1.0.0 <2.0.0"
    optional: false

assets:
  - { path: ui/style.css, role: style }
  - { path: assets/portal.svg, role: asset }

config:
  - { key: autosave, type: boolean, default: true }
```

**`when` conditions are formula-language expressions.** A mount condition is
an expression in the pack's formula language (ADR-0003), evaluated against a
small context of the current note — `note.type`, `note.data.kind`,
`note.system`, `note.visibility`. There is no second expression language:

```yaml
provides:
  - id: editor.surface
    kind: ui
    mount: note.body
    when: "note.type == 'character' && note.data.kind != 'npc'"
```

**`config` is rendered by the core.** The core draws a settings form from the
declared fields — text, number, boolean, select, path — so every plugin gets
a settings UI for free, including one with no UI of its own. A plugin needing
something richer registers its own config surface, which takes precedence.

### Capability scopes

A capability may be narrowed. This exists so a plugin does not need
`vault.write` when it only wants to write its own notes:

| Scope key | Meaning |
|---|---|
| `paths` | Glob over vault paths |
| `types` | Allowed note `type:` values |
| `uids` | An explicit allowlist of note uids |

A scope of `types: [character]` still permits writing *any* character note, so
scopes compose: the effective permission is the intersection of every declared
scope.

---

## 3. Capabilities

The complete list. Anything not on it does not exist.

### Vault data

| Capability | Grants |
|---|---|
| `vault.read` | Read a note by uid, path or link |
| `vault.write` | Create, update and delete notes |
| `vault.query` | Typed queries over the index — `list_by(type, filter)` |
| `vault.watch` | Subscribe to vault change events |
| `vault.assets` | Read binary assets — maps, portraits, table faces |
| `vault.journal` | Read the write journal and request a revert |
### Live session

| Capability | Grants |
|---|---|
| `session.read` | Read live session state |
| `session.write` | Publish changes to live session state |
| `session.roll` | Perform rolls through the core dice engine, so they land in the shared log |

### Identity

| Capability | Grants |
|---|---|
| `identity.self` | Read the caller's own player id and name — always granted |
| `identity.others` | See other participants' identities in the session |

### Outbound

| Capability | Grants | Default |
|---|---|---|
| `http.client` | Outbound HTTP | **off** |
| `process.spawn` | Spawn a helper process — phase 5's transcriber | **off** |
| `vault.css` | Contribute stylesheets | off |

### In-document

| Capability | Grants | Default |
|---|---|---|
| `dom.inline` | Mount into the app's own DOM rather than into a shadow root | **off** |

`dom.inline` controls **style and layout participation**, not privilege. A
shadow surface already runs in the app's document, so both kinds have the
app's privileges (ADR-0029). What differs is that a shadow surface's styles
are contained and its placement is scoped, while a light-DOM surface
participates in the app's own styles and can be placed anywhere the app
allows.

Compiled-in plugins have it implicitly. A WASM plugin must declare it, and
the import warning calls it out separately (ADR-0025).

Everything else in this list describes *data* reach. `dom.inline` describes
*placement*.

### Not extensible by plugins

**Note types are not extensible.** A plugin cannot define a new note type with
new fields.

The reason is that note types are load-bearing for the schema, the packs and
the projection layer. A plugin-defined type would need its own property
vocabulary, its own visibility rules and its own place in the write-back
contract. Allowing it means the projection layer can no longer reason about
what it is filtering.

Plugins work with the types the core defines, and with the system-specific
fields the packs define. If a genuinely new kind of object is needed, that is
a core change, made deliberately.

---

## 4. Services

The core resolves a service by id and version range. These are the core
services:

| Service | Provides |
|---|---|
| `vault` | The index: query, read, write, watch — always scoped |
| `packs` | The pack registry, entity parsing, formula evaluation |
| `formula` | Evaluate an expression in an entity's context |
| `dice` | The dice engine: rolls, tables, pools, opposed rolls, the log |
| `projection` | The GM's full view. **Players never receive this** |
| `session` | Live session state |
| `journal` | The write journal |
| `accounts` | Account and join-code administration. GM only |
| `assets` | Binary asset serving, scoped by role |

A plugin provides a service the same way, and other plugins consume it by id
and version range. The `dice` service in the manifest example above is the
core's; a plugin may provide its own.

---

## 5. Mount points

| Mount | Where |
|---|---|
| `note.body` | In the note's body area, below the title |
| `note.header` | Actions beside the note title |
| `note.sidebar` | The note's side panel |
| `sidebar.left` | Application-level left sidebar |
| `sidebar.right` | Application-level right sidebar |
| `panel.bottom` | The bottom drawer — dice log, session log |
| `dialog` | Modal surfaces |
| `command` | Command palette entries only, with no surface |

A mount point may host several plugins, ordered by the manifest's `order`
field, each collapsible. A plugin's `when` condition decides whether it
appears at all — `always`, or an expression over the current note.

### Surface hosts (ADR-0029)

A plugin surface is mounted into a **named host element** the app provides.
The app decides where hosts exist — a note's sidebar, a view's footer, a
table panel — and the plugin declares which hosts it wants.

By default the host carries a **shadow root**, and the plugin's UI is mounted
inside it. Styles are contained; the app's element styles do not reach in;
but **inherited properties and custom properties do**, so a theme expressed
as custom properties on `:root` reaches every surface automatically.

A plugin with `dom.inline` mounts into the same host in **light DOM**,
participating in the app's own styles.

The host is the app's, so an accidental surface does not end up covering the
app. Note the distinction from the editor plugin's extension registry
(ADR-0021): that is the sanctioned path for *inline content inside a note*,
such as a spell-slot block. `dom.inline` is for components in the app's
chrome, not for note content.

---

## 6. The browser bridge

Plugin UI runs in the app's document — in a **shadow root** by default
(ADR-0029) — and talks to the core over a versioned, role-scoped
`postMessage` API.

```js
const ctx = await demitable.ready()   // { id, role, config, capabilities }
const result = await demitable.call('vault.read', { uid })
const off = demitable.on('vault.changed', (e) => { … })
demitable.emit('note.opened', { uid })
```

- The bridge only exposes methods the plugin's granted capabilities allow. A
  player-scoped instance of a plugin with `vault.write` still cannot write,
  because the grant is per role.
- `demitable.ready()` resolves after the core has applied the plugin's config
  and role.
- Messages are namespaced by plugin id, so one plugin cannot forge another's.
- The bridge is the **sanctioned** data path. It is not the only one: plugin
  UI shares the app's document and can call the API directly. The
  role-scoped projection still applies — a player-scoped instance cannot
  reach GM-only notes — but capability scoping does not. **A plugin is
  trusted at the privilege level of whoever opened it** (ADR-0019,
  ADR-0027).

### Events

`vault.changed` · `session.state` · `note.opened` · `note.saved` ·
`command.invoked` · `roll.result`

---

## 7. The WASM ABI

Extism-shaped: one exported entry point taking and returning JSON.

```wat
(func (export "demitable_call") (param i32 i32) (result i32))
```

The pointer/length pair refers to a JSON request in the module's memory. The
host writes a JSON response back and returns a pointer to it.

**The shape matches Extism's, and we implement it ourselves on wazero.** We
do not take the Extism SDK as a dependency — it would bring its own manifest
and plugin model alongside ours, and we would have two competing descriptions
of what a plugin is. But the ABI *is* Extism's, so a plugin written against
the Extism PDK would work here unchanged, and the shape is battle-tested
rather than ours.

Keeping it pure Go also preserves the no-cgo, single-binary property
(ADR-0005).

### Host functions

| Function | Purpose |
|---|---|
| `host_log(ptr, len)` | Write to the app log |
| `host_vault_read(ptr, len)` | Read a note — returns JSON |
| `host_vault_write(ptr, len)` | Write a note — returns JSON |
| `host_vault_query(ptr, len)` | Query the index — returns JSON |
| `host_http_fetch(ptr, len)` | Outbound HTTP — **only if granted** |
| `host_now()` | Milliseconds since the epoch |
| `host_random(ptr, len)` | Cryptographically random bytes |

**A call to an undeclared capability returns an error JSON, not a crash.**
The host checks the grant before dispatching.

### What a WASM plugin does not get

- **No filesystem access to the vault.** WASI filesystem preopens are not
  offered. The vault is reached only through the host functions, which is
  what lets the core enforce role-scoped visibility.
- **No network, by default.** WASI preview 1 has no sockets, and wazero does
  not implement them.
- **No clock, no randomness beyond the host functions** above.

---

## 8. Lifecycle

```
discover → parse manifest → check api version → check capabilities
        → grant (per role) → instantiate → demitable_init()
        → register provides → ready
```

### Role-scoped instances

**A plugin is instantiated per (role, session), not per process.**

A player-scoped instance must never share state with the GM-scoped instance
of the same plugin. If they did, a player could learn GM state through a
plugin bug — and the plugin would become a side channel around the
projection layer.

So the host keeps a separate instance per role, with separate WASM memory and
separate server-side state. The bridge is bound to the instance's role.

### Disabling

A plugin can be disabled without restarting the app. Its surfaces unmount, its
instances are shut down (`demitable_shutdown`), and its assets stop being
served. Its writes remain in the journal and remain revertible.

---

## 9. Compiled-in plugins

Go source, linked at build time, registered in `init()`.

```go
func init() {
    plugin.Register(plugin.MustManifest(manifest), func(ctx plugin.Context) plugin.Instance {
        return &editor{ctx: ctx}
    })
}
```

A generated `plugins/registry_gen.go` imports every first-party plugin
package, so adding a plugin is adding a package and re-running the generator.

Compiled-in plugins are trusted: they are part of the program and have the
same access as the core. This is why they are the right home for first-party
features and the wrong home for anything third-party.

---

## 10. Vault CSS plugins

The lightest tier. A folder in the vault, no logic, no build step.

```
Plugins/my-theme/
  plugin.yaml      optional
  style.css
```

- The shell serves it at `/plugin-assets/<id>/style.css`.
- Every selector is **scoped** under `[data-plugin="<id>"]`, so a theme
  cannot restyle the shell by accident.
- Stylesheets are ordered by the manifest, then by plugin id.
- A CSP restricts them to same-origin assets.
- A CSS plugin cannot read or write anything. It is inert.

---

## 11. Versioning

| Thing | Rule |
|---|---|
| `api` | The plugin API major. The core supports the current major and the previous one. A breaking change means a new major, and a compatibility shim for the old one |
| Plugin `version` | The plugin's own semver, independent of `api` |
| Service versions | Consumed by semver range in `consumes` |
| Capabilities | Adding one is a minor bump; plugins must ignore capabilities they do not recognise |

A plugin must degrade gracefully when a capability it declares is not
granted, and when a service it consumes is absent.

---

## 12. Security rules

Stated as absolutes, because these are the properties the whole design rests
on:

1. **The core never hands a plugin the vault.** Only a scoped view.
2. **A plugin never gets filesystem access to the vault.**
3. **A plugin's role is fixed at instantiation and cannot change.**
4. **Role-scoped plugins get separate instances.** No shared state across
   roles.
5. **A plugin's grants are a subset of its manifest, intersected with the
   caller's role and the user's plugin permissions.**
6. **CSS is scoped.** A plugin cannot restyle the shell.
7. **Network and process spawning are off unless explicitly granted**, and
   granting them is a visible, per-plugin decision.

---

## 13. Open questions

Most of these are now settled; the remainder are for the two plugins to
answer in practice.

- **Capability naming.** *Settled:* `vault.*` for data access, flat names for
  everything else (`http.client`, `process.spawn`, `dom.inline`,
  `identity.self`). The namespace only where there is a family.
- **Service ids.** *Settled:* bare nouns — `vault`, `packs`, `formula`,
  `dice`, `session`, `journal`, `accounts`, `assets`. A plugin provides a
  service the same way.
- **Mount point granularity.** *Settled for now:* keep the list in §5, and
  rely on the editor's extension registry for inline content inside a note.
  Add mount points when a plugin demonstrably needs one.
- **The `when` expression language.** *Settled:* the formula language
  (ADR-0003), against a small note context. No second expression language.
- **Whether a plugin may contribute in-document components.** *Settled:*
  yes, under the `dom.inline` capability (ADR-0027) — and the default surface
  is a shadow root (ADR-0029).
- **Config UI.** *Settled:* the core renders the declarative block; a plugin
  may register its own surface, which takes precedence.
- **The WASM host ABI.** *Settled:* Extism's shape, implemented on wazero
  rather than via the Extism SDK.

Still genuinely open, and to be answered by the editor and the MCP plugin
rather than by argument:

- **How the editor exposes its extension registry** to other plugins — the
  exact shape of a registered node, and whether a plugin may register a
  *mark* as well as a node.
- **Whether `dom.inline` components get a first-class lifecycle** — a
  `connected`/`disconnected` hook, and whether they can be reconfigured
  without a reload.
- **Whether the bridge needs a batched call.** Every read currently costs a
  round trip; a live widget may want one call that returns a bundle.
- **The shadow host contract.** Whether the app provides the shadow root and
  the plugin mounts into it, or the plugin attaches its own. The first is
  simpler and keeps the app in control of the root; the second lets a plugin
  use `mode: 'closed'`.
