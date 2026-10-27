# Phase 4 Plan — The Table

> **Goal:** live play on the wiki the first three phases built — sessions,
> seats, the tabletop, chat, and the write-back — plus the webapp
> completion the demo exposed as missing: GM and players can log in,
> search, view, and edit notes from the browser.
>
> **Phase 4 in one line:** *who sits down, what happens at the table, and
> what the morning after keeps.*
>
> **Definition of done:** a GM runs a session from the webapp — players
> join by code, sit as characters, move tokens on a fogged map, roll dice
> that land in a shared log, chat in notes, and closing the session writes
> back character changes, an encounter summary, and a session log, each
> revertible. A player who missed the table still gets their
> between-sessions view.
>
> **And one thing it does not claim.** Recording, transcription, session
> logs-as-audio, and music stay out (phase 5). The tabletop is tokens, fog,
> and initiative — no map editing (ADR-0016).

Phase membership and what moved: [`phase-plan.md`](phase-plan.md).

---

## 1. What this phase costs

**Six milestones, fifteen sub-milestones.** Estimates are rough — phase 3
taught that a number without the artifact is a wish, so these are ranges
against the phase-3 machine.

| Block | Sub-milestones | Full-time | Part-time (≈10 h/week) |
|---|---|---|---|
| F0 — server close-out: auth hardening, missing read/write endpoints | 3 | 1–2 wks | 2–4 weeks |
| F1 — sessions, seats, codes, scratch log | 3 | 2–3 wks | 1–2 months |
| F2 — encounter runner + 2D tabletop | 3 | 3–4 wks | 2–3 months |
| F3 — chat, shared display | 2 | 1–2 wks | 1 month |
| F4 — session write-back contract | 2 | 1–2 wks | 1 month |
| F5 — webapp completion: login, search, editor, sheets | 2 | 2–3 wks | 1–2 months |
| **Phase 4 total** | **15** | **~10–16 weeks** | **~6–10 months** |

**The expensive part is measured first, not assumed.** O2's query path
scans and materialises the full matching set (~63–81 ms per call over
1,690 notes, no SQL LIMIT — `internal/store/query.go:162-167`). A live
table issuing per-token, per-frame, or per-keystroke queries through that
path blows any interactivity budget. F2 starts with that measurement,
before the encounter runner assumes a query shape.

---

## 2. What is in this phase, and what is not

**In:** the F0 close-out (passphrase-set flow, asset auth + upload,
`POST /compute`, note DELETE, links/backlinks, journal read, character
`requests` list, GM-secret rotation, GM-only-read audit); session codes,
seats, the session tier, the scratch log; the encounter runner and the
2D tabletop (tokens, fog, initiative, shared dice log); chat as notes,
the shared display; the write-back contract (partial apply, per-key
conflict reporting, journal-based recovery, preview-identical-to-apply);
the webapp login, search box, real (projected) preview, frontmatter
parity, conflict UI, secret-reveal buttons, the character panel, the
sheet submit/review UI.

**Out, deliberately:**

| Out | Where |
|---|---|
| Recording, transcription, session logs-as-audio, music | Phase 5 |
| Map editing on the tabletop | Cut (ADR-0016) |
| An out-of-process plugin tier | Recorded in ADR-0020, deliberately unbuilt |
| MCP write tools | After phase 4 — read-only is the contract (ADR-0014) |
| A native-client SDK (Swift/Kotlin) | Not implied by any contract (api.md §9a) |
| Starter Obsidian `.base` files | Declined (a plausible-looking one Obsidian rejects is worse than none) |
| `requires.app` version-range language | Undecided; refusal stands, range syntax does not |

---

## 3. Milestones

### F0 — Server close-out and auth hardening

Phase 3's demo proved the gap: the server authenticates (GM secret, join
codes, passphrases, bearer-on-every-route) and the browser authenticates
nobody. F0 closes the server side so the webapp has something true to
log into — and fixes the defects the surveys found in what exists.

