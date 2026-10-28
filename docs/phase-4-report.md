# Phase 4 — The Table: what it found

> Six milestones, fifteen sub-milestones, all landed. The phase put bodies
> around the table: sessions, seats, the encounter runner, the tabletop,
> chat, the write-back — plus the webapp completion the phase-3 demo
> proved missing.
>
> It holds itself to the same standard as the three reports before it:
> **a phase that found nothing is a phase that did not test anything.**

---

## The phase in one paragraph

A GM runs a session from the webapp — players join by code, sit as
characters, move tokens on a fogged map, roll dice into a shared log,
chat in notes, and closing the session writes back character changes,
an encounter record, a session log and discovery updates, each
revertible. Nothing against the plan is missing.

The phase found **nineteen things**, seven of them blocking defects no
worker reported — two live visibility leaks (upload arbitrary-write,
fog impersonation + raw state), a nondeterministic write-back digest,
a discovery-corrupting type coercion, a close path that silently
dropped every touched key, a display widening without an ADR, and
tripled fog logic. All seven were found by reading the diff and
running the code. None was in a worker's report.

---

## The six milestones

| | Milestone | The number that says it works |
|---|---|---|
| **F0** | server close-out, auth hardening, endpoints, audit, docs | join→set-passphrase→login round-trips; storm 10×200+20×429; asset 404/0-bytes; audit gmnote 2/1/2/2, pubnote 0 |
| **F1** | sessions, seats, codes, scratch log | 32/32 tests; 1–10 wrong → 401+count, 11th → 429; oracle set+order+count unchanged |
| **F2a** | cadence measurement | 12 shapes, p95s recorded, committed bounds, decision recorded as recommendation |
| **F2b** | encounter runner (server-side) | 5e budget 32, Blades opposition 2 + clocks; fog disagree per principal; private rolls silent |
| **F3** | chat as notes, shared display | audience-exact chat; display party-only (3 cases); 3 reveal targets, journal count 3 |
| **F4** | write-back contract | N=2/M=1 conflicts; preview==apply digest; batch revert byte-identical; close idempotent; post-close 7/7 error-JSON |
| **F5** | webapp completion | GM 21 / anon 1, one binary; search + snippets; preview hostile bytes; 409→rebase→save→409; both-pack round-trip |
| **F2c** | tabletop canvas + validated movement | own move 200, others 403, hidden 403; display bound-to-session; drag re-resolved; webkit 15/15 |

`make lint` clean; `go test ./...` 23 packages ok, 1 FAIL — the
pre-existing `TestInsertPerformance` load flake (see Findings §16 —
untouched file, fails only under parallel load, passes in isolation,
fails identically on pre-phase-4 main). No `fixtures/` committed. No
unexplained `dev/` churn. No note content, message body, or credential
in any new log line. No `docs/decisions/` edits by workers (two ADRs
landed by the orchestrator).

---

## The findings

### 1. The upload endpoint was an authenticated arbitrary-write primitive

F0b's asset upload took `?path=`/`?name=`/`?filename=`, Cleaned it for
`.`/`..`/absolute, then `os.WriteFile`'d anywhere:
`?path=.demitable/pwned.png` → 200 on disk;
`?path=Notes/evil.md` → 200 note created/overwritten; PNG bytes pass
the type gate regardless of extension, so `.demitable/accounts.json`
was overwritable by any player — full auth-DB compromise. The fix
confines uploads under `Assets/` (server-generated basenames, prefix
check on the joined path, no-overwrite of notes or `.demitable/`),
routes through a journaled `WriteAsset` seam + reindex, and is pinned
by attack-path tests asserting rejection AND absence from disk
(including `Assets/../.demitable/evil.png` and encoded variants).

### 2. Zero committed tests is a finding, not a gap

F0b's first delivery added 6 files and 0 test files: with limiter +
caps + `allowAll` + join-token binding all removed simultaneously, the
suite still passed. F1a/b's first delivery did the same (5 files, 0
tests). F2b's first delivery: 1,252 lines, 0 tests. F3's: ~600 lines,
0 tests. F5's: 12 files, 0 tests. F4's: 711 lines, 0 tests. In every
case the builder's report claimed green predicates. The verification
standard (cold runs + mutation probes + seam re-checks) caught all six.
**A predicate measured once by hand and then described in a report is
not a predicate** — phase 1's finding 14, six times in one phase.

