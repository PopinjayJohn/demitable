# Phase 5 Plan — The Session

> **Goal:** the table remembers what happened — the GM records the
> session, the app transcribes it locally by default, a reviewed
> transcript becomes the session log, and the table gets mood-driven
> music from the GM's own library.
>
> **Phase 5 in one line:** *what the morning after keeps, and what it
> sounded like.*
>
> **Definition of done:** a GM records a session from the webapp with
> the table's knowledge; the audio never leaves the machine unless the
> GM explicitly opts a campaign into a custom engine; a reviewed
> transcript with speaker labels becomes the session log's marked
> proposal section; the encounter's mood drives local music cues; and
> every surface built along the way holds the same projection,
> logging, and shadow-host discipline as the rest of the app.
>
> **And one thing it does not claim.** Live captions (ADR-0018 names
> the stretch goal and refuses the promise), cloud provider SDKs with
> shipped keys, and map editing (cut since ADR-0016).

Phase membership and what moved: [`phase-plan.md`](phase-plan.md).

---

## 1. What this phase costs

**Five milestones, twelve sub-milestones.** Estimates are rough — phase 4
taught that a number without the artifact is a wish, so these are ranges
against the phase-4 machine.

| Block | Sub-milestones | Full-time | Part-time (≈10 h/week) |
|---|---|---|---|
| P0 — contract conformance + carryover closure | 3 | 3–5 days | 1–2 weeks |
| P1 — capture: browser recording → blob store | 2 | 1–2 wks | 2–4 weeks |
| P2 — transcription: interface, local engine, custom engines | 3 | 2–3 wks | 1–2 months |
| P3 — session logs from transcript | 2 | 1–2 wks | 1 month |
| P4 — mood-driven music from local files | 2 | 1 wk | 2–3 weeks |
| **Phase 5 total** | **12** | **~5–9 weeks** | **~3–5 months** |

**The webapp rework is the floor, not the work.** Between phase 4 and
this plan the webapp was reworked (Viewer as read mode, split vault
explorer, campaign Home, in-body reveals, docked stat box, journal
drawers). P0 inventories those surfaces against the design before
phase 5 builds on them — a Safari-class divergence found after the
recorder ships costs what F2c taught.

---

## 2. What is in this phase, and what is not

**In:** the P0 conformance pass (api.md staleness, SURVEY refresh,
stale design refs, the session-writeback §8 answer); the guest chat
authorship rule (phase 4's top open item, closed here); the
ADR-0052 amendment; browser audio capture with table consent;
session-scoped blob storage outside the vault; the `Transcriber`
interface with a local sidecar default, an external-command engine,
and a mock; per-campaign engine choice with explicit opt-in
disclosure; the campaign-name lexicon; transcript review with
speaker labels; the marked-section log proposal; mood derivation as
a pure function; local-file music cues for GM and display.

**Out, deliberately:**

| Out | Where |
|---|---|
| Live captions during play | ADR-0018 stretch goal; baseline is record-then-transcribe |
| Cloud provider SDKs / shipped API keys | No keys, no new networked deps; custom engines via external command |
| faster-whisper as a shipped engine | Alternative, not shipped; the external-command engine covers it |
| Un-reveal, per-player reveal UI, inspector chain, campaign fields | Backlog stays backlog (api.md §10); per-player reveal needs no contract change |
| Map editing on the tabletop | Cut (ADR-0016) |
| Recording the players' devices (multi-source capture) | GM's browser is the single capture path; player-side backup is undecided |
| Native-client SDK, MCP writes | After phase 5 (ADR-0014 read-only stands) |

---

## 3. Milestones

### P0 — Contract conformance and carryover closure

Phase 4 closed with the docs behind the code: the api.md §1 gaps text
still promises F0/F5 futures, the §3 sessions header still says
"none implemented", §5 advertises three client→server intents the
server ignores and a `session.state` full-sync on reconnect that was
cut in F5. SURVEY.md is O5-era and does not name Viewer,
VaultExplorer, Chat, Home, Rail, or the journal drawers. `design.md`
still says the media worker ships in "phase 4". P0 closes the paper
before phase 5 builds on it.

**P0a — api.md staleness pass.** Rewrite the stale rows to match the
landed code (Gap 2/3 text, sessions status line, WS intent table,
reconnect paragraph), or mark-and-cut with a one-line reason each.
Verify `?expand=1` against the code (claimed in §4, never pinned by
F0/F5 verification) — implement nothing, document what is true.
`docs/api.md` only; no `docs/decisions/` edits.

**P0b — guest chat authorship rule (carryover).** Phase 4's top open
finding: `createNote` authorizes any non-anonymous principal with no
campaign-membership check, so a non-member guest can create notes
anywhere path-traversal allows. Reads stay projection-governed (no
leak), but authorship is unruled. The rule: **creation requires the
caller to see the channel's audience** — reuse `Visible`, no new
mechanism; a guest posting to a party channel it cannot read is
rejected with a named error. Committed tests per principal (GM /
account / seat / guest / anonymous). If the rule contradicts an ADR,
it lands as a new ADR amending it — the worker reports, the
orchestrator writes.

**P0c — SURVEY refresh + stale refs.** Inventory every `web/src`
surface against the design rules (ADR-0052 projection consequences,
ADR-0029 shadow mount, api.md contract, no client-only state):
each surface lands in SURVEY.md as conforming (with the discipline
cited) or recorded-deferred with a reason — the O5 standard, now for
the reworked webapp. Fix `design.md`'s stale phase refs (media
worker, anything else the inventory finds). File the ADR-0052
amendment (orchestrator writes it; workers gather the ordered-rules
residue as evidence).

