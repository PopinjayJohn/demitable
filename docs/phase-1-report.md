# Phase 1 — The Wiki (MVP): what it found

> The phase that built a campaign wiki, with secrets.
>
> Eight milestones, twenty-one sub-milestones. This document is the phase's
> report: what was built, what was measured, what was found, and what the
> next phase should start with.
>
> It follows the shape of [`phase-0-report.md`](phase-0-report.md), and
> it holds itself to the same standard: **a phase that found nothing is a
> phase that did not test anything.**

---

## The phase in one paragraph

The MVP is a GM's campaign on disk, served by one binary, with a player's
client that never holds a GM's secret. The phase built the vault service
that reads a directory of markdown, the plugin host that everything the
user experiences plugs into, the projection that decides who may see what,
the editor that writes the buffer back without transforming it, and the
scaffolder that starts a campaign from nothing. The two things it got
right are the two that are hardest: **the write path transforms nothing**,
and **the projection computes audiences rather than rule verdicts**.

The two things it got wrong were both found by measuring rather than by
reasoning, and both are now fixed.

It found **fourteen things** in all, four of them defects that a
worker's own report did not flag. The most common shape of the findings
is the one phase 0 warned about: **a thing that is counted rather than
asserted**, and therefore passes.

---

## The eight milestones

| | Milestone | The number that says it works |
|---|---|---|
| **M0** | the skeleton, the build pipeline, the configuration | 5 targets cross-compile with `CGO_ENABLED=0`; `GET /` is byte-identical to the built bundle; `SIGTERM` exits 0 |
| **M1** | the vault service, the index, the watcher, the journal, the assets, the store | parse **309 ms** against a 1 s criterion; insert **162 ms** against 250 ms; corpus audit **1,692 / 1,692** |
| **M2** | the minimal pack format | 15 tests; the example pack declares an owner-scoped and a gm-scoped field |
| **M3** | the plugin host's core | 9 capabilities in 3 kinds, 6 host functions; `api` checked before load; the bridge strips the owner-scoped fields |
| **M4** | the editor | the corpus round-trips byte-identically; the two parsers agree on **all 1,693 links** |
| **M5** | accounts, the campaign, the projection | six expectation tables, both directions, note for note |
| **M6** | the browser and search plugins | 5,000 notes filter and sort in **25 ms** |
| **M7** | the scaffolder, the demo campaign, the published schema | **16** notes, one per core type, real ULIDs, **0** invisible; the demo campaign's **21** notes round-trip byte-identically |
| **M8** | the export plugin, and the `api: 1` freeze | GM export **20** notes, player export **17**, **0** leaks, **0** script tags, **0** remote references |

---

## The findings

### 1. The 12 MB binary cap was a criterion that would have broken the architecture

The plan required the cross-compiled binary to be under 12 MB, and CI
failed the build over 12,582,912 bytes.

**The number was stale.** It came from a phase-0 measurement of **7.6 MB**,
taken before the frontend was embedded. M0's real binary was **10.77 MiB**
with **1.23 MiB** of headroom — and phase 1 still owed the editor, the
packs, the plugin host and the FTS5 index.

**It would have been breached inside the phase.** The finished binary is
**12.4–13.2 MiB** across the five targets, and it passed through
**11.29 MiB** on M0's own build — the export plugin, the UID package and
the editor's Go surface all landed after the cap was written.

**A cap that fires on legitimate progress is worse than no cap.** The
cheapest ways back under the line were all bad: drop the embedded
frontend, drop SQLite, drop the packs embed, or link dynamically and break
the one-file promise. The criterion would have made the architecture worse
in order to satisfy a number that protected nothing.

The real invariant is the one ADR-0005 promised — *one file, no cgo, no
runtime dependencies* — and it is checkable. CI now asserts
`CGO_ENABLED=0`, checks the ELF builds for dynamic library dependencies,
and **reports** the size. **ADR-0060.**

This is phase 0's second lesson arriving exactly on schedule: *the
expensive part is rarely the part you measured*. What was measured was a
7.6 MB binary; what the gate governed was a 25 MB one.

