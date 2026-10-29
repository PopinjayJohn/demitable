# Phase 5 — The Session: what it found

> Five milestones, twelve sub-milestones, all landed. The phase put the
> morning after into the product: browser capture into a session-scoped
> blob store, a pluggable transcription pipeline with a local default,
> a reviewed transcript that becomes the session log's marked proposal
> section, and mood-driven music from the GM's own library — plus the
> contract-conformance pass the webapp rework left behind.
>
> It holds itself to the same standard as the four reports before it:
> **a phase that found nothing is a phase that did not test anything.**

---

## The phase in one paragraph

A GM records a session from the webapp with the table's knowledge; the
audio never leaves the machine unless the GM explicitly opts a campaign
into a custom engine; a reviewed transcript with speaker labels becomes
the session log's marked proposal section; the encounter's mood drives
local music cues; and every surface built along the way holds the same
projection, logging, and shadow-host discipline as the rest of the app.

The phase found **nineteen things**, four of them blocking defects no
worker reported — a capture endpoint whose POST and DELETE gates were
vacuous with an empty WebSocket test behind them, a transcription import
cycle that turned `make test` and `make lint` red tree-wide, a second mood
mechanism computed on the client while the server function had zero
callers, and a proposal writer that wrote raw bytes through `os.WriteFile`
with a record-only journal and so could not be reverted. A fifth failure
was docs-only: the final gate failed because SURVEY missed two phase-5
rows and api.md left the P1–P3 routes undocumented. None of the five was
in a worker's report. All were found by reading the diff and running the
code.

---

## The five milestones

| | Milestone | The number that says it works |
|---|---|---|
| **P0** | contract conformance + carryover closure (P0a api.md staleness; P0b guest-authorship guard; P0c SURVEY refresh + ADR-0064) | P0a 8/8 checks; P0b 7/7 tests; hostile probes fail-as-expected; P0c 24/24 surfaces; ADR-0064 landed |
| **P1** | capture: browser recording → blob store | XDG datadir store; chunked GM upload; GET 404/0-bytes; DELETE; WS via `vault.changed`/`note.saved` broadcast; `CaptureRecorder.svelte` + consent exact-string; 12/12 cross-token |
| **P2** | transcription: interface, local engine, custom engines | `Transcriber` interface; mock/sidecar/external engines; `transcript_segments` table; lexicon; `ResolveEngine`; routes; 5/5 probes + WS text-leak probe clean |
| **P3** | session logs from transcript | `SessionReview.svelte`; `UpdateTranscriptSegment`; correction→lexicon; extractive proposal into `sp:begin/end-session-log`; ownership guard; 5/5 pinning groups |
| **P4** | mood-driven music from local files | `internal/mood` + TS mirror; in-memory cues; server mood endpoint; persisted `.demitable/cues.json`; server seat-checked select; 13 Go + 8 TS table-driven; playback no-fetch asserted |

`make lint` clean; `go test ./...` 27 packages ok, 0 FAIL (the
pre-existing `TestInsertPerformance` load flake passed throughout). e2e
shadow matrix **45/45** across Chromium, Firefox and WebKit (Xvfb). No
`fixtures/` committed. No note content, message body, or credential in any
new log line. No `docs/decisions/` edits by workers (one ADR landed by
the orchestrator).

---

## The findings

### 1. api.md had drifted from the shipped webapp

P0a's conformance pass found the contract stale against the webapp the
rework had shipped: eight committed checks, all 8/8 green after the pass.
The staleness was the kind that compounds — a route documented as one
shape while the handler shipped another — and it is what made the later
P4 "two-sentence contradiction" (finding 8) possible: a contract nobody
re-reads is a contract nobody checks.

### 2. Guest chat authorship was unruled until phase 5

Phase 4's top open item (its finding 10): `createNote` authorized any
non-anonymous principal with no campaign-membership check, so a
(non-member) guest could create notes anywhere path-traversal allowed.
Reads stayed projection-governed (low severity, no leak), but the rule for
who may speak in which channel was never specified. P0b closed it with a
rule that reuses the existing mechanism instead of inventing one: creation
requires the caller to see the channel's audience, via `vault.Visible`,
no new mechanism. Seven tests, hostile probes fail-as-expected. The
deliberate non-smuggling (phase 4 refused to fold it into F3) paid off:
the rule landed as its own decision, with its own evidence.

