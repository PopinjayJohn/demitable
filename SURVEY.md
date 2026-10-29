# UI surface survey (phase-5 P0c)

Canonical checklist: every surface in `web/src/surfaces` and every component in
`web/src/components`, plus the shell, is either conforming (with the
discipline cited) or recorded-deferred with a reason. Nothing is silently
dropped. Replaces the O5-era survey, which named only spike surfaces.

**Canonical visibility helper:** `surfaceVisibility()` / `assertVisibilityAgreement()`
in `web/src/lib/dashboard.ts`. A surface that shows a note's visibility must
either call it (fail-closed on raw/unprojected data) or be flagged below as
reading the server-projected `effective_visibility` directly.

**Projection authority:** the server. No client recomputation of visibility;
`spikes/ui/src/lib/projection.ts` is deliberately not ported. Core surfaces
render in the app document (light DOM); shadow-mount isolation (ADR-0029)
applies to plugin UI only — no shadow roots exist under `web/src`.

**Network authority:** `web/src/lib/api.ts` (the `api.md` contract). Surfaces
do not hand-roll endpoints; the only direct `fetch`-adjacent call is the
Tabletop's `apiFetch` for token moves, and the WebSocket upgrade in
`VaultExplorer` / `NoteEditor` / `Chat`.

**Client-only state:** `appState` and `vaultStore` hold UI state and
server-derived caches; the server remains the authority. No surface claims
client-side projection or client-side note authority.

## Surfaces (`web/src/surfaces`) — 9

| Surface | Status | Discipline |
|---|---|---|
| `Viewer.svelte` | Conforming | Read mode. Note body comes from `fetchNote(uid, {as})` — server projection. Character stat box (`CharacterCard`) is fetched via `fetchCharacter` only when `!eyeAs` (GM context), so projection-withheld fields cannot reach a player. Reveals fire over the WebSocket only when `!eyeAs`. Two-column layout is presentation. |
| `VaultExplorer.svelte` | Conforming (caveat) | List + reading pane over `vaultStore` (server-projected) and `fetchNote(uid, {as})`. Badge uses `surfaceVisibility(n)`; `vis()` catches and falls back to `"gm"` — **fail-open fallback flagged** (should not mask a missing projection). Filtering/sorting are presentation. |
| `PlayerView.svelte` | Conforming | `fetchNotes`/`fetchNote` with `as`; filters to notes where `surfaceVisibility(n)` succeeds (fail-closed on raw data); badge via `surfaceVisibility`. Read-only between sessions; "missing note is indistinguishable from nonexistent" preserved. |
| `Tabletop.svelte` | Conforming | `loadTabletop` server; `surfaceVisibility(n)` asserted on every loaded note; token moves go through the server (`POST .../tokens`); fog/hidden state and display mode are server-driven; 1 Hz polling + WS is cadence, not authority. |
| `NoteEditor.svelte` | Conforming (caveat) | Source pane is always the GM's raw file (no client projection of a secret-stripped body). Preview is `fetchNote(uid, {as})`. Frontmatter edits go through `patchNote`. Badge uses `doc.effectiveVisibility` directly — **no `surfaceVisibility` call (flagged)**. Conflict (409) rebase is server-driven. |
| `Home.svelte` | Conforming (caveat) | Campaign home resolves the home note via `fetchNote` (GM raw file for the `home:` link; `type: home` convention otherwise). Badge uses `effective_visibility ?? effectiveVisibility ?? "party"` — **fallback, flagged**. Attention heuristic uses declared `visibility` (presentation only). `createNote` for new notes. |
| `Chat.svelte` | Conforming read; creation deferred | Reads `type: message` notes via `fetchNotes`; renders server-projected list. `sendChatMessage` → `createNote`. **Authorship rule (P0b) not yet enforced** — creation requires the caller to see the channel's audience; carried as a P0b item, not a client change. |
| `CharacterPanel.svelte` | Conforming | `fetchCharacter` / `patchNote` / `submitCharacter` / `reviewCharacter`; withheld fields are absent from non-GM payloads server-side. Review queue from `vaultStore`. No visibility badge; no client-side field filtering. |
| `SessionReview.svelte` | Conforming (caveat) | P3 transcript review: segments with editable speaker labels, promote, propose. Reads `/transcript/review` (GM-only; non-GM → 404) via direct `fetch` — **network-authority caveat (not `api.ts`)**. Server-projected data; GM-gated and audience-governed server-side (review/segment edits/promote/propose all require a GM principal). No client-side projection. |

