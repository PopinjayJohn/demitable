# Tech Stack

> The complete stack, with a reason for each choice and the ADR that decided
> it. Where a choice is provisional or unverified, it says so.
>
> Last updated: 2026-10-22.

---

## 1. Principles

1. **One language per side, and the boundary is explicit.** Go for everything
   in the core process; TypeScript for everything in the browser. The seam
   between them is the REST/WebSocket API and the plugin bridge — never
   anything sneakier.
2. **The binary is the deliverable.** A user runs one file. No runtime
   dependencies, no `npm` on the user's machine, no container.
3. **A dependency is a cost, not a default.** Every one below is there
   because writing it ourselves would be worse. Section 12 lists what was
   considered and rejected, so those decisions do not get re-litigated.
4. **The vault is the database.** There is no second store. No SQLite, no
   Postgres, no in-memory model that shadows the files.
5. **Local-first.** Everything works offline after the initial fetch. The
   only network calls in the product are the archive import and the optional
   MCP endpoint.

---

## 2. Summary

| Layer | Choice | Version |
|---|---|---|
| Core language | Go | 1.27+ |
| Frontend language | TypeScript | 5.9.x — see §12 |
| Frontend framework | Svelte | 5.x (runes) |
| Build tool | Vite | 8.x |
| Svelte plugin | `@sveltejs/vite-plugin-svelte` | 7.x |
| Editor | CodeMirror | 6.x |
| Block widgets | CodeMirror `Decoration.replace` / `.widget` | 6.x |
| Markdown (browser) | `remark` / `micromark` | latest |
| Tiptap / ProseMirror | *optional* — a plugin may ship a Tiptap surface | 3.x |
| Markdown parsing | `marked` | latest |
| WASM runtime | wazero | latest |
| Database | `modernc.org/sqlite` | latest — **pure Go, no cgo**, FTS5 built in |
| YAML | `gopkg.in/yaml.v3` | v3 |
| WebSocket | `github.com/coder/websocket` | v1 |
| File watching | `github.com/fsnotify/fsnotify` | v1 |
| Password hashing | `golang.org/x/crypto` (argon2id) | latest |
| Test framework (Go) | `go test` | stdlib |
| Test framework (web) | Vitest | latest |
| E2E | Playwright | latest |
| CSS | hand-written, custom properties | — |
| CI | GitHub Actions | — |

### What lives in SQLite, and what does not

SQLite holds the two **derived caches** (ADR-0030), both outside the vault in
the user-level application directory:

```
~/.local/share/demitable/vaults/<vault-id>/
  search.db     the FTS5 index over note bodies — rebuildable
  session.db    live session state — disposable
```

Everything else stays where it belongs:

| Data | Home |
|---|---|
| Note metadata, aliases, tags, link graph, frontmatter | **In memory** — cheap to rebuild, and the projection layer needs it live |
| Literal search over names and wikilink targets | **In memory** — exact lookup |
| Ranked body search | **SQLite FTS5** — expensive to rebuild, persisted |
| Live session state — HP, positions, initiative, the dice log | **SQLite** — mutable, crash-safe |
| Everything durable | **The vault** |

The test for anything that wants to go in the database: *can it be rebuilt
from the vault in one pass?* If yes, it is a cache. If no, it belongs in the
vault.

---

## 3. Backend — Go

Everything in the core process is Go. The standard library does the heavy
lifting; the dependencies below are the ones that would be worse to write.

| Dependency | Why |
|---|---|
| `net/http` (stdlib) | Go 1.22+ routing is sufficient. No framework |
| `gopkg.in/yaml.v3` | Frontmatter, packs, manifests. `yaml.Node` gives the round-trip fidelity the whole write path depends on (ADR-0004, design doc §5.5) |
| `github.com/fsnotify/fsnotify` | The vault watcher. Load-bearing for the concurrent-edit strategy (design doc §5.5) |
| `github.com/coder/websocket` | The live session transport. Small, context-aware, and the maintained successor to nhooyr |
| `github.com/tetratelabs/wazero` | The WASM runtime (ADR-0020). **Pure Go, no cgo** — this is what keeps the single static binary and trivial cross-compilation intact |
| `modernc.org/sqlite` | The derived caches (ADR-0030): the FTS5 search index and the live session store. **Pure Go, no cgo**, and FTS5 is compiled in by default. `mattn/go-sqlite3` is ruled out — it needs cgo, which breaks the single-binary and cross-compilation story |
| `golang.org/x/crypto` | argon2id for passphrase hashes (ADR-0009) |