### 3. The SURVEY inventory found ADR-0052 residue — and four fail-open surfaces

P0c inventoried 24/24 surfaces and found the residue of ADR-0052's
ordered-rules language surviving ADR-0057's audience model: twelve
locations across ADR-0052, design §5.4, threat-model §3, api.md §10 and
`VisibilityInspector.svelte` still spoke of ordered rules and rule chains.
Worse, four surfaces (`VaultExplorer`, `NoteEditor`, `Home`, `NoteContext`)
read `effective_visibility` directly with a fail-open fallback — a missing
projection rendered a permissive label. Only `PlayerView`, `Tabletop`, the
`VaultExplorer` badge and the dashboard counts went through the canonical
`surfaceVisibility()` / `assertVisibilityAgreement()` helper.

ADR-0064 landed the amendment: D1 (the inspector explains constraints, not
a rule chain), D2 (surfaces go through the canonical helper or record
why not), and D3 (a missing projection renders fail-closed — `gm`, never
`party`/`public`). The four fail-open fallbacks are defects against D3;
fixing them is follow-up work, not a new milestone. Two design stale-ref
fixes landed in the same pass. The audit count: 24/24 surfaces surveyed.

### 4. P1's POST and DELETE gates were vacuous

P1's first delivery added the blob store, chunked GM upload, GET/DELETE
and the WebSocket broadcast, and the suite passed. Verification then found
the POST and DELETE gates **vacuous**: mutations passed the suite because
the tests never exercised the guards with real tokens, and the WS test was
an empty placeholder. The same shape as phase 4's finding 13 (empty
credentials hiding behind `if token != ""`), recurring under a different
label.

Fix `166c54b` added hostile tests plus a real WS test plus a LAN grep pin
plus a single broadcast path, and re-verification went claim-by-claim. The
numbers after: 12/12 cross-token (404 + 0 bytes), traversal battery
clean, GET 404/0-bytes for an excluded caller. **A predicate measured
once by hand and then described in a report is not a predicate** — phase 4
finding 2, recurring.

### 5. P2's import cycle broke the whole tree

P2's first delivery (6346d46) added the `Transcriber` interface, mock/
sidecar/external engines, the `transcript_segments` table, the lexicon and
`ResolveEngine`, and routes. Verification found a **blocking import
cycle**: `transcribe → vault`, `session → transcribe`, `vault-tests →
session`. The result was that `make test` and `make lint` were red
tree-wide — not in one package, everywhere — because the cycle touched the
core seams.

Fix `246b7e1` moved vault-walking into `internal/vault/lexicon.go` behind
a `LexiconSource` interface, breaking the cycle at the seam rather than at
the call site. Merged as `f128714`. The lesson: a worker per sub-milestone
owns the seam collision; the merger owns the resolution, and only when the
conflict is structural, not when it is a typo.

### 6. P4 had a second mood mechanism

P4's first delivery (fd615d7) shipped `internal/mood` plus a TS mirror,
and the TS side computed mood client-side in `Tabletop.svelte:24` while
the Go `Derive` had **zero callers**. That is two mechanisms for one
value, and it violates three things at once: the one-mood-function rule,
api.md §4 ("server computes"), and the one-mechanism rule the phase-4
brief named after the SQL-vs-in-memory burn.

The rework (9cc30e0) moved mood to a server endpoint, persisted the cue
library in `.demitable/cues.json`, added a server seat-checked select, and
reduced the TS side to a stub. The mechanism count went from two to one.
The TS mirror survived the first pass because "server computes" was treated
as internalized knowledge rather than a written rule in the brief (finding
17).

### 7. P4b's core was missing

Alongside the second mechanism, P4's first delivery never populated cues:
`cues=[]` was returned and nothing ever filled it. The rework added the
server mood endpoint, the persisted `.demitable/cues.json`, the server
seat-checked select and the TS stub — the core that the first pass omitted
while claiming the milestone green. A milestone whose "Done when" is
"mood drives cues" and whose code returns an empty slice is a milestone
that was not tested.

### 8. P4's 747-line mood file had zero Go tests

After the rework, `p4_mood.go` (747 lines) still had **zero Go tests**;
two mutations were vacuous — the suite stayed green with the guards
removed. Pinning (ab868e7) added four hostile Go tests: projection, select
403s, other-session 401, unknown-404. Each was FAIL-probed with the guard
removed before being accepted.