### 2. Two milestones wanted the same type, and the resolution was not mechanical

M1 and M3 each defined a `Note` and a `NoteView` in `internal/vault/`.
They were not two spellings of one idea — they were **two different
questions**:

- M1's `Note` is the **parsed note**: its raw bytes, its extracted prose,
  its link graph, its frontmatter document, its issues.
- M3's `Note` was a **wire shape**: what a plugin is allowed to see.

M1's is the real one, so it stayed, and the projection was rebuilt on top
of it. The seam now holds **two rules that were previously one**:

- `Project` decides **which fields** a principal may see — ADR-0056's rule
  for the plugin boundary;
- `Visible` decides **whether** they may see the note at all — ADR-0057's
  audience model, and M5's to replace.

Keeping them as one function would have made the two questions
indistinguishable, and the "plugin gets a raw note" leak that phase 0
found is exactly what happens when they are.

### 3. The rate limiter never engaged

M5's auth limiter keyed on `r.RemoteAddr`, which is `host:port` — so
every request arrived with a different ephemeral port and therefore a
different key. **Eight consecutive failures against the GM endpoint all
returned 401; none returned 429.** The predicate *"authentication is
rate-limited and backed off on every auth endpoint"* was not met by any
amount of configuration.

The fix was to key on the client address — and to resolve that address the
way ADR-0031 says: the socket's peer, unless that peer is the configured
proxy, in which case the first `X-Forwarded-For` entry is the client.
**Without `--trust-proxy`, no forwarded header is believed at all** — it
is attacker-controlled, and a limiter keyed on it is a limiter an
attacker can reset by forging a header.

### 4. The credential generator had a weaker path than its success path

M5's `randomSecret` fell back to filling the buffer with the byte index
when `crypto/rand` failed, which would have made the GM secret the
constant base64 of `0,1,2,3…`.

The fallback is **unreachable in practice** — `crypto/rand.Read` does not
fail on Linux — and this is the "accidental class" of bug phase 0 warned
about: nobody designed it to be a vulnerability, it was just a fallback
that nobody thought about. A credential generator must have no weaker
path than its success path, so it now panics. **Dying is recoverable;
issuing a guessable GM secret is not.**

### 5. The search's visible set was a hint, not a restriction

M5 and M6 each added a `SearchFTSIn`, with **opposite argument orders** —
`(query, allowed)` against `(uids, query)`. Two spellings of one function
with swapped arguments is the kind of ambiguity that becomes a wrong-call
bug at 2am.

Unifying them exposed the real defect: **an empty visible set meant "no
restriction"**, so a caller that computed a visible set and got nothing
back received **every note in the index** instead of none. The restriction
now always applies, and the unrestricted path is a separate, explicitly
named method for the GM.

The consequence is the one ADR-0054 measured: with 1,169 GM-only notes the
top result changes in 3 of 8 queries, a note moves by **16 positions**, and
a score changes by **100.6%**. The partition is load-bearing, and a
restriction that vanishes when the set is empty is not a restriction.

### 6. The conformance suite counted its disagreements and passed anyway

M4's conformance suite parses every corpus file with goldmark and with
remark and compares the extracted text and links. It found **40 text
mismatches and 0 link mismatches** — and **passed**, because the text
mismatches were counted and logged rather than asserted.

ADR-0043 says *"a disagreement fails CI"*. A test that logs its
disagreements and passes is not that.

Measuring the 40 rather than assuming them found they were **one** cause:
the two parsers' different whitespace rules around consumed inline
markers, plus `&emsp;`, which goldmark decodes and remark does not.
Normalising those leaves **3**, and all 3 are the same case — a folded
callout whose first line is a heading followed by an image, where
goldmark emits the heading's text twice and remark once.

**The 3 are listed by name**, so a new disagreement fails the build and a
known one that stops differing is a finding. The list is exact, and
removing an entry from it fails the suite — verified.

