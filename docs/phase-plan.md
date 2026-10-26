# The phase map

> What is in each phase, and what moved. Canonical.
>
> Superseded the phase descriptions in [`design.md`](design.md) §7 and the
> thirteen-milestone plan in [`phase-1-plan.md`](phase-1-plan.md).

---

## Why the phases were re-cut

The plan grew. Thirteen milestones became thirty-seven, and the honest estimate
was **~70–90 weeks full-time, or ~3.5–4.5 years part-time, for phase 1
alone** — a phase that was supposed to be the *quick* one.

So phase 1 is now an **MVP**: the smallest thing that is genuinely the
product, rather than the smallest thing that is merely a foundation.

### What the MVP is

> **A campaign wiki, with secrets.**
>
> A GM opens their vault and browses it as a typed campaign database, edits
> it in a good markdown editor, and gives their players a filtered view of the
> same campaign. The campaign is the product. Everything else — the rules, the
> table, the recording — is downstream of it.

That is a coherent, useful, demonstrable product on its own. It is also
*distinctive*, because no wiki tool has a trust model: a GM-only note is
structurally absent from a player's view rather than merely hidden.

### What the MVP is not

It does not know what game you are playing. It has no character sheet, no
dice, no encounters and no tabletop. Those are phase 2 and phase 4, and they
sit on top of a schema that phase 1 has already proven against real use.

---

## The phases

### Phase 0 — The Prototype

Six weeks, about sixty hours. Its output is **knowledge, not code**: ten
questions answered, and only the frontmatter extractor promoted into phase 1.

It exists because the design rests on things nobody has built or measured —
the frontmatter extractor, the projection over real data, the plugin API's
actual shape, the interface itself, and the performance numbers. The full
plan is in [`phase-0-plan.md`](phase-0-plan.md).

### Phase 1 — The Wiki (MVP)

| | |
|---|---|
| **Core UI/UX** | The shell, navigation, the editor experience, the frontmatter form panel, the typed views, the dashboard |
| **The vault** | Read, index, watch, write. The literal index, rated search, the write journal and revert |
| **The user system** | Accounts, the campaign join code, the principal model, the projection, the visibility model, the campaign note |
| **Markdown** | Rendering, source editing with live preview, byte-identical round-trip |
| **The plugin host's core** | Registry, capabilities, services, surface hosts, the bridge, the shell. **Compiled-in plugins only** |
| **The pack format, minimally** | Content type declarations with typed fields, and the visibility default. **No formulas, no resources, no dice** |
| **The scaffolder** | A new campaign, the templates, the demo campaign, the published schema and starter Bases |
| **The export plugin** | Campaign handover, and the second plugin that proves the API is not editor-shaped. **`api: 1` freezes here** |

**Note types.** The core declares the campaign's own vocabulary, which is
system-independent: `campaign`, `location`, `faction`, `item`, `quest`,
`session`, `handout`, `message`, `journal`, `table`.

The **pack** declares the system's vocabulary, under `type: content` with a
`content_type` and a `system` (ADR-0033). Phase 1's example pack declares
`character` with a couple of fields. Phase 2's 5e pack declares a rich one,
and `species`, `background`, `feat`, `spell`, `monster` and the rest.

**Deliberately deferred from phase 1** — all of it lands in phase 2 or
later: the formula engine, the dice engine, the 5e pack, the SRD content
vault, the Blades gate, the creation wizard, character sheets, the rich
editor's block widgets, the archive import endpoint, the WASM plugin tier,
the query language, MCP, live sessions, seats, the encounter runner, the
tabletop, chat and the shared display.

### Phase 2 — The System

The rules and the content. The pack format completed — resources,
formulas, dice and sheets — and the machinery that interprets it, plus the
way content arrives.