### 3. The session-code limiter counted two different things

F1's wrong-code response reported the in-memory per-IP limiter count
while a persistent per-session counter incremented forever and never
reset on success. After one success the DB count stayed elevated while
the limiter restarted at 0 — "the count" was undefined and untested.
Fixed: success resets both; limiter keyed per session+IP (it was
globally per-IP: 10 wrong against session A locked every session);
exact boundary pinned (1–10 → 401 + count, 11th → 429, correct-while-
limited → 429 by explicit decision). Plus time decay (5-minute window):
without it 10 failures locked session+IP until restart.

### 4. Fog had two live leaks: impersonation and raw state

F2b's fog endpoint accepted `?seat=` with no credential check —
anonymous + valid seat id → 200 with that seat's view; seat A as B →
200. And `GET encounter` returned raw state (hidden token objects,
GM-only note uids) to every authorized seat. Both live-proven. Fixed:
`?seat=` is GM-only preview; get AND list project through the same
per-principal fog function; hidden token IDs stripped for non-GM.
Each pinned by disagree-fails tests that fail when the guard is
removed (verified by mutation, repeatedly).

### 5. The write-back digest was nondeterministic (33% spurious 409s)

F4's preview/apply identity hashed Go-map-iteration order: preview
`keys:[xp,hp]` vs apply `keys:[hp,xp]` → 409 with zero changes,
measured 4/12 at 2 keys, worsening with key count. Done-when (2) was
luck, not contract. Fixed by canonicalization (sort uids, keys,
conflicts, applied): ~110 applies, 0 spurious 409s, plus a digest-
determinism unit test.

### 6. Discovery append corrupted the list into a string

Merged `[]string` hit `jsonValueToFrontmatter`'s `default:` branch →
`discovered: "[a b]"` (quoted string), destroying the flat-list type
ADR-0050 requires. Fixed in the frontmatter writer (`[]string` →
YAML list); test re-parses the file and asserts sequence type, length,
preservation, no duplicates.

### 7. Close-with-touches silently dropped every touched key