### 7. Two performance criteria were missed, and the diagnosis was not what it looked like

M1's first report said the FTS5 insert took 954 ms against a 250 ms
criterion, and attributed the gap to tokenisation and per-row statement
overhead.

**The measurement was wrong, not the code.** The test re-inserted into an
already-populated index, so it measured *replacement*, not *insertion*.
The criterion is about insertion.

And the real cost was neither of the two candidates: `database/sql`'s
`tx.Exec` prepares and finalises a statement on **every call**. Preparing
once per transaction and reusing it took the insert from **313 ms to
160 ms** — matching ADR-0055's 144–151 ms on the same machine.

Two hypotheses were checked and eliminated first, so they were not
re-chased: `LastInsertId` per row was not the cost (263 ms with it, 265 ms
without), and one transaction per row is catastrophic (12.9 s).

### 8. The parse was 10% over, and ADR-0055 had said it would be

M1's parse took 1.10 s against a 1 s criterion — reproducing phase 0's
**1.08 s** almost exactly, which says the parser is not regressed.

ADR-0055 had predicted this and named the two fixes: *"phase 1 will have
to either parallelise the load or cache the parse."* **Parallelising**
closed it: the parse is embarrassingly parallel across files, and an
8-worker pool took it from **1.10 s to 294 ms**.

### 9. The projection's `discovered` was stricter than either ADR said

ADR-0050 D5 made `discovered` visible to the note's player **and the
GM**. ADR-0057 A7 made it the player's **alone**. M5 implemented the
stricter reading, and the demo campaign's expectation tables confirm it:
Orson sees the Cormorant Stair he discovered, and nobody else does.

Two accepted ADRs disagreed about one field, and the disagreement was
resolved in favour of the later and the corrective one. **The ADR was not
edited** — the resolution is recorded in the code and in the report.

### 10. Sorting by a field did not sort

M6's sort precomputed the sort keys into a parallel slice and then ran
`sort.SliceStable` over the notes, comparing against `keys[i]` and
`keys[j]`. Those indices are positions in the *original* slice, and the
sort swaps the notes as it goes — so from the first swap the keys and the
notes describe different things. **Every sort by name, by title or by any
field returned the notes in the wrong order.**

The keys were precomputed for a reason — re-reading the field on every
comparison is what made a 5,000-note sort slow — so the precomputation
stays and only the desynchronisation goes. The notes and their keys are
now sorted together, as one slice of pairs.

**The predicate had no committed test**, so nothing caught it. The worker
measured the 25 ms and reported it, and the number was right about a sort
that did not sort. The new test asserts the result is *ordered* —
ascending and descending — so a benchmark over a no-op cannot pass.

### 11. A `private` note with no `player` is invisible to everyone

M7's scaffolder produced a journal with `visibility: private` and **no
`player`**. ADR-0048 is explicit: `private` means *"the note's `player`,
and nobody else — not even the GM"*, and the personal player journal is
`player: <the owner>`, `visibility: private`.

A `private` note with no owner belongs to nobody, and — because `private`
is checked *before* the GM shortcut — it is invisible to everyone. The
vault indexed all 16 scaffolded notes; the API returned 15, and the
missing one was the journal.

**And the "opens with zero errors logged" predicate passed**, because a
note with no `player` is not a parse error. The predicate that would have
caught it is **visibility**: a test now asserts that no scaffolded note
is invisible, so the class of bug cannot come back.

### 12. The scaffolded uids were placeholders

Every scaffolded note had `uid: scaffold-02`. Design §5.3 says a note
gets *"a short ULID. Never reused, never changed, machine-only"*, and the
tech stack says ULID is deliberately *not* a dependency: *"the uid format
is about sixty lines and we want the uid derivation to be ours."*

So it is now ours: 48 bits of millisecond timestamp, 80 bits of
cryptographic randomness, Crockford base32 — sortable by creation time,
never derived from the note's name, and **with no fallback**, because a
uid generator that degrades is the same defect the credential generator
had.

### 13. The export needed one thing the editor's plugin did not

