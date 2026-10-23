# ADR-0032: Fifteen things we will not build

- **Status:** Accepted

- **Date:** 2026-10-22

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

A project with a plugin host, a pack system and a VTT is under constant
pressure to grow. Every one of the items below is individually defensible, and
a few of them are things a competitor already has. Collectively they would
multiply the surface by an order of magnitude.

The gap research (see `docs/research/player-gm-needs.md`) produced a list of
things a new tool would be **foolish to build**. Scope discipline only works
if it is written down somewhere people will look, and if a future proposal has
to explicitly supersede a recorded decision rather than quietly re-argue it.

## Decision

**We will not build these. A proposal to add one must supersede this ADR
explicitly, with a reason.**

1. **A second VTT, or map authoring.** The tabletop does tokens, fog,
   initiative, dice and measurement. Map editing is a separate application —
   Foundry's scene editor is a project of its own. The strongest evidence:
   the Obsidian community built a free open-source VTT with maps, tokens, fog,
   dice, initiative and a player view, and the reaction was warm but
   qualified — "not Foundry level (yet)". We build the *campaign-aware*
   board, not the better map editor.
2. **Voice or video chat.** Discord, Campfire and Roll20's built-in voice are
   all good enough. Building voice is the clearest possible scope mistake. We
   do the *interop* instead.
3. **Dynamic lighting, walls, line of sight, 3D.** Already excluded
   (ADR-0016). Agree.
4. **A rules compendium or an SRD search UI.** The SRD is a *vault template*
   (ADR-0024), which is the correct call. A rules-reference browser is a
   solved problem and is a data problem, not a product problem.
5. **An AI GM, or AI-generated content.** Phase 4's framing is right and
   should be protected: *AI earns its place not in generating content, but in
   **filing** it into a schema.* Generating content at the table is the classic
   failure mode, and it changes the game in a way players did not consent to.
6. **A pack or plugin marketplace.** ADR-0011's reasoning holds: a registry
   we run is a supply-chain surface of our own making. Discovery is a README, a
   forum post and a link. We ship a **schema and starter Bases** instead.
7. **A force-directed graph editor.** Already rejected in the phase-1 plan.
   Keep it rejected. A relation *schema* is worth having; a graph editor is
   not.
8. **Native mobile apps.** Rule 5 is right. A native client later is a
   benefit of the API contract, not a day-one requirement.
9. **Full-blown chat.** Table chat, GM→player, player→GM, and an OOC marker.
   No threads, no reactions, no chat search. The test: *can a GM hand a note
   to one player, and can a player ask the GM something privately, without
   leaving the app?*
10. **A full print and PDF designer.** Export the pack's sheet layout to a
    paginated document and be done. No template editor for print.
11. **A full calendar and moon-phase simulator.** A calendar *model* — era,
    date, optional moon — at the level of "the session happened on 3rd of
    Highwind, Year 512" is worth having. A fantasy-calendar simulator with
    moon cycles and seasonal weather is a hobby.
12. **Analytics, leaderboards, a dice-log dashboard.** Nobody asked, and it is
    a competitive-mode feature, not a table feature.
13. **Automatic map generation, AI handouts, AI-generated NPCs at the
    table.** Generic content at the exact moment the table is least
    forgiving, and it desynchronises from the vault. Random *tables* —
    deterministic, authored, in the vault — are the right version of this
    idea, and we already have them.
14. **A social layer** — friend requests, DMs, activity feeds. The table is
    four to six people who already know each other. That is scope for a
    different product.
15. **Sync or hosting.** The design is explicit that the internet is optional
    and the user brings their own sync. A sync service contradicts ADR-0025
    and ADR-0031's philosophy, and creates a data-loss liability.

## Consequences

**Good:**

- The plugin surface stays small enough to be coherent, and the core stays
  auditable.
- A future proposal has to argue against a recorded decision rather than
  against nothing.
- The list is itself a design statement: it says what kind of tool this is.

**Bad:**

- **The list will be wrong sometimes.** Some of these are the right call now
  and the wrong call later, particularly a native mobile app and a rules
  browser. The ADR is meant to be superseded, not treated as scripture.
- It is a constraint, not a prohibition: a good argument can win, but it has
  to be made explicitly.

**Neutral:**

- Two items are already excluded elsewhere (ADR-0016 for 3 and 7, the phase-1
  plan for 7). They are listed here so the constraint has one home.
- The list came out of research rather than intuition, which is the point —
  it is not our taste, it is what the evidence says does not pay.

## How to change it

Write a new ADR that supersedes this one, naming the item and the reason.
Do not edit this file. The history of which constraints we chose, and when we
learned to relax one, is worth more than the constraint itself.
