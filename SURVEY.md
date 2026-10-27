# O5 UI surface survey (ADR-0052)

Canonical checklist: every surface in `spikes/ui/src/surfaces` and `spikes/ui/src/components`
is either landed in `web/` or recorded-deferred with a reason. Nothing is silently dropped.

## Surfaces

| Spike surface | Status | Landed at / Reason |
|---|---|---|
| `GmDashboard.svelte` | Landed | `web/src/surfaces/GmDashboard.svelte` (O5b); data via `web/src/lib/dashboard.ts` over `GET /api/v1/notes`. |
| `NoteEditor.svelte` | Landed (pre-O5) | `web/src/lib/Editor.svelte` + `web/src/App.svelte` frontmatter panel. |
| `PlayerView.svelte` | Landed | `web/src/surfaces/PlayerView.svelte` (O5c) with projection discipline. |
| `CharacterPanel.svelte` | Deferred (phase 4) | Needs pack-declared character fields + formula engine wiring (N1) in a form surface; out of O5 scope. |

## Components

| Spike component | Status | Landed at / Reason |
|---|---|---|
| `VisibilityBadge.svelte` | Landed | `web/src/components/VisibilityBadge.svelte`. |
| `VisibilityInspector.svelte` | Landed | `web/src/components/VisibilityInspector.svelte`. |
| `MarkdownView.svelte` | Partially landed | Markdown rendering goes through `web/src/lib/conformance.ts` + CodeMirror; full construct renderer deferred to phase 4. |
| `Icon.svelte` | Cut | Inline SVG / text glyphs; no icon font dependency (ADR-0005). |

## Spike library (deliberately not ported)

| Spike path | Status | Reason |
|---|---|---|
| `spikes/ui/src/lib/` (esp. `lib/projection.ts`) | Not ported | Client-side projector under retired ordered-rules ADR-0033. The server owns projection (trust boundary); no client recomputation of visibility. |

## Projection discipline

Every landed surface that shows visibility uses `surfaceVisibility()` from
`web/src/lib/dashboard.ts`, which requires a server-projected
`effective_visibility` and fails on raw data. `assertVisibilityAgreement()`
fails a surface whose displayed visibility contradicts the projection.
