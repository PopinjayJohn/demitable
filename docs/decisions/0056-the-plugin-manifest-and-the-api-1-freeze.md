# ADR-0056: The plugin manifest, the capability kinds, and the `api: 1` freeze

- **Status:** Accepted
- **Date:** 2026-10-23
- **Amends:** ADR-0045 (the plugin ABI, the capability mapping, and the
  `api: 1` freeze), ADR-0046 (the surface contract)
- **Extends:** ADR-0019 (core and plugins), ADR-0020 (plugin tiers),
  ADR-0027 (light DOM), ADR-0029 (shadow DOM), ADR-0050 (the schema gaps)
- **Addresses:** the three open items ADR-0045 left open — the capability
  to host-function mapping, how `api: 1` is frozen, and the deprecation
  mechanism — plus the `abi:`/`api:` name split and the freeze milestone

> **Origin:** phase 0, question 5 — *what does the manifest actually need
> to say, and does the bridge carry what a plugin needs?* The evidence is
> two real plugins written against the manifest, and
> [`spikes/plugin-host/`](../spikes/plugin-host/README.md).

## Decision

### 1. Capabilities come in three kinds

| Kind | Who calls it | Host function |
|---|---|---|
| `invocable` | the guest | yes, exactly one, named in the table |
| `subscription` | the guest opens a stream | no — browser bridge only |
| `declaration` | nobody — the host reads it from the manifest | no |

A declaration capability (`dom.inline`, `vault.css`) changes how the host
treats the plugin. It is read from the manifest and never called, so it is
not a bridge operation and cannot be refused at call time.

### 2. The capability vocabulary (confirmed)

Nine capabilities, six host functions. Only five capabilities and three host
functions were exercised by the two plugins; the freeze covers the exercised
set and the rest arrive additively.

| Capability | Kind | Host function | Exercised |
|---|---|---|---|
| `vault.read` | invocable | `host_vault_read` | yes (declared) |
| `vault.query` | invocable | `host_vault_query` | yes |
| `log` | invocable | `host_log` | yes (export) |
| `vault.watch` | subscription | — | yes (recap) |
| `dom.inline` | declaration | — | yes (recap) |
| `vault.write` | invocable | `host_vault_write` | no |
| `vault.assets` | invocable | `host_vault_assets` | no |
| `http.client` | invocable | `host_http_fetch` | no |
| `vault.css` | declaration | — | no |

`vault.css` is redundant with the manifest's `style:` field and should be
dropped or folded into it. `http.client` keeps its existing host function
name `host_http_fetch` (a v1 fact; renaming would be a breaking change).

### 3. The host primitives are part of the freeze

ADR-0045 freezes the capability vocabulary but is silent about the bridge
operations that are **not** capabilities. The spike found four, all
load-bearing, and a plugin with zero capabilities still receives all four:

```
context.get   pref.get   pref.set   command.register
```

They grant no reach into anything the calling role cannot already see, which
is why they are not capabilities. They are nonetheless part of `api: 1` and
must be frozen with it, together with:

- `emit` (plugin-namespaced event publication),
- `ready()` and its payload (`id`, `name`, `version`, `api`, `principal`,
  `role`, `config`, `capabilities`, `surfaces`),
- the event channel scheme (`core`, `plugin:<id>`) and the core event names
  (`context.changed`, `vault.changed`, `note.saved`, `command.invoked`),
- the error code set in `docs/api.md` §7.

### 4. `api: 1` is frozen at M8, with additive-only changes

`api: 1` is frozen when the export plugin lands — **M8**, not M11. The
export plugin is the second, non-UI consumer of the API, which is the
condition ADR-0019 sets for a freeze.

Within `api: 1`, changes are **additive only**:

- new capabilities, with their host functions;
- new optional manifest fields;
- new event types in the bridge and the WebSocket protocol;
- new host primitives.

Never a changed signature, a repurposed field, or a changed meaning. A host
function that changes shape gets a new name (`host_vault_read_v2`) and the
old one stays. A breaking change means `api: 2`, served side by side with
`api: 1`, dispatching on the guest's declared major.

### 5. The manifest field is `api`, not `abi`

The manifest declares `api: 1`. The host checks it **before loading the
module**, so a mismatched plugin fails with a legible error rather than a
link error. `abi:` is not a synonym and should not be accepted; the
phase-1 plan's `abi:` references are updated to `api:`.

### 6. The bridge hands out a projected view, never a raw note

A plugin never receives `vault.Note`. It receives `vault.NoteView`, built by
`vault.Project(principal, note)`, which keeps the fields that inherit the
note's visibility (`campaign`, `visibility`, `workflow`, and the GM-owned
fields) and strips the owner-scoped ones (`player`, `grants`,
`discovered`) unless the principal is the GM or the note's player
(ADR-0050 D5). The `NoteView` shape is part of `api: 1` and is frozen with
it.

### 7. The surface contract (ADR-0046, unchanged in shape)

- The app creates the host element and its shadow root; the plugin mounts
  into it and never attaches its own root.
- `entry:` names the module the surface loads; a shadow surface exports
  `mount(host, ctx, demitable)`, a light surface registers a custom
  element and exports the same `mount`.
- Shadow is the default; light DOM is opt-in via `dom.inline`; frame is
  opt-in for external or untrusted content.
- The core renders the declarative `config` block; a plugin's own config
  surface takes precedence.

### 8. The deprecation window

At least one major release of the core. A plugin built for `api: 1` keeps
working for that window. The **golden ABI test** — a module importing every
host function the host offers, built at compile time — makes the freeze
real: if a signature changes, the module fails to instantiate and CI
catches it.

## Consequences

**Good:**

- **One vocabulary** for capabilities, host functions and manifest fields,
  now honestly split into three kinds instead of pretending they are all
  calls.
- **The freeze has a date (M8), a mechanism (the `api:` check and the
  golden test), and a scope written from what two real plugins needed**, not
  from ADR-0045's table.
- **The projection leak is closed at the bridge.** A plugin cannot receive
  `grants` or `discovered` for a note its principal may not see, because
  the bridge does not serialise them.
- **Host primitives stop being an undocumented second API.** They are
  frozen with everything else.

**Bad:**

- **Freezing fewer capabilities than ADR-0045 listed** means `vault.write`,
  `vault.assets` and `http.client` are not part of `api: 1` yet; a plugin
  that needs them waits for an additive release. That is the honest price of
  not freezing what nobody has used.
- **Serving two majors is work** that nobody will exercise until it is
  needed, and untested shims rot. The golden test covers signatures only,
  not behaviour.
- **The `abi:`/`api:` rename touches the phase-1 plan and every manifest
  example.** Cheap now, embarrassing later.

**Neutral:**

- The shape stays Extism's, so an Extism-PDK plugin's shape is familiar
  even though our host functions and manifest differ.
- The freeze is additive-only, so the manifest does not change shape when
  WASM host functions arrive in phase 2.

## What this decision does not settle

- The compiled-in plugin registration contract (how `demitable.export` is
  loaded and ordered). M3d.
- The `discovered` visibility value and the reserved layer's per-field
  visibility table in the projection. M5 (ADR-0050).
- Cross-browser shadow behaviour (Firefox, Safari). The `getSelection()`
  retargeting rule depends on it.
- Whether `vault.css` survives as a capability or folds into `style:`.
- The markdown sanitisation policy and the search-ranking remedy. Phase 1.
