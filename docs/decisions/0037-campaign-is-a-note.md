# ADR-0037: A campaign is a first-class note

- **Status:** Accepted

- **Date:** 2026-10-22
- **Extends:** ADR-0033 (the visibility model) and ADR-0009 (accounts)
- **Addresses:** the campaign and identity model — a pre-phase-1 decision

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

ADR-0033 gave every note a `campaign:` field, so that "party" could mean
"everyone in this campaign" rather than "everyone in this session" and stop
leaking across groups. But it never said **what a campaign is**:

- Where does one live?
- Who is in it?
- How is one created, named, and joined?
- Does an account belong to a vault, or to a campaign?

That last question matters most, because ADR-0033's fix is only real if
membership is *per campaign*. If every account in the vault is in every
campaign, then `visibility: party` collapses back to "all vault accounts",
and the leak it was written to close reopens.

The Obsidian community's convention — one folder per campaign — was
considered and rejected: folders hold no membership, no packs and no identity,
so a `campaign:` field in frontmatter would still be the machine contract,
and the two could disagree.

## Decision

**A campaign is a note of type `campaign`.**

```yaml
---
type: campaign
name: Stormdark
campaign: "[[Stormdark]]"    # its own link, for self-reference
visibility: party
members: [p_8f2a, p_91bc]
packs: [dnd5e]
---
```

> **Amended by ADR-0050, decisions D1 and D2.** `campaign:` holds a **link to
> the campaign note**, not a slug. Nothing resolves a slug: identity must not
> move with renames, and resolution goes through the vault index, which maps
> filenames and aliases to uids. Every other reference in the vault is a
> wikilink.
>
> `campaign` is also now a **note type** in design §5.1's table, so phase
> 1's M7a creates the campaign note rather than skipping it.

A campaign note is the campaign's **definition and its home**. It carries:

| Field | Meaning |
|---|---|
| `members` | The account ids in this campaign. **This list is what "party" means** for every note in the campaign |
| `packs` | The system packs this campaign uses |
| `visibility` | Always `party` for a campaign note — resolved against its own `members` list |
| body | The campaign's description, its dashboard, links to its sessions and notes |

### "Party" is defined by the campaign note

For an ordinary note, `visibility: party` means "the members of the note's
campaign". For a **campaign note**, it means "the members listed on it".

That is one rule, not a new concept: a campaign note defines the party, and
everything else refers to it. The circularity — a campaign note whose
`campaign:` is itself — resolves because membership is read from the note's
own `members` list rather than by looking up the campaign.

### Accounts stay app-private; membership is a campaign property

- **Accounts** live in `.demitable/players.yaml` — id, name, passphrase hash,
  owned characters. Identity is app-private and never in a note.
- **Membership** lives on the campaign note, as a list of account ids. It is
  not secret: an account id is not a credential. It changes rarely, and it is
  exactly the kind of thing a GM wants to see and edit in Obsidian.

The split is deliberate: *who exists* is app state, *who is in this campaign*
is campaign state. The first must never travel with a vault; the second should
be visible in it.

### A campaign is created by the scaffolder

`demitable init` creates the vault and one campaign note. Adding a second
campaign in the same vault is a new note of type `campaign` — the same
gesture as adding a quest.

### Consequences for the projection

The projection gains one rule: for a campaign note, `party` resolves against
its own `members` list. Everything else is unchanged. This is the
project's "one place where the trust boundary lives" growing by a single
lookup, which is the cost of making "one vault, many campaigns" mean
something.

The **"why can this role see this note?" inspector** (ADR-0033) must be able
to explain a campaign-scoped answer, which means it must resolve membership.

### What this does not settle

- **How a player joins a campaign** — the credential model, and whether a
  guest can exist at all, is a separate decision.
- **The seat model** — who is at *this table, right now*, which is not the
  same as who is in the campaign.
- **Whether one server may serve several vaults.** This decision assumes one
  vault per server, with campaigns inside it.

## Consequences

**Good:**

- "One vault, many campaigns" becomes real, and ADR-0033's `party` fix has
  something to scope against.
- **The campaign is visible and linkable.** `[[Stormdark]]` works, the
  dashboard has a home, and the campaign note is the MOC for everything in it.
- **Per-campaign packs** are possible — a campaign can use 5e while another
  uses Blades, in the same vault.
- **Membership is editable in Obsidian**, by the GM, in a place they will
  look.

**Bad:**

- **Membership is app state in a user file.** It can be edited by hand, and a
  hand edit can contradict the accounts file — a member id that does not
  exist. The app must tolerate that rather than crash.
- **The projection grows a special case.** One more rule in the one place
  that must be obviously correct.
- **A campaign note is a note that must not be deleted casually.** Deleting it
  orphans every note in the campaign. The app should refuse, or warn.
- **The vault becomes a multi-tenant space**, which is a shape Obsidian users
  are not used to. One vault now holds several campaigns, and a link between
  two of them crosses a boundary a human may not have intended. The
  projection handles it; the *author* might not notice.