**Done when**
- api.md has no row the code contradicts — committed check per
  rewritten row (a grep-test or request test that fails if code and
  doc disagree on route existence, WS type emission, or intent
  handling).
- Guest creation outside its visible audience is rejected —
  committed test per principal; no widening of any read path.
- SURVEY.md names every current surface with its discipline or its
  reason; `design.md` carries no stale phase number; ADR-0052's
  ordered-rules residue is amended, not edited.

### P1 — Capture: browser recording to a session blob store

The server cannot hear the room; the GM's browser must. GM-only
capture UI (`getUserMedia` + `MediaRecorder`), gated on an explicit
table-consent affordance: recording starts with a visible,
audible-state indicator and a per-campaign setting whose enabling
text states plainly that audio is recorded (ADR-0018's privacy
shape — the GM's decision, statable, reversible).

**P1a — capture + upload.** Chunked upload endpoint
(`POST /api/v1/sessions/:id/audio`, GM bearer or GM seat only),
resumable within the session, WS progress events on the existing
envelope (no new WS types — reuse `vault.changed` scoping or a
session-namespaced event on the existing `note.saved` pattern; the
brief decides, one mechanism). Blob store **outside the vault** in
the XDG app-data dir (ADR-0030's rule: rebuildable caches live
outside; audio is bulky, undiffable, and must never sync — and the
transcript, which *is* the durable text, lives in the vault).
Session-scoped, GM-only, never indexed, never MCP-listed, never
served to players (404 with 0 bytes, same posture as assets).

**P1b — retention + consent.** Per-campaign retention (keep until
deleted vs auto-delete after transcript review — default: keep
until the GM deletes; the delete is a committed affordance, not a
UI dead end). Raw audio contains untranscribed speech including
secrets spoken aloud — it is GM-only by default and stays so. Works
on the LAN with no internet (hard requirement; the server is
present and local).

**Done when**
- A GM records 60 s from the webapp; chunks land; the blob is
  present session-scoped and byte-identical to the upload.
- A player (or anonymous) fetch of any audio URL gets 404 with 0
  bytes — committed hostile test per surface (direct URL, guessed
  session id, other-session token).
- Capture with the network down except the LAN round trip works;
  the consent text is pinned (exact-string test on the enabling
  copy).

### P2 — Transcription: interface, local default, custom engines

ADR-0018's interface, implemented for real:

```go
type Transcriber interface {
    Name() string
    Transcribe(ctx context.Context, audio io.Reader, opts Options) (<-chan Segment, error)
}
```

A `Segment` carries text, start/end times, optional speaker labels.

**P2a — the local engine.** whisper.cpp as a sidecar over JSON on
stdio, supervised by the server. Distribution is the known hard
part (ADR-0018 says so): **download-on-first-run, checksum- and
version-pinned, per platform, with an explicit missing-engine
message** — never a silent fallback to anything remote. No cgo
(the sidecar is a process; the one-binary promise constrains the
Go build, and the engine is a separate asset by decision).

**P2b — custom engines without shipped keys.** An external-command
engine: the GM points a campaign at their own command (a cloud CLI,
a faster-whisper wrapper, anything speaking the segment protocol).
This satisfies "configurable" with no API keys, no SDK deps, no
supply-chain surface. The opt-in disclosure UI states, at the
point of enabling, that audio leaves the machine — exact-string
test on the warning copy; no configuration selects a non-local
engine implicitly (committed test: fresh campaign defaults local
even with a custom engine present in config).

**P2c — lexicon + segments.** Campaign-name lexicon (characters,
locations, factions — the proper nouns ADR-0018 flags) fed to the
recogniser where the engine accepts one, and applied as a
post-pass correction where it does not. Segments land in the
session tier during the session (live state, crash-surviving like
the rest of it); the transcript note is written at review/close.
A mock engine (deterministic fixture audio → fixed segments)
pins the whole downstream in CI.

**Done when**
- Mock-engine transcription runs in CI: fixture audio in, fixed
  segments out, lexicon applied (campaign terms asserted in the
  output).
- Sidecar protocol test with a fake binary (JSON/stdio contract
  pinned; missing binary → named error + the exact user message).
- Fresh campaign resolves local even with custom config present;
  enabling text contains the leaves-the-machine warning verbatim —
  committed tests.
- Unreviewed transcript segments are invisible to players —
  committed hostile test (seat/anonymous reads get nothing).

### P3 — Session logs from the reviewed transcript

`session-writeback.md` §8 asked who writes the prose summary. This
milestone answers it by building the default: **the pipeline
proposes into the marked section; the GM rewrites.** The machine's
proposal and the GM's rewrite never fight over the same text, and
the preview/apply discipline from F4 means the GM sees exactly what
a proposal would write.

**P3a — review UI.** Transcript segments with editable speaker
labels, GM text edits, promotion of corrections back into the
lexicon (a name the recogniser mangled becomes a lexicon entry —
the loop ADR-0018 implies). Encounter summary and dice log join
the review (both exist from F2b/F4); discovery "met" records
append flat, consistent with F4 (no granularity change — §8's
second question stays at its default until a note type exists).

**P3b — the proposal.** Extractive proposal (segments → marked
section in the session note, scene beats from encounter
boundaries), GM edit preserved byte-identical, proposal re-run
never clobbers the GM's rewrite (marked-section ownership,
tested). The session log note remains the F4 shape — one new note
per session, journaled, revertible.

