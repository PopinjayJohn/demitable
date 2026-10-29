# ADR-0065: Un-reveal revokes a revealed audience, it does not erase the broadcast

- **Status:** Accepted
- **Date:** 2026-10-29
- **Amends:** ADR-0035 (secret reveals — the monotonicity), ADR-0057 §finding (per-id audience bug statement)
- **Evidence:** phase-6 R1 hostile suite (9/9: `TestR1UnrevealConcealsPerAudience`, `TestR1UnconcealedAudienceUnaffected`, `TestR1SiblingIsolation`, `TestR1GMOnlyNoteStaysHidden`, `TestR1JournalRecordsRevocation`, `TestR1TableAudienceUnreveal`, `TestR1DisplayAudienceUnreveal`, `TestR1SeveralPlayersAudienceUnreveal`, `TestR1PinTargetTableDisplayOnly`), each FAIL-probed; independent hostile re-verification (revoked refetch clean in raw bytes, revert restores)

## Context

ADR-0035 made reveals monotonic and journalled: a GM reveals a secret block
to an audience, the server persists the audience per block id in frontmatter
(`revealed:`) plus an implied grant, journals `op: "reveal"`, and broadcasts
`secret.revealed`. Clients that received the broadcast hold the text, and the
api.md §10 backlog row states the hard part honestly: revocation needs its own
semantic — per-audience or per-block, what the journal records — because
already-broadcast text cannot be un-sent.

Two adjacent findings arrived with the R1 investigation. First, ADR-0035
§Durability describes a two-layer story (live revealed set in the session
tier, written back to frontmatter at session end); what ships is one layer —
`Vault.RevealSecret` writes frontmatter synchronously and journals
immediately, and nothing touches a session store. Second, ADR-0057 records as
a *finding* that "a reveal to one player is a reveal to every player" because
the durable record was `revealed: [id]` with no audience; the code has since
fixed exactly that (per-id audiences in `parseRevealed`, set-membership in
`revealVisibleTo`) with no ADR recording the fix.

## Decision

**Un-reveal is per-audience removal from a block's revealed audience.** The
GM names a block and an audience (`secret.unreveal`, same audience grammar as
`secret.reveal`); the server removes that audience from the block's revealed
set. A block is concealed for a principal when no remaining audience includes
them; the block leaves the `revealed:` record only when no audience remains
(`revealed: []` round-trips and is a valid empty collection). Note-level
grants are untouched: revocation narrows the block audience, never the note's
grant set.

**The documented limit is part of the semantic.** Already-broadcast text
cannot be recalled: a client holding the text keeps it. Un-reveal guarantees
only that future fetches, re-renders, and replays conceal the block for the
revoked audience — and the webapp states this where the affordance lives. This
is a limit, not a footnote.

**The journal records `reveal-revoke`, revertible like any write.** The
revocation is a journaled batch (before/after blobs) through the ordinary
per-key write path, so it reverts as a unit per ADR-0039/ADR-0015, and the
"which files landed" reporting applies unchanged.

**The canonical mechanism stays one.** Frontmatter plus journal is the reveal
store; no session-tier revealed set is introduced. ADR-0035's two-layer
durability sentence is superseded by this: the thing that ships is the thing
that is specified.

**`target: "table"` is display-only.** The map-target `"table"` audience maps
to `Display`, not `All` (the R1 fix: a refactor had widened it to everyone;
hostile verification caught it and the pin test holds the narrowing). Bare
`"table"` strings and `type: "table"` keep their existing `All` reading; only
the map-target position is display-scoped.

## Consequences

**Good:**

- Revocation is a narrowing, so it needs no widening ADR of its own; every
  R1 hostile test (revoked refetch clean, un-concealed unaffected, sibling
  isolation, GM-only-note regression, journal revert restoring visibility) is
  a narrowing test.
- The propagation boundary is stated where it can be quoted (UI + docs),
  not implied by silence.

**Bad:**

- A revoked audience that cached the note still holds the secret text until
  it refetches. That is the documented limit, and it is permanent.

**Neutral:**

- The `revealed: []` empty-collection frontmatter fix and the
  `grants: ["*"]` lifecycle on full revoke are writer mechanics, not model
  changes; they round-trip byte-identically through the corpus suite.
