# ADR-0019: The app is a minimal core; every feature ships as a plugin

- **Status:** Accepted, amended by ADR-0020, ADR-0027 and **ADR-0029** (the
  plugin UI isolation primitive is a shadow root, not a sandboxed iframe)
- **Date:** 2026-10-21

## Context

The plan grew. Phase 1 acquired an editor, accounts, projection, MCP, a write
journal, dual search and two system packs; phases 2–4 add a wizard, sheets,
encounters, dice, a tabletop, recording and music.

Every feature that lands in the core makes the core harder to change — and the
core is the thing everything else depends on. A core with nine subsystems
cannot be refactored without breaking every plugin.

There is also a consistency argument. The project's central promise is that
the *system* is data, not code, and that the *campaign* is files, not a
database. Extending that to the application itself — the app is a small
kernel plus swappable features — is the same idea applied one level up. A
system pack makes a new ruleset cheap. A plugin system makes a new feature
cheap. And it is the difference between a program and a platform.

## Decision

**Split the application into a minimal core and plugins. Everything the user
experiences is a plugin.**

### What must be core, and why

| Core area | Why it cannot be a plugin |
|---|---|
| Vault service — read, write, watch, index, journal | Owns the user's data. Every plugin depends on it |
| Pack format and its runtimes — the formula and dice engines | The core's own data contract. If the formula engine were a plugin, a pack would be meaningless without it |
| Accounts, roles, projection | **The trust boundary.** A plugin able to define its own visibility rules would defeat the entire design |
| Plugin host — registry, capabilities, lifecycle, WASM runtime | Self-evidently |
| HTTP and WebSocket server, asset serving | Self-evidently |
| UI shell | Something must exist for a plugin to render into |

### What becomes a plugin

The editor · the note browser and typed views · dashboards · search ·
character sheets · the creation wizard · the encounter planner and runner ·
the dice interface · the tabletop · session recording · music · the MCP
server · the player-facing client.

### The rule that makes this safe

**The core never hands a plugin the vault.** It hands the plugin a *scoped
view* — a capability object exposing exactly the operations the plugin's
manifest declares, filtered by the caller's role.

This is the same principle as the projection layer (ADR-0002), applied one
level down.

> **Amended by ADR-0027.** This rule constrains *data reach*, not *privilege*.
> A plugin's **logic** is sandboxed; a plugin's **UI** is not — it runs in the
> app's document and can read and change what the user sees, including GM-only
> notes (ADR-0029). A plugin is therefore trusted at the privilege level of
> whoever opened it.

### The trust model, stated honestly

> **A plugin is trusted at the privilege level of whoever opened it.** The
> capability model constrains *accidental* overreach and provides role-scoped
> instances. It is not a defence against a hostile plugin.

This is the position ADR-0027's own import warning already took, and it is
what Obsidian plugins and VS Code extensions take. It is defensible for a
local tool whose user is the one installing the plugin.

Concretely:

- **Compiled-in plugins are the program.** They are trusted with everything.
  Every first-party plugin is in this tier.
- **A sandboxed plugin's *logic* cannot escape its capabilities.** That is a
  real boundary, and it is what the WASM tier buys.
- **A sandboxed plugin's *UI* is not sandboxed.** It runs same-origin, so it
  can call the API directly and bypass the bridge. The role-scoped projection
  still applies — a player-scoped instance cannot reach GM-only notes — but
  the *capability* scoping does not.
- **So a GM running a plugin is running code with the GM's authority**, and
  the import warning says so.

The consequence for the phase-1 definition of done: the claim that GM-only
notes are "unreachable by plugins" is **withdrawn**. What remains true is that
they are unreachable by a *player-scoped* plugin instance, and that a
sandboxed plugin's *logic* cannot reach them.

The alternative — giving plugin UI a real origin boundary — is what ADR-0029
argued away, and it would cost back the native interaction the shadow-root
model bought.

### Method: the API is not designed up front

The plugin API is the highest-risk artifact in the project, because everything
depends on it and changing it breaks everything. Designing it on paper is how
that goes wrong.

So:

1. Write the **editor** as the first plugin, against a provisional API.
2. Let the API grow to fit it.
3. Write a second, **non-UI** plugin — the MCP server is a good candidate —
   to prove the API is not accidentally editor-shaped.
4. Only then freeze it at `api: 1`.

An API shaped by two real plugins is worth more than one shaped by imagination.

### Compatibility

`api: 1` in every manifest. The core supports the current major and the
previous one for a deprecation window. Breaking changes require a new major
and a compatibility shim, because third-party plugins will exist and cannot
be forced to update in lockstep.

## Consequences

**Good:**

- The core stays small enough to read in an afternoon and audit in a week.
- The trust boundary is one directory of code, not a scattering.
- Features version independently. The tabletop can ship weekly without
  touching the vault service.
- A plugin can be disabled without breaking the app, and the app can run with
  no plugins at all — it becomes a vault browser.
- The system-agnostic thesis extends coherently to the application layer.

**Bad:**

- **Phase 1 grows.** The plugin host is new work on top of everything already
  scoped, and the editor becomes a plugin rather than a feature.
- **The plugin API is the product's biggest risk.** It is the one artifact
  where being wrong is expensive, and it is the one hardest to change later.
- **Two SDKs.** Go for compiled-in plugins, a guest SDK for sandboxed ones.
  Two languages, two test paths, two security models to reason about.
- **"Everything is a plugin" is aspirational.** The trust boundary genuinely
  cannot be a plugin, and the line has to be drawn explicitly and defended
  against the temptation to move a feature into the core for convenience.
- **Plugin UI runs in a shadow root** (ADR-0029), which contains its styles
  while leaving the document shared. A frame would have contained more, at
  the cost of breaking keyboard, IME, clipboard and drag-and-drop across the
  boundary.
- **A plugin ecosystem is a long-term bet.** The platform is worthless without
  plugins, and plugins are worth nothing without users. First-party plugins
  must be good enough to carry the product alone.

**Neutral:**

- The phase boundaries blur. What was "phase 2" is now "the wizard plugin",
  and the plan should be read as *which plugins exist when*, not as separate
  projects.
