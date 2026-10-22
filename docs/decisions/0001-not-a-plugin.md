# ADR-0001: Not an Obsidian plugin — a local web application

- **Status:** Accepted
- **Date:** 2026-10-21

## Context

The project must become a campaign manager (phase 2), a live 2D tabletop with
multiple simultaneous players (phase 3), and a session recorder (phase 4).

All three of those need things an Obsidian plugin structurally cannot provide:

- A **persistent server** that players on other devices connect to. Plugins run
  inside one user's app; the only way to reach players is to bind a local HTTP
  server (possible on desktop via Node/Electron access, but it is a side-channel
  around the plugin model, not part of it, and unavailable on mobile).
- A **real-time board** shared by N clients with fog of war and initiative.
  The plugin API's DOM belongs to Obsidian; a table would be a guest in someone
  else's app.
- **Continuous audio capture and streaming transcription** (phase 4).

The alternative was a standalone desktop app, which would require rebuilding a
markdown editor, graph view, search, and sync — all things Obsidian already
does better than we would, for years of work.

## Decision

Build a **local web application**:

- A local server process owns the vault, the packs, and all live session state.
- The UI is a browser app. The GM and the players use the *same* app with
  different roles, distinguished by a session token.
- Players install nothing. They open a URL on their phone or laptop.
- Obsidian remains in the picture as a peer editor over the same files, reached
  by deep link (`obsidian://open?file=...`).

We do **not** depend on Obsidian's plugin API, Bases, Canvas API, or Dataview.
Where Obsidian formats are useful (frontmatter, wikilinks, markdown) we stay
compatible with them as a *file format*, not as a *runtime dependency*.

## Consequences

**Good:**

- No plugin review process, no sandbox limits, no Obsidian version churn.
- Full control over the UI: a VTT needs game-specific affordances that Bases
  and Canvas cannot provide.
- One codebase serves GM and player roles; the player client is a subset of
  the same bundle, routed by role.
- The server becomes an explicit, inspectable trust boundary (see ADR-0002).

**Bad:**

- **We now own the editor.** We must build note browsing, search, and some
  form of note editing. Mitigated by leaning on Obsidian for prose editing
  (open question #2) rather than writing a full markdown editor.
- **Concurrent-edit risk.** The GM may have a note open in Obsidian while the
  app writes to it. Obsidian can hold a stale buffer and overwrite our write,
  or show a reload prompt mid-typing. Mitigation: per-key frontmatter writes,
  explicit write actions, and a filesystem watcher that re-reads on external
  change.
- **Distribution friction.** "Install a server and open a browser" is worse
  onboarding than "install a plugin" for some users. Later mitigated by
  wrapping the server in a Tauri/Electron shell that ships a single binary.

**Neutral:**

- We lose free access to Obsidian's Bases views and Canvas. We build our own
  typed views, which are better suited to game objects anyway (portraits, HP
  bars, initiative).