**Not a dependency:** ULID. The uid format (design doc §5.3) is about sixty
lines and we want the uid derivation to be ours, because it has to be
deterministic for the SRD pipeline (ADR-0024).

### Package layout

```
cmd/demitable/          entrypoint
internal/
  vault/                index · frontmatter · write semantics · watcher · journal
  pack/                 loading · validation · lint
  formula/              expression language: lexer, parser, evaluator
  dice/                 dice engine: faces, tables, pools, opposed rolls
  entity/               note model · universal fields · visibility
  projection/           role and visibility filtering — the trust boundary
  accounts/             local accounts, join codes, sessions
  plugin/               host: registry · capabilities · WASM runtime · assets
  import/               archive import: fetch, extract, validate, apply
  store/                the SQLite caches: FTS5 search index, live session state
  server/               routes · REST · WebSocket · embedded static assets
  assets/               the /asset endpoint and its visibility rules
  session/              live session state · write-back
```

---

## 4. Frontend — Svelte

| Choice | Why |
|---|---|
| **Svelte 5** | Small runtime and small bundles — every player loads the app on a phone over a LAN. Compiles away, so the phase-3 board's high-frequency token updates cost no virtual-DOM overhead. Its reactivity primitives map onto the live session store |
| **Vite 8** | The build tool. Fast, and the dev server proxies to the Go server, so there is one process for the user and two for the developer |
| **No framework beyond Svelte** | The app's complexity is in the domain, not the view layer |
| **No CSS framework** | Tailwind's utility classes fight the shadow-DOM isolation model (ADR-0029), and a hand-written token layer is enough for an app with one design language |
| **CSS custom properties for theming** | They inherit through shadow roots, so a plugin surface picks up the app's theme for free. This is the theming mechanism ADR-0029 depends on |

### SvelteKit?

No. The app is entirely client-side — it talks to a local server over HTTP
and WebSocket, and there is nothing to server-render. SvelteKit would add
routing and SSR machinery we would immediately opt out of. A Vite + Svelte SPA
with client-side routing is the smaller, faster thing.

---

## 5. The editor

| Piece | Choice | Why |
|---|---|---|
| Editor | **CodeMirror 6** | The buffer *is* the source, so the byte-identical round-trip contract holds by construction rather than by hope. Measured: no markdown library re-serializes byte-identically, because CommonMark's parse is lossy by design (ADR-0021) |
| Structured content | **Block widgets** — `Decoration.replace` for a block, `Decoration.widget` for inline | Obsidian's model, and our compatibility target already proves it works. Plugins contribute structured content this way |
| Character sheet | A **form panel** rendered from the pack's `views.sheet` | The pack already declares the layout, so the sheet, the print view and any future client all render from one declaration |
| Markdown (browser) | `remark` / `micromark` | Retains byte offsets on every AST node, so a minimal splice is computable |
| Tiptap / ProseMirror | *optional* | Not a dependency of the editor plugin. A plugin may ship a Tiptap-based surface through the extension registry; it is not the prose editor, and it does not carry the byte-identity contract |

The editor is a **compiled-in plugin mounted in light DOM** (ADR-0021,
ADR-0027) — it is the app's own content surface, not an isolated one.

**A Go markdown parser is still needed**, for the FTS5 body preprocessing and
the link checker, which both run in the core process. The JavaScript parser
above cannot serve them. See
[`docs/research/implementation-risks.md`](research/implementation-risks.md)
§3.19.

---

## 6. Plugin surfaces

| Kind | Mechanism | Used by |
|---|---|---|
| **Shadow** (default) | A `<div>` host with an attached shadow root | Almost every plugin |
| **Light DOM** | Mounted into a named host the app provides | The editor, the table |
| **Frame** | A separate origin, `sandbox`ed | Embedding a third-party page or untrusted HTML |

CSS custom properties set on `:root` inherit into every shadow root, which
is how a plugin surface gets the app's theme without the app's styles
bleeding into it (ADR-0029).

---

## 7. Data formats

