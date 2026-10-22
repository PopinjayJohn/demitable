# Implementation risks — Demitable's stack

> Research note, 2026-10-22. Every claim below is either cited to a source or
> **measured** on this machine (Go 1.27.1, Node 22.23.3, wazero v1.12.0,
> modernc.org/sqlite v1.61.0, yaml.v3 v3.0.1, Tiptap/@tiptap/markdown v3.32.0,
> fsnotify v1.10.1, MDN browser-compat-data v8.1.6, OSV.dev).
> Measurement harnesses live in `/tmp/opencode/{wazcheck,yamlprobe,sqlprobe,mdprobe,memprobe,gitbench}`.

---

## 0. Executive summary

The stack is in good shape. `modernc.org/sqlite` is a non-issue; the WASM ABI
shape is right; cross-compilation and binary size are fine; git and the Go
in-memory index are fine at 50,000 notes.

**One decision is invalid as written.** ADR-0021's byte-identical round-trip
contract cannot be met by `@tiptap/markdown` — measured at **1 of 46
fixtures** — and, more importantly, it cannot be met by *any* markdown
library, because markdown's parse is lossy by design. This is not a "beta, we
will patch around it" problem. It is an architecture problem, and it needs a
decision before the editor plugin is written.

Four other things will bite: `busy_timeout`/pool sizing on SQLite, the
absence of any fuel/epoch API in wazero, **fsnotify's macOS backend opening
one file descriptor per file** (a 50k-note vault cannot be watched on macOS
at all), and Obsidian's vault-wide-by-name property types.

---

## 1. Ranked risk list

Severity = how bad if it happens. Likelihood = how likely it is in the
stated design. "Will definitely break" is marked as such.

