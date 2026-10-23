# ADR-0002: The vault is GM-only; players access through server projections

- **Status:** Accepted

- **Date:** 2026-10-21

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

The project serves a GM and their players. There are four ways to distribute
campaign material:

1. One shared vault, synced to everyone.
2. Separate per-player vaults, synced selectively.
3. Filtered exports handed out between sessions.
4. The vault stays on the GM's machine; players receive a live, filtered view
   over the network during play.

Options 1 and 2 both require a sync layer between multiple writers, which means
conflict resolution for markdown files edited concurrently by several people —
a genuinely hard problem with a poor answer for prose.

Option 1 also leaks the GM's secrets, because in a shared vault there is no
technical boundary between "the note about the traitor" and "the note about the
tavern".

## Decision

**The vault lives only on the GM's machine. Players never receive files.**

The server is the trust boundary. When a player requests campaign data, the
server returns a **projection**: a filtered view of the vault assembled per
request, per role, per session. A projection can:

- omit notes entirely (a GM-only secret never leaves the machine),
- omit individual *properties* (a player's HP is visible to them and the GM,
  not to the other players),
- redact or mask values (an NPC's true allegiance shown as "unknown" to the
  party),
- be assembled on the fly rather than stored.

Players see the campaign through the server: their character sheet, what the
party has discovered, their dice log, the encounter board. Not the file tree.

## Consequences

**Good:**

- No sync layer. No merge conflicts between players. The GM's existing
  Obsidian Sync or git setup keeps working, unchanged, for the GM alone.
- Hidden content is hidden *structurally*, not by convention. A GM-only note is
  not in the vault the players can see, because there is no such vault.
- Permissions become a server concern — the same place as session tokens and
  the VTT — rather than a distributed-systems problem.

**Bad:**

- **Players cannot browse lore at home.** They only get what the server serves.
  This is the main downside. Mitigation: the projection API can expose a
  read-only "campfire" view for between-sessions reading, if wanted.
- **The GM is the single point of failure.** If the GM's machine is down, the
  session is over. Acceptable for a tabletop tool.
- **Projection cost.** Every player request touches the vault. Mitigated by
  caching the parsed vault in memory with a filesystem watcher for invalidation.

**Neutral:**

- Requires a **visibility model** to exist in the schema from day one
  (section 5.4 of the design doc). This is the expensive-to-add-later part, and
  it is why this ADR matters at phase 1 rather than phase 2.