| Thing | Format | Notes |
|---|---|---|
| Notes | **Markdown + YAML frontmatter** | The canonical format. Obsidian-compatible (ADR-0006) |
| Packs | **YAML** | Mechanics and content declarations (ADR-0023) |
| Plugin manifests | **YAML** | `plugin.yaml` |
| Import manifests | **JSON** | `demitable-import.json` |
| Journal entries | **JSON**, one file per day | Append-only (ADR-0015) |
| Import archives | **zip** | With an optional manifest (ADR-0025) |
| Assets | **WebP, PNG, SVG, JPEG** | Referenced by path, served role-scoped (ADR-0026) |
| Live session state | **JSON over WebSocket** | Disposable; written back at session end (ADR-0004) |
| The vault's app-private state | **YAML and files** under `.demitable/` | Never importable, never indexed (ADR-0025) |

---

## 8. Build and release

```
web:   npm ci && npm run build        → web/dist
sync:  copy web/dist → internal/server/webdist
go:    go build -trimpath -ldflags "-s -w" -o dist/demitable ./cmd/demitable
```

- `internal/server` embeds `webdist` with `//go:embed`; `internal/pack`
  embeds `packs/`. **One binary contains the API, the WebSocket, the packs,
  the built frontend and every first-party plugin.**
- The embed directory is kept in the source tree, gitignored, and populated
  by the build. A committed `.gitkeep` keeps `go build` working on a fresh
  clone.
- **Cross-compilation matrix:** `linux/amd64`, `linux/arm64`,
  `darwin/amd64`, `darwin/arm64`, `windows/amd64`. No cgo, so this is one
  command.
- `CGO_ENABLED=0` is enforced in CI, because a single cgo dependency would
  break the whole distribution story.

---

## 9. Testing

| Suite | Tool | What it covers |
|---|---|---|
| Core unit | `go test` | The formula engine, the dice engine, the pack linter, the frontmatter patcher |
| **Golden corpus** | `go test` + `dev/srd-vault/` | 1,692 real-world markdown files: byte-identical round-trip, schema validation, link resolution, and a scan for non-SRD content (ADR-0024, ADR-0021) |
| **Projection** | property tests | For every note, property and role, the projection never leaks a `gm` field. The single most important suite in the project |
| Frontend | Vitest | Components, the bridge, the shadow-DOM surface |
| End-to-end | Playwright | The editor, a session, the board |
| Import | `go test` | Zip slip, zip bombs, symlinks, path traversal, rejected private paths |

`testing/quick` from the standard library covers the property tests. If
they need more power later, `rapid` is the upgrade — but a dependency is a
cost (§1.3).

---

## 10. CI

GitHub Actions, on every push:

```
lint      gofmt -l, go vet, golangci-lint, prettier --check, eslint, svelte-check
test      go test -race ./..., vitest, playwright
corpus    the golden-corpus suites (round-trip, validation, links, non-SRD scan)
build     cross-compile matrix, CGO_ENABLED=0, smoke-test the binary
```

The corpus suites need no SRD source corpus — they run against the
**committed** vault. Only the "does the pipeline still reproduce this?"
check needs the corpus, and that is a maintainer task (ADR-0024).

---

## 11. Development workflow

```
make dev        # Go server on :8080, Vite dev server on :5173, proxying /api, /ws, /asset
make test       # go test + vitest
make corpus     # the golden-corpus suites
make build      # the full pipeline, producing dist/demitable
make lint       # format and vet
```

In dev, Vite proxies the API and the WebSocket to the Go process, so the
frontend has hot module replacement and the backend is a plain restart. In
production there is one process and no proxy.

The plugin API is versioned separately from the app (`api: 1` in a manifest).
The core supports the current major and the previous one.

---

## 12. Tooling, versions, and one open question

| Tool | Version | Note |
|---|---|---|
| Go | 1.27+ | Current release policy: the two most recent majors |
| Node | 22.x LTS | Only needed to *build* the frontend. Never required at runtime |
| npm | 10.x | Ships with Node 22 |
| `golangci-lint` | latest | `gofmt`, `go vet`, `staticcheck`, `errcheck` |
| `prettier` | latest | Formatting for the frontend and the docs |
| `eslint` + `typescript-eslint` | latest | Linting the frontend |
| `svelte-check` | latest | Type checking Svelte components |
| `playwright` | latest | E2E, and the verification harness |

**Dependency updates:** automated for patch and minor, manual for major.
Monthly, via Dependabot or Renovate. A major bump to `wazero`, `yaml.v3`,
`modernc.org/sqlite` or the CodeMirror packages is reviewed by hand, because
each touches a load-bearing invariant. Tiptap is optional now (ADR-0021), so
it is not on this list.

### The one open question: TypeScript 7 — settled

TypeScript 7.0 is the native (Go-based) port of the compiler. It is the
current release, and it would be the natural choice — but **`typescript@7`
alone breaks `svelte-check`**, with a hard version-guard error.