**Done when**
- A fixture transcript reviews end to end: speaker labels edited,
  correction promoted to the lexicon, proposal lands in the marked
  section, GM rewrite survives a proposal re-run byte-identical —
  committed test per step.
- An unreviewed transcript is GM-only; the reviewed log follows
  the note's audience — committed hostile test on the promotion
  boundary.

### P4 — Mood-driven music from local files

"Mood-driven" is barely specified anywhere (one phase-plan line,
one risk row, one ADR-0018 mention), so this milestone defines it
narrowly and implements exactly that: **mood tags derived
deterministically from session state and transcript keywords,
driving cues from the GM's own local library.** No streaming, no
licensing surface (the user supplies files — design §8 risk row),
no network in the music path, no model.

**P4a — mood derivation.** A pure function: inputs are encounter
state (active? round? HP fractions per side? conditions?) plus
transcript keywords (combat terms, rest terms, names spoken);
outputs are mood tags (`tense`, `triumphant`, `sombre`, `calm` —
a fixed vocabulary in the pack or the core, decided once and
tested). Deterministic, table-driven committed tests on fixture
sessions. It is not sentiment analysis; it is a stated mapping,
and the mapping is the test.

**P4b — local cues.** The GM points the campaign at local audio
files (served through the existing role-scoped asset path — no
new serving mechanism); the session view offers cues per mood to
GM and display principals (party-only, like everything the
display sees). Playback with the network blocked still works —
committed test (the music path must not fetch). Display shows
current mood + track, never GM-only context.

**Done when**
- Fixture sessions map to the stated moods — committed
  table-driven test (every vocabulary tag reachable, unknown
  input maps to a documented default, never to nothing).
- A cue plays a local file for GM and display with the network
  down — committed test; no fetch leaves the machine (asserted,
  not described).
