# ADR-0041: One server serves one vault, and that vault holds many campaigns

- **Status:** Accepted

- **Date:** 2026-10-23
- **Extends:** ADR-0037 (a campaign is a note) and ADR-0038 (two credentials)
- **Addresses:** the last of the four pre-phase-1 decisions

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

ADR-0037 made a campaign a note inside a vault, so "one vault, many
campaigns" is now the model. That leaves a question the earlier design never
asked: **can one server process serve more than one vault?**

The question has a dependency that is easy to miss. Accounts live in
`.demitable/players.yaml` — **inside the vault**. So a server serving two
vaults has to decide where the accounts for each go, and the answer changes
what `player`, `member` and `owner` mean across the boundary.

The candidates were:

- one server, one vault, many campaigns;
- one server, many vaults, accounts per vault;
- one server, many vaults, accounts server-wide.

## Decision

**One server process serves exactly one vault. That vault holds any number of
campaigns. Accounts live in the vault.**

```bash
demitable --vault ~/vaults/stormdark        # serves that vault only
demitable --vault ~/vaults/emberspire        # a second process, a second port
```

### Why one vault per server

**The vault is the unit of everything.** It is what gets backed up, synced,
shared and handed to a new GM. It is the trust boundary (ADR-0002) — the
vault never leaves the GM's machine, and everything the app knows about
campaigns lives inside it. A server that held several vaults would be a
server whose compromise reaches all of them, and whose URL has to carry the
vault selection, which puts the vault back into the session and the auth
model at exactly the point those were simplified.

**The accounts file stays with the campaign.** `.demitable/players.yaml` is
inside the vault, so it travels with it. A GM who copies their vault to
another machine brings their players with them, and the account hashes never
leave the vault — which keeps the accounts file inside the one directory that
is already GM-only and already backed up.

**The URL does not carry the vault.** It carries the campaign, which the
player selects by their join code or by their membership. One server, one
vault, one thing to remember.

### The cost, stated plainly

**A GM running five campaigns runs five processes on five ports.**

That is real friction, and it falls on exactly the power user the project
should serve best. Three things make it tolerable:

- The per-port overhead is small — one binary, one vault, a few hundred
  megabytes of memory for the index.
- A reverse proxy can route by port or path, so the *player's* experience is
  still one URL. The GM manages the routing, not the players.
- When multi-vault becomes a real need rather than a plausible one, the
  change is additive: a vault parameter on the request, and an account store
  that can be per-vault or server-wide.

We are choosing the simple shape now and the complex shape later, rather than
building the complex shape for a case we have not seen.

## Consequences

**Good:**

- **The vault is genuinely self-contained.** Backup it, sync it, hand it to a
  new GM, and the accounts come with it.
- **The trust boundary is one directory.** Compromising the server compromises
  one campaign, not five.
- **The URL stays simple.** Campaign, not vault.
- **The model matches the Obsidian community's**, where one vault holds
  several campaigns.

**Bad:**

- **A GM with several campaigns runs several processes**, and has to remember
  which port is which. A reverse proxy fixes the player-facing half of this
  but not the GM's half.
- **Moving a campaign to a different server means re-issuing its join codes**,
  because the session codes are server-scoped. The account join codes are
  vault-scoped and survive; the session ones do not.
- **There is no cross-vault query.** A GM who wants "every character across
  all my campaigns" opens each vault. That is a real limitation for the
  power user, and it is the shape we are choosing.

**Neutral:**

- A vault may still *contain* material for several campaigns — a shared
  setting, a house-rules note — and the `campaign` field says which campaign
  it belongs to, or leaves it campaign-less and therefore GM-only.
- The account file's location is unchanged; only the statement that one
  server serves one vault is new.
