# ADR-0051 (proposed): The threat model, the not-defended list, and the derived phase-1 exposure checklist

- **Status:** Proposed — phase 0, question 10 wrote this document; the orchestrator lands the ADR
- **Date:** 2026-10-23
- **Extends:** ADR-0002 (the trust boundary), ADR-0009 (accounts), ADR-0031 (internet exposure),
  ADR-0033 (the visibility model), ADR-0035 (secrets), ADR-0038 (two credentials), ADR-0020/0027/0029
  (plugin tiers)
- **Addresses:** the phase-1 exposure checklist that was unowned and seat-blind
  (`phase-1-plan.md` §5); the plugin-UI trust claim that the phase-1 DoD got wrong; the search
  boundary that was never analysed; the sanitisation gap; and the superseded ADR-0010 threat model.

## Context

Phase 0, question 10 asked: *What is actually being protected, from whom, and by what?*

The phase-0 deliverable is [`spikes/threat-model/README.md`](README.md). It states the assets
(GM-only vault content, secret blocks, private player content, credentials), the actors, the
boundary (server-side projection), the mitigations, and — the part the brief asked for and the
phase-0 research then stress-tested — **the things that are not defended**, stated plainly.

Before this model existed, three documents disagreed about the threat posture:

1. The phase-1 exposure checklist (`phase-1-plan.md` §5) was checkable in form but **unowned**, and
   did not cover the newest credential in the system — the session code introduced by ADR-0038.
2. The phase-1 Definition of Done claimed GM-only notes were “provably unreachable by players,
   **and by plugins**” — a claim the phase-0 research showed false for the compiled-in tier and for
   plugin UI (ADR-0029 correcting ADR-0027; ADR-0020's compiled-in tier “trusted with everything”).
3. The design's “search is part of the security boundary” was never analysed against the measured
   IDF behaviour.

The phase-0 spike found the load-bearing leak class: **the leaks were not in the designed
mechanism, they were in the accidental ones.** The design said a GM aside was “hidden in preview”;
the projection serialised it anyway (`%%comments%%`, ADR-0050 D12). Hiding was treated as partly a
rendering concern and partly a serialisation concern.

## Decision

**The threat model is the source of truth for what Demitable defends, and the phase-1 exposure
checklist is derived from it, with an owner.**

1. The threat model must, at minimum, cover: the assets; the actors; the attack surfaces (projection
   leak paths, credential surface, vault-as-input, server exposure, plugin tiers, search, assets,
   write-back, MCP); the mitigations; and an explicit not-defended list.
2. The phase-1 exposure checklist is **derived from** that model, not authored alongside it. Every
   checklist item maps to a threat-model section; every threat-model defence maps to a milestone
   criterion.
3. The not-defended list is **binding** for phase 1: items listed there are accepted risks or known
   gaps, and phase 1 must either close them or record them as accepted with the milestone that
   owns the closure.
4. The honest position on plugin UI stands: **a plugin's logic is sandboxed; its UI is trusted.** The
   capability model constrains accidental overreach, not hostile plugins.

## Consequences

**Good:**

- The phase-1 checklist stops being a list someone liked the look of and becomes a derived,
  checkable artefact with an owner.
- “What is not defended” is written down, so phase 1 cannot quietly ship an undeclared defence.

**Bad:**

- The not-defended list will be long and uncomfortable, and must be maintained.
- The threat model adds a documentation surface that phase 1 must keep in sync with the code.

**Neutral:**

- The threat model is a phase-0 deliverable and does not itself change the implementation; the
  changes it implies land in phase 1 through the milestones named in the spike.

## What this decision does not settle

- Whether plugin UI should move behind a real origin boundary (position (b) in the phase-0 finding)
  rather than adopting the honest position (a).
- The markdown sanitisation policy.
- The search-ranking remedy (filter before BM25/IDF).
- Phase 3's live-session threat surface (seats, chat, display, scratch log) beyond what ADR-0038 and
  ADR-0048 already model.