F4's close path marshaled the small `body` struct (batch ids only, not
the touched-key map) into a synthetic request with no path id — the
forwarded apply 404'd, the result was ignored, measured `xp
applied=false`. Only the session note was created. Fixed by forwarding
the real touched set with the session id routed, failing close loudly
on forward error. The same round fixed: refused applies that still
mutated state (validate-before-mutate now), empty-Path tombstones
poisoning revert (paths set, tombstones skipped), close splitting one
write across two batch IDs (now one — the whole close reverts via its
returned id).

### 8. The F2a bounds don't bite — and tightening them would be wrong

F2a's committed bounds have 1.6–4.3× headroom; halving stays green.
But measured run-to-run variance is large (board p95 swung
57.5→100.1ms, ~75%; fog-single 2.4×). Bounds that fail when halved
would sit inside the noise band — flaky CI by construction. Decision:
keep the bounds, correct the words. Every rationale comment now states
measured worst, bound, what regression size actually trips it, and
observed variance. The "trips on a 2× regression" fiction is gone.
A bound that cannot fail is not a test; a bound that fails on noise is
not a bound either.

### 9. F2a's decision overclaimed against ADR-0047

The suite header recorded "(b) live in-memory index: decided" — but
the numbers don't force it (worst live shape ~16 ms; per-keystroke
affordable, per-frame tight-but-unmeasured), and (b) is a second query
implementation beside the store, contradicting ADR-0047 ("fast
*without* a second query implementation") and the phase's no-second-
mechanism rule. Downgraded to a recommendation with the forcing
numbers; any move to (b) needs a new ADR amending ADR-0047. Nothing
was built beyond the suite, so no rule is violated in code.

### 10. Three F3/F5 cuts deserve explicit notice

- F5 cut `session.state`/`session.patch` from api.md §5 (accurate: no
  engine emits them). F2's live table will need exactly these — it must
  re-add server emission; client stubs are retained for that.
- F5's `kind: "created"` is never emitted for notes (only characters).
  Clients learn of new notes by refetch; recorded, not fixed.
- F3 leaves guest chat *authorship* unruled: `createNote` authorizes any
  non-anonymous principal with no campaign-membership check, so a
  (non-member) guest can create notes anywhere path-traversal allows.
  Reads stay projection-governed (low severity, no leak), but the rule
  for who may speak in which channel was never specified. Proposed rule
  (phase 5): creation requires the caller to see the channel's audience
  (reuse `Visible`, no new mechanism). Needs a decision before
  implementation — deliberately not smuggled into F3.

### 11. The display is party-only, three ways, all pinned

Owner-scoped-to-one, granted-to-one, and discovered-by-one notes are
each absent from display bytes (three committed cases), plus binding
(one token value per session, binds no seat, seat token → 401) and
three journaled reveal targets (everyone / one / display, journal
count exactly 3). The widening-gate line itself (`!IsDisplay()`) is
defense-in-depth over an unreachable state (display principals always
carry empty ID/Bound/Discovered) — pinned by a synthetic-principal
unit test anyway.

### 12. The old path must be deleted, and deletion must be proven

F0a deleted the passphrase-is-code path (join code *was* the
passphrase hash). The pin is a 401 test on the old behavior — a test
that fails if the path is reintroduced. Same shape everywhere this
phase: rotation persistence (redirect the write → test fails),
upload confinement (remove the gate → attack test fails), fog
projection (raw state → disagree test fails). Deletion is a behavior;
it gets a test like any other.

### 13. Vacuous tests hide behind empty credentials

Three separate rounds produced tests that passed with their guards
removed, all with the same shape: a test that looked up a token that
was never issued (`seat["token"]` with no `token` key → `""`), then
guarded every assertion behind `if token != ""`. The fix in each case:
real tokens via join-by-code, `t.Fatal` on empty, unconditional
assertions. A conditional assertion is an untested assertion.

### 14. Pre-existing seams the phase chose not to retrofit

`main.go` PUT/PATCH `SaveNote` (unjournaled), GM PATCH unjournaled,
`verifyToken` HMAC `!=` (not constant-time), WS
`session.command`/`roll.request`/`note.command` advertised but
ignored, in-memory limiter reset on restart, `TestInsertPerformance`
250 ms bound with 7–13% headroom + a tmpdir ordering fragility
(fails with clean TMPDIR — depends on another test's artifact).
Each recorded where found; none widened by phase-4 code.

### 15. The `approved` → `active` alignment

F5 found the approve handler setting a `workflow` value (`approved`)
matching nothing in the declared `draft|submitted|active|retired`
vocabulary; existing tests never asserted the value. Corrected to
`active` with a one-line pin. Alignment to declared vocabulary, not a
decision change — no ADR.

### 16. The cold-run red is environmental and pre-existing

Final `go clean -testcache && go test ./...`: 23 packages ok, 1 FAIL —
the known `TestInsertPerformance` load flake (see §16; fails identically
on pre-phase-4 main, passes in isolation). Vault insert is untouched by
phase-4 code paths. Carried as a known flake, not a regression. e2e
shadow matrix: **45/45** across Chromium, Firefox and WebKit (Xvfb).

### 17. The model plan did not survive contact with quota

The brief prescribed `opencode-go/beam-free` for all building and
`opencode-go/muse-spark-1.3-contributor` for all verifying. Beam-free
rate-limited twice mid-phase; the personal free tier then exhausted
itself. From F2b on, all building ran serially on
`opencode-go/iceman-contributor-free` (the pre-authorized fallback),
with outputs treated as untrusted and hostilely re-verified — a cost
paid in rounds: iceman's first deliveries were consistently thinner
(zero-test F2b/F3/F4/F2c first passes) and needed more fix cycles. The
verification model never rate-limited. Beam-free recovered for the
F3-fix, F4 and F2c rounds. Lesson for phase 5: budget the
fallback's round-trip cost up front, or fund the primary.

### 18. A display that could read any session

F2c's `allowedSession` display branch granted any display principal
read of any *existing* session — a session-existence check, not the
session binding the comment above it promised. Widening without an
ADR, live-proven. Fixed by checking the display's bound session id
(unbound → refuse), which is a narrowing and needs no ADR. Pinned by
bound/cross-session/unbound tests. The adjacent pre-existing seam
(`/display/state` does no session-binding check) is flagged, not
touched.

### 19. Fog existed in three copies that disagreed

F2c's move path and update path each re-implemented fog beside the
canonical `projectEncounter`: the update copy only checked
`old.Hidden`, allowing position changes for `VisibleTo`-excluded
tokens and note-invisible tokens; the move copy denied GMs moving
hidden tokens (wrong — GMs see hidden) while allowing non-GM moves of
tokens pointing at unknown notes. All three divergences live-proven,
then fixed by routing all three paths through one `tokenVisible`
helper inheriting full `vault.Visible` semantics, with the hand copies
deleted. The phase-4 brief named "SQL-vs-in-memory" as a previously-
burned dual mechanism; this was its in-milestone reprise, caught
before merge by the seam re-check.

---

## What was measured

| | measured | criterion |
|---|---|---|
| join→set-passphrase→login; code-as-passphrase dead | 200/401/200 round-trip; reuse/expired → 401 | per-step pins |
| rotation invalidates | old bearer → 401; config re-read asserts new secret | committed |
| compute storm (30 rapid, one player) | **10×200 + 20×429** | sustained-rate bound |
| asset excluded caller | **404, 0 bytes**; GM 200; 304; 206/4 exact bytes | hostile |
| upload attacks (3 paths + variants) | ≥400, files absent; legit 200 under `Assets/` | hostile |
| audit per surface (single/list/render/characters) | gmnote 2/1/2/2; unknote 1; pubnote 0; anon adds 0; no content leak | committed |
| seat kinds (account/guest/spectator/GM) + 5 unplayable rejects | all seated/rejected correctly | per-kind |
| 10 wrong codes → limit + count; 11th | 1–10 → 401 + `attempts==i`; 11th → 429; decay 5 min | committed |
| oracle (3 seat kinds × 5 hidden classes) + hidden-absence asserts | set+order+count unchanged; `hidden_* ∉ uids` 9/9 | mutation-probed |
| cadence (12 shapes, dev/srd-vault) | worst p95s: board ~100 ms, search ≤31 ms, session ≤856 µs; bounds committed w/ honest rationale | CI suite |
| encounter 5e / Blades | budget 32 (levels 3+5) / opposition 2 + clocks; log has all 7 fields | pack-native |
| fog disagree + private-roll silence | per-principal sets; private bytes in zero non-GM surfaces | hostile |
| chat audience (2 players + guests + non-member) | byte-exact separation; display 3 absences; 3 reveals, journal 3 | committed |
| write-back N=2/M=1 | hp conflicts, xp applies, motto survives; preview==apply digest over ~110 applies | mutation-probed |
| batch revert / idempotent close / post-close | byte-identical restore; close×2 identical; 7/7 error-JSON | committed |
| GM 21 / anon 1, one binary | **21 / 1** live | UI request set |
| search + snippets + 409 rebase + both-pack round-trip | snippet from passage; 409→rebase→save→409; active both packs | committed |
| tabletop moves + fog unity + display binding | own 200, others/hidden/out-of-turn 403, closed 409; one fog fn; bound-session display; drag re-resolved | mutation-probed |
| e2e shadow matrix, all three browsers | **45/45** (chromium 15 + firefox 15 + webkit 15, Xvfb) | green |
| cold suite | `make lint` 0; `go test ./...` 23 ok + known flake (see §16); vitest 23/23 | green-ish |

---

## The ADRs landed

| | |
|---|---|
| **ADR-0062** | The session cookie is `SameSite=Lax`, not Strict (amends ADR-0009; evidence: F0a builder + independent reviewer agreement on the LAN/link-sharing shape) |
| **ADR-0063** | F0 closes with WebKit red-with-reason; F2 surfaces wait for green (environmental EGL gap, zero behavioral evidence; unblock requirement documented) |

Proposed but not written (each needs its own decision pass): amending
ADR-0047 if a live index is ever pursued (§9); amending ADR-0052's
ordered-rules language (§4-background — accepted ADR, needs its own
amendment, not an edit); a guest-chat-authorship rule (§10).

---

## What was cut, deferred, or left undone

- **F2c tabletop canvas + drag-drop**: LANDED after the hold was lifted
  (user fixed the GL runner; WebKit 15/15 green under Xvfb, recorded in
  `docs/cross-browser-results.md`; ADR-0063's gate satisfied as green,
  not as waiver — no ADR needed, per its own terms). Canvas tokens,
  validated movement, fog, display mode, shadow-boundary drag-drop all
  landed with mutation-probed tests; e2e 45/45 across all three
  browsers.
- **Guest chat authorship rule** (§10): proposed (creation requires
  seeing the channel's audience), undisposed. Owner: phase 5.
- **ADR-0052 amendment**: ordered-rules/inspector language survives
  alongside ADR-0057. Needs its own amending ADR. Owner: phase 5.
- **macOS watcher + session-tier numbers**: still need a Mac. Fourth
  report running. Recorded, not forgotten.
- **`requires.app` range language, dashboard third entity, demo
  session wing** (decided: tests build their own fixtures — no demo
  churn): carried open items, unchanged.
- **MCP write tools, out-of-process plugin tier, native SDK**: stay
  out (ADRs 0014/0020, api.md §9a). First request reopens the list.
- **Pre-existing hardening notes** (§14 + verifyToken/grant-nit list):
  recorded at each finding site, none blocking.

---

## What phase 5 should start with

**The recording pipeline** (phase 5's own scope: transcription,
session logs-as-audio, music) now has everything it needs: session
notes always created, dice log persisted per session/encounter, chat
archived as notes, discovery appended per character, write-back
journaled and revertible, and — as of the F2c close-out — a verified
tabletop canvas with the shadow matrix green in all three browsers.
The contract phase 5 writes into held through this phase without
amendment — the strongest validation this phase can offer the next
one.

Second, **the guest-authorship and display-surface hardening**: the
chat-authorship rule (§10), display rotation/revocation (currently one
token value, no revocation — documented), and the W3 overlap (gm seat
vs GM bypass — documented, fail-closed).

---

## What I would do differently

- **Put the corpus decision in the first brief of each wave.** F2a
  burned a full round measuring against `fixtures/` (gitignored,
  absent in CI, suite skipped itself green) because the brief said
  "the SRD vault" without naming `dev/srd-vault`. The repo convention
  existed; the brief didn't cite it.
- **Treat "no test files in the diff" as a merge gate, mechanically.**
  Seven first-deliveries had zero tests with green-claiming reports. A
  `git diff --stat` for `*_test.go` at dispatch-review time would have
  bounced all seven before verification spent cycles proving it.
- **Budget the fallback model explicitly.** The brief named one
  builder model with no contingency; quota forced serial iceman
  mid-phase, roughly doubling fix rounds. Phase 5's brief should name
  the fallback, its serial discipline, and its round-trip cost up
  front — this phase learned it under fire instead.
- **Keep the waiver question ahead of the wave.** ADR-0063's gate was
  written while F5 was building; F2c's hold decision came later and
  cleanly. Asking "what can't run here" before wave-planning beats
  discovering it at the merge gate.

---

## A note on how the phase ran

Ten waves (F0, F1, F2a+F5, F2b, F3, F3-fix, F4, F4-fix, F2c, F2c-fix),
serial fallback builders after quota, investigation-model verification
throughout. The review found what the workers didn't — the upload
primitive, the fog leaks, the digest lottery, the discovery
corruption, the dropped close keys, the display widening, the tripled
fog, seven zero-test deliveries: none of
those was in a worker's report. Every one was found by reading the
diff and running the code.

That is the whole argument for the review step, and it is why this
document is longer about what was found than about what was built.

---

## Wave/merge record (for the log)

F0 → merged `1872721` (+ ADR-0062/0063 `7d5165a`) · F1 → merged
`d736dd1` · F2a → merged `27a8e24` · F5 → merged `67904f0` · F2b →
merged `49f07c2` · F3 → merged `e81f522` · F4 → merged `550f5e3` ·
F2c → merged `45c0de9` · WebKit green + matrix update → `47a71a7`.
All merges `--no-ff`. No pushes (per rule — push on request).
