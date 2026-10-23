# Spike: the plugin host shape — manifest, capabilities, bridge, two plugins

> **Question 5, phase 0:** *What does the manifest actually need to say, and
> does the bridge carry what a plugin needs?*

## The answer, in three sentences

The manifest needs to say **less than ADR-0045 imagined and more than it
specified**: nine capabilities instead of six, split into three kinds
(invocable, subscription, declaration), because three of them are not
request/response operations at all, and only five of the nine were actually
exercised by the two plugins. The bridge carries what a plugin needs —
`ready`, `call`, `on`, `emit`, `pref`, `command.register`, plus the
context it injects — but only if it hands out a **projected `NoteView`**,
never a raw note, or ADR-0050's owner-scoped fields leak straight through.
Both a shadow-root surface and a light-DOM surface mount into app-provided
hosts, and the difference is visible in the browser: the app's round, filled
chip rule reaches the light surface and stops at the shadow boundary, where
the plugin's square, outlined chip wins.

## What was built

A disposable Go server plus a browser harness. It is not a deliverable and
must not be promoted.

```
spikes/plugin-host/
  manifest/        schema, YAML profile, validator, tests
  caps/            capability vocabulary, grants, scoping, tests
  bridge/          dispatch, events, prefs, commands, tests
  vault/           fake vault with the projection rules
  shell.go         embedded web shell
  web/             app.js, host.js, bridge.js, app.css, index.html
  plugins/export/  plugin A — no UI, compiled-in Go literal
  plugins/recap/    plugin B — UI, YAML manifest, shadow + light surfaces
  cmd/spikehost/    the driver
  verify/browser.mjs the browser verification harness
```

Run it:

```
go test ./spikes/plugin-host/...
go run ./spikes/plugin-host/cmd/spikehost   # then open ?role=gm and ?role=account
SPIKE_BASE=http://127.0.0.1:8777 node spikes/plugin-host/verify/browser.mjs
```

## The two plugins

They are the point of the spike: a manifest is a proposal until two real
plugins have had to use it.

| | Export (`demitable.export`) | Recap (`demitable.recap`) |
|---|---|---|
| Tier binding | compiled-in Go literal | runtime YAML, embedded |
| UI | none | two surfaces |
| Capabilities | `vault.read`, `vault.query`, `log` | `vault.read`, `vault.query`, `vault.watch`, `dom.inline` |
| Provides | a `service` (`export`, v1.0.0) | `recap.panel` (shadow) + `recap.inline` (light) |
| Consumes | `vault >=1.0.0 <2.0.0` | `vault >=1.0.0 <2.0.0` |
| Config | `format` (select), `title` (text) | `entries` (number) |
| Bridge use | none — touches the store directly | `ready`, `call`, `on`, `pref`, `command.register`, `emit` |

Together they declare **7 capability instances and only 5 distinct
capabilities** (`vault.read`, `vault.query`, `vault.watch`, `dom.inline`,
`log`). The export plugin is the second, non-UI consumer ADR-0019 asks for;
it is what makes freezing `api: 1` meaningful rather than aspirational.

## The manifest that resulted

`manifest.Manifest` is 11 top-level fields, but the two plugins exercised
far fewer:

- **Capabilities:** 9 in the vocabulary; 5 declared by the plugins; 4
  carried over from ADR-0045 or the design and used by neither plugin
  (`vault.write`, `vault.assets`, `http.client`, `vault.css`).
- **Scope:** 3 axes (`paths`, `types`, `uids`); only `types` was used
  (recap scopes both its capabilities to `session`). `paths` and `uids`
  are unexercised but cheap and clearly useful, so they stay.
- **Provides:** 12 fields per entry; `icon` was never set by either
  plugin; `dom: frame` was never used (only `shadow` and `light`).
- **Config field types:** 5 (`text`, `number`, `boolean`, `select`,
  `path`); the plugins used 3 (`number`, `select`, `text`). `boolean` and
  `path` are unexercised.
- **Consumes:** `optional` was never set.
- **Mount points:** 8 defined, 2 used by the plugins (`sidebar.right`,
  `note.sidebar`); `note.body` is reserved for the editor.

### The capability audit (ADR-0045)

ADR-0045 said "a capability with no host function does not exist." The
spike found that **3 of the 9 capabilities have no host function and must
not**, because they are not operations at all:

| Capability | Kind | Host function | Used by a plugin | Note |
|---|---|---|---|---|
| `vault.read` | invocable | `host_vault_read` | both (declared) | recap declares it but never calls it |
| `vault.query` | invocable | `host_vault_query` | both | the only data call either UI makes |
| `log` | invocable | `host_log` | export | |
| `vault.watch` | **subscription** | *none* | recap | a stream, not a call; bridge-only |
| `dom.inline` | **declaration** | *none* | recap | changes how the host mounts the surface |
| `vault.write` | invocable | `host_vault_write` | neither | from ADR-0045, unexercised |
| `vault.assets` | invocable | `host_vault_assets` | neither | unexercised |
| `http.client` | invocable | `host_http_fetch` | neither | unexercised; **host function name does not match the capability** |
| `vault.css` | **declaration** | *none* | neither | unexercised; redundant with the manifest's `style:` field |

So the vocabulary is **9 capabilities, 6 host functions, 3 kinds**. Only
**5 capabilities and 3 host functions** were exercised by the two plugins.
The freeze should cover the exercised set and let the rest arrive
additively.

### Host primitives are a second, unfrozen vocabulary

ADR-0045 freezes the capability vocabulary but says nothing about the
bridge operations that are **not** capabilities. The spike found four of
them, all load-bearing:

```
context.get   pref.get   pref.set   command.register
```