A second, smaller block then surfaced: a two-sentence contradiction in
api.md about whether a cue GET for another session is 200-projected or
401. The doc fix (41a0a44) settled it as 200-projected. The orchestrator
merged P4 with a `main.go` route-registration conflict (P2 + P4 wiring)
resolved by keeping all three registrations; merge `b2323d2`. Post-merge
smoke: live boot, projection holds, suite green.

### 9. P3's proposal was not revertible

P3's first delivery (9eff4af) added `SessionReview.svelte`,
`UpdateTranscriptSegment`, correction→lexicon, the extractive proposal
into `sp:begin/end-session-log`, and the ownership guard. Verification
found one blocking defect: `p3DoProposal` wrote through raw `os.WriteFile`
with a record-only journal — **not revertible** — with no closed-409, an
empty digest-mismatch branch, and no `note.saved` broadcast; review on a
closed session silently returned 200. Plus containment gaps (vault escapes
GM-only; the term unquoted) and vacuous speaker-persistence.

Fix `1f9e879` made the code structurally right — journaled revertible
proposal, closed-session guards, containment, digest rejection — **but
added zero tests**, so all five mutations stayed vacuous. The pinning round
(e66c660) added five test groups (revert-restores-bytes, closed-409s,
digest-mismatch, containment, speaker-persistence), each FAIL-probed.
Verified merge-ready; merged `657a7b4`.

### 10. The final gate failed on docs only

After P3 merged, the whole-tree gate ran: `make lint` exit 0, `go test
./...` 27 ok / 0 FAIL (TestInsertPerformance passed throughout), e2e
shadow matrix 45/45. The gate then failed **narrowly on docs**: SURVEY
missed two phase-5 rows, and api.md left the P1–P3 routes undocumented.
Fix `f9f549f` added +21 route rows and 2 SURVEY rows, verified
row-by-row against the handlers; merged `2c47242`. The code was green; only
the paperwork was red. That is the best kind of gate failure, and it is
still a failure.

### 11. P2's download story is constants and a missing message

P2's download story landed as **constants plus a missing-message only** —
honestly disclosed, not papered over. Per-campaign retention and
auto-delete-after-review are proposed but not built. The honesty is the
finding: a milestone that ships a partial story must say so in its own
report, and P2 did. The remainder is carried (finding 15).

### 12. ADR-0064 D3's fallback is still live

ADR-0064 D3 (a missing projection renders fail-closed) made `Home.svelte`'s
`?? "party"` a defect on paper, but the fix is P0-scope follow-up work
that was not closed in-phase. The fallback is still live in the tree at
merge time. A rule written and not enforced is a rule that has not yet
been tested; the pinning is the remaining work.

### 13. SessionReview's promote() is unwired

`SessionReview`'s `promote()` is not wired to anything. The review UI
exists; the act of promoting a reviewed transcript into the session log's
marked section is not reachable from it. Recorded, not fixed — the
structural code landed (finding 9), the wiring did not.

### 14. Segment text sits at rest in session-log-proposals.json

The session-log proposal persists segment text at rest in
`session-log-proposals.json`. That is a recorded note, not a leak under
the current threat model, but it is a new at-rest surface that the
projection and logging rules (ADR-0058) were not written for. It needs its
own decision before anyone treats it as safe to widen.

### 15. Cue and mood disagree across sessions