## Components (`web/src/components`) — 16

| Component | Status | Discipline |
|---|---|---|
| `CaptureRecorder.svelte` | Conforming | P1 consent-gated capture: consent checkbox gates recording; chunks upload through `api.ts` to the GM-only `/audio` endpoint; no visibility/projection logic. Client consent is a UI gate only — the server enforces GM. |
| `CharacterCard.svelte` | Conforming | Docked stat box (Viewer). Renders server-provided `computed` + flattened `data` only; pack-agnostic; no visibility logic. GM-context gated by caller. |
| `Footer.svelte` | Conforming | Status strip; counts from `vaultStore` (server-projected `effective_visibility`); `DASHBOARD_BUDGET_MS` display only. |
| `Icon.svelte` | Conforming | Presentational SVG glyphs; no icon font (ADR-0005). |
| `JournalDrawer.svelte` | Conforming | `fetchJournal` / `revertJournal` server; revert is a server write + journal. |
| `MarkdownView.svelte` | Conforming | Renders the server-projected body (`renderMarkdown`); secrets arrive only in GM source; reveal buttons gated by `canReveal`; wiki links resolved against the server note index. |
| `NoteContext.svelte` | Conforming (caveat) | Rail context for explorer/viewer. Reads `effective_visibility` with fallback `"gm"` — **no `surfaceVisibility` call (flagged)**. Secrets/reveals come from server (`revealed` list). |
| `NoteCrumbs.svelte` | Conforming | Navigation affordance over server note paths; `pathFilter` is UI state only. |
| `Rail.svelte` | Conforming | Renders the published `appState.rail` snippet; show/hide is UI state. |
| `SessionButton.svelte` | Conforming | `createSession` / `closeSession` server; session id lives in shared state. |
| `SessionJournal.svelte` | Conforming | `fetchNotes`/`createNote` type `journal`; server projection governs reads. |
| `SideSection.svelte` | Conforming | Collapsible sidebar section; collapsed state persisted in `localStorage` (UI preference, non-authoritative). |
| `Sidebar.svelte` | Conforming | Navigation + `vaultStore` notes/campaigns; no projection logic. |
| `VaultTree.svelte` | Conforming | Builds a folder tree from server note paths; selection routes through `appState`. |
| `VisibilityBadge.svelte` | Conforming | Displays the server-projected effective visibility in glyph + word + hue + rule-bar, with a `*` marker and tooltip for defaulted values (ADR-0052). |
| `VisibilityInspector.svelte` | Deferred (data) | UI surface present and renders a supplied decision list, but every caller passes `decisions = []`; no server source for the rule chain/verdict yet (api.md §10 backlog: "Inspector decision chain"). Surface is landed; data is not. |

## Shell

| File | Status | Discipline |
|---|---|---|
| `App.svelte` | Conforming | Routes wiki views (home/explorer/editor/player/chat); holds no projection logic; state in `appState`/`vaultStore`. |

## Canonical-helper audit (P0c)

`surfaceVisibility` / `assertVisibilityAgreement` are the one helper. Callers
that use it: `PlayerView`, `Tabletop` (load), `VaultExplorer` (badge, inside
try/catch), and the dashboard counts in `dashboard.ts`. **Readers that bypass
it** (read `effective_visibility` directly, with a fallback that can mask a
missing projection): `NoteEditor` (`effVis`), `Home` (badge fallback `"party"`),
`NoteContext` (`vis()` fallback `"gm"`), `VaultExplorer.vis()` catch
(fallback `"gm"`). Flagged here; no code change in this pass — the
fail-closed behaviour is the amendment's to settle.

## Shadow mount (ADR-0029)

Core surfaces and components render in the app document (light DOM). No
shadow roots exist under `web/src`. Shadow isolation applies to plugin UI
only, which is not present in this tree. No surface bypasses ADR-0029.