**F0a — auth completion.** A set-passphrase endpoint (join redeems a code,
the player *chooses* a passphrase — ADR-0009's flow; today the passphrase
*is* the consumed code, `auth.go:373`). GM-secret rotation invalidating
outstanding sessions. `POST /api/v1/auth/login` gets its first direct
test. Proposal for the amending ADR: SameSite=Lax is the implementation
(`auth.go:434-443`) against ADR-0009's Strict — record the deliberate
choice with evidence.

**F0b — missing read/write endpoints.** `POST /api/v1/compute` (the
explicit escape hatch api.md §4 promises; the wizard preview needs it),
`DELETE /api/v1/notes/:uid`, `GET /api/v1/notes/:uid/links` (index-backed —
not a body scan; the O3 `get_links` lesson), `GET /api/v1/journal`,
`GET /api/v1/characters/:uid/requests` (+ approve/reject), asset upload
with size/type caps, and `GET /asset/` through the projection (today
`allowAll`, `main.go:522`/`717` — a bypass, not a decision).

**F0c — audit + docs.** Every read of a GM-only note lands in the audit
log (threat-model row, never asserted); the WebKit column closes first —
phase 4 builds its most interactive surfaces on the least-tested UI
floor, and a Safari divergence found after the tabletop costs more than
one found now. api.md gaps 2–3 marked closed-or-scoped, unimplemented
rows marked or cut (§10 rewrite).

**Done when**
- Join → set-passphrase → login round-trips with the passphrase only the
  player knows; the old passphrase-is-code path is deleted, not
  deprecated; a committed test pins each step.
- GM-secret rotation invalidates outstanding sessions (old bearer → 401);
  committed test.
- `POST /compute`, DELETE note, links, journal read, character requests,
  asset upload answer with measured values; `GET /asset/` returns 404
  (not the bytes) for a caller whose visible set excludes it — committed
  hostile test per endpoint.
- A GM-only note read appears in the audit log; committed test.
- WebKit runs the O4 matrix green, or red with an ADR number — before any
  F2 surface work starts.

### F1 — Sessions, seats, codes, scratch log

