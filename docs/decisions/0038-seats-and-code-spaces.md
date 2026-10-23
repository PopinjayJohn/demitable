# ADR-0038: Two credentials in disjoint code spaces, and principals instead of roles

- **Status:** Accepted
- **Amended by:** ADR-0057 — a seat's binding is part of the seat's
  identity rather than a grant, so it survives the workflow gate; and the
  campaign boundary constrains the *base level* rather than the grants
- **Date:** 2026-10-23
- **Extends:** ADR-0009 (accounts), ADR-0017 (between-sessions access),
  ADR-0033 (the visibility model), ADR-0037 (a campaign is a note)
- **Addresses:** the seat and guest identity — which the adversarial review
  found had no ADR, no place in the role model, and **both code spaces
  illustrated with the same example value**

## Context

ADR-0009 gave players **accounts**: a campaign join code exchanged for a
passphrase, creating a permanent identity that owns characters and survives
between sessions.

`api.md` §3 later introduced **seats**: "who is at *this table, right now*",
session-scoped, possibly belonging to a guest with no account.

Three problems with that state of affairs:

**1. The two credentials were not separated.** ADR-0009's account code was
"a one-time join code (`K7F2-9QX4`), valid for 24 hours, single use".
`api.md` §3's session code was "short, unambiguous (no `O`/`0`, `I`/`1`,
`L`), shown on the GM's screen, and rotates per session" — **illustrated with
the same value, `K7F2-9QX4`**. Nothing said the two code spaces were
disjoint. A session code read aloud in voice chat could be redeemed against
the account endpoint for a permanent account; an account code photographed for
a 24-hour window could be mistaken for a session code.

**2. A guest seat was not in the role model.** M8 said "Role model: `gm`,
`player`, and later `observer`". A guest seat is none of those. And the
adversarial review found the seat table contradicted itself: one row permitted
a player seat to name an NPC the caller does not own, and three lines later
"A seat naming a character the caller does not own is rejected" — which,
combined with "a guest owns nothing", meant **a guest who joined with the
session code could not play a character at all**.

**3. The role model was the wrong shape anyway.** The projection resolves
visibility from campaign membership, ownership, grants and discovery. A static
role enum cannot express "the person playing Ilia right now", which is
precisely what a seat is — and the seat-scoped projection was chosen
deliberately, for the shared-device case.

## Decision

**Two credentials, in disjoint code spaces. And the projection takes a
*principal*, not a role.**

### The two credentials

| | Campaign join code | Session code |
|---|---|---|
| Scope | One campaign | One session |
| Lifetime | 24 hours, single use | The session |
| Exchanges for | An **account** and a campaign membership | A **guest seat** |
| Resulting credential | A passphrase, then a bearer token | A session-scoped bearer token |
| Revocable | Revoke the membership | End the session, or revoke the seat |
| Ends with | Never | The session ends |

**They are accepted by different endpoints and validated against different
stores.** That is the guarantee — not the alphabet. A campaign code presented
to the session endpoint is simply not a code that endpoint knows. Distinct
lengths (eight characters against six) are a second line of defence, so the
codes are also not *confusable* by eye or by voice.

The session code is **short-lived and revocable**, which is what makes it
acceptable to read aloud over voice chat. The GM can end the session and the
code stops working.

### A seat binds to a character

A seat names the character being played:

- an **account seat** — one of the account's own characters, or one the GM
  assigned to it;
- a **guest seat** — a character the GM assigned in advance, since a guest
  owns nothing;
- a **spectator seat** — no character, for someone watching.

**The seat's bound character is implicitly visible to that seat**, as if the
seat had been granted it. That is the seat-scoped projection, and it makes
the guest case work: a guest sees their character, and nothing else.

### Principals, not roles

The projection's input is a **principal**:

| Principal | Who it is | What it resolves against |
|---|---|---|
| `gm` | The vault's owner | Everything. The GM's view is never filtered |
| `account` | An account, identified by id | Its campaign memberships, its owned notes, its grants |
| `seat` | A session seat, identified by id | Its bound character, and the grants made to it |
| `anonymous` | No credential | `visibility: public` only |

**The role enum goes away.** A principal says *who is asking*; the visibility
model says *what they may see*. An account and a seat differ only in how they
resolve — one through campaign membership and ownership, the other through its
character binding and its grants — and that difference is now explicit rather
than implied by a role string.

This also fixes the guest contradiction: a guest seat is a principal like any
other, so it can hold a character binding, so it can play.

### What a guest can and cannot see

| | Guest seat |
|---|---|
| The session's live state — board, initiative, dice log | **Yes.** That is the table, not the campaign |
| Its bound character | **Yes** |
| Anything explicitly granted to it | **Yes** |
| Campaign notes | **No.** A guest is not a campaign member |
| Other players' characters | **No** |

A guest is tightly scoped by construction, and the scoping is the *same*
projection the rest of the system uses. Nothing new had to be invented for
guests.

## Consequences

**Good:**

- **The credential-confusion hazard is gone.** Different endpoints,
  different stores, different lengths.
- **Both real cases are covered** — a regular player who needs persistence,
  and a friend who came along once and should not need an account.
- **A leaked session code dies with the session.** A leaked campaign code
  creates an account with a passphrase, which the GM can revoke by removing
  the membership.
- **The role enum disappears**, and with it the contradiction the review
  found. The projection takes a principal, which is a better shape for a
  system that already has campaigns, grants, ownership and discovery.
- **A guest can play a character**, which it could not under the previous
  rules.

**Bad:**

- **Two code types to explain to a GM.** The GM must mint the right one, and
  a mistake is a player standing at the table unable to join.
- **The session code is read aloud**, so it is heard by whoever else is in the
  voice channel. It is short-lived, but it is not secret in the way a
  passphrase is.
- **Guests are not campaign members**, so they do not see campaign notes at
  all — not even a shared world document the GM assumed they would. That is
  correct, but it will surprise someone.
- **The principal model is a change to the projection's signature**, which
  touches the plugin bridge (`identity.self` / `identity.others`) and the MCP
  endpoint's view.

**Neutral:**

- The seat remains a *session-scoped* concept; the campaign membership is
  what persists. This is the split ADR-0009 and ADR-0017 implied, now
  explicit.
- The account join code's 24-hour single-use lifetime is unchanged; only its
  scope and its name changed.
