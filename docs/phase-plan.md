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
dice, no encounters and no tabletop. Those are phase 2 and phase 3, and they
sit on top of a schema that phase 1 has already proven against real use.

---

## The four phases

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

**Deliberately deferred from phase 1** — all of it lands in phase 2 or 3:
the formula engine, the dice engine, the 5e pack, the SRD content vault, the
Blades gate, the creation wizard, character sheets, the rich editor's block
widgets, the WASM plugin tier, the query language, live sessions, seats, the
encounter runner, the tabletop, chat and the shared display.

### Phase 2 — The System

The rules layer. The pack format completed — resources, formulas, dice and
sheets — and the machinery that interprets it.

- The **formula engine** (ADR-0042's layered scope).
- The **dice engine**: tables, faces, symbol dice, pools, opposed rolls.
- The **5e pack**, the **SRD content vault** and the **Blades gate** — the
  format proof (ADR-0012, ADR-0023, ADR-0024, ADR-0036).
- The **creation wizard** and **character sheets**, for the GM and for players
  (ADR-0009's review workflow).
- The **rich editor**: block widgets, callouts, stat blocks, the extension
  registry, the cross-browser harness (ADR-0021, ADR-0027, ADR-0035).
- The **WASM plugin tier** and the full host ABI (ADR-0020, ADR-0045).
- The **query language** for `list_by`, shared by REST and MCP (ADR-0047).

### Phase 3 — The Table

Live play.

- **Live sessions**: session codes, seats, the session tier, the scratch log
  (ADR-0038, ADR-0048).
- The **encounter runner** and the **2D tabletop**: tokens, fog, initiative,
  the shared dice log.
- The **shared display** mode and **chat** (ADR-0048).
- The **session write-back contract**: partial apply, per-key conflict
  reporting, journal-based recovery (ADR-0039).

### Phase 4 — The Session

Recording, transcription, session logs and mood-driven music (ADR-0018,
ADR-0034).

---

## What moved, and where it went

| Was | Now | ADR |
|---|---|---|
| The formula engine | Phase 2 | 0042 |
| The dice engine | Phase 2 | 0008 |
| The 5e pack, the SRD vault, the Blades gate | Phase 2 | 0023, 0024, 0012 |
| The creation wizard, character sheets | Phase 2 | 0009 |
| Block widgets, the extension registry, the cross-browser harness | Phase 2 | 0021, 0027, 0035 |
| The WASM plugin tier, the host ABI | Phase 2 | 0020, 0045 |
| The query language | Phase 2 | 0047 |
| Live sessions, seats, session codes, the scratch log | Phase 3 | 0038, 0048 |
| The encounter runner, the tabletop, chat, the shared display | Phase 3 | 0016, 0048 |
| The session write-back contract | Phase 3 | 0039 |
| The archive import endpoint | Phase 1.5 | 0025 |
| The MCP plugin | Phase 1.5 | 0014 |

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