**F1a — the session.** `POST /api/v1/sessions` (create), state, seats in
advance, close (triggers F4's write-back path, stubbed until F4 lands).
Session codes: six chars, per-session rotation, read aloud — short-lived,
not secret.

**F1b — seats and joining.** `POST /api/v1/sessions/:id/join` by code
(guest seat, GM-assigned character) or passphrase (account seat, own or
assigned character); spectator seats. The seat token is session-scoped
and grants nothing outside it. The session-code endpoint is rate-limited
and counts attempts (deferred since phase 1; lands with the endpoint,
not after it).

**F1c — the seat projection + scratch log.** Seat-token resolution
through the audience model (ADR-0057 — *not* the ordered rules
threat-model §3 still describes; that section is corrected by this plan).
The oracle-style suite for seat projection: adding a GM-only note (or a
second seat's private note) changes neither a seat's result set, order,
nor count. The scratch log as the third, disposable state tier.

**Done when**
- Join-by-code and join-by-passphrase both seat correctly (account /
  guest / spectator / GM), with committed tests per seat kind; a seat
  naming an unplayable character is rejected.
- Ten wrong session codes in a row rate-limit and count; committed test.
- The seat oracle: a hidden note with the table's rare term changes
  neither the seat's set, order, nor count — committed, mutation-probed.
- A seat token outside its session (wrong session, ended session) buys
  nothing — committed test per case.

### F2 — Encounter runner + 2D tabletop

**F2a — the measurement.** Session-tier read/write cadence against the
O2 full-scan cost, on the SRD vault, committed as a failing suite: the
number that decides whether the table polls queries, holds a live index,
or pushes cursor predicates into SQL. No runner work starts until this
number exists.

**F2b — the runner.** Initiative, opposition budgets from the pack's own
difficulty model (5e XP, Blades clocks — never an app-assumed XP budget),
the shared dice log with full provenance, `shared: false` private GM
rolls.

**F2c — the table.** Tokens, fog of war, movement; the display principal
(ADR-0048: party-visible only, never owner/granted/discovered — not the
union). Native drag-and-drop across the shadow boundary (O4's proven
path). OS-level and plugin→plugin drag stay deferred (named, phase-3
report).

**Done when**
- The cadence measurement runs in CI with committed bounds; exceeding any
  bound fails the suite.
- An encounter runs end to end on both packs (5e + Blades) with pack-native
  budgets; the dice log shows roller, workflow, expression, faces,
  outcome, timestamp.
- Fog hides tokens/notes from seats that may not see them; a committed
  disagree-fails test per principal; private rolls never reach the shared
  log (committed hostile test).

### F3 — Chat and shared display

**F3a — chat as notes.** `type: message` notes in `Campaigns/<id>/Chat/`,
channel and audience from visibility (ADR-0048). Between-sessions access
rides this: a player's own view of the world between games.

**F3b — the shared display.** Display-principal surfaces (party-visible
only) for the table screen; handout reveal promoted and journalled;
display binds to no seat and holds no token beyond the table.

**Done when**
- Two players chat and each sees exactly their audience (committed
  projection tests); chat history is searchable and archivable as notes.
- The display shows party-visible content only — an owner-scoped note
  visible to one player does not appear; committed test.
- A reveal targets everyone / one player / the display; each is a
  journalled event; committed tests.

### F4 — Session write-back contract

ADR-0039 + `session-writeback.md`, unbuilt until live sessions exist —
now they do. Partial apply with per-key conflict reporting,
journal-based recovery, **preview-identical-to-apply** (the preview the
GM approves is byte-identical to what applies, or the apply refuses).

**F4a — the apply.** Character changes, encounter summary, session log
note; per-key conflicts reported, not clobbered; batch revert per
`batchId` through `WriteNoteJournaled` (the O1 lesson: no direct-write
path).

**F4b — the close lifecycle.** Session close triggers write-back;
`lifeMu`-style closed-check discipline on every lifecycle transition
(the O1 crash lesson); close-after-close and dispatch-after-close are
tested, not described.

**Done when**
- A session with conflicting GM/player edits closes with conflicts
  reported per key and non-conflicting keys applied; committed test with
  measured conflict set.
- Preview bytes equal apply bytes, or apply refuses — committed test
  (not a log line).
- A full batch reverts through the journal; committed test.
- Close is idempotent; post-close dispatches return error JSON, never
  panic — committed regression tests.

### F5 — Webapp completion: login, search, editor, sheets

The demo gap, closed as a milestone: from anonymous-only to
GM-and-player-native.

**F5a — login + principals.** Login/join/GM-secret surfaces, token store,
`Authorization` injection on every `fetch`, principal switcher (GM /
account / seat), session-code join view. WS client (`?token=`) with
`secret.revealed` → refetch and `vault.changed` handling (server must
emit what api.md §5 promises — today it emits two of six types).

**F5b — search, view, edit.** Search box over the O2 params (`q`,
`filter`, `type`, cursor pagination — no N+1); real preview via
`GET /:uid/render` (the client-side 500-char extract leaks secrets and
is deleted); frontmatter parity (`visibility`, `workflow`, `grants`,
`campaign`, `kind`, `player` + server error display); 409 conflict UI
with rebase, not "save failed"; secret-reveal buttons wired to the WS
intent; character panel ported from the spike (form-from-pack, masked
values) with sheet submit/review UI.

**Done when**
- GM login shows all 21 demo notes; anonymous still sees 1 (the same
  binary, two principals — committed test through the UI request set).
- Search answers through O2 with snippets, empty states, and cursor
  paging; committed tests.
- Preview renders the projection (a GM-only secret is absent for a
  player — committed hostile test); the client extract is deleted.
- A 409 surfaces the diff and offers rebase; committed test.
- Character create → submit → GM approve → active round-trips from the
  webapp on both packs; committed end-to-end test.

---

## 4. Sequencing

```
F0 (close-out) ──► F1 (sessions/seats) ──┬──► F2 (runner/table)
                                         ├──► F3 (chat/display)
F0 ──► F5 (webapp; needs F1 principals for seats)
F2 ──► F4 (write-back needs the live table to close)
```

- **F0 first.** Auth completion and the missing endpoints are the floor
  F1 and F5 build on; the WebKit column closes before F2 builds on the
  shadow host.
- **F1 before F2/F3.** Seats are the principal space both serve; the seat
  oracle is the trust boundary they inherit.
- **F2 and F5 run together** — different surfaces (canvas vs forms), one
  shared rule: both read through O2 + projection, never around them.
- **F4 last.** The close path is the highest-risk lifecycle; it needs the
  real table to close.
- **F5 needs F1's seat principals** for the seat view; GM/player views
  ride F0 alone and can land early.

**The critical path is F0 → F1 → F2 → F4**, with F5 alongside F2 and F3
alongside either.

---

## 5. The exposure checklist, and who owns it

Phase 3's checklist stands. New and newly-due rows:

| Check | Owner |
|---|---|
| Session-code endpoint rate-limited and counts attempts | **F1** (lands with the endpoint) |
| Seat token grants nothing outside its session | **F1** (oracle-pinned) |
| Guest seat sees bound character + live table, nothing else | **F1** (oracle-pinned) |
| Join passphrase chosen by the player, never the code | **F0** (passphrase-set flow; old path deleted) |
| GM secret rotatable, rotation invalidates sessions | **F0** |
| Every GM-only-note read in the audit log | **F0** |
| `GET /asset/` role-scoped; upload capped by size/type | **F0** (bypass closed) |
| WS parity: token auth = REST auth; server emits what api.md promises | **F5** (or cut the promised types) |
| Player writes constrained to own character note (session-writeback §1) | **F4** (enforced, not documented) |
| Write-back preview identical to apply, or apply refuses | **F4** |
| Chat audience = note visibility; display = party-only | **F3** |
| No client holds GM secrets (projection + disagree-fails on every new surface) | **F5** (extends O5 discipline) |
| Compute preview storm bounded (1 keystroke/100 ms/player — N6's open question, answered here) | **F0** (`POST /compute` ships with its limit) |
| App starts offline; no auto-update | Standing (re-verified) |

---

## 6. What phase 3 taught this phase

Four findings arrive as requirements, not advice:

1. **The hostile test ships in the first brief.** The sort injection,
   the SSRF guard, the oracle — every one arrived as a verification
   finding against code that "passed". F-briefs carry exact hostile
   inputs (forged seat token, tampered cursor in live queries, hidden
   note with the table's rare term, close-after-close dispatch).
2. **Prove the path, not the value.** The `list_by` pin asserted equal
   outputs on one query; a fork passes it. Seat projection, write-back
   apply, and fog need adversarial corpora and path-level assertions —
   stated up front in each brief.
3. **One discipline per seam.** Counts-vs-badges, SQL-vs-in-memory,
   ordered-vs-audience: every dual mechanism became a finding. F4's
   apply path, F2's live reads, F5's preview each name one canonical
   mechanism in their brief.
4. **The merger owns the seam collisions.** O2's `Load` lesson and the
   N6/N7 precedent stand: the vault owns notes, the store owns queries,
   the projection owns visibility, the session tier owns live state.
   Only one milestone per wave extends shared code.

And the standing lessons: measure the criterion's subject, not a proxy;
read the diff — workers' reports are evidence, not proof; a test that
cannot fail is not a test.

---

## 7. Open items (carried, with owners)

- macOS watcher + session-tier numbers — still need a Mac, still owned by
  nobody; recorded, not forgotten a fourth time.
- Discovery granularity (`visited` vs `heard of`) — needs a separate note
  type (session-writeback §6 punts per ADR-0050 D4); the write-back output
  that feeds the between-sessions view has nowhere to live until this
  closes. F4 proposes, human disposes.
- `requires.app` range language — refusal stands, syntax undecided.
- Dashboard third entity shape — O5 left it; F5 decides only if a surface
  needs it.
- Demo-campaign session wing — phase 4's session fixtures need a campaign;
  F1 decides whether the demo grows one or tests build their own.
- Out-of-process plugin tier (ADR-0020), MCP writes (ADR-0014) — stay out;
  first request reopens this list, not the milestones.