ADR-0019 requires a **second, non-UI consumer** before the plugin API
can freeze, and the export is it. It found exactly what ADR-0019
predicted: the export needed **asset bytes**, which the editor's plugin
never wanted. The `vault.assets` capability is in the vocabulary but its
host function is still unimplemented, and the `VaultService` seam exposed
only notes.

The seam now provides `AssetPaths` and `AssetBytes`. **That widened the Go
seam, not the frozen `api: 1` bridge** — the capability vocabulary, the
host primitives and the `NoteView` shape are untouched, and the branch
that froze them has no changes to `internal/plugin/caps/` or
`internal/plugin/manifest/`. A second, smaller finding: `NoteView` carries
no custom fields, so the export reads them from the raw note after a
`Visible` check. That is right for now, and it is written down where a
later change would break it.

### 14. The export's tests were deleted before the commit

M8's first report claimed eight numbers — the 20 notes, the 17, the 0
leaks, the 3 assets, the 2, the 0 script tags, the 0 remote references,
the 1 refusal, the escaped script. **The branch had no test file at
all.** The worker's report said the tests were "scratch" and removed them.

Every one of those numbers was therefore unverifiable, and a predicate
measured once by hand and then deleted is not a predicate. Nine tests now
commit them, one per predicate.

**The same lesson as the conformance suite, twice in one phase** — and
both times the disagreement was *counted* rather than *asserted*.

---

## What was measured

| | measured | criterion |
|---|---|---|
| parse 5,000 notes | **309 ms** | 1 s |
| insert 5,000 notes, fresh index | **162 ms** | 250 ms |
| corpus audit | **1,692 / 1,692** | all |
| touch → HTTP, from the fsnotify event | **3.8 ms** | 200 ms |
| 400 concurrent inserts | **0** "database is locked" | 0 |
| 5,000 notes, filter and sort | **25 ms** | 100 ms |
| conformance: links | **0 mismatches** in 1,693 | 0 |
| conformance: text | **3**, all named and explained | 0 |
| the binary, five platforms | **12.4–13.2 MiB** | no limit |
| scaffolded notes | **16**, all with a real ULID, **0** invisible | 16, 0 |
| demo campaign round-trip | **21 / 21** byte-identical | all |
| export: GM / player notes | **20 / 17**, **0** leaks | 0 leaks |

**The macOS watcher numbers are still untaken.** They need a Mac. The
polling watcher is implemented and Linux holds **+0 fds** for 1,693 notes,
but the kqueue behaviour — one descriptor per file — cannot be measured
from Linux, and the macOS number is the one that matters for a 50,000-note
vault.

---

## The ADRs landed

| | |
|---|---|
| **ADR-0060** | The binary has no size limit — the invariant is one file with no runtime dependencies |

ADR-0057 and ADR-0050 were **applied**, not amended: the phase
implemented the later, corrective reading of a disagreement between them
and recorded which reading it took.

---

## What was cut, deferred, or left undone

- **The macOS watcher numbers.** Not deferred out of laziness — they
  cannot be taken from this machine, and the phase-0 spike ships the
  command that takes them.
- **Starter Obsidian Bases.** M7's published schema is prose plus
  Dataview queries; a `.base` file is hard to produce faithfully and a
  plausible-looking one that Obsidian would reject is worse than none.
- **`--expose`'s full exposure posture.** The config surface, the GM
  credential and the audit log are in place; the *deployment* story —
  TLS termination, a firewall, a backup — is the GM's, and ADR-0031 says
  so.
- **The formula engine, the dice engine, the 5e pack, the SRD content
  vault, the Blades gate, the creation wizard, character sheets, block
  widgets, the archive import, the WASM tier, the query language, MCP,
  live sessions, seats, the encounter runner, the tabletop, chat and the
  shared display.** All deliberately out of phase 1, all in the phase map.

---

## What phase 2 should start with