- The **formula engine** (ADR-0042's layered scope).
- The **dice engine**: tables, faces, symbol dice, pools, opposed rolls.
- The **5e pack**, the **SRD content vault** and the **Blades gate** — the
  format proof (ADR-0012, ADR-0023, ADR-0024, ADR-0036).
- The **creation wizard** and **character sheets**, for the GM and for players
  (ADR-0009's review workflow).
- The **rich editor's content support**: block widgets, callouts, stat
  blocks, the extension registry (ADR-0021, ADR-0027, ADR-0035).
- The **archive import endpoint** — how content arrives (ADR-0025).

> **Done when** two deliberately differently-shaped game systems both work
> end to end, and a GM can import an archive into a campaign.

### Phase 3 — The Openings

The app's boundaries with other programs: what extends it, what can ask it
questions, and what can read it.

- The **WASM plugin tier** and the full host ABI (ADR-0020, ADR-0045).
- The **query language** for `list_by`, shared by REST and MCP (ADR-0047).
- The **MCP plugin** (ADR-0014).
- The **cross-browser harness** for the shadow host, which gates the plugin
  host's UI tier (ADR-0029).
- The **full UI surfaces**: the GM dashboard and whatever else of
  `spikes/ui/` phase 2 did not need — badge, inspector and player view
  land in phase 2 (N6d), the rest lands here, because a dashboard over
  live content wants the query language above (ADR-0047).

> **Done when** a third party can write a WASM plugin that installs without
> rebuilding the app; a program can query the vault with the fixed
> predicate language over REST or MCP; and the shadow host is verified in
> Firefox and Safari as well as Chromium.

### Phase 4 — The Table

Live play.

- **Live sessions**: session codes, seats, the session tier, the scratch log
  (ADR-0038, ADR-0048).
- The **encounter runner** and the **2D tabletop**: tokens, fog, initiative,
  the shared dice log.
- The **shared display** mode and **chat** (ADR-0048).
- The **session write-back contract**: partial apply, per-key conflict
  reporting, journal-based recovery (ADR-0039).

### Phase 5 — The Session

Recording, transcription, session logs and mood-driven music (ADR-0018,
ADR-0034).

---

### Phase 0 — the prototype

Ten questions, sixty hours, and the list fixed up front. The corpus is a
**test corpus**, not a deliverable: phase 0 parses the 1,692 real
frontmatters and finds out what breaks, which is minutes of work rather than
weeks. The SRD pipeline stays in phase 2.

## What moved, and where it went

| Was | Now | ADR |
|---|---|---|
| The formula engine | Phase 2 | 0042 |
| The dice engine | Phase 2 | 0008 |
| The 5e pack, the SRD vault, the Blades gate | Phase 2 | 0023, 0024, 0012 |
| The creation wizard, character sheets | Phase 2 | 0009 |
| Block widgets, the extension registry | Phase 2 | 0021, 0027, 0035 |
| The archive import endpoint | Phase 2 (was 1.5) | 0025 |
| The WASM plugin tier, the host ABI | Phase 3 | 0020, 0045 |
| The query language | Phase 3 | 0047 |
| The MCP plugin | Phase 3 (was 1.5) | 0014 |
| The cross-browser harness | Phase 3 | 0029 |
| Live sessions, seats, session codes, the scratch log | Phase 4 | 0038, 0048 |
| The full UI surfaces (GM dashboard; badge, inspector, player view land in phase 2) | Phase 3 | 0052 |
| The encounter runner, the tabletop, chat, the shared display | Phase 4 | 0016, 0048 |
| The session write-back contract | Phase 4 | 0039 |
| Recording, transcription, session logs, music | Phase 5 | 0018 |

## What did not move, because it is the floor

Some things are in phase 1 because leaving them out would make the MVP
unsafe, incomplete, or dishonest about what it is.

| Stays in phase 1 | Why |
|---|---|
| **The write journal and revert** | The app writes to the user's files. A write that cannot be undone is not acceptable in any phase |
| **The frontmatter form panel** | Without it the app is a viewer, not an editor |
| **The campaign model and the projection** | The premise. A wiki with no secrets is an Obsidian clone |
| **The vault watcher** | Obsidian-compatibility means "I edit in Obsidian and it changes here" |
| **Rated search** | A wiki whose search only finds exact titles is a weak wiki, and the store is one pure-Go dependency |
| **The plugin host's core** | Cutting it would undo the architecture's shape. Only the WASM sandbox is deferred |
| **The export plugin** | It is what proves the plugin seam is real, and it is how handover works |

## The honest cost

The MVP **does not demonstrate the system-agnostic claim**, because that is a
property of the pack format and the pack format is minimal here. What it
demonstrates instead is the **campaign** claim and the **secrets** claim.

That is a real change in positioning, and it is the trade for a phase 1 that
can actually be finished. The system-agnostic claim is proved in phase 2, by
the Blades gate.

---

## The re-cut of 2026-10-24

The map above is the result of one change, recorded in
[ADR-0059](decisions/0059-the-phase-map-is-re-cut.md):

- **Phase 1.5 is abolished.** Its two items — the archive import endpoint
  and the MCP plugin — land in phases 2 and 3. A number between two
  integers is a scheduling dodge, and it had no acceptance test of its own.
- **Phase 2 is split in two.** "The System" was a rules layer and a
  platform layer wearing one number. Phase 2 is now **The System** and
  phase 3 is **The Openings**.
- **Old phase 3 is now phase 4**, and **old phase 4 is now phase 5**.

Nothing was dropped and no work changed. The order changed, and the two
phases that had no coherent shape now have one each.
