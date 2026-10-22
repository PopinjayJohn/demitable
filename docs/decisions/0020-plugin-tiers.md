# ADR-0020: Three plugin tiers — compiled-in, WASM, and vault CSS

- **Status:** Accepted, amended by ADR-0025 (a WASM plugin may arrive in an
  imported archive, with a warning; compiled-in plugins remain impossible to
  install at runtime) and superseded in part by **ADR-0029** (plugin UI is
  isolated by a **shadow root**, not a sandboxed iframe)
- **Date:** 2026-10-21

## Context

ADR-0019 made the plugin API the most important artifact in the project. It
still does not answer *what a plugin is* — code in which language, running
where, trusted how much, installable how.

Three tiers were proposed, and each answers a genuinely different need:

- The **maintainer** needs to iterate fast on first-party features and ship
  them inside the binary.
- The **community** needs to write features in their language of choice and
  have them run somewhere we can constrain.
- The **cosmetics crowd** needs to restyle the app for their campaign without
  writing or compiling anything.

## Decision

### Tier A — Compiled-in

Go source, **linked into the binary at build time**. First-party plugins live
in `plugins/<id>/` and are registered by a generated file. Fully trusted: they
are part of the program.

**Go's `plugin` package is explicitly not used.** It is Linux/macOS only, it
requires an exact toolchain and dependency match between host and plugin, it
breaks cross-compilation, and it breaks the single-binary promise (ADR-0005).
Build-time linking has none of those problems and is simpler.

### Tier B — WASM

Runtime-installable, sandboxed, **wazero** as the runtime.

**wazero specifically**, because it is pure Go with no cgo. That is what keeps
the single static binary intact and cross-compilation trivial (ADR-0005), and
it is the choice Helm made for its own plugin system.

**The host ABI is Extism-shaped**: one exported entry point taking and
returning JSON, plus a small set of host functions for capabilities.

```wat
(func (export "demitable_call") (param i32 i32) (result i32))
```

Host functions: `vault.read`, `vault.write`, `vault.query`, `http.fetch`,
`log`, `now`. Each is granted only if the plugin's manifest declares it.

**A plugin never gets raw filesystem access to the vault.** It reaches the
vault only through host functions, which is what allows the core to enforce
role-scoped visibility. WASI filesystem preopens are not offered at all.
Network access is absent by default: WASI preview 1 has no sockets, and
wazero does not implement them.

Guest SDKs: Go (TinyGo), Rust, TypeScript, AssemblyScript.

### Tier C — Vault CSS

Pure styling, from the vault. `Plugins/<id>/style.css` plus a small manifest.
No logic, no network, no build step.

The shell serves it, scopes every selector under a plugin-scoped root
attribute so a plugin cannot restyle the whole app by accident, orders it by
manifest, and applies it with a CSP limited to same-origin assets.

This tier is not a token gesture. Community "themes" for a TTRPG app are a
real and active category, and a CSS-only tier lets people participate without
compiling anything.

### UI is orthogonal to the tier

**The tier describes how a plugin's server-side logic is delivered.** Any
plugin may contribute browser UI, which runs client-side in the app's
document and communicates through a versioned, role-scoped `postMessage`
bridge.

> **Superseded in part by ADR-0029.** The UI surface is a **shadow root**, not
> a sandboxed iframe. A frame is a *document* boundary, and the only reason to
> want one is to keep untrusted content out — which a same-origin frame does
> not do, while breaking keyboard shortcuts, IME, clipboard and drag-and-drop
> across the boundary. See ADR-0029 for the full reasoning and for the
> corrected privilege statement.

So a plugin is a bundle of, optionally:

| Part | Runs in | Tier-independent |
|---|---|---|
| Server logic | Core process | Delivered as compiled-in (Go) or WASM (sandboxed) |
| UI | Browser, in a shadow root — or in light DOM under `dom.inline` | Yes |
| Assets | Served by the core | Yes |
| CSS | Served by the core | Vault CSS is the standalone form of this |

A plugin with UI and no server logic is valid and common — a dashboard widget
that reads through the bridge.

> **Amended by ADR-0027.** A plugin may *additionally* contribute
> **in-document web components** — custom elements mounted into the app's own
> DOM, outside any iframe. This is a separate, explicitly granted capability
> rather than a change to the tier model, because it changes the trust
> property rather than the delivery mechanism.

### The bundle

A directory or a zip:

```
demitable.editor/
  plugin.yaml        manifest
  logic.wasm         optional, sandboxed server logic
  ui/index.html      optional, iframe entry point
  ui/app.js
  ui/style.css
  assets/…           optional
```

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

### Trust summary

| Tier | Trusted with | Installable at runtime |
|---|---|---|
| Compiled-in | Everything — it *is* the program | No |
| WASM | Only declared capabilities, scoped by role | Yes |
| Vault CSS | Nothing; inert | Yes, by dropping a folder in |

> **Amended by ADR-0025.** A WASM plugin may arrive inside an archive imported
> from a URL. The import **warns and asks** before installing it, and a
> non-interactive import skips plugins unless explicitly told not to.
> Compiled-in plugins remain impossible to install at runtime, by
> construction — they are linked into the binary.
>
> The reason the warning is worth having: a WASM plugin is sandboxed at the
> OS level, but it reads the vault through the core's capability API, so its
> reach is exactly its declared capabilities — running as whoever opened it.
> For the GM, that means the whole vault, including GM-only notes.

## Consequences

**Good:**

- Each tier has a clear trust story, and the tiers match how people actually
  want to contribute.
- WASM plugins are portable and sandboxed, and the sandbox costs the
  single-binary promise nothing because wazero is pure Go.
- CSS-only plugins make community styling possible with zero build tooling.
- Compiled-in plugins keep first-party development fast — no sandbox tax, no
  ABI boundary, full Go.

**Bad:**

- **Two SDKs and two security models.** A bug in the WASM ABI or in the
  capability enforcement is a security bug, and it will be tested by
  strangers.
- **The iframe model constrains plugin UI.** No shared DOM, no shared CSS
  variables for free, an extra bridge hop for every interaction. Worth it for
  isolation, but it is real friction. ADR-0027 relieves some of this by
  allowing in-document components, at the cost described there.
- **Plugin UI is trusted, not sandboxed.** ADR-0029 corrects this explicitly:
  all plugin UI runs in the app's document, so it has the app's privileges.
  What we offer is style and layout isolation. The real boundaries are the
  WASM sandbox and the capability model.
- **The capability model is the hard part.** It must be small enough to be
  correct and expressive enough that plugins do not need `vault.write` when
  they only want `vault.read`.
- Compiled-in plugins cannot be distributed separately from the binary, so
  first-party features ship on the app's release cadence.

**Neutral:**

- A future **out-of-process tier** — plugins as separate executables speaking
  the same RPC, giving process isolation and any language — is compatible with
  this design, since it is the same capability protocol with a different
  transport. Deliberately out of scope for v1.