| # | Sev | Like | Risk |
|---|-----|------|------|
| R1 | **High** | **Certain** | `@tiptap/markdown` cannot round-trip. 1/46 fixtures byte-identical. **Will definitely break** the ADR-0021 contract. |
| R2 | **High** | **Certain** | No markdown library round-trips byte-identically. The contract itself is unsatisfiable by re-serializing. **Will definitely break.** |
| R3 | **High** | **Certain** | wazero has **no fuel and no epoch API** in any released version. Only `WithCloseOnContextDone` + a context deadline can stop a runaway module. **Will definitely break** any plan assuming fuel metering. |
| R4 | **High** | **Certain** | `busy_timeout` defaults to **0** and `database/sql` opens unbounded connections → "database is locked". Measured 243/400 concurrent writes failing. **Will definitely break.** |
| R5 | **High** | **Certain on macOS** | fsnotify's **kqueue backend opens a file descriptor for every file in a watched directory** (to mimic inotify). A 50k-note vault needs 50,000 fds on macOS → `EMFILE`. **Will definitely break** on macOS at scale. |
| R6 | **High** | Likely | Obsidian's property types are **vault-wide by name** (`.obsidian/types.json`) and a *hint, not a constraint*. The writer must emit the right YAML *type*, not an equal-looking string. |
| R7 | **High** | Likely | `ShadowRoot.getSelection()` is **Chromium-only** (Firefox: false, Safari: false). The project's shadow verification was Chromium 156 only. |
| R8 | **High** | Likely | yaml.v3's `yaml.Node` round-trip is **not byte-identical**: folded scalars, CRLF, indentation, `---`, `!!merge` all change. |
| R9 | **High** | Likely | Unquoted wikilinks in frontmatter parse as **nested YAML lists**, not strings. `related: [[Note]]` → `[["Note"]]`. |
| R10 | **Med** | Likely | wazero's default memory limit is **4 GiB per instance**; a guest can allocate it. |
| R11 | **Med** | Likely | ADR-0020's "wazero does not implement [sockets]" is **factually wrong** — `wasi_snapshot_preview1` exports `sock_send`/`sock_recv`/`sock_accept`/`sock_shutdown`. "No network" is configuration, not a runtime property. |
| R12 | **Med** | Likely | The ADR-0021 "raw-node escape hatch" cannot fire as designed: `@tiptap/markdown` silently mangles unsupported syntax instead of flagging it. Requires a separate pre-scanner. |
| R13 | **Med** | Likely | Obsidian's YAML schema ≠ yaml.v3's. `yes`/`no` are strings in both, but yaml.v3 decodes `2026-10-15` to `time.Time` while js-yaml leaves it a string. |
| R14 | **Med** | Likely | No plugin-ABI versioning scheme is defined. ADR-0020 specifies the ABI shape but not how version N stays compatible with N+1. |
| R15 | **Med** | Possible | yaml.v3's alias-bomb guard only fires on the **decode** path, not the `yaml.Node` path. |
| R16 | **Med** | Possible | `adoptedStyleSheets` on a shadow root needs Safari 16.4 / Firefox 101. ADR-0021 relies on CodeMirror using it. |
| R17 | **Med** | Possible | `delegatesFocus` and `ElementInternals` have a Safari 15 / 16.4 floor. |
| R18 | **Med** | Possible | fsnotify: file-level watches break under atomic save; Linux reports REMOVE as CHMOD when a file is open. |
| R19 | **Med** | Possible | Catastrophic backtracking in the *browser* (JS regex backtracks; Go's RE2 does not). |
| R20 | **Med** | Possible | Obsidian itself degrades around 10k–12k notes. "Obsidian-compatible" is a format promise, not a scale promise. |
| R21 | **Med** | Possible | yaml.v3 accepts duplicate keys on the `Node` path (errors only on the map path) → the writer may patch the wrong occurrence. |
| R22 | **Low** | Unlikely | Obsidian property names are matched **case-insensitively**. |
| R23 | **Low** | Unlikely | Guest traps are recoverable — the module stays alive and re-traps. Not a crash, but a "poisoned instance" if unhandled. |
| R24 | **Low** | Unlikely | wazero `experimental` package: "All features here may be changed or deleted at any time." |
| R25 | **Low** | Unlikely | modernc.org/sqlite's own bug class: "transpilation faults … unique to a project that ships SQLite's C as generated Go" is explicitly in scope for their security process. |
| R26 | **Low** | Unlikely | ProseMirror does not virtualize; a very long single note will be slow to render/scroll/map. |
| R27 | **Low** | Unlikely | Very long YAML lines are fine (5 MB scalar parses in 108 ms) — but cap file and frontmatter sizes. |

---

## 2. The five things most likely to force a design change

These are the ones that would invalidate decisions already made.

### 2.1 The editor architecture (ADR-0021) — R1, R2, R12

**The finding.** `@tiptap/markdown` v3.32.0 + StarterKit, round-tripping 46
hand-picked fixtures, produced **1 byte-identical result** (the empty
document). This is not a list of edge-case bugs; it is systematic lossiness
on *ordinary* markdown:

| Input | Output | What was lost |
|---|---|---|
| `\| a \| b \| c \|\n\|:--\|:-:\|--:\|\n\| 1 \| 2 \| 3 \|` | `""` | **The entire table.** Silent, total data loss. |
| `![alt](img.png)` | `alt` | **The image.** Becomes its alt text. |
| `<div>\n  <p>raw</p>\n</div>` | `raw` | **The HTML block.** Becomes its text content. |
| `- [ ] todo\n- [x] done` | `- todo\n- done` | **Checkbox state.** |
| `\# not a heading` | `# not a heading` | **The escape** — and it is *not idempotent*: re-parsing gives a heading. |
| `AT&amp;T &copy;` | `AT&amp;T &amp;copy;` | **Double-escaping** — repeated saves progressively corrupt text. |
| `Heading\n=======` | `# Heading` | Setext → ATX. |
| `[text][ref]\n\n[ref]: https://x` | `[text](https://x)` | Reference style + the definition. |
| `_italic_` | `*italic*` | Underscore emphasis. |
| `    indented code` | ` ```\nindented code\n``` ` | Indented → fenced. |
| `***` / `___` | `---` | Thematic break style. |
| `line one\` (backslash break) | `line one  ` (two spaces) | Hard-break style. |
| `line one  ` (two-space break) | `line one\` | The other direction. |
| `<https://x>` | `[https://x](https://x)` | Autolink. |
| `text with <b>bold</b> html` | `text with **bold** html` | Inline HTML. |
| every document | trailing newline changes | The file's final newline. |

**And it is not a library-quality problem.** I tested the alternatives:

| Implementation | Byte-identical |
|---|---|
| `commonmark` (JS port of the CommonMark reference impl) → HTML | 1/29 |
| `markdown-it` → HTML | 1/29 |
| `marked` → HTML | 1/29 |
| `remark` + `remark-gfm` (mdast → markdown) | **8/21** on the hard subset (20/29 on the easier one) |

`remark` — the only JS stack with a real *markdown* serializer — still
mangles setext headings, `_emphasis_`, indented code, `1)` ordered markers,
`---`/`___` rules, hard-break style, entities, table cell padding, blank-line
runs and nested-blockquote spacing.

**Why this is structural.** CommonMark's parse is *lossy by design*: the spec
defines "ambiguous constructs" where two different source texts produce the
same AST (emphasis delimiter runs resolved by left/right-flanking rules, the
list-item paragraph rule, etc.). A re-serializing editor therefore cannot
recover the original bytes — it can only pick *a* canonical form. **No
reference implementation round-trips CommonMark byte-identically, and none
can.** `cmark` (the C reference) has a `--to commonmark` renderer, but its
round-trip test asserts *semantic* equivalence (re-parse yields the same
AST), not byte identity.

**What does work.** `remark`/`micromark` retain `position`
(line/column/**offset**) on every AST node — I verified this. So the only
architecture that satisfies the contract is: **keep the original bytes and
splice, never re-serialize.** That is CodeMirror 6 + a position-aware
parser, not ProseMirror.

**Options.**

| Option | Cost | Outcome |
|---|---|---|
| **A. CodeMirror 6 primary, live-preview pane** | Lose node views for structured blocks | Contract is *trivially* satisfiable (the buffer is the source). Loses the "structured content" argument from ADR-0021. |
| **B. Tiptap for structured content only; CodeMirror for prose** | Two editors | Character sheets/encounters keep node views; prose keeps byte-identity. Best of both, more work. |
| **C. Keep Tiptap; weaken the contract to "blocks we understand round-trip, the rest is preserved verbatim"** | Requires a **pre-scanner** (a second markdown parser) that classifies the source into supported/unsupported regions *before* Tiptap sees it | The "raw-node escape hatch" in ADR-0021 becomes real, but the cost is a second parser. |
| **D. Keep the contract, keep Tiptap, and implement the round-trip at the *node* level with source slices** | Deep ProseMirror work | Doable but large: every node needs to carry its source span and be re-serialized by splicing. |

**Recommendation: B.** Keep Tiptap for the things ADR-0021 actually wants it
for — character sheets, encounters, dice results, embedded entities, where
"the document is structured, not a string" is the whole argument — and make
**CodeMirror 6 the prose editor**, which is also what ADR-0006 originally
specified before ADR-0021 overrode it. Note ADR-0021's own text already
admits CodeMirror was the prior choice and that the byte-identical contract
is the *reason* Tiptap was adopted — that reasoning is now falsified.

**The pre-scanner problem (R12).** ADR-0021's mitigation is "any block the
parser cannot represent becomes a `raw` node holding its source verbatim."
That requires the parser to *know* it is looking at something it does not
understand. `@tiptap/markdown` does not tell you that — it silently mangles.
So the escape hatch needs a **front-end classifier** that runs before Tiptap
and marks unsupported regions. That classifier is a markdown parser. Budget
for it.

**Separate, smaller finding.** `@tiptap/markdown` v3.32.0 depends on
**`marked` ^17.0.1** — the same library the project already uses for the
preview pane. That is *good* for consistency (preview and editor agree on
the tokenizer), but it means the editor's fidelity ceiling is `marked`'s
token model, and `marked` is a tokenizer, not a lossless parser.

### 2.2 The WASM execution model (ADR-0020) — R3, R10, R11, R14

**wazero has no fuel meter and no epoch ticker.** I checked the module cache
for ten released versions:

| Version | `experimental/` contents |
|---|---|
| v1.0.0 | `experimental.go gojs listener.go logging testdata` |
| v1.3.0 – v1.6.0 | `close.go` (had `CloseWithExitCode` — the old epoch mechanism) |
| v1.7.3, v1.8.2, v1.9.0, v1.10.1, v1.11.0, **v1.12.0** | `checkpoint.go close.go compilationworkers.go experimental.go features.go importresolver.go listener.go logging memory.go sock sys sysfs table wazerotest` |

There is **no `experimental/fuel` and no `experimental/epoch` in any release**.
The old epoch API (`experimental/close.CloseWithExitCode`) is gone. A
community PR to add fuel — [wazero#2500 "feat: experimental Fuel API
support"](https://github.com/wazero/wazero/pull/2500) — was opened
2026-05-18 and **closed unmerged on 2026-09-13**. v1.12.0 (2026-05-28) added
exception handling, typed references and extended const as *experimental
feature flags*, but still no fuel and no epoch.

**The only supported way to stop a runaway module.** Measured, on
wazero v1.12.0, with a hand-assembled infinite-loop module
(`(func (export "run") (loop $l (br $l)))`), in both the compiler and the
interpreter engines:

```
infinite loop + closeOnContextDone + 200ms   took=200ms   err=module closed with context deadline exceeded
call after termination                        took=0s      err=module closed with context deadline exceeded
```

And **without** it:

```
infinite loop, no closeOnContextDone         STILL RUNNING after 600ms
  -> Runtime.Close while spinning            Close() returned
  -> spinning call                           call STILL SPINNING after Close
```

**`Runtime.Close()` does not stop a spinning guest.** The call returns, the
goroutine keeps burning a core forever, and there is no API to stop it. This
is the single most important operational fact in this section.

**Mitigations to adopt.**

1. `wazero.NewRuntimeConfig().WithCloseOnContextDone(true)` is **mandatory**,
   not optional. The docstring warns it "costs a bit of extra" because it
   inserts periodic checks — take the cost.
2. Every guest invocation gets `context.WithTimeout`. The host's plugin
   dispatcher owns the deadline; a plugin cannot extend it.
3. Treat `module closed with context deadline exceeded` as **"this plugin
   instance is dead"** — discard it and re-instantiate on the next call. The
   module is closed permanently (verified: the next call fails identically).
4. **Per-instance memory cap.** `RuntimeConfig.WithMemoryLimitPages(n)`. The
   default is 65536 pages = **4 GiB per instance**. Measured: growing to
   1024 pages succeeded by default and failed (returned `0xFFFFFFFF`) with
   `WithMemoryLimitPages(2)`. Set this to something like 256 pages (16 MiB)
   and raise it per-plugin if needed. Note the corollary from the docstring:
   `WithMemoryCapacityFromMax(true)` with no max in the binary allocates 4 GiB
   eagerly — never enable it.
5. **Do not promise fuel-based metering** to plugin authors, and do not put
   "fuel" in the plugin API docs. If you need a budget, use wall-clock
   deadlines and a per-call host-function rate limit.
6. **Reject modules that import threads/exception-handling.** `api.CoreFeaturesV2`
   (the default) is mutable-global + bulk-memory + multi-value +
   non-trapping-float-to-int + reference-types + sign-extension + SIMD —
   **no threads**. Threads/exception-handling/tail-call/typed-function-references
   are `experimental` and require an explicit `WithCoreFeatures`. Assert
   `WithCoreFeatures(api.CoreFeaturesV2)` explicitly and add a test that a
   module importing `wasi_thread_start` fails to instantiate.

**The "no network" claim in ADR-0020 is wrong as written.** It says "WASI
preview 1 has no sockets, and wazero does not implement them." wazero *does*
implement them: `imports/wasi_snapshot_preview1/sock.go` exports
`sock_accept`, `sock_recv`, `sock_send`, `sock_shutdown`, and
`experimental/sock` can pre-open TCP listeners. The *default* is still safe
(no pre-opened socket ⇒ the calls fail), so the sandbox assumption holds —
but it holds because of **configuration**, not because of a missing
implementation. Fix the ADR, and add a test that asserts `sock_send` from a
guest returns an error.

**Memory growth per instance (R10).** Measured: 200 live trivial module
instances produced no measurable heap delta. But that is a 4-byte module. A
real guest (Go/TinyGo) declares a large minimum linear memory — budget
2–16 MiB per instance and cap it with `WithMemoryLimitPages`.

**JSON-over-the-ABI cost (not a risk).** Measured: Go `json.Unmarshal` of a
97-byte request = **3.0 µs**; `json.Marshal` of a 42-byte response =
**1.8 µs**; a 97-byte `memcpy` = **4 ns**. Total ≈ **5 µs** per call, which
is dominated by JSON, not by the ABI. Fine unless a plugin calls the host in
a tight loop — add a per-call rate limit.

**Known CVEs (not a risk).** OSV.dev query for `github.com/tetratelabs/wazero`
returns **no advisories**. wazero publishes a semver API-stability promise.
The caveat is the `experimental` package's own warning: "All features here
may be changed or deleted at any time."

**Is the ABI shape right? Yes.** "One exported entry point taking and
returning JSON, plus a small set of host functions" is *literally* Extism's
ABI, and `github.com/extism/go-sdk` implements it on wazero:
`plugin.Call(name, data []byte) (exit int, out []byte, err error)` plus
`NewHostFunctionWithStack`. ADR-0020's decision to implement Extism's ABI
rather than depend on Extism is correct: Extism's kernel is a second wasm
module and its Go SDK pulls in a Rust-built kernel, which does not fit a
single-binary, no-runtime-dependency promise. Other production patterns
(all of which are *worse* fits for this project): HashiCorp `go-plugin`
(gRPC subprocess — breaks the single binary), Wasmtime's component model /
WIT (Rust + cgo), Envoy `proxy-wasm` (explicit `proxy_abi_version` in the
host ABI struct). **Your shape is the right one. The gap is versioning, not
shape — see R14.**

### 2.3 The vault watcher (ADR-0029/§5.5) — R5, R18

**The platform behaviour is wildly different, and only one of the three is
safe at 50,000 notes.** I read fsnotify v1.10.1's backends to establish this.

| Backend | Cost of watching a vault | 50,000 notes in 121 dirs |
|---|---|---|
| **Linux (inotify)** | **one watch per directory added.** `backend_inotify.go` has no `ReadDir`/`watchDirectoryFiles` — a watched directory is one inotify watch, and the kernel reports entries within it. | **121 watches. Fine.** |
| **Windows (ReadDirectoryChangesW)** | **one directory handle per added path** (`backend_windows.go:117`). | **121 handles. Fine.** But see the overflow note below. |
| **macOS / BSD (kqueue)** | **one file descriptor per FILE.** `backend_kqueue.go:430` calls `watchDirectoryFiles(dir)`, whose own comment reads *"watchDirectoryFiles to mimic inotify when adding a watch on a directory"* — it does `os.ReadDir(dir)` and calls `internalWatch` on **every entry**, which does `unix.Open(name, …)`. There is no option to disable it. | **50,000 fds → `EMFILE` ("too many open files"). Broken.** |

macOS's `kern.maxfilesperproc` is commonly 10240 (256 on older releases) and
`kern.maxfiles` 12288–24576. **A 50,000-note vault cannot be watched with
fsnotify on macOS at all** — `Add()` fails partway through and leaves the
watch set half-installed. And **FSEvents is not supported by fsnotify yet**
("Needs support in x/sys/unix", issue #11), so there is no library escape
hatch.

**This is the one item in the watcher section that will *definitely* break,
and it breaks on a platform the project targets.**

**The other fsnotify v1.10.1 hard limits (from its README):**

- **No recursive watch.** "Are subdirectories watched? No, you must add
  watches for any directory you want to watch (a recursive watcher is on the
  roadmap: #18)." Every directory in the vault is one watch — so on Linux
  the limit that matters is the **directory** count, not the file count.
- **inotify watch limit.** "The `fs.inotify.max_user_watches` sysctl variable
  specifies the upper limit for the number of watches per user, and
  `fs.inotify.max_user_instances` specifies the maximum number of inotify
  instances per user. Every Watcher you create is an 'instance', and every
  path you add is a 'watch'. Reaching the limit will result in a 'no space
  left on device' or 'too many open files' error." The default varies
  widely by distro and version — 8192 (older Debian/Ubuntu), 65536, 524288,
  or 1024 (RHEL) — and this machine reports 4194304, so **you cannot assume
  the user's limit is generous**. A user with a folder-per-note vault hits
  it. Also: **one `fsnotify.NewWatcher()` per instance** (default
  `max_user_instances` = 128 on most distros) — use exactly one.
- **No events on network filesystems.** "Why don't notifications work with
  NFS, SMB, FUSE, /proc, or /sys? … The current NFS and SMB protocols does
  not provide network level support for file notifications." And the polling
  watcher is "Not yet" implemented (#9). **A vault on a NAS, a Dropbox folder
  or a Syncthing mount gets no events at all, silently.**
- **Atomic save breaks file-level watches.** "Watching individual files
  (rather than directories) is generally not recommended as many programs
  (especially editors) update files atomically: it will write to a temporary
  file which is then moved to a destination, overwriting the original (or
  some variant thereof). The watcher on the original file is now lost."
- **Linux REMOVE becomes CHMOD when a file is open.** "When a file is removed
  a REMOVE event won't be emitted until all file descriptors are closed; it
  will emit a CHMOD instead." And "As a rule, it's typically best to ignore
  Chmod events" — but ignoring CHMOD loses deletions.
- **Chmod storms.** "Some programs may generate a lot of attribute changes;
  for example Spotlight on macOS, anti-virus programs, backup applications,
  and some others are known to do this."
- **Windows drops events on buffer overflow.** `backend_windows.go:582` sends
  `ErrEventOverflow` when the `ReadDirectoryChangesW` buffer fills — events
  are *lost*. Raise it with `fsnotify.WithBufferSize`, and still rescan.

**Debounce + re-read is the right strategy** — but it needs a **content
hash**, not an mtime, and it needs a **rescan backstop**.

**Mitigations to adopt.**

1. **Use a polling watcher as the primary mechanism on macOS and BSD**, not
   fsnotify. A `filepath.WalkDir` + stat-cache scan of 50,000 files takes
   well under a second on an SSD, and it costs **zero file descriptors**.
   Run it every 5–15 s, and diff against a cached (size, mtime, inode)
   triple per file. This is ~150 lines of Go and removes the single
   *definitely-breaks* item in this section.
   - Better still: use the polling watcher as the **universal backstop** on
     every platform, and treat fsnotify only as a *latency optimisation*
     where it is safe (Linux, Windows).
2. **If fsnotify is used on macOS anyway, cap the watched file count** and
   refuse to start above the fd limit — but prefer (1).
3. **Watch directories, never files.** Filter on `Event.Name` and re-read.
4. **Debounce ~200–500 ms**, coalescing by path, then re-read and compare a
   **content hash** (SHA-256 of the bytes, or a cheap 64-bit hash if you
   trust it). Only re-parse on a hash change. mtime alone is unreliable
   (1-second granularity on some filesystems; editors that preserve mtime
   will fool you).
5. **Chmod handling:** on a `Chmod` event, `stat` the path. If it no longer
   exists → treat as a removal. Otherwise → ignore. This one rule absorbs
   both the "REMOVE reported as CHMOD" case and the Spotlight storm.
6. **Add a periodic full rescan** (every 30–60 s) as a correctness backstop,
   and run it unconditionally on startup. This is the only thing that covers
   (a) the network-filesystem case, (b) the macOS fd-exhaustion case, (c)
   any event fsnotify dropped, including the Windows overflow case.
7. **Count directories at startup** and compare against
   `/proc/sys/fs/inotify/max_user_watches` on Linux; if you are within 20%
   of the limit, warn the user. On macOS, count *files* and compare against
   `kern.maxfilesperproc` (via `sysctl`).
8. **Detect the filesystem** (`statfs`) and show a "live reload unavailable
   on this filesystem — reloading every 30 s" notice on NFS/SMB/FUSE.
9. Add a "watcher died" path: if `watcher.Errors` yields an overflow or the
   watcher closes, fall back to rescan-only mode and tell the user.


### 2.4 Obsidian's property system — R6, R9, R13, R22

**The rules.** Obsidian's property model, as of 1.14.4 (2026-10-05), per
the [Obsidian help page](https://obsidian.md/help/properties) and the
[fictty Obsidian data-model research](https://fictty.com/data-model/obsidian/):

> "Properties have types that are **vault-wide by name**: once `due` is a
> Date, every note's `due` is shown and edited as a date. The types are
> Text, List, Number, Checkbox, Date, Date & time and Tags (the last only for
> `tags`). **The type is a hint to the editor and to Bases, not a
> constraint:** a note can still hold `due: soon`, and Obsidian shows it as
> **invalid** rather than refusing it. (The type assignments are kept in the
> vault's config, **`.obsidian/types.json`**; the help page does not
> document the file.) Default properties are `tags`, `aliases` and
> `cssclasses`; the singular `tag`, `alias` and `cssclass` were deprecated
> in 1.4 and dropped as defaults in 1.9."

> "Properties are deliberately flat. The help says **nested properties are
> not supported in the properties editor** (they survive in the YAML and show
> in source mode). **JSON frontmatter is accepted but read and saved back as
> YAML.**"

**What this means for the writer — and it is a design change:**

1. **The writer must be type-aware of `.obsidian/types.json`.** If the user
   registered `due` as *Date* and you write `due: "2026-10-15"` (quoted),
   Obsidian sees a *string* against a *Date* type → **"invalid"**. If you
   write `due: 2026-10-15` (unquoted), Obsidian sees a date → correct. The
   rule is not "emit a semantically equal value" but "**emit the YAML type
   that matches the registered property type**".
2. **Never emit `yes`/`no` for booleans.** Both yaml.v3 (YAML 1.2) and
   js-yaml (YAML 1.2 core) treat `yes`/`no` as **strings**. Obsidian will
   show a text property where the user expected a checkbox. Emit `true` /
   `false`.
3. **Always quote wikilinks in properties.** Measured with yaml.v3:
   `related: [[Note]]` decodes to `[]interface{}{[]interface{}{"Note"}}` — a
   **list containing a list**, not the string `[[Note]]`. Obsidian's own
   example file uses `related: "[[Priya Natarajan]]"`, quoted, "so YAML
   doesn't read it as a list". Same for `[[Note|Alias]]`,
   `[[Note#^blockid]]` and `![[embed.png]]`.
4. **Property names are case-insensitive.** `Book` and `book` are the same
   property to Obsidian. Your index must key on the lowercased name.
5. **Nested frontmatter is a dead end for Obsidian users.** It survives in
   the YAML, but Obsidian's property editor cannot show or edit it. If the
   schema uses nested frontmatter, Obsidian users will be unable to manage it
   in the UI. Keep the frontmatter schema flat.
6. **`tags`/`aliases`/`cssclasses` are list properties** and are special-cased
   by Obsidian. Write them as YAML lists, always. A scalar `tags: job` is
   not what Obsidian expects.
7. **Obsidian never merges a property's type across notes** — the type is
   global, the *value* is per-note and may be invalid. Your app should
   tolerate invalid values rather than reject them, because Obsidian will.

**The yaml.v3 ↔ Obsidian schema mismatch (R13).** Measured:

| Source | yaml.v3 `any` | Obsidian (js-yaml, YAML 1.2 core) |
|---|---|---|
| `yes` / `no` / `on` / `off` | `string` "yes" | `string` "yes" |
| `2026-10-15` | **`time.Time`** | `string` |
| `2026-10-15T10:30:00` | `string` | `string` |
| `017` | `int` 17 (decimal, not octal) | `int` 17 |
| `0o17` | `int` 15 | `int` 15 |
| `1:30` | `string` | `string` |
| `.inf` | `float` | `float` |
| `5` / `"5"` | `int` / `string` | `int` / `string` |

The `time.Time` divergence is the one that bites: the same bytes are a
timestamp in Go and a string in Obsidian. **Compare frontmatter values as
strings (or by YAML tag), never by decoded Go type.** At the `yaml.Node`
level the original text is preserved, so the *bytes* are safe — only the
decoded type diverges.

### 2.5 The frontmatter writer — R8, R15, R21

**yaml.v3's `yaml.Node` round-trip is not byte-identical.** Measured over 40
cases:

| Input | Output | Verdict |
|---|---|---|
| `a: >\n  line1\n  line2\n` (folded) | `a: >\n  line1 line2\n\n` | **Value destroyed** — the line structure is folded and a blank line added |
| `a: 1\r\nb: 2\r\n` | `a: 1\nb: 2\n` | **CRLF → LF** — rewrites every line of a Windows-authored file |
| `a: 1   ` | `a: 1` | Trailing whitespace stripped |
| `a: "\u00e9"` | `a: "é"` | Unicode escapes decoded |
| `["x","x"]` | `["x", "x"]` | Flow-style spacing changed |
| 4-space indent | 2-space | Indent normalised (`SetIndent(2)` is the default; `SetIndent(4)` is the other choice, but you cannot preserve the *original* indent with yaml.v3 — you must detect it) |
| `a:\n    - 1` | `a:\n  - 1` | Sequence indent style changed |
| `---\na: 1\n` | `a: 1\n` | **Document-start marker dropped** |
| `` (empty) | `null\n` | Empty document becomes `null` |
| `derived:\n  <<: *b` | `derived:\n  !!merge <<: *b` | **Semantic change** — an explicit `!!merge` tag appears |
| `a: 1` (no trailing newline) | `a: 1\n` | Newline added |

**Preserved correctly:** comments (head *and* trailing, though the space
before `#` is normalised from two to one), key order, anchors and aliases,
duplicate keys, non-string keys (`1:`, `true:`), literal blocks `|`, quoting
style, single quotes, very long values, and surgical edits — changing
`b: 2  # keep this comment` to `b: 999` in a `yaml.Node` yields
`b: 999 # keep this comment`. **The comment survives the edit.** That is the
good news: the "patch a frontmatter key in place" workflow works.

**The implication.** If the byte-identical contract is meant to cover
frontmatter as well as the body, **a `yaml.Node` re-encode cannot deliver
it.** The frontmatter writer must instead:
- locate the key's line/column span in the original text and **splice** the
  replacement in, or
- restrict itself to a **canonical subset** of YAML and document that the
  writer normalises frontmatter (and accept that a file the app has written
  is no longer byte-identical to a file the user wrote).

The second is much cheaper and is probably fine — but it must be a
*decision*, because the golden-corpus suite in ADR-0021 will otherwise fail
on every CRLF file and every folded scalar.

**Alias bombs (R15).** Measured: the billion-laughs bomb decoded into
`map[string]any` → `yaml: document contains excessive aliasing` (error —
safe). Decoded into `yaml.Node` → **no error**, anchors preserved and
*not expanded* (also safe, as long as you never resolve them). **Never call
`node.Decode(&someMap)` on untrusted frontmatter — walk the `yaml.Node`
tree.** Also note: **yaml.v3 v3.0.1 is required.** `GHSA-hp87-p4gw-j4gq`
(DoS) is fixed in 3.0.1 and `GO-2022-0603` (panic) in
3.0.0-20220521103104. You are on v3.0.1. Good.

**Duplicate keys (R21).** Measured: `a: 1\na: 2` → error when decoded into
a map, **no error** into a `yaml.Node` (and it is re-emitted verbatim).
Obsidian's parser will take the *last* occurrence. If the writer patches
"the" `a`, it may patch the wrong one. **Detect duplicate keys on load and
refuse to patch (or patch all occurrences deliberately).**

---

## 3. Per-topic detail

### 3.1 wazero plugin host

Sources: [wazero docs](https://wazero.io/docs/),
[pkg.go.dev/github.com/tetratelabs/wazero](https://pkg.go.dev/github.com/tetratelabs/wazero),
[pkg.go.dev/.../experimental](https://pkg.go.dev/github.com/tetratelabs/wazero/experimental),
[releases](https://github.com/wazero/wazero/releases),
[PR #2500 (fuel, closed unmerged)](https://github.com/wazero/wazero/pull/2500),
[OSV.dev](https://osv.dev/).

| Question | Answer | Evidence |
|---|---|---|
| How to bound a runaway module? | **Only** `WithCloseOnContextDone(true)` + a context deadline. No fuel, no epoch. | `experimental/` listing across 10 releases; measured 200 ms interrupt |
| What happens on a guest trap? | The call returns `wasm error: …` **with a wasm stack trace**. The module stays alive and a second call re-traps. | Measured: `i32.div_s` by zero → `wasm error: integer divide by zero`; second call identical |
| Can you kill a hot-looping module? | Only with `WithCloseOnContextDone`. `Runtime.Close()` **does not** stop it. | Measured: Close() returned, call still spinning |
| Memory growth per instance | Trivial modules: negligible. Real guests: their declared minimum linear memory. **Default cap is 4 GiB/instance.** | Measured: `grow(1024 pages)` OK by default, fails with `WithMemoryLimitPages(2)` |
| Cost of the JSON ABI | ≈ 5 µs/call (3.0 µs unmarshal + 1.8 µs marshal + 4 ns copy) | Measured |
| WASI threads? | **Not enabled by default.** `experimental.CoreFeaturesThreads` exists but is off; `api.CoreFeaturesV2` has no threads. | `api/features.go` |
| Anything that breaks the sandbox? | WASI **sockets** (`sock_send`/`sock_recv`/`sock_accept`/`sock_shutdown`) are implemented. Safe only because no socket is pre-opened. | `imports/wasi_snapshot_preview1/sock.go`; `experimental/sock` |
| Known CVEs? | **None** in OSV.dev. | OSV query |
| Is the ABI pattern good? | Yes — it *is* Extism's ABI. | `github.com/extism/go-sdk` |

### 3.2 modernc.org/sqlite

Sources: [godoc.org/modernc.org/sqlite](https://godoc.org/modernc.org/sqlite),
[SQLite FTS5 docs](https://sqlite.org/fts5.html),
[the project issue tracker](https://gitlab.com/cznic/sqlite).

**FTS5 is complete — this is not a risk.** Measured on v1.61.0
(`sqlite_version()` = 3.54.1, `ENABLE_FTS5` on): 14/14 features work —
`bm25()`, `highlight()`, `snippet()`, prefix queries, column filters,
`NEAR`, external-content tables, `porter`/`trigram`/`unicode61
remove_diacritics 2` tokenizers, `detail=none`, contentless, `rank` config,
`fts5vocab`. `ENABLE_FTS4`/`FTS3` are off (irrelevant). `ENABLE_JSON1`
shows false but that compile option was **removed from SQLite in 3.38** —
JSON functions are always compiled in now, so this is not a gap.

**The locking pitfall is real and is the whole risk (R4).** Measured
concurrency matrix, 4 goroutines × 100 inserts:

| Configuration | inserted | errors |
|---|---|---|
| default DSN, unlimited pool | 157 | **243 "database is locked" (SQLITE_BUSY)** |
| default DSN, `SetMaxOpenConns(1)` | **400** | **0** |
| `_pragma=busy_timeout(5000)`, unlimited pool | **400** | **0** |
| `_pragma=busy_timeout(5000)`, `SetMaxOpenConns(4)` | **400** | **0** |
| `busy_timeout=5000` + WAL, `SetMaxOpenConns(4)` | **400** | **0** |

`pragma busy_timeout` reads **0** by default. ADR-0030 already says "WAL mode
plus a busy timeout is the configuration" — that is right — but the *other*
half, the pool size, is missing, and modernc's own README warns about it:
"database/sql opens connections without limit by default. Each connection
carries its own page cache and its own libc thread state, and a periodic
query that takes longer than its period piles up without bound. Bound the
pool with `sql.DB.SetMaxOpenConns`."

**Mitigation.** DSN: `file:<path>?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)`.
Then `db.SetMaxOpenConns(1)` for the writer and `SetMaxOpenConns(N)` for a
separate read pool.

**Caveat (measured, and it is a trap):** with `SetMaxOpenConns(1)`, holding
a `*sql.Conn` and then using the `*sql.DB` for a second statement
**deadlocks** — I hit `fatal error: all goroutines are asleep - deadlock!`.
Never mix `db.Conn()` with `db.Exec()` on a one-connection pool. If you need
a dedicated connection, use a separate `*sql.DB` opened on the same file, or
a transaction.

**WAL mode** works: `pragma journal_mode` → `wal`. ADR-0030's concern about
WAL side files growing without bound is real — run `PRAGMA wal_checkpoint(TRUNCATE)`
periodically.

**Performance.** 50,000 documents of ≈4.4 KB inserted in **1.41 s**; a
`MATCH` over all 50,000 in **5 ms**; index file **81 MB** for those
documents (a real vault's notes are much smaller).

**Binary size and cross-compilation — not a risk.** Measured with
`CGO_ENABLED=0 -trimpath -ldflags "-s -w"`, all five targets:

| Target | Size |
|---|---|
| linux/amd64 | 7.6 MB |
| linux/arm64 | 7.3 MB |
| darwin/amd64 | 7.5 MB |
| darwin/arm64 | 7.2 MB |
| windows/amd64 | 7.4 MB |

Without `modernc.org/sqlite`: 3.8 MB. So the transpiled SQLite costs
**~3.8 MB**. ADR-0030's "a few megabytes" is accurate.

**One caveat (R25).** modernc's own SECURITY.md scopes in "transpilation
faults, the bug class unique to a project that ships SQLite's C as generated
Go." No advisories today, but the class exists. Pin the exact
`modernc.org/libc` version modernc pins ("When you import this package you
should use in your go.mod file the exact same version of modernc.org/libc as
seen in the go.mod file of this repository"), keep the DB a rebuildable
cache (ADR-0030 already does), and run `govulncheck` in CI.

### 3.3 Tiptap 3 and `@tiptap/markdown`

Sources: [tiptap/markdown v3.32.0 on npm](https://www.npmjs.com/package/@tiptap/markdown),
[Tiptap issues labelled `area: markdown`](https://github.com/ueberdosis/tiptap/issues?q=is%3Aissue+markdown+beta),
[ADR-0021](https://github.com/ueberdosis/tiptap/issues/8215).

**Live issues in the extension (all 2026, all `area: markdown`):**
[#8215](https://github.com/ueberdosis/tiptap/issues/8215) wrong underline
yield · [#8182](https://github.com/ueberdosis/tiptap/issues/8182) nested
ordered-list hierarchy changed on serialization ·
[#8168](https://github.com/ueberdosis/tiptap/issues/8168) invalid nested
paragraph JSON for unclosed HTML · [#8166](https://github.com/ueberdosis/tiptap/issues/8166)
ordered-list tokenizer swallows `$$` block math via lazy continuation ·
[#8154](https://github.com/ueberdosis/tiptap/issues/8154) table cell `|` not
re-escaped on serialize, **corrupting the table on round-trip** ·
[#8152](https://github.com/ueberdosis/tiptap/issues/8152) U+001F cell
separator leaks into serialized markdown ·
[#8136](https://github.com/ueberdosis/tiptap/issues/8136) CommonMark soft
line breaks become literal `\n` in text nodes ·
[#8134](https://github.com/ueberdosis/tiptap/issues/8134) escaped blocks not
preserved after round-trip · [#8116](https://github.com/ueberdosis/tiptap/issues/8116)
HTML comment inside a heading **crashes** with `RangeError`.

**Maturity of round-tripping arbitrary markdown: it does not exist.** The
measured 1/46 is not a maturity gap waiting to close; it is the consequence
of a lossy parse (see §2.1). The individual bugs above are the *visible*
part; the invisible part is that the *serializer* picks canonical forms for
every construct where the source was non-canonical.

**Alternatives for a WYSIWYG markdown editor with a byte-identical
requirement:**

| Approach | Byte-identical? | Notes |
|---|---|---|
| **Our own ProseMirror schema + parser/serializer** | Only if the parser records source spans and the serializer splices | Very large; you are writing a markdown parser *and* a WYSIWYG editor |
| **CodeMirror 6 + a preview pane** | **Yes, trivially** — the buffer *is* the source | The source of truth is the buffer; edits are diffs. This is what Obsidian's Live Preview and VS Code do. |
| **`remark`/`micromark` + CodeMirror 6, patching by AST position** | **Yes** | micromark keeps `position` (offset) on every node — verified. Compute a minimal splice from the changed block's offset range. This is the "real" implementation of the contract. |
| **Tiptap + `@tiptap/markdown`** | **No** (1/46) | Plus a pre-scanner is needed for the raw-node escape hatch to work |
| **Tiptap with a ProseMirror `prosemirror-markdown` serializer** | No | `prosemirror-markdown` is a *serializer*, not a round-tripper; it also canonicalizes. |
| **CodeMirror 6 + Tiptap as a *preview* only** | Yes | WYSIWYG look without a WYSIWYG editor. Loses inline editing in the rich view. |

**Recommendation: CodeMirror 6 + `remark`/`micromark` (position-aware) +
Tiptap as an optional inline preview**, and keep Tiptap as the editor *for
structured blocks* (character sheets, encounters) where the document genuinely
is a tree. That satisfies ADR-0021's contract, keeps ADR-0021's
"structured content is first-class", and drops the `@tiptap/markdown` beta
dependency — which ADR-0021 already says "the core must not depend on its
internals, so the editor plugin must be able to swap it out."

**ProseMirror document size.** No hard limit, but ProseMirror is a single
`contenteditable` and does not virtualize. Comfortable editing is roughly
≤10k words; a 50k-word document will be visibly slow to render, scroll, and
map, and `doc.descendants` over a large doc is O(n) per transaction. The
project's corpus is 1,692 real files of note-sized prose — fine. **Cap the
rich editor at ~50k words and fall back to the source view beyond that.**

### 3.4 Shadow DOM in Safari and Firefox

Source: [MDN browser-compat-data v8.1.6](https://github.com/mdn/browser-compat-data)
(Sept 2026 browser data). The project's
[shadow-dom-verification.md](shadow-dom-verification.md) was run in
**Chromium 156 only** — that is the gap.

| Feature | Chrome | Edge | Firefox | Safari | iOS Safari | Verdict |
|---|---|---|---|---|---|---|
| `ShadowRoot.getSelection()` | 53 | 79 | **false** | **false** | **false** | **Chromium-only. Hard difference.** |
| `Document.getSelection()` | 2 | 12 | 57 | 4 | 3.2 | Fine (FF ≥ 57) |
| `Window.getSelection()` | 1 | 12 | 1 | 1 | 1 | Universal |
| `ShadowRoot.adoptedStyleSheets` | 73 | 79 | 101 | 16.4 | 16.4 | Safari 16.4+ / FF 101+ |
| `Document.adoptedStyleSheets` | 73 | 79 | 101 | 16.4 | 16.4 | Same |
| `Document.adoptedStyleSheets.mutable` | 99 | 99 | 101 | 16.4 | 16.4 | Same |
| `CSSStyleSheet` constructor | 73 | 79 | 101 | 16.4 | 16.4 | Same |
| `CSSStyleSheet` `options.baseURL` | **false** | **false** | 101 | 16.4 | 16.4 | Chrome lacks it |
| `::part()` | 73 | 79 | 72 | 13.1 | 13.4 | **Universal** |
| `ShadowRoot.mode` (closed) | 53 | 79 | 63 | 10.1 | 10.3 | **Universal** |
| `Element.attachShadow` | 53 | 79 | 63 | 10 | 10 | **Universal** |
| `attachShadow({delegatesFocus:true})` | 53 | 79 | 94 | 13.1 | 13.4 | FF 94+, Safari 13.1+ |
| `ShadowRoot.delegatesFocus` | 53 | 79 | 94 | 15 | 15 | **Safari 15+** |
| `ElementInternals` | 77 | 79 | 93 | 16.4 | 16.4 | **Safari 16.4+** |
| `ElementInternals.setFormValue` / `setValidity` | 77 | 79 | 98 | 16.4 | 16.4 | **Safari 16.4+** |
| `attachShadow({slotAssignment:'manual'})` | 86 | 86 | 92 | 16.4 | 16.4 | **Safari 16.4+** |
| `HTMLSlotElement.assign` | 92 | 92 | 92 | 16.4 | 16.4 | **Safari 16.4+** |
| `HTMLSlotElement.assignedElements` | 65 | 79 | 66 | 12.1 | 12.2 | Universal |
| `Element.shadowRoot` | 35 | 79 | 63 | 10 | 10 | Universal |

**What actually differs from Chromium, concretely:**

1. **`ShadowRoot.getSelection()` does not exist in Firefox or Safari.** This
   is the one that will bite a plugin surface that wants to read the user's
   selection inside its shadow root.
   **Mitigation:** never call it. Use `window.getSelection()` (universal) and
   determine the root via
   `sel.getRangeAt(0).commonAncestorContainer.getRootNode()`. Fall back to
   `document.getSelection()` where `getRootNode()` is unavailable. In
   Firefox/Safari `window.getSelection()` returns a selection whose anchor may
   be the shadow *host* rather than the inner element — so resolve through
   `shadowRoot.activeElement` or `composedPath()`.
2. **Constructable stylesheets need Safari 16.4 / Firefox 101.** ADR-0021
   records that "CodeMirror 6 … detects the shadow root and adopts its theme
   via `adoptedStyleSheets`". That is a **Safari 16.4+ / Firefox 101+**
   requirement for the source view. Older Safari will fall back to
   `document.head` injection (which does not reach a shadow root) or to
   nothing.
   **Mitigation:** feature-detect
   `'adoptedStyleSheets' in Document.prototype &&
    'adoptedStyleSheets' in ShadowRoot.prototype`, and always ship a
   `<style>`-element fallback inside the shadow root. Note also that
   `CSSStyleSheet`'s `options.baseURL` is **unsupported in Chromium**, so
   never depend on it.
3. **`delegatesFocus` needs Firefox 94 and `ShadowRoot.delegatesFocus` needs
   Safari 15.** Both are old enough to be safe in 2026, but a plugin that
   reads `shadowRoot.delegatesFocus` must feature-detect.
4. **`ElementInternals` (form-associated custom elements) needs Safari
   16.4.** If a plugin surface ships a form control, the Safari 16.4 floor
   is a hard product constraint. **Mitigation:** implement form behaviour in
   light DOM, or feature-detect
   `Object.getOwnPropertyDescriptor(customElements.get('x'), 'formAssociated')`.
5. **Manual slot assignment needs Safari 16.4.** Use default (named) slots
   if you need to support older Safari.

**What is universal and can be relied on:** `::part()`, closed shadow roots,
`attachShadow`, `assignedElements`, `element.shadowRoot` (for open roots),
and CSS custom-property inheritance (which is the ADR-0029 theming
mechanism). Note: **`::part()` works even through a closed root** — that is
its purpose — but `element.shadowRoot` is `null` for a closed root, so the
app cannot *script* into it. **Use open roots if the app is ever expected to
reach into a plugin surface programmatically.**

**What a cross-browser plugin surface should avoid:** `shadowRoot.getSelection()`;
relying on `adoptedStyleSheets` without a `<style>` fallback; relying on
`document.head` CSS reaching into the shadow root (it never does — this is
already documented in ADR-0021 and is why Tiptap needs `injectCSS: false`);
`ElementInternals` form association without a fallback; manual slot
assignment; and `event.target` instead of `event.composedPath()[0]`.

### 3.5 Obsidian compatibility — YAML frontmatter

See §2.4 and §2.5 for the full findings. Sources:
[obsidian.md/help/properties](https://obsidian.md/help/properties),
[fictty: Obsidian the data model](https://fictty.com/data-model/obsidian/)
(quoting the help page, Obsidian 1.14.4, 2026-10-05),
[obsidian-conditional-properties README](https://github.com/diegoeis/obsidian-conditional-properties)
(a plugin author who reverse-engineered the typed-property behaviour).

**The exact rules, as best they are documented:**

- Six property types: **Text, List, Number, Checkbox, Date, Date & Time**,
  plus a special **Tags** type for `tags` only.
- **Types are vault-wide by name**, stored in `.obsidian/types.json`. Once
  `due` is a Date, *every* note's `due` is edited as a date.
- **The type is a hint, not a constraint.** A note may still hold
  `due: soon`; Obsidian shows it as invalid rather than refusing it.
- The **YAML value must carry the right native type** for the widget to
  render: boolean for checkbox, ISO date (`YYYY-MM-DD`) for date, ISO
  datetime (`YYYY-MM-DDTHH:mm:ss`) for datetime. "Strings won't trigger the
  widgets, even if the property is registered with the right type."
- Property **names are case-insensitive**; `Book` and `book` are one
  property.
- Default properties are `tags`, `aliases`, `cssclasses` (plural). The
  singular forms were deprecated in 1.4 and dropped as defaults in 1.9.
- **Nested properties are not supported in the properties editor** — they
  survive in the YAML and appear only in source mode.
- **JSON frontmatter is accepted but read and saved back as YAML.**
- Obsidian's **date parsing** follows the user's configured date format
  (Daily Notes / Templates plugin formats, then common civilian formats), so
  `08-08-2025` may be a date and `2025-08-08` always is.
- A **wikilink in a property must be quoted** so YAML does not read it as a
  list.

**yaml.Node round-tripping quirks — measured, 40 cases.** See the table in
§2.5. Summary: comments, key order, anchors/aliases, duplicate keys,
non-string keys, literal blocks and quoting style are preserved; **folded
scalars, CRLF, indentation, the `---` marker, empty documents, and merge keys
are not.**

**Features Obsidian supports that a `yaml.Node` round-trip would mangle:**
the `---` document-start marker (dropped), the folded `>` block style
(value corrupted), the `!!merge` key (gains an explicit tag), CRLF line
endings (normalised), and 4-space indentation (normalised to 2). Plus
anything Obsidian-specific that yaml.v3 has no opinion about: `%%comments%%`
are in the *body*, not the frontmatter, so they are fine.

**And a `yaml.Node` round-trip will not mangle, to be fair:** anchor/alias
pairs (preserved verbatim, and *not* expanded — which is also the bomb
protection), duplicate keys (preserved — but see R21), non-string keys
(`1:`, `true:` — preserved), multi-line literal blocks `|` (preserved
exactly), and long values (preserved).

### 3.6 Markdown dialects

**CommonMark vs GFM vs Obsidian:**

| Feature | CommonMark | GFM | Obsidian |
|---|---|---|---|
| Tables with alignment | no | yes | yes |
| Strikethrough `~~x~~` | no | yes | yes |
| Task lists | no | yes | yes |
| Autolinks (bare URL) | no | yes | yes |
| Footnotes | no | no | yes (its own syntax) |
| Wikilinks `[[x]]` | no | no | **yes** |
| Embeds `![[x]]` | no | no | **yes** |
| Callouts `> [!note]` | no | no | **yes** |
| Block refs `^id` | no | no | **yes** |
| `%%comments%%` | no | no | **yes** |
| YAML frontmatter | no | no | **yes** (and Obsidian's own) |
| Dataview / Bases blocks | no | no | yes (plugin) |

Obsidian's dialect is **CommonMark + GFM + a substantial Obsidian
extension**. No library implements it; the project will be writing those
pieces regardless.

**Where they differ in ways that break a byte-identical round-trip** (all
measured — see §2.1's table and the remark results): setext headings,
reference links, table alignment and cell padding, HTML blocks, autolinks,
list markers (`*`/`-`, `1.`/`1)`), hard line breaks (two spaces vs
backslash), backslash escapes, entity references, thematic-break style,
emphasis delimiter style (`_` vs `*`), indented vs fenced code, blank-line
runs, and trailing whitespace.

**The CommonMark "ambiguous constructs"** are the formal reason this is
impossible. The spec's *Ambiguity* section and the emphasis rules
(left-flanking / right-flanking delimiter runs) mean multiple source texts
map to one AST. A serializer therefore cannot know which one it was given.

**Is there a reference implementation that round-trips CommonMark exactly?**
**No.**
- [`cmark`](https://github.com/commonmark/cmark) (the C reference) has a
  `--to commonmark` renderer, but its round-trip test asserts *semantic*
  equivalence (re-parse → same AST), not byte identity.
- [`commonmark.js`](https://github.com/commonmark/commonmark.js) (the JS
  port) has `HtmlRenderer` and `XmlRenderer` only — **no markdown
  serializer** (confirmed: its exports are `HtmlRenderer, Node, Parser,
  Renderer, XmlRenderer`).
- [`markdown-it`](https://github.com/markdown-it/markdown-it) is an HTML
  renderer.
- [`marked`](https://github.com/markedjs/marked) is an HTML renderer.
- [`remark`](https://github.com/remarkjs/remark) / `micromark` is the only
  JS stack with a real markdown serializer — and it is 8/21 on the hard
  cases.

**Therefore: the only way to satisfy the contract is to not re-serialize.**
Keep the source and splice. This is the finding that makes option A/B in
§2.1 the only viable paths.

### 3.7 fsnotify

See §2.3 for the full findings and mitigations. Sources:
[fsnotify README v1.10.1](https://github.com/fsnotify/fsnotify/blob/main/README.md),
[`backend_kqueue.go`](https://github.com/fsnotify/fsnotify/blob/main/backend_kqueue.go),
[`backend_inotify.go`](https://github.com/fsnotify/fsnotify/blob/main/backend_inotify.go),
[`backend_windows.go`](https://github.com/fsnotify/fsnotify/blob/main/backend_windows.go),
[recursive watch issue #18](https://github.com/fsnotify/fsnotify/issues/18),
[polling watcher issue #9](https://github.com/fsnotify/fsnotify/issues/9),
[FSEvents issue #11](https://github.com/fsnotify/fsnotify/issues/11).

**Per-platform cost, from the source (not the README):**

| Platform | Backend | Cost | Safe at 50k notes? |
|---|---|---|---|
| Linux | inotify | 1 watch per **directory** | **Yes** (121 watches) |
| Windows | ReadDirectoryChangesW | 1 handle per **directory** | **Yes** (121 handles), but events are lost on buffer overflow (`ErrEventOverflow`) |
| macOS, BSD | kqueue | **1 fd per FILE** | **No** — 50,000 fds, `EMFILE` |

**Is debounce + re-read sufficient?** Yes, with three additions:
(1) compare a **content hash**, not an mtime; (2) reconcile `Chmod` by
`stat`ing the path; (3) **add a periodic full rescan** as a backstop for
missed events, network filesystems, the Windows overflow, and the macOS fd
limit.

**The current recommended approach for "watch a directory other programs
also write":** watch the *directories*, debounce, re-read, hash-compare,
reconcile removals by stat, and **run a periodic rescan**. The rescan is
the part the README cannot give you (because fsnotify has no polling
watcher) and the part that makes the whole thing robust. On macOS, make the
rescan the *primary* mechanism and skip fsnotify entirely.

### 3.8 Go plugin API versioning

**Established practice, across the Go ecosystem:**

- **[HashiCorp `go-plugin`](https://github.com/hashicorp/go-plugin)** (Vault,
  Terraform, Nomad, Packer, Boundary, Waypoint — "used on millions of
  machines"): a **handshake**. The client sends a `CoreProtocolVersion`; the
  server replies with its own `CoreProtocolVersion` and an
  `AppProtocolVersion`; a mismatch produces a *human-friendly error*, not a
  crash. The README: "**Protocol Versioning.** A very basic 'protocol
  version' is supported that can be incremented to invalidate any previous
  plugins. This is useful when interface signatures are changing, protocol
  level changes are necessary, etc. When a protocol version is incompatible,
  a human friendly error message is shown to the end user." Its roadmap adds:
  "**Semantic Versioning.** Plugins will be able to implement a semantic
  version. This plugin system will give host processes a system for
  constraining versions."
- **[Extism](https://github.com/extism/go-sdk)**: the *manifest* carries a
  `version`; the runtime checks it; the PDK ABI (`extism_alloc`,
  `extism_input_load`, `extism_output_set`, `extism_var_get`, …) is frozen.
  The Go SDK embeds `extism-runtime.wasm` plus a
  `extism-runtime.wasm.version` file, and pins a wazero version.
- **[Envoy `proxy-wasm`](https://github.com/proxy-wasm/spec)**: an explicit
  `proxy_abi_version` in the host ABI struct; the host rejects unknown
  versions.
- **[Wasmtime component model / WIT](https://component-model.bytecodealliance.org/)**:
  versioned interface definitions, but Rust + cgo.
- **[AWS Lambda Runtime API](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-api.html)**:
  a versioned interface.

**For a WASM plugin host, the "Go module version" problem does not exist** —
the ABI is a data contract, not a Go package, and it is frozen for the life
of the wasm module. What you need is a **version negotiation on the ABI**,
in the go-plugin style.

**Concrete scheme to adopt:**

1. **The guest declares its ABI version.** Add an exported function
   `demitable_abi_version() -> i32` to the guest's required exports, and
   check it *before* any other call. (Or put it in the first request
   envelope; an export is cleaner because it works before the guest has
   allocated anything.)
2. **The host declares a supported range** — e.g. `1 <= v <= 2` — and
   **dispatches on the guest's version**, never on its own. Code paths for
   v1 and v2 coexist.
3. **Separate the ABI version from the data version.** The envelope carries
   its own version: `{"v":1,"op":"vault.read","…":…}`. You can change the
   *payload* without changing the *ABI*.
4. **Never change an existing host function's signature.** Add
   `vault.read_v2` rather than changing `vault.read`. Add new capabilities
   rather than repurposing old ones. wazero's host-module ABI makes this
   easy — a host module is just a set of exported functions.
5. **Make the JSON envelope forward-compatible by construction:** ignore
   unknown fields; reject unknown `op` with a clear error; never change the
   meaning of an existing field.
6. **Fail loudly and legibly.** "Plugin `foo` requires ABI 3; this build
   supports 1–2. Update the app." — the go-plugin lesson exactly.
7. **Put the version in `plugin.yaml`** (`abi: 1`) so it is greppable and
   visible in a plugin directory, and mirror it in the guest export. Then
   the host can reject a mismatched plugin *before* loading the wasm.
8. **Say in `docs/plugin-api.md` that it is provisional** (it already is)
   and make the versioning policy part of it: "ABI v1 is frozen; changes
   within v1 are additive only; a new major means a new manifest field."

**One more thing:** the host's *own* host-function set is the real API
surface, and it is what will drift. The go-plugin discipline — "never change
a signature, add a new one" — plus a **golden ABI test** (a wasm module
that imports every host function the host offers, built at compile time)
will catch accidental breakage at CI time. That test is cheap and worth
having.

### 3.9 Scale

**Git on a 50,000-note + 5,000-attachment vault — measured, not a risk.**
Synthetic vault: 50,000 markdown files across 121 directories, 5,000 × 4 KB
random attachments, 215 MB total.

| Operation | Time |
|---|---|
| `git add -A` (initial) | 4.9 s |
| `git commit` | 0.30 s |
| `git status --porcelain` | 0.08 s |
| `.git` after `git gc` | 51 MB (215 MB worktree) |

Git is **not** the scale problem. (A cold clone of 50k files is slow, and
`git gc` on 51 MB is not instant, but that is the user's git, not the app.)

**Obsidian at 50,000 notes — this is the weak link.** Community reports put
Obsidian's comfortable ceiling around **10,000–12,000 notes**: "When there
are 12,000 notes in Obsidian, daily operations can feel sluggish. Most
functions except Graph View can work normally"
([Obsidian forum](https://forum.obsidian.md/t/maximum-number-of-notes-in-vault/1509)).
Obsidian's metadata cache lives in IndexedDB and is rebuilt from scratch on a
cold start. **Implication: the app may be fine at 50k while Obsidian is not.
"Obsidian-compatible" is a format promise, not a scale promise.** Do not
design the product around Obsidian keeping up. Also note the project's own
golden corpus is 1,692 files — 50k is a stress case, not the design point.

**One Go process holding the whole index — measured, not a risk.**

| Measurement | Value |
|---|---|
| 50,000 notes walked, read and YAML-parsed | **2.2 s** |
| Body text total | 114 MB |
| Metadata index (path/name/uid/links maps + frontmatter + aliases + tags) | **13.3 MB (266 bytes/note)** |
| Same, with all bodies in memory | **166 MB** |

**The metadata index is trivially comfortable.** The bodies are not — 166 MB
of live heap for a background process is a lot on a laptop. **This validates
ADR-0030's split exactly: metadata in memory (13 MB), bodies on demand, FTS5
on disk.** Do not keep bodies in memory. The 2.2 s cold index build is also
fine, and the FTS5 index means you do not rebuild it on every launch.

**The memory profile to expect.** Per note, budget:
- path string ~ 60–120 B
- name + lowercased name ~ 120 B
- uid string ~ 40 B
- frontmatter `map[string]any` ~ 300–800 B depending on key count
- aliases/tags slices ~ 100–300 B
- link/backlink slices ~ 100–300 B
- map overhead ~ 100 B/note

**≈ 250–400 bytes/note** → **13–20 MB for 50,000 notes.** If you cache
parsed frontmatter as `yaml.Node` instead of `map[string]any`, expect
**3–5×** that — yaml.Node is a heavy struct. Prefer `map[string]any` or a
purpose-built struct, and keep the *raw* frontmatter bytes only if you need
byte-identity (another ~200 B/note).

**FTS5 at scale — not a risk.** 50k docs in 1.41 s, MATCH in 5 ms.

**The real scale wall is the watcher (R5/R31/R32), not memory, not git,
not SQLite.**

### 3.10 YAML and markdown security

**What a hostile vault file can do, and what you should defend against:**

| Vector | Status | Mitigation |
|---|---|---|
| **YAML billion laughs / anchor bomb** | **Already handled by yaml.v3 v3.0.1** — measured: `yaml: document contains excessive aliasing` on the decode path. But the `yaml.Node` path does **not** error (it preserves anchors unexpanded). | **Never `node.Decode(&map)` on untrusted frontmatter.** Walk the `yaml.Node` tree. Require yaml.v3 ≥ 3.0.1 (`GHSA-hp87-p4gw-j4gq`, `GO-2022-0603`). |
| **Deeply nested YAML** | Measured: yaml.v3 parsed **5,000 levels** without error or stack overflow. Decoding that into `any` recurses. | Cap frontmatter depth (e.g. 32) and node count when walking the tree. |
| **Very long lines / huge scalars** | Measured: a 5 MB single-line scalar parsed in **108 ms**, no error. Legal YAML. | Cap the **file** size you will read (e.g. 8 MB/note) and the **frontmatter block** size (e.g. 1 MB). A 2 GB file in the vault must not OOM you. |
| **Many aliases / many nodes** | A 50k-key frontmatter is legal and slow. | Cap the frontmatter key count (e.g. 10,000). |
| **Duplicate keys** | yaml.v3 errors on the map path, **silently accepts** on the Node path. Obsidian takes the last. | Detect duplicates on load; refuse to patch, or patch all. |
| **Catastrophic backtracking in *Go*** | **Not a risk** — Go's `regexp` is RE2, which is linear-time and cannot backtrack. | Keep using `regexp`, never a backtracking engine. |
| **Catastrophic backtracking in the *browser*** | **This is the real risk.** JS regexes backtrack. A note containing thousands of unbalanced `(` or `[` can hang the editor. | In the browser: linear-time patterns only (no nested quantifiers), and cap the input length passed to any regex. Prefer hand-written scanners for wikilink / embed / block-ref parsing — they are simple enough not to need a regex at all. |
| **Link-resolution regex** | If you resolve `[[wikilinks]]` with a regex like `\[\[([^\]]*)\]\]` it is linear and fine. The killers are greedy `.*?` with nested groups, and *building* a regex from vault content. | Hand-written scanner. Anchor every pattern. **Never construct a regex from vault content.** |
| **Path traversal / symlink escape** | The *import* suite covers zip slip, but the **asset endpoint** and the **watcher** need their own confinement. | Resolve every served path with `filepath.EvalSymlinks` and verify it is still under the vault root. Never follow a symlink out of the vault when serving. |
| **Huge inline base64 in a note** | Legal, and a memory hazard. | The asset endpoint must stream, not buffer. Cap the body you read into memory. |
| **A note with 10,000 wikilinks** | The link graph is O(links) and O(links²) for resolution if you are careless. | Index links by target name in a map; resolution is a map lookup, not a scan. Cap links per note (e.g. 5,000). |
| **A pathological *formula* or *dice* expression** | Your own engine, but it is attacker-reachable if a plugin or a pack can supply one. | The formula engine must have a **step budget** and a **recursion depth limit**, and must not allocate unboundedly. (Not researched here — flag it.) |

**The general rule:** a vault is *untrusted input*. It is a folder the user
may have downloaded, synced from a share, or received from a collaborator.
Every parser in the pipeline should have a size, depth and time budget, and
every failure should degrade to "this note is not indexed" rather than "the
server died".

---

## 4. Where the current design is fine — stop worrying

1. **modernc.org/sqlite's FTS5.** Complete and correct. 14/14 features
   verified working (bm25, highlight, snippet, prefix, column filter, NEAR,
   external content, porter/trigram/unicode61, detail=none, contentless,
   rank, fts5vocab). Not a risk.
2. **Cross-compilation and binary size.** All five targets build with
   `CGO_ENABLED=0`; 7.2–7.6 MB. The transpiled SQLite costs ~3.8 MB.
   ADR-0030's "a few megabytes" is accurate.
3. **Git at 50,000 files.** 0.08 s `status`, 0.30 s `commit`. Not a risk.
4. **The in-memory index at 50,000 notes.** 13 MB of metadata, built in
   2.2 s. Not a risk — and it validates ADR-0030's split.
5. **The JSON-over-WASM ABI cost.** ≈ 5 µs per call. Not a risk unless a
   plugin calls the host in a tight loop.
6. **wazero's sandbox defaults.** No threads, no exception handling, no tail
   calls, no typed references (all `experimental`, off by default); no
   filesystem (`FSConfig` gates it, default denies); no clock (`ModuleConfig`
   gives a *fake* wall clock by default — you must call `WithSysWalltime` to
   get a real one); deterministic `rand` by default. The sandbox assumption
   holds.
7. **wazero CVEs.** None in OSV.dev.
8. **yaml.v3's alias bomb.** Guarded in v3.0.1 (which you have).
9. **`::part()`, closed shadow roots, `attachShadow`, `assignedElements`.**
   Universal support. Not a risk.
10. **The Extism-shaped plugin ABI.** A known-good production pattern
    (Extism, Dapr, Envoy wasm filters, Cloudflare Workers all use
    variations). ADR-0020's decision to implement the ABI rather than
    depend on Extism is correct.
11. **The `yaml.Node` surgical-edit workflow.** Comments, key order and
    quoting survive a targeted edit. Editing one frontmatter key and
    re-emitting preserves the rest — measured.
12. **ProseMirror document size for note-sized documents.** The corpus is
    1,692 real files of prose. Fine.
13. **WAL mode in modernc.** Works; `journal_mode=wal` confirmed.
14. **The `io/fs` + `net/http` + `coder/websocket` choices.** Standard, no
    surprises found.
15. **Go's `regexp` cannot catastrophically backtrack.** The regex risk is
    in the frontend, not the backend.

---

## 5. Recommended action list, in order

| # | Action | Effort | Why |
|---|---|---|---|
| 1 | **Decide the editor architecture** (§2.1). Recommended: CodeMirror 6 + `remark`/`micromark` position-aware splicing for prose; Tiptap retained for structured blocks. | S | Everything about the editor depends on this. |
| 2 | **Add `WithCloseOnContextDone(true)` and a per-call deadline to the wazero host; add `WithMemoryLimitPages`; fix ADR-0020's socket claim.** | S | R3, R10, R11. A runaway plugin is a hung server. |
| 3 | **Set the SQLite DSN pragmas and `SetMaxOpenConns(1)` on the writer.** | S | R4. 60% of concurrent writes fail without it. |
| 4 | **Add the plugin ABI version handshake** (§3.8): a `demitable_abi_version` export, a host-side range, and a `abi:` field in `plugin.yaml`. Plus a golden ABI test. | S | R14. Cheap now, painful later. |
| 5 | **Make the vault writer type-aware of `.obsidian/types.json`** (§2.4): emit booleans as `true`/`false`, quote wikilinks, keep frontmatter flat, key property names case-insensitively. | M | R6, R9, R22. |
| 6 | **Decide the frontmatter byte-identity policy** (§2.5): either a splice-based frontmatter writer, or a documented canonicalisation. Then write the golden-corpus fixtures accordingly. | M | R8. Otherwise the corpus suite fails on every CRLF file. |
| 7 | **Rewrite the watcher** (§2.3): a polling watcher as the primary mechanism on macOS/BSD (zero fds) and the universal backstop everywhere; fsnotify only as a latency optimisation on Linux/Windows. Watch dirs → debounce → re-read → hash-compare → stat-reconcile Chmod → periodic full rescan. | M | R5, R18. fsnotify's kqueue backend is one fd per *file* — 50k notes = 50k fds = `EMFILE` on macOS. **Will definitely break.** |
| 8 | **Add Firefox and WebKit to the Playwright matrix** and re-run the shadow-DOM verification. Fix any use of `shadowRoot.getSelection()`. | M | R7, R16, R17. The verification is Chromium-only. |
| 9 | **Add a front-matter/parser budget**: max file size, max frontmatter size, max depth, max key count; never `Decode` untrusted YAML into a Go value. | S | §3.10. |
| 10 | **Add `govulncheck` to CI** and pin `modernc.org/libc` to modernc's version. | S | R25, yaml.v3 ≥ 3.0.1. |

---

## 6. Sources

**wazero** — [docs](https://wazero.io/docs/) ·
[pkg.go.dev](https://pkg.go.dev/github.com/tetratelabs/wazero) ·
[`experimental` package](https://pkg.go.dev/github.com/tetratelabs/wazero/experimental) ·
[releases](https://github.com/wazero/wazero/releases) ·
[PR #2500 (fuel, closed unmerged)](https://github.com/wazero/wazero/pull/2500) ·
[RATIONALE.md](https://github.com/wazero/wazero/blob/main/RATIONALE.md) ·
[OSV.dev](https://osv.dev/)

**modernc.org/sqlite** — [godoc](https://godoc.org/modernc.org/sqlite) ·
[SQLite FTS5](https://sqlite.org/fts5.html) ·
[issue tracker](https://gitlab.com/cznic/sqlite) ·
[SECURITY.md](https://gitlab.com/cznic/sqlite/-/blob/master/SECURITY.md)

**Tiptap** — [`@tiptap/markdown` v3.32.0](https://www.npmjs.com/package/@tiptap/markdown) ·
[markdown-labelled issues](https://github.com/ueberdosis/tiptap/issues?q=is%3Aissue+markdown+beta)

**Markdown** — [CommonMark spec](https://spec.commonmark.org/) ·
[cmark](https://github.com/commonmark/cmark) ·
[commonmark.js](https://github.com/commonmark/commonmark.js) ·
[markdown-it](https://github.com/markdown-it/markdown-it) ·
[marked](https://github.com/markedjs/marked) ·
[remark / micromark](https://github.com/remarkjs/remark)

**Shadow DOM** — [MDN browser-compat-data](https://github.com/mdn/browser-compat-data) ·
[caniuse: shadowdom](https://caniuse.com/shadowdom) ·
[caniuse: declarative-shadow-dom](https://caniuse.com/declarative-shadow-dom)

**Obsidian** — [help: properties](https://obsidian.md/help/properties) ·
[help: data storage](https://obsidian.md/help/data-storage) ·
[help: links](https://obsidian.md/help/links) ·
[fictty: Obsidian the data model](https://fictty.com/data-model/obsidian/) ·
[obsidian-conditional-properties](https://github.com/diegoeis/obsidian-conditional-properties) ·
[forum: maximum number of notes](https://forum.obsidian.md/t/maximum-number-of-notes-in-vault/1509)

**fsnotify** — [README v1.10.1](https://github.com/fsnotify/fsnotify/blob/main/README.md) ·
[#18 recursive watch](https://github.com/fsnotify/fsnotify/issues/18) ·
[#9 polling watcher](https://github.com/fsnotify/fsnotify/issues/9) ·
[#11 FSEvents](https://github.com/fsnotify/fsnotify/issues/11)

**Go plugin versioning** — [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) ·
[extism/go-sdk](https://github.com/extism/go-sdk) ·
[proxy-wasm spec](https://github.com/proxy-wasm/spec) ·
[Bytecode Alliance component model](https://component-model.bytecodealliance.org/)

**Security** — [OSV.dev](https://osv.dev/) ·
[GHSA-hp87-p4gw-j4gq (yaml.v3 DoS)](https://osv.dev/GHSA-hp87-p4gw-j4gq) ·
[GO-2022-0603 (yaml.v3 panic)](https://osv.dev/GO-2022-0603)
