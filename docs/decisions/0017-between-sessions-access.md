# ADR-0017: Between sessions, players get read-only campaign access and their own sheet

- **Status:** Accepted
- **Date:** 2026-10-21

## Context

ADR-0002 made the vault GM-only: players receive filtered projections of it,
served live. That is clean, and it is the right trust boundary.

It has a cost that was accepted at the time: outside a session, a player sees
nothing at all. The app is invisible to them for weeks. For a group that plays
every other Sunday, the campaign has effectively become the GM's private
notebook between sessions — and the players' engagement with their own
characters drops to zero until game night.

The fix is not to give players the vault. It is to extend the projection
boundary so that a player can be *served* outside a session, with the same
filters that apply during one.

## Decision

**A player can log in at any time and see their campaign as their character
sees it.**

Between sessions a player gets:

- **Their own character**, fully editable by them (proposing changes, per the
  workflow in ADR-0009) and computed live by the pack.
- **Read-only campaign access**, filtered by the same projection rules used
  during a session: locations their character has visited, NPCs they have met,
  factions they know about, items they carry, quests they are party to.
- **Their party's shared view** of anything marked `visibility: party`.
- **Nothing** marked `gm`, and nothing belonging to another player.

What they do **not** get between sessions:

- The ability to change the world. The world is the GM's; players propose,
  the GM decides.
- Other players' characters, journals or private notes.
- Any draft or submitted character that has not been approved.
- Anything the GM has not made visible, even indirectly — including through
  backlinks, search results, or an MCP-style enumeration.

## Consequences

**Good:**

- The app becomes something players open between sessions, not just on game
  night. That is the difference between a GM tool and a group tool.
- It makes the projection layer do real work continuously rather than only
  under pressure at game time, so its bugs surface early and cheaply.
- It gives players a reason to care about their character's world knowledge,
  which is a behavioural change worth having.
- The between-sessions view is essentially the "campfire" view that ADR-0002
  anticipated as a possible future — pulled forward, because the projection
  layer is being built anyway.

**Bad:**

- **"Visited" and "met" need to be real concepts**, not guesses. The app
  needs to record, at session end, which locations a character was present at
  and which NPCs they actually encountered. That is session write-back
  content, and it must be designed rather than guessed at.
  - This is a real dependency: the between-sessions view is only as good as
    the write-back that populates it.
- More surface area for the visibility rules to leak. The projection must be
  exercised in both modes, and the same tests must run for both.
- Players will expect the GM to have seen their changes. Approval is async
  and the GM may be away for days. The UI must make the state of a proposal
  obvious.

**Neutral:**

- The write-back contract from ADR-0004 now has a second consumer. That is
  good — a contract with one consumer tends to be shaped by that consumer.
