# ADR-0026: Maps and assets are served by the core, role-scoped, and cached by clients

- **Status:** Accepted
- **Date:** 2026-10-22

## Context

Maps, portraits, token images, table face art and handouts are binary files
in the vault, under `Assets/`. Two things have to work:

1. **Players need them.** The tabletop is useless without a map image, and a
   player on a phone needs it to load fast. The GM may have 200 maps; the
   player needs the one on the table.
2. **Players must not have all of them.** A map of a location the party has
   not discovered, a portrait of a villain they have not met, a handout for a
   secret they have not been shown — handing those over spoils the game.

The second point is the same problem the note-level visibility model already
solves (design doc §5.4). Assets just have not been brought into it.

## Decision

**The core serves vault attachments, filtered by the projection, with
content-addressed caching.**

### The endpoint

```
GET /asset/<vault-relative-path>
```

- Requires the caller's session token, like every other endpoint.
- **The projection applies.** An asset is served to a player only if a note
  *visible to that player* references it, or if it is explicitly marked
  public.
- **Assets that the caller cannot see return 404, not 403.** A 403 confirms
  the path exists, which is itself a leak.
- Streamed with `Content-Length` and an `ETag` derived from the file's content
  hash. `If-None-Match` yields a 304.
- Range requests are supported, so the board can load a large map
  progressively rather than blocking on a multi-megabyte fetch.

### The rule for what a player can see

> **An attachment is visible to a player if a note visible to that player
> references it, or if it is explicitly marked public.**

This reuses the existing model rather than inventing a parallel one. If the
party has discovered the Drowned Quarter, the location note is visible, and
the location note's `map:` reference makes the map visible too. If they have
not, neither is.

Assets referenced by no note at all are **GM-only by default**. A GM who
wants an asset public says so explicitly.

### Caching

Clients cache assets by ETag and reuse them across sessions. The board
plugin requests a map once and it stays in the browser's cache, so a
reconnect or a page reload at the table is instant.

This matters more than it sounds: a phone on a table in a room with poor
wifi is the common case, not the exception.

### What this means for the vault schema

A note references an asset by path, in the same way it references any other
attachment:

```yaml
type: location
name: The Drowned Quarter
map: Assets/Maps/drowned-quarter.png
portrait: Assets/Portraits/quarter.webp
```

The reference is a plain path, resolvable from the note's frontmatter or its
body. The asset endpoint resolves it against the vault root, refusing
anything that escapes it.

## Consequences

**Good:**

- **One mechanism.** Assets flow through the same projection as notes, so
  there is no second rule to get wrong and no second place for a leak.
- **The board gets its maps over the same channel** it already uses for
  everything else. No separate asset pipeline.
- Players cache locally, so the table works on bad wifi.
- 404 rather than 403 means the path space is not enumerable.
- The GM's files stay the GM's files. Nothing is duplicated for players.

**Bad:**

- **Every asset request is a server round trip** the first time, and the
  first load of a session can be heavy if the GM has a lot of art.
- **A note's visibility now indirectly governs files**, which is a slightly
  surprising coupling. A GM who links an image from a GM-only note and
  expects players to see it will be confused. The failure mode is safe, but
  it should be explained in the UI.
- **The cache has to be invalidated** when a file changes. The content hash
  handles it, but a GM replacing a map mid-session means a re-fetch for
  everyone.
- Assets referenced by nothing are invisible to players, which is
  occasionally inconvenient — and correct.

**Neutral:**

- Portraits on character notes follow the same rule, so a character's
  portrait is visible exactly when the character is.
- The VTT's fog of war hides tokens; this hides *files*. Both are needed, and
  they are different things: fog is about the map's contents, visibility is
  about the file's existence.