Recap needs `pref.get`/`pref.set` (collapsed state, role-scoped) and
`command.register` (the palette entry); `context.get` rides along as an
event. A plugin with zero capabilities still gets all four — they grant no
reach. **They are not in ADR-0045 at all**, which is a gap the freeze
criteria must close. `emit`, `ready()` and the event channel scheme
(`core`, `plugin:<id>`) are in the same boat.

### Does the bridge carry what a plugin needs?

**Yes, for the UI tier.** Recap used every primitive it needed and nothing
else. **Two caveats:**

1. The compiled-in tier (export) has no bridge at all — it touches
   `*vault.Store` directly and its command is registered by hand in
   `cmd/spikehost`. The manifest declares a `service` and a `consumes`,
   but nothing in it describes how a compiled-in plugin is wired, loaded
   or ordered. Phase 1 (M3d) needs a registration contract.
2. The bridge must return a **projected view**, never a raw note. The
   first version returned `vault.Note` wholesale, which would have handed
   a plugin `player`, `grants` and `discovered` for notes the principal
   cannot see. The uncommitted mid-edit fixed this: `vault.Project`
   returns `vault.NoteView`, stripping `Player`, `Grants` and `Discovered`
   unless the principal is the GM or the note's player (ADR-0050 D5).

### Shadow root vs. light DOM

Both mount, and the difference is visible:

- **Shadow** (`recap.panel`, `sidebar.right`): the app created the open
  shadow root; the plugin's stylesheet was adopted into it; the plugin's
  `.chip` is square, grey and outlined (radius `3px`).
- **Light** (`recap.inline`, `note.sidebar`): the custom element sits in
  the app's document; the app's `.chip` is a round, filled pill (radius
  `999px`).

The browser harness asserts: `chipRadius !== '999px'` in the shadow root,
`radius === '999px'` in light DOM, and the two differ. It also asserts the
shadow-DOM rules from `docs/shadow-dom-verification.md`: `event.target` is
retargeted to the host (`DIV`), while `event.composedPath()[0]` is the real
element (`n-session-*`); and `vault.write` / out-of-scope reads / unknown
operations are refused.

### ADR-0050: the projection is respected

The mid-edit made the bridge return `vault.NoteView` instead of `vault.Note`.
`Project` keeps `Campaign`, `Visibility` and `Workflow` (which inherit the
note's visibility) and strips `Player`, `Grants` and `Discovered` for
anyone who is not the GM or the note's player. Concretely:

- Ilia (`p_aria`) sees her own `grants` (`n-session-1`, `n-location-1`)
  and `discovered` (`n-location-1`, `n-character-2`); the GM sees them too.
- A player querying sessions gets **2**, not 3 (the GM-only session is
  `not_found`, indistinguishable from missing).
- An unknown `visibility` value already fails closed in `Visible`
  (default `false`).

The `discovered` visibility value itself (ADR-0050 D3) is **not**
implemented in this stub — `Visible` has no `discovered` case yet. That is
phase 1's job (M5), not this spike's.

### What failed, and what was wrong

1. **ADR-0045's "no host function ⇒ no capability" is false.** Three
   capabilities (`vault.watch`, `dom.inline`, `vault.css`) legitimately
   have none. The rule as written would have deleted the subscription and
   declaration kinds.
2. **`abi:` vs `api:`.** ADR-0045 writes `abi: 1`; ADR-0019, ADR-0020,
   `docs/plugin-api.md` and every example manifest write `api: 1`. The
   spike used `api:` because that is what the manifests already say. One
   of them is wrong; the proposal resolves it in favour of `api:`.
3. **Milestone mismatch.** ADR-0045 freezes `api: 1` at "M11"; the
   phase-1 plan freezes it at M8 (the export plugin). The spike's export
   plugin is the M8 one, so M8 is the right milestone.
4. **Over-declaration in the manifests.** Recap declares `vault.read` but
   its UI only ever calls `vault.query`; both plugins carry capabilities
   they never use. The manifests are aspirational, not minimal.
5. **`vault.css` is a duplicate of `style:`.** Nothing in either plugin
   uses the `vault.css` capability; the manifest's `style:` field already
   delivers a stylesheet. It should be dropped or folded into `style:`.
6. **`http.client`'s host function is misnamed** (`host_http_fetch`), so
   the "same name, prefixed" rule is already broken in v1.
7. **Compiled-in plugins have no loading contract.** Export works only
   because `cmd/spikehost` hand-wires it.
8. **The browser harness could not be re-run in this container** (missing
   `libnspr4`); the assertions above are taken from the committed harness
   and the code paths it drives, not from a fresh run here.

### What phase 1 should change

| Change | Milestone |
|---|---|
| Split capabilities into invocable / subscription / declaration; add the 4 host primitives (and `emit`, `ready`, the event scheme) to the freeze | M3a |
| App-provided shadow root + light-DOM opt-in via `dom.inline`; adopt styles into the root | M3b |
| `api:` field check before module load; golden ABI test | M3c |
| Freeze `api: 1` when the export plugin lands | M8 |
| Projected `NoteView` at the bridge, not `vault.Note` | M3b/M5 |
| Implement `discovered` visibility in the projection | M5 |
| Cross-browser shadow harness (Firefox/Safari) before committing to shadow DOM | before M3 |
| Manifest-driven registration for compiled-in plugins | M3d |
| Drop or fold `vault.css`; reconcile `abi:`/`api:` | M3a |

### What I would do differently

Write the two plugins first and derive the manifest from their `import`s
and calls, rather than starting from ADR-0045's table and trimming — which
is how `vault.read`, `vault.css` and `homepage` survived. Run the browser
harness in CI on every change; the R1/R5 rules are exactly the kind of thing
that rots silently. And settle the `abi`/`api` name before writing the
first manifest, not after.