- No GM-only state reaches the display through mood or track
  metadata — committed hostile test.

---

## 4. Sequencing

```
P0 (contract + carryover) ──► P1 (capture) ──► P2 (transcribe) ──► P3 (logs)
                                              └─► P4 (music; needs P2 keywords)
```

- **P0 first.** The api.md/SURVEY truth and the authorship rule are
  the floor everything builds on; the ADR-0052 amendment unblocks
  nothing but must not be forgotten a fourth time.
- **P1 before P2.** The engine needs audio shaped the way the
  uploader stores it; the mock engine lets P2's downstream land
  before any binary downloads.
- **P2 and P4 run together** — different surfaces (transcript
  pipeline vs cue surface), one shared rule: local files, no
  network, never around the projection.
- **P3 last.** The log proposal needs the real transcript pipeline
  and the real lexicon loop to close against.
- **P0b (authorship) can land early** — it touches only chat
  creation and its tests, and unblocks nothing else.

**The critical path is P0 → P1 → P2 → P3**, with P4 alongside P2/P3.

---

## 5. The exposure checklist, and who owns it

Phase 4's checklist stands. New and newly-due rows:

| Check | Owner |
|---|---|
| Audio blob endpoint GM-only; players get 404/0 bytes per surface | **P1** (lands with the endpoint) |
| Unreviewed transcript invisible to players | **P2** (oracle-pinned) |
| Opt-in disclosure exact-string; fresh campaign defaults local | **P2** (lands with the setting, not after) |
| No message body, segment text, or credential in any log line | **P1–P3** (transcript text in logs is a vault leak — hand-review to the investigation model) |
| Music path fetches nothing; display gets party-only metadata | **P4** |
| No new networked dependency; sidecar is a pinned download, not a bundle | **P2** |
| Guest creation confined to its visible audience | **P0b** |
| WS parity for any new event type (or cut the promised type) | **P1** (F5 rule: emit or cut) |

---

## 6. What phase 4 taught this phase

Four findings arrive as requirements, not advice:

1. **The hostile test ships in the first brief.** Player fetch of
   audio URLs, unreviewed-transcript reads, implicit-engine
   selection, music-path fetch — each brief carries its exact
   hostile inputs, not the fix brief.
2. **Prove the path, not the value.** Seat-chat reads, transcript
   visibility, cue playback need adversarial corpora and path-level
   assertions — stated up front in each brief.
3. **Name the corpus or measure the wrong tree.** F2a burned a round
   on `fixtures/` (absent in CI) because the brief said "the SRD
   vault". Every measurement brief names `dev/srd-vault` or its
   own fixture, never "the corpus".
4. **"No test files in the diff" bounces the brief.** Seven
   first-deliveries proved the review step catches it late and
   expensively. The orchestrator checks `git diff --stat` for
   `*_test.go` / `*.test.ts` before dispatching verification — a
   testless branch never reaches a reviewer.

And the standing lessons: measure the criterion's subject, not a
proxy; read the diff — workers' reports are evidence, not proof; a
test that cannot fail is not a test; one canonical mechanism per
seam; the merger owns the seam collisions.

---

## 7. Open items (carried, with owners)

- macOS watcher + session-tier numbers — still need a Mac, still
  owned by nobody; recorded, not forgotten a fifth time.
- Discovery granularity (`visited` vs `heard of`) — needs a separate
  note type (session-writeback §6 punts per ADR-0050 D4); P3 keeps
  the F4 flat appends. P3 proposes the note type only if the review
  UI proves the flat list insufficient; a human disposes.
- `requires.app` version-range language — refusal stands, syntax
  undecided.
- Dashboard third entity shape — O5 left it; P0 decides only if a
  surface needs it.
- `TestInsertPerformance` 250 ms bound (7–13% headroom, tmpdir
  ordering fragility) — pre-existing environmental flake; owned by
  nobody; fix-on-touch, not a milestone.
- Un-reveal / per-player reveal UI / inspector chain / campaign
  fields — backlog stays backlog (api.md §10). Per-player reveal
  is UI-only on an existing contract; first request reopens the
  list, not the milestones.
- Out-of-process plugin tier (ADR-0020), MCP writes (ADR-0014),
  native SDK — stay out; first request reopens this list, not the
  milestones.
