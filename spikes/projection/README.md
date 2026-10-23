# Spike 2 — The projection prototype

> **Question:** *Does the visibility model produce the right thing on real
> data?*

## The answer, in one paragraph

**No — and the reason is more useful than the model.** The SRD corpus cannot
exercise the visibility model at all: its 1,693 markdown files carry exactly
four top-level frontmatter keys (`cssclasses`, `obsidianUIMode`, `aliases`,
`tags`) and **not one of ADR-0040's twelve reserved fields**. So the model
was built against a **synthetic corpus** of 33 notes written to carry the
reserved fields in the combinations that matter, and it was then run over the
real corpus as well. The synthetic corpus found **twenty** defects, of which
**five** are contradictions between accepted ADRs and **three** make a
promised feature inexpressible. The single most consequential: **the GM
cannot see a player's hit points**, because ADR-0033's property-level
`owner` rule omits the GM, and the property-level rule and the note-level
rule disagree about what `owner` means. The second most consequential:
**a grant to one player publishes the grant list to the whole party**, and
**a reveal to one player is a reveal to every player** once it is written
back. The full list, with the ADR that now disagrees, is in
[`adr-proposal.md`](adr-proposal.md).

---

## What was built

A disposable Go program. It is **not** a deliverable and must not be
promoted: it is a resolver plus a report writer, written to be read.

```
spikes/projection/
  main.go          the CLI
  model.go         the vault loader (reads notes through the extractor)
  projection.go    the resolver: principals, rules, properties, masking
  secrets.go       the body scanner: secret blocks and their identity
  report.go        the report
  projection_test.go  the property test (the phase-1 M5c analogue)
  testdata/synthetic/   the synthetic corpus (33 notes + a pack)
```

Run it:

```
go run ./spikes/projection --vault fixtures --out /tmp/report
go run ./spikes/projection --vault spikes/projection/testdata/synthetic \
    --pack spikes/projection/testdata/synthetic-pack.yaml --out /tmp/synth
go test ./spikes/projection/            # the property test
DEMITABLE_CORPUS=../../fixtures go test ./spikes/projection/ -run TestCorpus
```