## api.md contract

All surfaces reach the server through `web/src/lib/api.ts` (or `apiFetch` for
the Tabletop token move). No surface fabricates an endpoint. Reads of a
single note (`fetchNote`) are legitimate selected-note reads; the dashboard
additionally asserts zero client vault reads for its own request set.

## Deferred items (recorded, not dropped)

- `VisibilityInspector` server data — no endpoint yet (api.md §10).
- Chat authorship rule — P0b carries it (creation must see the channel's
  audience); reads stay projection-governed.
- Fail-open `surfaceVisibility` fallbacks in `VaultExplorer`, `NoteEditor`,
  `Home`, `NoteContext` — flagged above for the ADR-0052 amendment.

## ADR-0052 ordered-rules residue (evidence for the amendment)

The amendment itself is the orchestrator's (new ADR amending ADR-0052); this
list is the gathered evidence. ADR-0052 and design.md §5.4 still speak in
ordered rules ("six ordered rules", "rule chain", "first rule that applies
wins"), while ADR-0057 computes audiences: the intersection of every
applicable constraint, then widened only by the grants permitted to widen it.
2 (absolute deny) and 3 (workflow gate) narrow absolutely; no grant reopens
them.

| Location | Line(s) | Residue | Correct audience-model reading |
|---|---|---|---|
| `docs/design.md` | 579–580 | "visibility is resolved in this order; the first rule that applies wins." | Not an ordered rule list. The projection computes an audience = intersection of all applicable constraints, widened only by permitted grants (ADR-0057). |
| `docs/design.md` | 592–593 | "No grant overrides it" / "absolute deny" framed as rule 2 winning | Correct as a constraint: `gm` is an absolute narrowing that no grant may reopen. |
| `docs/design.md` | 597–599 | "regardless of its visibility and regardless of any grant. A grant-suppressing deny." | Correct as a constraint: the workflow gate narrows and grants cannot widen past it. |
| `docs/design.md` | 621–622 | "A grant never overrides rules 2 or 3." | Rephrase as: grants (constraint 5) may only widen, and never past the absolute deny or the workflow gate. |
| `docs/design.md` | 651–652 | "applied **last** — after grants and denies have decided whether a value is present." | Masking is a constraint applied to the already-computed audience, not a later rule winning. |
| `docs/design.md` | 684–685 | "Six rules in order is more than one lookup." | The UI explains constraint verdicts that produced the audience, not a winning rule. |
| `docs/decisions/0052-the-projection-consequences-belong-in-the-ui.md` | 13–14 | "resolves a note per principal through six ordered rules: scope, absolute deny, workflow gate, base level, grants, default." | Six constraints, not an ordered procedure; audience = intersection then permitted widening. |
| `docs/decisions/0052-the-projection-consequences-belong-in-the-ui.md` | 38–40 | Inspector "lists the rule chain that produced the answer, citing ADR-0033 section numbers". | Inspector lists the constraint verdicts behind the audience (and must stay non-leaking to non-GM callers). |
| `docs/threat-model.md` | 27 | "resolved in a fixed order (scope → absolute deny → workflow gate → base level → grants → default → property level → masking)." | The list is the complete constraint set; presentation order is not evaluation order. Audience = intersection then permitted widening. |
| `docs/threat-model.md` | 61–67 | Numbered constraints 1–8 (already framed as "supersedes the ordered rules … complete list of constraints"). | Already the audience reading; keep, but drop any "fixed order" implication in §3 line 27. |
| `docs/api.md` | 591 | "the projection returns a verdict, not the rule chain that produced it." | The projection returns the audience (and, eventually, the constraint verdicts); no single winning rule. |
| `web/src/components/VisibilityInspector.svelte` | 9 | "No rule chain from the server yet." | No constraint-verdict source from the server yet; the badge remains the projected audience. |

## Counts

- Surfaces: 9 (9 conforming, 0 fully deferred; 4 with flagged fail-open fallbacks; 1 with a network-authority caveat: `SessionReview`).
- Components: 16 (15 conforming; 1 deferred-data: `VisibilityInspector`).
- Shell: 1 conforming.
- Total inventoried: 25.
