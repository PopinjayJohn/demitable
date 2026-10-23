# ADR-0005: Go server + Svelte SPA, shipped as a single binary

- **Status:** Accepted, amended by ADR-0019 and ADR-0020 (the application is a

  minimal core with every feature as a plugin, and a plugin host is added to
  the core) and by ADR-0021 (the editor is a Tiptap plugin rather than a
  CodeMirror component).
- **Date:** 2026-10-21

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

ADR-0001 established a local web application with a persistent server. The
implementation language and packaging shape determine:

- how live session state is coordinated (phase 3's board and initiative),
- how audio capture and transcription workers are spawned (phase 4),
- how the app is distributed to users,
- and how much of the frontend is shipped to a player's phone.

## Decision

**Backend: Go. Frontend: Svelte. Packaging: one static binary with the
frontend embedded.**

```
demitable/
  cmd/demitable/          entrypoint: config, open vault, start server, open browser
  internal/
    vault/           index, frontmatter parse, write semantics, watcher
    pack/            pack loading, validation, lint
    formula/         expression language: lexer, parser, evaluator
    entity/          note model, universal fields, visibility rules
    projection/      role/visibility filtering — the trust boundary
    session/         live session state, write-back
    server/          HTTP routes, REST, WebSocket, embedded static assets
  web/               Svelte app (Vite), built to web/dist
  packs/             built-in system packs, embedded
```

Build pipeline:

```
web:  npm run build      → web/dist
go:   go build ./cmd/demitable → embeds web/dist + packs/ → single `demitable` binary
```

The binary serves the API, the WebSocket, and the SPA. Starting the app is
`demitable --vault ~/vaults/my-campaign`. No runtime dependencies, no `npm` on the
user's machine, no container.

### Library choices

- HTTP: Go standard library `net/http`. Go 1.22+ routing is sufficient; no
  framework.
- WebSocket: `github.com/coder/websocket` (formerly nhooyr) — small, correct,
  context-aware.
- YAML: `gopkg.in/yaml.v3` — needed for frontmatter and packs.
- File watching: `github.com/fsnotify/fsnotify`.
- Markdown rendering: client-side, `marked` + a wikilink extension.
- Editor: **Tiptap**, client-side (amended — was CodeMirror 6).

### Why Go

- **Live state fits Go's concurrency model.** A session is a hub: many
  WebSocket clients, one authoritative state, fan-out on every change. Hub +
  goroutines + channels is idiomatic Go and needs no actor framework.
- **Single binary, no cgo.** Users get one file. Cross-compilation to Windows,
  macOS and Linux is trivial.
- **fsnotify** gives the vault watcher, which is load-bearing for the
  concurrent-edit strategy.
- **Phase 4 fits.** Whisper transcription runs as a sidecar binary; Go spawns
  and supervises it cleanly and speaks JSON over stdio.
- **Low memory.** A player client on a phone is fine, but so is the server
  running next to Obsidian all evening.

### Why Svelte

- Small runtime and small bundles — matters because every player loads the app
  on a phone over LAN.
- Compiles away: no virtual DOM runtime cost during the phase-3 board's
  high-frequency token updates.
- Reactive primitives map cleanly onto the store model the live session needs.
- Vite build output is directly embeddable.

### Why Svelte and not SvelteKit

The app is entirely client-side: it talks to a local server over HTTP and
WebSocket, and there is nothing to server-render. SvelteKit would add routing
and SSR machinery we would immediately opt out of. A Vite + Svelte SPA with
client-side routing is the smaller, faster thing. Revisit if the app ever needs
a public marketing/docs site, which can be a separate project.

## Consequences

**Good:**

- One file to install and run. Excellent for a hobby project with a
  volunteer user base, and it removes npm from the user's machine entirely.
- The server can be embedded in other tools later; it is a library plus a
  thin `main`.
- Go's static typing across the vault model, pack loader and projection layer
  catches a lot of schema bugs at compile time — valuable when the schema is
  the product.
- No CGO means no compiler toolchain requirements for contributors who only
  want to run it.

**Bad:**

- **Two toolchains for contributors.** Go and Node. Node is needed only to
  build the frontend; a prebuilt frontend is embedded in released binaries.
- **Go is not the fastest language to iterate UI in.** Svelte's ergonomics
  offset most of this.
- **Embedding the frontend means the binary must be rebuilt when the frontend
  changes.** Standard for this pattern, but it couples the release cycle of
  the two halves.

**Neutral:**

- If a Rust/Tauri shell is added later for native windowing, the Go server
  binary is still the payload — the split holds up.