**Notes are read through `internal/vault/frontmatter` and through nothing
else.** The package was not modified. Where the projection wanted something
the extractor does not provide, it was worked around *outside* the package —
see [§7](#7-what-the-extractor-could-not-do).

The output is three files per run:

| File | What it is |
|---|---|
| `report.md` | The vault's shape, what each principal sees, every pairwise diff, and the rule that decided every note |
| `fields.md` | The field-level projection: every field of every visible note, with the rule that decided it |
| `decisions.csv` | One row per note, one column per principal: visible or not, and why |

---

## 1. The real corpus: the model has nothing to resolve

The projection ran over all 1,693 files in `fixtures/`. The result is
degenerate, and the degeneracy is the finding.

| | |
|---|---|
| markdown files | **1,693** |
| with a frontmatter block | **1,692** (the 1,693rd is `fixtures/data/generated/README.md`, 100 bytes) |
| extractor issues | **0** |
| notes carrying `type` | **0** |
| notes carrying `uid` | **0** |
| notes carrying `campaign` | **0** |
| notes carrying `visibility` | **0** |
| notes carrying `player` | **0** |
| notes carrying `workflow` | **0** |
| notes carrying `grants` | **0** |
| notes carrying `discovered` | **0** |
| notes carrying `revealed` | **0** |
| **notes carrying any ADR-0040 reserved field** | **0 of 1,693** |

The four keys the corpus does have, and how many blocks carry them:

| Key | Blocks | YAML type |
|---|---|---|
| `cssclasses` | 1,692 | sequence |
| `obsidianUIMode` | 1,692 | string |
| `aliases` | 1,663 | sequence |
| `tags` | 1,662 | sequence |

**Every corpus note is campaign-less and visibility-less, so rule 6's default
makes every one of them `gm`.** The projection over the real corpus is
therefore:

| Principal | Notes | Fields | Body bytes |
|---|---|---|---|
| `gm` | 1,693 | 6,709 | 2,738,337 |
| `anonymous` | **0** | **0** | **0** |

There is no account principal to compare against, because no corpus note
names one: the corpus has no `player`, no `members` and no `grants`, so the
projection derives **no accounts at all**. The corpus cannot exercise the
model, and it cannot exercise the *secrets* either — **0 of 1,693 files
contain a secret block** in any of the four syntaxes ADR-0035/0036 name
(`[!secret]`, `ad-secret`, `gm: true`, `<!-- secret:begin -->`), and none
contains a `> [!secret]`-style callout of any kind.

> **This is the finding the brief anticipated, quantified.** The corpus
> proves the extractor handles real files. It proves nothing whatsoever
> about the visibility model. The synthetic corpus below is the only thing
> that tested it.

## 2. The synthetic corpus

33 notes, written by hand, each one carrying exactly one rule or one trap.
It is committed, because it is the only regression suite for the model that
exists.

**Campaigns and accounts**

| | |
|---|---|
| `stormdark` | members `p_ilia`, `p_bram`, `p_sana` |
| `emberfall` | members `p_ilia`, `p_kade` |
| `ashfall` | **no campaign note** — the trap |

**The cases, and what each one exercised**

| Note | What it exercises | Result under ADR-0033 as written |
|---|---|---|
| `world/saltmarsh.md` | `visibility: party` | the party sees it |
| `world/drowned-quarter.md` | **no `visibility:`** — rule 6's default | the party sees it (campaign ⇒ `party`) |
| `world/gm-plan.md`, `world/undead-tomb.md` | `visibility: gm` — rule 2 | nobody but the GM |
| `chars/npc-harlan.md` | **the owner fallback**: pack declares `resources.hp` as `owner`, the note has **no `player`** | rule 7: falls back to the note's visibility ⇒ the party sees the NPC's hit points. The rule works. |
| `chars/ilia-vane.md` | a property declared `owner` on a note that *has* a player; a `display:` mask; a `discovered:` list | **the GM cannot see `hp`** (finding 9); the party sees `hp` masked to `??`; the party sees Ilia's `discovered` list (finding 12) |
| `chars/draft-bram.md`, `chars/submitted-bram.md` | the **workflow gate** | owner and GM only, "regardless of its visibility and any grant" — the rule works |
| `chars/retired-ilia.md` | `workflow: retired` | **party-visible** under ADR-0033, author-and-GM under ADR-0009 (finding 8) |
| `journals/ilia-private.md` | the **`private` value** | visible to p_ilia; **not** to the GM — ADR-0038 disagrees (finding 6) |
| `journals/ilia-owner.md` | `visibility: owner` | p_ilia and the GM |
| `world/public-lore.md` | `visibility: public` | everyone, authenticated or not |
| `world/party-no-campaign.md` | `visibility: party`, **no `campaign`** | **nobody but the GM** (finding 3) |
| `world/ashfall-town.md` | `campaign: ashfall`, **no campaign note** | **nobody but the GM** (finding 4) |
| `world/emberfall-shared.md` | a grant **across campaigns** | hidden under the strict reading, visible under the other (finding 14) |
| `handouts/duke-letter.md` | ADR-0033's own handout example: `visibility: gm` + `grants:` | **the grant does nothing** (finding 10) |
| `handouts/duke-letter-revealed.md` | the handout *after* its visibility was mutated | the party sees it — the only shape in which a handout can reach a player |
| `world/tavern-letter.md` | a `> [!secret]` callout | the block is stripped for every non-GM; the id is the explicit `^tavernkey` |
| `world/revealed-secret.md` | a secret in `revealed:` | kept, for everyone |
| `world/secret-to-one.md` | a secret revealed to **one** player | **reaches four non-GM principals** (finding 16) |
| `world/secret-codeblock.md`, `world/secret-marker.md` | the code-block and marker-block secret syntaxes | stripped |
| `world/secret-trailing-id.md` | a block reference at the end of the block's last line | stripped; the id is found |
| `world/lost-relic.md` | a note owned by someone else, **discovered** by Ilia's character | p_ilia sees a note she does not own (finding 13) |
| `world/ownerless.md` | `visibility: owner` with **no `player`** | **nobody but the GM** — the note-level rule has no fallback, and the property-level rule does (finding 5) |
| `world/odd-visibility.md` | `visibility: secret` — an unknown value | **nobody but the GM** (finding 19) |
| `content/owlbear.md` | `content_type: monster`, **no `visibility:`**, while the pack declares `monster` as `gm` | **the party sees the monster** (finding 2) |
| `content/owl-familiar.md` | the same content type, with the visibility stamped | the party sees it |
| `chat/*.md`, `tables/rumours.md` | chat-as-notes and tables-as-notes (ADR-0048, ADR-0022) | projected like anything else |
| `campaigns/*.md` | the campaign note as the definition of "party" (ADR-0037) | `p_ilia` is in both campaigns and sees both |

## 3. What each role sees, over the whole synthetic corpus

| Principal | Notes | Fields | Body bytes | Secret blocks stripped |
|---|---|---|---|---|
| `gm` | **32** of 33 | 204 | 4,076 | 0 |
| `account:p_ilia` | 23 | 148 | 2,790 | 4 |
| `account:p_bram` | 22 | 148 | 2,550 | 4 |
| `account:p_sana` | 18 | 118 | 2,280 | 4 |
| `account:p_kade` | 3 | 17 | 326 | 0 |
| `display:stormdark` | 17 | 114 | 2,208 | 4 |
| `display:emberfall` | 2 | 13 | 254 | 0 |
| `display:ashfall` | **0** | 0 | 0 | 0 |
| `seat:s_guest` | **1** | 4 | 72 | 0 |
| `seat:s_apprentice` | **1** | 4 | 72 | 0 |
| `anonymous` | **1** | 4 | 72 | 0 |

The GM sees 32 of 33 because ADR-0048's `private` hides one note from the GM
— which ADR-0038 says must never happen (finding 6).

The two seats see **one** note each, and it is the public lore note. Neither
seat can see the character it is bound to (findings 14 and 15).

## 4. The diffs, read

The pairwise diffs are in `report.md`. These are the ones that matter.

**`gm` vs `account:p_bram`** — 22 notes in common, 10 only the GM sees:
`campaigns/emberfall` (Bram is not in Emberfall), `handouts/duke-letter`
(rule 2 beats the grant), `journals/ilia-owner` (owner-scoped to Ilia),
`world/ashfall-town` (no campaign note), `world/emberfall-shared` (the
cross-campaign grant, under the strict reading), `world/gm-plan` and
`world/undead-tomb` (rule 2), `world/odd-visibility` (unknown value),
`world/ownerless` (`owner` with no player), `world/party-no-campaign`
(`party` with no campaign). **Nothing leaks.** The absolute deny holds on
every one of them.

**`gm` vs `account:p_ilia`** — the same 10, plus one note Ilia sees and the
GM does not: `journals/ilia-private`. That is the ADR-0038/ADR-0048
contradiction, visible in a diff.

**`gm` vs `account:p_kade`** — 29 notes only the GM sees. Kade is in
Emberfall only, so every Stormdark note is out of scope. Correct.

**`account:p_bram` vs `account:p_ilia`** — Ilia sees one note Bram does not
(`world/lost-relic`, via the discovery grant), and Bram sees one Ilia does
not (`chat/gm-to-bram`, owner-scoped to Bram). The two differ by exactly
their ownership and their discoveries. That is the model working.

**`display:stormdark` vs `account:p_bram`** — the display sees 17 notes,
Bram 22. The five the display does not see are the ones that are not
`party`: Bram's own character sheet, the owner-scoped journal, the
handout, the lost relic and the GM's message. **The display is not the
union**, exactly as ADR-0048 requires — but it is also *smaller* than any
individual player, which is the point.

**`seat:s_guest` vs everything** — one note. The seat is not a campaign
member, so it sees no campaign notes at all; it cannot see its bound
character either (finding 14).

**`anonymous`** — one note, the public one.

## 5. The twenty findings

Ordered by consequence. Each is measured, and each names the ADR it
contradicts.

| # | Finding | Where |
|---|---|---|
| **1** | **The SRD corpus cannot exercise the visibility model.** 0 of 1,693 notes carry a reserved field; every note defaults to `gm`; 0 secret blocks exist. | — (the corpus's shape) |
| **2** | **The pack's content-visibility defaults do not protect anything.** They are applied when the pipeline *emits* a note, not when the projection resolves it. Measured on the real corpus: stamping `campaign:` and forgetting `visibility:` makes one player see **1,693 notes, 8,407 fields and 2,738,250 body bytes** — the whole SRD, including the 370-note, 1,065,488-byte bestiary. One omitted line in the enrichment publishes it. | ADR-0033, phase-1 plan M2 |
| **3** | **`visibility: party` with no `campaign` is a black hole.** "Party" is the members of the note's campaign; there is none, so there is no party. Invisible to everyone but the GM, silently. | ADR-0033 rules 4/6 |
| **4** | **A campaign with no campaign note is a black hole.** `campaign: ashfall` with no campaign note ⇒ the party is empty ⇒ 0 accounts and 0 displays see the note. No diagnostic. | ADR-0037 |
| **5** | **`visibility: owner` with no `player` is a black hole, and the note-level rule disagrees with the property-level rule.** The property falls back to the note's visibility (rule 7); the note does not (rule 4). Two answers to "no owner". | ADR-0033 rules 4/7 |
| **6** | **`private` and "the GM's view is never filtered" contradict each other.** | ADR-0038 vs ADR-0048 |
| **7** | **The workflow gate fires before the base level, so `private` is not private in `draft`/`submitted`.** Rule 3 grants the GM and rule 4 never runs. ADR-0048's claim is unconditional. | ADR-0033 rules 3/4, ADR-0048 |
| **8** | **`workflow: retired` is not gated, but ADR-0009's table says it is.** | ADR-0009 vs ADR-0033 |
| **9** | **The GM cannot see a player's hit points.** A property declared `owner` "resolves against the note's `player`" — and, unlike rule 4, does not include the GM. ADR-0002 says explicitly that a player's HP is visible to "them and the GM". | ADR-0002 vs ADR-0033 rule 7 |
| **10** | **ADR-0033's own handout example does not work.** `visibility: gm` + `grants: [p_8f2a]`: rule 2 says no grant overrides `gm`, so the grant does nothing. | ADR-0033 rules 2/5 |
| **11** | **"Reveal to the shared display" is not expressible.** The display resolves to `visibility: party` content; anything `party` is also visible to every account in the campaign. There is no "the screen, but not the players". ADR-0033 promises three reveal targets; the schema expresses one. | ADR-0033, ADR-0048 |
| **12** | **The reserved fields are published wholesale, and `discovered` is per-character private state.** A party-visible PC shows the whole party `player:`, `workflow:`, `visibility:`, `grants:` **and `discovered:`** — the character's private record of what it has seen and met, including the uid of a GM-only note. ADR-0048 says the display never sees "discovered by one particular character"; nothing in the model can make it private. | ADR-0040, ADR-0048 |
| **13** | **A discovery grant widens a note beyond its base level, with no bound.** Ilia's character discovered Bram's `visibility: owner` item, so p_ilia sees a note she does not own, in no campaign at all. | session-writeback §6 |
| **14** | **Rule 1 versus rule 5: the two readings of ADR-0033 differ on 2 of 33 notes — and one of them breaks the guest.** Under the strict reading a seat cannot see its own bound character, because a seat is not a campaign member and the implicit grant cannot cross the campaign boundary. **ADR-0038's "a guest sees their character, and nothing else" is not expressible.** | ADR-0033 rules 1/5, ADR-0038 |
| **15** | **The workflow gate also blocks the seat's bound character.** Rule 3 fires before rule 5, and the bound character is visible "as if the seat had been granted it" — and a grant never overrides rule 3. A seat bound to a draft character sees only the public lore note. | ADR-0033 rule 3, ADR-0038 |
| **16** | **`revealed:` has no audience, so a reveal to one player is a reveal to everybody.** Measured: 4 non-GM principals receive a secret that was revealed to one. | ADR-0035 |
| **17** | **ADR-0035's block-id example does not match how Obsidian writes them.** All **614** of the corpus's block references are on the line *after* the block, not at the end of the block's last line as ADR-0035's example shows. A scanner that looks inside the block gives every real vault's secret a content hash — exactly what ADR-0035's amendment was written to avoid. Worse: **355 blocks share the id `^statblock`**, so a `revealed:` list keyed by id would reveal 355 blocks at once. | ADR-0035, ADR-0036 |
| **18** | **Masking has no syntax and no place in the schema.** ADR-0033 rule 8 and design §5.4 both say `display:` overrides a value per audience; neither says what it looks like, where it lives, or which audiences exist. The spike had to invent all three. And the shape it invents — `{ value: …, display: { party: "??" } }` — makes a masked property a **mapping** rather than a scalar, which ADR-0040's "simple-typed, top-level, Bases-queryable" rule does not allow. | ADR-0033, ADR-0040 |
| **19** | **An unknown `visibility` value has no default.** Rule 6 covers an *absent* value; `visibility: secret` has no level. The spike fails closed; the model does not say so. | ADR-0033 rule 6, ADR-0040 |
| **20** | **The projection needs a body parser the design has not specified.** ADR-0043 says the write path uses no parser, and ADR-0035/0036 assume the projection can find and strip blocks. Nothing specifies how. The spike wrote a 150-line scanner; the real one needs to be specified, because "the projection must strip blocks" is the part of the model that is *not* obviously correct. | ADR-0035, ADR-0036, ADR-0043 |

### The shape underneath all twenty

**"The first rule that applies wins" is the root defect.** Seven of the
findings (3, 5, 6, 7, 9, 10, 15) are the same bug wearing different clothes:
because the rules are an ordered *decision procedure* rather than a set of
*narrowing constraints*, a rule that is later in the list **never runs**, and
a rule that is earlier in the list **overrides** a rule that should have
narrowed it further.

The model that does not have this class of bug computes an **audience set**
per note:

> the audience of a note is the intersection of every applicable narrowing
> rule — scope, the absolute deny, the workflow gate, the base level, the
> owner's exclusions — widened by the grants and the discoveries that are
> explicitly permitted to widen it.

Under that shape, `private` excludes the GM even in `draft`; `owner` on a
property includes the GM; a grant cannot override the absolute deny because
the deny is an intersection term and not a rule that "wins". The proposal in
[`adr-proposal.md`](adr-proposal.md) writes this out.

## 6. The counterfactual, measured on real data

The corpus's usefulness is as a *counterfactual*. Two runs over the real
1,693 files, with the fields stamped by a script:

| Run | `gm` sees | one player sees |
|---|---|---|
| as the corpus is | 1,693 notes / 6,709 fields / 2,738,337 bytes | **0** |
| + `campaign: stormdark`, `visibility: party` | 1,694 / 10,099 / 2,738,388 | **1,693 notes / 10,099 fields / 2,738,250 bytes** |
| + `campaign: stormdark`, **no `visibility:`** | 1,694 / 8,407 / 2,738,370 | **1,693 notes / 8,407 fields / 2,738,250 bytes** |

The third row is the bug. The pack declares `monster` as `gm`; the note does
not know that, because the pack's default is a write-time default. The
player gets the entire SRD, including **370 bestiary notes and 1,065,488
bytes of stat blocks** — 34.5% of the corpus's body — and **647 items**.

A projection-time fallback to the pack's declared content-type default would
have caught it. That is a phase-1 decision with a number attached.

## 7. What the extractor could not do

**Nothing was needed that the extractor could not do, and the package was not
modified.** It parsed all 1,693 corpus files with **0 issues**, and it read
the synthetic corpus's nested `discovered:` records without complaint. Two
things the projection wanted that live outside the package:

1. **No nested-path accessor.** The extractor returns whole values; the
   projection walks the `Value` tree itself to reach `data.resources.hp`.
   That is fine — but it means the *path syntax* the pack uses to declare a
   property's visibility (`resources.hp`) is a convention the projection
   invents, not something the schema or the extractor defines. ADR-0040
   says the reserved fields are top-level and simple-typed; it says nothing
   about how a pack names a nested property.
2. **`discovered` has two shapes.** session-writeback §5 specifies a list of
   records with a `uid`; the extractor's own fixture uses a list of bare
   uid strings and wikilinks. The extractor reports both faithfully; the
   projection accepts both. **One of the two should be chosen**, and the
   schema should say which.

One thing the extractor *did* decide, and the projection inherited: property
names are matched **case-insensitively** (`Block.Get`), which is right, and
which means `Resources.HP` and `resources.hp` are the same property.

## 8. What changed as a result

- **A new ADR is proposed**: [`adr-proposal.md`](adr-proposal.md) — *the
  visibility model is a set of audiences, not an ordered rule list*, plus the
  nine amendments the findings force. It does not edit `docs/decisions/`;
  the orchestrator lands it.
- **The phase-1 plan needs three changes** (M2, M5, M7), each with a number:
  the projection-time content default, the reserved-field visibility rules,
  and the demo campaign's coverage of the cases the corpus could not
  provide.
- **The demo campaign (question 8) now has a specification**: the synthetic
  corpus's 33 notes are the checklist it must satisfy, and the property test
  in `projection_test.go` is the shape of the test it must pass.

## 9. What this spike did not do

- It did not build the API, the index, the watcher or the editor. It built a
  resolver that reads a directory.
- It did not test the **write** path, optimistic concurrency, or the
  round-trip of a note whose frontmatter the projection rewrote.
- It did not test **assets** (ADR-0026), **search** (the IDF side channel,
  which is question 6), or the **MCP** endpoint.
- It did not test **performance**. 1,693 notes project in 0.5 s including
  the parse; that number is not a baseline (question 9 owns that).
- It did not decide the **audience-set rewrite**; it proposed it. The
  rewrite is phase-1 work and it is the milestone that makes the rest of
  phase 1's "the GM-only note is provably unreachable" criterion testable.