The cause is visible in the package: `require('typescript')` returns only
`{ version, versionMajorMinor }`. The package exports **no JavaScript compiler
API**; it lives only under `./unstable/*`. Every tool that embeds the TS
compiler — `svelte-check`, `svelte-language-server`, ESLint's TypeScript
parsers — needs the native binary explicitly.

Two working configurations, both verified:

```jsonc
// A. Recommended
"devDependencies": { "typescript": "~5.9.3" }   // or ~6.0.3, what create-vite scaffolds

// B. With the native compiler's speed
"devDependencies": {
  "typescript": "~6.0.3",
  "@typescript/native": "npm:typescript@7"
},
"scripts": { "check": "svelte-check --tsconfig ./tsconfig.app.json --tsgo" }
```

Configuration B was confirmed to actually catch type errors rather than
silently no-op. But it is fragile: `svelte-check` gates it behind an
unguarded version check, it depends on an npm-alias trick the TS team has not
standardised, and the two installs fight over `node_modules/.bin/tsc`.

**Decision: configuration A.** TypeScript 5.9.x (or 6.x) is the primary
`typescript` dependency. `npm run build` is unaffected either way — it
succeeded with 5.9.3, 6.0.3 and 7.0.2.

The native compiler can be added later, deliberately, once the ecosystem
settles. Full evidence:
[`shadow-dom-verification.md`](shadow-dom-verification.md) §6.

---

## 13. What was considered and rejected

| Considered | Rejected | Why |
|---|---|---|
| **Electron** | A second Electron next to Obsidian, ~150 MB, two Chromium trees, and Node at runtime | Breaks the single-binary, no-runtime-dependency promise (ADR-0005) |
| **Tauri** | A Rust toolchain and a second language for what is a local server | The app is a server; a native shell can wrap the binary later if it ever needs one |
| **Node/Electron server** | Node at runtime, `npm` on the user's machine | Same failure as Electron, without the native shell |
| **React** | Heavier than Svelte, and no advantage here | Svelte 5's reactivity suits the live session store; smaller bundles matter for the player client |
| **Tailwind** | Utility classes fight shadow-DOM isolation and produce a large generated stylesheet | A hand-written custom-property token layer is enough, and it inherits into shadow roots for free |
| **go-git** | Heavyweight for a feature we dropped | The archive import replaced the git route (ADR-0025); git is the user's business |
| **The Extism SDK** | Brings its own manifest and plugin model alongside ours — two descriptions of what a plugin is | We implement Extism's *ABI* on wazero instead (ADR-0020, `docs/plugin-api.md` §7) |
| **A package registry** | A supply-chain surface we would have to operate, moderate and pay for | Packs and content are local files and archives at a URL (ADR-0011, ADR-0025) |
| **SQLite as the source of truth** | A second store means a second source of truth, and a sync problem | The vault *is* the database. **SQLite is used — but only for derived caches outside the vault** (ADR-0030) |
| **Postgres or any server database** | A server to run, a network to depend on, an account to manage | The app is one local process and one file |
| **A JS markdown parser in Go** | Would need a JavaScript runtime in the core process | The parser lives in the browser, where the editor is |
| **Tiptap as the prose editor** | An excellent structured editor, and a lossy markdown serialiser | The byte-identical contract cannot be met by re-serializing markdown, by any library (ADR-0021) |
| **A JS interpreter for plugin conditions** | A large dependency in the core process, and a much weaker sandbox than the formula engine | The `when` condition reuses the formula language (ADR-0003) |
| **Monaco** | Large, and aimed at code | CodeMirror 6 covers the editor |
| **A Go markdown parser written from scratch** | A parser is a parser | One is still needed for the FTS indexer and link checker, but it is a known quantity, not a research project |

---

## 14. Where each decision lives

| Area | ADRs |
|---|---|
| Not a plugin, stack | 0001, 0005, 0006 |
| Trust boundary, projection, accounts | 0002, 0009, 0010, 0014, 0017, 0026 |
| Packs, content, the SRD | 0003, 0007, 0011, 0012, 0023, 0024, 0025 |
| Core + plugins, tiers, surfaces | 0019, 0020, 0027, 0029 |
| The editor | 0021 |
| Dice | 0008, 0022 |
| Write-back, journal | 0004, 0015 |
| Search | 0013, 0030 |
| Phase 3 and 4 scope | 0016, 0018 |
