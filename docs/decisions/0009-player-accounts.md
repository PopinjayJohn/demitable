# ADR-0009: Player accounts, character ownership, and a character review workflow

- **Status:** Accepted, extended by ADR-0037 (a campaign is a note),

  ADR-0038 (the account join code is campaign-scoped; a session code is a
  second, separate credential) and ADR-0040 (the `owner` field is now
  `player`)
- **Date:** 2026-10-21

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

The character creation wizard is used by **both the GM and players** (phase 2).
That single decision has consequences ADR-0002 did not anticipate.

ADR-0002 assumed players were ephemeral — a session token for tonight's
table. But if a player *creates and owns* a character, then:

- A character's lifetime spans sessions. Something must persist between
  sessions that identifies the player.
- A player who drafts a character must not have it visible to the table until
  it is approved, or the party sees a half-finished character.
- The GM must be able to review and approve characters before play.
- `visibility: owner` (ADR-0002, design doc §5.4) needs a concrete owner.

A session token is the wrong primitive here. The right primitive is a local
account.

## Decision

### Local player accounts, stored in the vault

Accounts live in `.demitable/players.yaml` at the vault root — app-private
metadata, hidden from the note browser, but inside the vault so it travels with
the campaign and is covered by the user's existing backup and sync.

```yaml
players:
  - id: p_8f2a
    name: Ilia
    passphrase: $argon2id$v=19$…      # never plaintext, never reversible
    created: 2026-10-21T19:04:11Z
    characters: [01JBX8K2M9Q]         # owned character uids
```

There are no accounts on the internet and no account server. The server
authenticates against this file. An account is a line in the GM's vault.

### Joining without password juggling

The GM does not have to transmit a passphrase:

1. The GM creates a player (name only).
2. The server issues a **one-time join code** (`K7F2-9QX4`), valid for 24 hours,
   single use.
3. The player opens `http://<gm-lan-ip>:<port>/join`, enters the code, and
   chooses a passphrase.
4. The code is consumed; the account now has a passphrase only the player
   knows.

Authentication uses an httpOnly, SameSite=Strict session cookie. On a LAN the
cookie is not the only thing protecting the account — see ADR-0010.

### Ownership and the character lifecycle

`owner` becomes a universal field on notes, holding a player id. It is what
`visibility: owner` resolves against.

Player-created characters move through a review workflow before the table sees
them. This is app-level, not pack-level: it is a rule about who may speak for
a character, and it does not vary by ruleset.

```
   draft  ──submit──►  submitted  ──approve──►  active
     ▲                     │                        │
     └──────edit───────────┘                        │
     (GM may edit any state)          reject ──────┘
```

| State | Visible to | Editable by |
|---|---|---|
| `draft` | Author, GM | Author, GM |
| `submitted` | Author, GM | GM (author sees read-only) |
| `active` | Per the note's own `visibility` — normally `party` | Author (proposed changes), GM |
| `retired` | Author, GM | GM |

> **Amended by ADR-0050, decision D6.** The `retired` row above is **not** a
> visibility gate. ADR-0033's rule 3 gates `draft` and `submitted` only, and
> a GM closing a quest must not silently remove it from the players' board.
> A retired note keeps the visibility it had. The rule 3 gate is about who
> may *speak for* a thing — a player's own character — not about whether it
> disappears.

The GM creates characters and approves them in one step. A player submits and
waits — a wait that is socially useful, because the GM's review is where
character concepts get discussed at the table.

> **Amended by ADR-0038.** The "one-time join code" here is the
> **campaign** join code, scoped to one campaign, valid 24 hours and single
> use. It is one of **two** credentials, in **disjoint code spaces**: the
> other is the **session** code, which creates a session-scoped guest seat.
> They are accepted by different endpoints and validated against different
> stores, and their lengths differ, so neither can be redeemed as the other.

### Accounts and seats are different things

An **account** is who you are *in this campaign* — long-lived, it owns your
characters, and it is what gives you the between-sessions view (ADR-0017).

A **seat** is who is *at this table, right now*. It is session-scoped, it
names the character being played, and it may belong to an account or to a
guest who has none.

They are separate because the cases are: a player may have an account and not
be at the table today; a guest may come to one session without ever making an
account; a player may sit at two tables in a campaign playing a different
character at each.

Joining a session takes a **session code** — short, unambiguous, shown on the
GM's screen and rotating per session — or an account's passphrase. A guest
joining with the code gets a session-scoped identity and **no between-sessions
access**, because a guest at the table should not be handed a passphrase
mid-conversation. The token a join returns is scoped to that session and that
seat; it grants nothing outside it.

A seat naming a character the caller does not own is rejected. A seat is not
a privilege: the projection applies to it exactly as it applies to everything
else. The full model is in [`api.md`](../api.md) §3.

### Field changes

Two universal fields are added to the note model (design doc §5.2):

```yaml
player: p_8f2a                      # the player account that owns this note
workflow: draft|submitted|active|retired
```

`workflow` applies to any note a player owns, so a player-authored location
or item can use the same gate.

> **Scope note.** The review workflow covers **characters only**. Players may
> not propose changes to the world — quests, locations, factions, items are
> the GM's alone, with exactly one writer. There is no proposal queue for
> world changes. See
> [`session-writeback.md`](../session-writeback.md) §1, "The world is the
> GM's".
>
> This narrows what was proposed earlier in this ADR, and it is a
> simplification: one writer for world state means no merge, and it keeps the
> between-sessions view read-only, which is what ADR-0017 promised.

## Consequences

**Good:**

- The wizard becomes genuinely useful to players, which is where most of the
  enthusiasm for character builders comes from.
- The GM gets a review queue, which is a real workflow that plain markdown
  vaults have no way to express.
- `owner` becomes concrete, so `visibility: owner` works as designed.
- Accounts travel with the vault. If the GM moves the campaign to another
  machine, the players come with it.

**Bad:**

- **Phase 1 needs an account system**, even though the wizard is phase 2. The
  role model, ownership and projection layer cannot be built correctly
  without it. This is the single largest schedule consequence of choosing
  "GM and players both".
- Passwords. Even on a LAN. The passphrase is hashed with argon2id and the
  server rate-limits join attempts, but a GM's living room network is not a
  trusted network.
- A rejected character leaves an orphan note. The workflow needs a way to
  return to `draft` with feedback, and the author needs to be told.
- Players cannot use their characters if the GM forgets the vault — there is
  no off-server fallback.

**Neutral:**

- Accounts are local only. If the project ever grows a hosted mode, this file
  becomes the natural import format rather than something to design twice.