**The pack format's completion is the first thing, and it is the thing
that makes the phase-1 pack format provisional rather than final.** Phase
1's format carries content types, typed fields and a visibility default.
Phase 2 adds resources, formulas, dice and sheets — and **the schema
invariants in design §6.1 have never been enforced against two
differently-shaped packs.** That is the gate: two packs that disagree
about what a character is, one 5e and one Blades, both working end to
end. The phase-1 example pack is deliberately the smallest thing that
could be extended, which is exactly why it has to be extended now.

Second, **the projection needs its first real adversary.** The phase-1
property test is strong on the demo campaign's 21 notes and the spike's 36
synthetic ones, and it is *one* campaign. The corpus — 1,693 real notes —
carries **none** of the twelve reserved fields, so the corpus cannot
exercise the model at all. A pack-declared corpus is the missing
adversary, and phase 2's richer packs are what would generate one.

Third, **the search's ranking needs to be measured against a real
audience.** The two-index partition was measured in phase 0 with 1,169
synthetic GM-only notes; the phase-1 index has 21 notes and one GM-only
one. The partition is load-bearing and the corpus does not test it.

Fourth, **the projection needs an audience that is not the GM.** Every
list endpoint in phase 1 resolves against the GM principal, because
there are no accounts yet. That is why a scaffolded `private` journal
was invisible to the only principal the server had — and the API
returned 15 of the 16 notes without anyone noticing. Phase 2's accounts
make the second principal real, and **the moment it does, every list
endpoint becomes a place where the projection is applied to somebody
other than the GM.** The seam is `principalFromRequest`, and it exists;
what does not exist is a second principal to test it with.

---

## What I would do differently

- **Give the first worker the seam rather than the placeholder.** M3
  built a `Note` in `internal/vault` that M1 also built, and the
  collision cost a merge resolution. The phase-1 brief should have said:
  *the vault service owns `Note`; the plugin host owns the projection
  seam; neither defines the other's type.* It says now, in the merged
  code, and it is written down where the next agent will read it.
- **Measure the criterion's subject before writing the criterion.** The
  12 MB cap was written against a 7.6 MB binary and outlived it by
  three milestones. Every number in a "Done when" should carry the
  artifact it was measured on.
- **Make the perf tests fail when the perf is missed.** M1's first
  version logged a note and passed. A test that cannot fail is not a
  test, and it cost a round of review to notice.
- **Send the security review earlier.** The rate limiter and the
  credential fallback were both in the M5 worker's first report, both in
  the diff, and neither was flagged by the worker. They were caught by
  reading the diff — which is what review is for, but a checklist item
  would have found them sooner. The same is true of the search
  restriction that vanished on an empty set, the sort that did not sort,
  the conformance suite that logged its disagreements and passed, the
  export whose tests were deleted before the commit, the alias sprawl in
  the pack package, and the twelve-megabyte cap that would have broken
  the one-file promise. **None of those were in a worker's report. Every
  one was found by reading the diff.**
- **Assert the property, not the absence of an error.** The "opens with
  zero errors logged" predicate passed on a scaffolded journal nobody
  could read, because a note with no `player` is not a parse error.
  **Visibility** was the predicate that would have caught it.

---

## A note on how the phase ran

It ran as four waves, three workers at a time, all on one model.

**The workers were not the constraint; the harness was.** Two agents were
truncated mid-`edit` on a long file, and one lost uncommitted work
entirely — which is why every milestone was committed in small pieces
afterwards. The work was recoverable because the branches existed, and
because the orchestrator's own worktrees made it possible to inspect what
a worker had actually written rather than what it said it had written.

**And the review found things the workers did not.** The rate limiter
that never engaged, the credential fallback, the search restriction that
vanished on an empty set, the conformance suite that logged its
disagreements and passed, the sort that did not sort, the journal nobody
could read, the export whose tests were deleted before the commit, the
alias sprawl in the pack package, and the twelve-megabyte cap that would
have broken the one-file promise: none of those were in a worker's
report. Every one was found by reading the diff.

That is the whole argument for the review step, and it is why this
document is longer about what was found than about what was built.