Cue and mood handling is asymmetric across sessions: documented, not
harmonized. The two surfaces (P4's cue library and mood derivation) were
built in one milestone but not reconciled for the cross-session case. A
documented asymmetry is a known defect with a name; an undocumented one is
a latent one.

### 16. No real-browser audio verification; no e2e specs for the new surfaces

There is no real-browser audio verification — capture, transcription and
cue playback were exercised in jsdom/Xvfb and by Go tests, never in a
browser that can actually open a microphone or play audio. And `e2e/`
holds only `shadow-host.spec.ts`: there are no specs for capture,
transcription, review or cues. The shadow matrix stayed 45/45, but it
tests the shadow host, not the phase-5 surfaces. Both are recorded gaps,
not green ticks.

### 17. "Server computes" was assumed internalized

The TS mirror in P4 survived because api.md §4 was assumed internalized by
the builders and the brief did not restate it. The one-mechanism rule was
named in the phase-4 brief; "server computes" was not repeated in the
phase-5 brief, and the second mechanism was the price. **A rule that lives
in an ADR and not in the brief that governs the wave does not govern the
wave.**

### 18. Code-only fixes cost a round

P3's fix (`1f9e879`) made the code structurally right and added zero
tests, so all five mutations stayed vacuous and a pinning round (e66c660)
was required to make them real. The cost was a full fix cycle that a
test-bearing fix would not have needed. **A fix brief that does not name
the test files it must produce produces no tests.**

### 19. One worker per seam, not per sub-milestone; never compound shell commands around git merge

Two process findings, both paid for in merge time. (a) P2 and P4 both
wired routes in `main.go`; the collision was owned by the merger and
resolved by keeping all three registrations, but it existed because two
workers owned the same seam. **One worker per seam.** (b) A typo in a
compounded shell command around `git merge` masked a `MERGE_HEAD` state;
the conflict was genuine and was resolved, but the masking made it look
like a no-op. **Never compound shell commands around `git merge`.**

---

## What was measured

| | measured | criterion |
|---|---|---|
| P0a conformance | **8/8** checks | committed |
| P0b guest authorship | **7/7** tests; hostile probes fail-as-expected | committed |
| P0c SURVEY coverage | **24/24** surfaces; 2 design stale-ref fixes; ADR-0064 landed | committed |
| P1 capture | GET **404, 0 bytes**; 12/12 cross-token; traversal battery clean; real WS test | hostile |
| P2 transcription | **5/5** probes + WS text-leak probe clean | committed |
| P4 mood/cues | table-driven **13 Go + 8 TS**; playback no-fetch asserted | hostile |
| P3 pinning | **5/5** groups (revert-restores-bytes, closed-409s, digest-mismatch, containment, speaker-persistence), each FAIL-probed | mutation-probed |
| cold suite | `make lint` exit 0; `go test ./...` **27 pkgs ok, 0 FAIL** | green |
| e2e shadow matrix | **45/45** (Chromium + Firefox + WebKit, Xvfb) | green |
| TestInsertPerformance | passed throughout (pre-existing flake, not triggered) | green |

---

## The ADRs landed

| | |
|---|---|
| **ADR-0064** | The UI reads audiences, not ordered rules (amends ADR-0052 and design §5.4; D1 inspector explains constraints, D2 canonical helper, D3 fail-closed missing projection) — evidence: P0c inventory (SURVEY 24/24), phase-4 report's proposed-but-not-written item |

Proposed but not written (each needs its own decision pass): per-campaign
retention and auto-delete-after-review (P2 download story); amending
ADR-0064 D3's fallback sites, of which `Home.svelte`'s `?? "party"` is
the live one (finding 12); a decision on `session-log-proposals.json`
segment text at rest (finding 14); harmonizing cue/mood cross-session
asymmetry (finding 15); a real-browser audio verification gate (finding
16); e2e specs for the phase-5 surfaces (finding 16). Plus the
phase-4 carried items that remain open: display rotation/revocation, the
W3 overlap (gm seat vs GM bypass), the pre-existing hardening seams
(`verifyToken` HMAC non-constant-time, unjournaled `SaveNote`/GM PATCH,
advertised-but-ignored WS commands, in-memory limiter reset on restart,
`TestInsertPerformance`'s 250 ms bound), the macOS watcher and session-tier
numbers, the `requires.app` range language, the dashboard third entity, the
demo session wing, and the out-of-scope set (MCP write tools,
out-of-process plugin tier, native SDK) which stays out per
ADRs 0014/0020 and api.md §9a.

---

## What was cut, deferred, or left undone

- **P2 download story, partial.** Constants plus a missing message only;
  per-campaign retention and auto-delete-after-review proposed, not built
  (finding 11). Owner: phase 6.
- **ADR-0064 D3 fallbacks.** `Home.svelte`'s `?? "party"` is still live;
  the other three fail-open sites need the same fix (finding 12).
- **`SessionReview.promote()` unwired.** Review UI exists; promotion is not
  reachable (finding 13).
- **Segment text at rest** in `session-log-proposals.json` (finding 14).
- **Cue/mood cross-session asymmetry**, documented not harmonized
  (finding 15).
- **No real-browser audio verification** and **no e2e specs for the new
  surfaces**; `e2e/` holds only `shadow-host.spec.ts` (finding 16).
- **Pre-existing seams** (phase 4 finding 14 + verifyToken/grant-nit list):
  recorded at each finding site, none blocking, none widened by phase-5
  code.
- **macOS watcher + session-tier numbers**: still need a Mac. Fifth report
  running. Recorded, not forgotten.
- **`requires.app` range language, dashboard third entity, demo session
  wing**: carried open items, unchanged.
- **MCP write tools, out-of-process plugin tier, native SDK**: stay out
  (ADRs 0014/0020, api.md §9a). First request reopens the list.
- **Guest chat authorship rule**: CLOSED in P0b (finding 2).
- **ADR-0052 amendment**: CLOSED by ADR-0064 (finding 3).

---

## What phase 6 should start with

Phase 6 is not a numbered phase in the phase map — phase 5 is the last one
— so it is maintenance and closure, and its floor is the deferred list
above. Start with the items that are rules already written but not yet
enforced: close the four ADR-0064 D3 fallbacks (finding 12) so the
fail-closed rule is tested, not just stated. Then wire `promote()`
(finding 13) and add the e2e specs the shadow matrix does not cover
(finding 16), because a surface with no spec is a surface that regresses
silently. Then take the deferred P2 items — per-campaign retention and
auto-delete-after-review — as a small, self-contained milestone with its
own ADR, and settle the segment-text-at-rest question (finding 14) before
anyone widens that surface.

Second, the pre-existing hardening seams (phase 4 finding 14 and this
report's finding 12's neighbours): `verifyToken`'s non-constant-time
comparison, the unjournaled `SaveNote`/GM PATCH paths, the advertised-but-
ignored WS commands, the in-memory limiter's reset on restart, and
`TestInsertPerformance`'s 250 ms bound with its tmpdir ordering fragility.
None is blocking; all are recorded where found; none was widened by
phase-5 code. They are the cheapest security and reliability wins left.

The contract phase 5 wrote into held through the phase without amendment —
the blob store, the transcript segments, the cue library, the mood
endpoint, the proposal's marked section — which is the strongest validation
this phase can offer the next one, and it is why the deferred list is
short and named rather than long and vague.

---

## What I would do differently

- **Put "server computes" in every brief.** The TS mirror survived because
  api.md §4 was assumed internalized; the one-mechanism rule was named in
  the phase-4 brief but "server computes" was not repeated in the phase-5
  brief, and the second mechanism was the price (finding 6, finding 17).
- **Require FAIL-line evidence and test files in the fix brief.** P3's
  code-only fix (`1f9e879`) cost a full pinning round (e66c660); a fix
  brief that names the test files it must produce produces tests, and one
  that does not produces none (finding 18).
- **One worker per seam, not per sub-milestone.** P2 and P4 both wired
  `main.go` and the collision landed on the merger; two workers owning one
  seam is a merge conflict waiting for a merge (finding 19a).
- **Never compound shell commands around `git merge`.** A typo masked a
  `MERGE_HEAD` state and made a genuine conflict look like a no-op; the
  conflict was resolved, but only after it was mistaken for nothing
  (finding 19b).

---

## A note on how the phase ran

Five milestones (P0 with its three sub-milestones, P1, P2, P3, P4), serial
fallback discipline where quota forced it, investigation-model verification
throughout. The review found what the workers did not — the vacuous gates,
the import cycle, the second mood mechanism, the missing core, the
unrevertible proposal, the docs-only gate: none of them was in a worker's
report. Every one was found by reading the diff and running the code. The
zero-tests pattern recurred (P1, P3) and was caught both times by the
same standard; the mutation-probe discipline (phase 4 finding 2) is the
reason the suite is green rather than merely passing.

That is the whole argument for the review step, and it is why this
document is longer about what was found than about what was built.

---

## Wave/merge record (for the log)

P0a → merged `3ccd136` · P0b → merged `44f8fd6` · P0c → merged
`311b50d` · ADR-0064 → merged `f39959d` · P1 → merged `bb2e98e` ·
P2 → merged `f128714` · P4 → merged `b2323d2` · P3 → merged `657a7b4` ·
docs → merged `2c47242`. All merges `--no-ff`. No pushes during phase
(pushed after, on request).