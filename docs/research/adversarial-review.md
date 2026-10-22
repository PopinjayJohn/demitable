# Adversarial review — Demitable design set

> **Note on ADR-0028.** The review noted that ADR-0028 was missing from the
> sequence. It was never created: the numbering gap is permanent, because
> ADR-0027 was rewritten in place when the shadow-DOM decision landed
> (ADR-0029) and no separate ADR-0028 was ever written. The sequence now
> runs 0001–0027, 0029–0032. The finding about *in-place amendments to
> accepted ADRs* remains valid and is recorded as issue #37.

Reviewed: `README.md`, `docs/design.md`, `docs/phase-1-plan.md`, `docs/tech-stack.md`,
`docs/api.md`, `docs/plugin-api.md`, `docs/session-writeback.md`, `docs/srd-vault.md`,
`docs/shadow-dom-verification.md`, `docs/operations.md`, `docs/decisions/README.md`,
`docs/decisions/0001…0031.md` (30 ADRs; **ADR-0028 is missing** — see §3.14),
plus the corpus actually present in `fixtures/`.

Where a claim is checkable against the corpus in the repository, I checked it and give
the measurement. Where it is not checkable, I say so.

> **Note on document state.** `README.md`, `docs/design.md`, `docs/api.md`,
> `docs/phase-1-plan.md`, `docs/operations.md` and `docs/decisions/0009-player-accounts.md`
> were edited at 12:43–12:45 while this review was in progress. Findings below reflect
> the state as of those edits: the plan gained a "Phase-1 exit criteria for internet
> exposure" checklist, `api.md` gained a "Sessions and seats" section, `M8` gained a
> "Sessions and seats" work item, and `design.md` §9 #11 was reworded to point at
> `operations.md`. Three findings that were true of the earlier state have been
> corrected or withdrawn in place; the withdrawn one is marked.

---

## 1. The five things most likely to be wrong

### 1.1 The licence boundary is specified against the wrong numbers, and the corpus in the repo proves it

**The claim.** `docs/decisions/0023-pack-mechanics-content-in-vault.md` §Context:

> The SRD is **7,012 entities** — 4,469 monsters, 2,417 items, 936 spells, 847 class
> features, 1,785 subclass features, 334 subclasses, 265 feats, 157 backgrounds,
> 157 species, 230 variant rules.

`docs/srd-vault.md` §1 repeats it as a table row:

> | `fixtures/data/**.json` | 17,939 entities | The **full 5e corpus** — only 7,012 are SRD |

`docs/decisions/0024-transient-source.md` §Context:

> That dataset is 17,939 entities, of which only **7,012 are SRD**; the other 10,927
> belong to copyrighted sourcebooks that we have no right to redistribute.

**Why it is wrong.** The corpus is in the repository (`fixtures/`, gitignored but present
on this machine). I counted it.

Every single number in ADR-0023's breakdown is a count of the **whole 5e corpus**, not
of the SRD subset:

| ADR-0023 says | Corpus reality | Where |
|---|---|---|
| 4,469 monsters | **4,469** monsters | sum over `fixtures/data/bestiary/*.json` (mm 450, xmm 503, vgm 143, mtf 140, tftyp 133, wdh 95, wbtw 52, …) |
| 2,417 items | **2,417** | `fixtures/data/items.json` → `item` |
| 936 spells | **936** | `all-index.json` `keys` → type `spell` (the corpus spell list, of which 658 are in `srdKeys`) |
| 847 class features | **847** | `classFeature` across `fixtures/data/**.json` |
| 1,785 subclass features | **1,785** | `subclassFeature` across `fixtures/data/**.json` |
| 334 subclasses | **334** | `subclass` across `fixtures/data/**.json` |
| 265 feats | **265** | `fixtures/data/feats.json` |
| 157 backgrounds | **157** | `fixtures/data/backgrounds.json` |
| 157 species | **157** | `fixtures/data/races.json` |
| 230 variant rules | **230** | `fixtures/data/variantrules.json` |

The ten numbers sum to **11,597**, not 7,012 — so the breakdown does not even sum to its
own stated total. And the actual SRD key set, which `srd-vault.md` §1 itself names as
"the authoritative SRD filter" (`all-index.json` → `srdKeys`), is **3,990** entities:

> 658 monsters · 1,150 items · 658 spells · 537 class features · 153 subclass features ·
> 24 subclasses · 18 feats · **0 backgrounds** · 18 species · 117 variant rules

And the build rule written in ADR-0024 and `srd-vault.md` §2.5 — "only `xphb`, `xdmg` and
`xmm` entities may reach the output" — does not select a subset of that. Of the 3,990
`srdKeys`, **1,991** carry a bare 2014-generation source (`phb` / `dmg` / `mm`) and only
**1,999** are in both sets. Meanwhile the x-source rule on its own admits **3,407**
entities that `srdKeys` excludes (including 69 `xphb` spells `srdKeys` drops). The two
rules differ in *both* directions:

| Set | Size |
|---|---|
| `srdKeys` ("the authoritative SRD filter", `srd-vault.md` §1) | 3,990 |
| x-sourced index keys ("only `xphb`, `xdmg`, `xmm` may reach the output", ADR-0024) | 5,406 |
| In both | 1,999 |
| `srdKeys` only (2014-generation sources) | 1,991 |
| x-sourced only | 3,407 |
| Union | **7,397** — close to the claimed "7,012", and not equal to it |

That last row is probably where the error came from: some union of the two filters
yields a number in the same neighbourhood as 7,012, and the per-type breakdown was then
taken from the whole corpus. Whatever the provenance, the number the licence argument
rests on is not a number the corpus supports.

Three consequences, in descending severity:

1. **The hard build error is specified against the wrong reference.** `srd-vault.md` §7
   and ADR-0024 say a non-SRD entity reaching the output is a hard error, and M5's
   "Done when" says "No non-SRD entity appears anywhere in the output". That per-entity
   assertion is fine. But *every count* the docs give for "the SRD" describes the
   copyrighted corpus. Anyone who sanity-checks the build with "the vault should hold
   7,012 entities" or "the bestiary should have 4,469 monsters" will get a pass on a
   vault that ships 4,469 monsters' worth of non-SRD content. The whole point of the
   filter is to make that impossible.
2. **`srd-vault.md` §8's headline claim is ~7× wrong.** "**It is the content the wizard
   and the VTT need.** 4,469 monsters and 936 spells are a real bestiary and a real
   spell list." The SRD bestiary is 658 monsters (per `srdKeys`); 936 is the count of
   spells in the *index*, of which 658 are in `srdKeys` and 278 are not — so the
   "936 spells" figure is the corpus's, not the SRD's.
3. **The SRD contains zero backgrounds.** The pack declares a `background` content type
   with six fields (`srd-vault.md` §3), and the wizard flow in §4 is "pick species →
   pick **background** → pick class". There is no background content in the SRD at all;
   the 157 backgrounds are 2014 PHB. The wizard's second step has no data, and the data
   that would fill it is not SRD content and so may not be shipped.

Also: "17,939 entities" matches nothing I can measure — the index has 23,117 keys
(17,428 excluding fluff types), the JSON files hold ~26,900 records.

**What would resolve it.** Recompute every number from the corpus and state the filter
exactly once, as one set. Then make M5's acceptance criteria *numeric* against that set
("the output contains exactly N content notes, of which M are monsters; a run that
emits any entity whose index source is not in the allowed set fails the build"), and
either drop `background` from the pack's `content:` declarations or write down where
background content is supposed to come from. Until then, the licence argument — the
project's most consequential legal claim — rests on numbers that are demonstrably the
non-SRD corpus.

---

### 1.2 "GM-only notes are unreachable by plugins" is false, and the two halves of the plugin model contradict each other

**The claim.** `docs/phase-1-plan.md`, "Definition of done for phase 1":

> 4. A GM-only note, a GM-only property and an unapproved character are
>    provably unreachable by players, **and unreachable by plugins**.

**What the ADRs actually say.** `docs/decisions/0027-in-document-components.md` §Decision,
in the text of the import warning:

> Plugins are executable code. A plugin's logic is sandboxed — no network, no
> filesystem — but its interface code runs in the app's own page and can read
> and change what you see. **That includes every note in the campaign, including
> ones marked GM-only.**

`docs/design.md` §4 says the opposite:

> The governing rule: **the core never hands a plugin the vault.**

And `docs/decisions/0019-core-and-plugins.md` §Decision:

> **The core never hands a plugin the vault.** … A plugin is not more trusted than
> the user who installed it just because it is code.

**Why the claim is wrong.** Three ways, in order of how much they matter:

1. **The tier that ships every first-party feature is trusted with everything by
   construction.** `docs/decisions/0020-plugin-tiers.md` trust summary: "| Compiled-in |
   Everything — it *is* the program | No |". The editor, the browser, the search plugin
   and the MCP plugin are all compiled-in. So the "core never hands a plugin the vault"
   rule is true only for WASM *logic*, and false for the plugins that actually exist in
   phase 1. The phase-1 DoD asserts a property that the phase-1 architecture cannot
   have.
2. **For the WASM tier, the sandbox constrains the part of the plugin that is not the
   dangerous part.** A WASM plugin's *logic* is sandboxed; its *UI* is not — ADR-0029 is
   explicit that "all plugin UI runs in the app's document, and therefore has the app's
   privileges". And the UI reaches data through the bridge: `plugin-api.md` §6 shows
   `demitable.call('vault.read', { uid })`, and calls the bridge "The **data** boundary".
   A malicious plugin therefore puts its payload in the UI, calls the bridge, and is
   never subject to the WASM sandbox. The WASM tier's security model is not a model
   against a hostile plugin; it is a model against a hostile *compiled-in author*, which
   is not a threat anyone has.
3. **A plugin's UI can bypass the bridge entirely.** It runs in the GM's document, so
   `fetch('/api/v1/notes/…')` carries the GM's cookie. `plugin-api.md` §6 says "The
   bridge is the **data** boundary", which is false for same-document code. Nothing in
   the set acknowledges this.

**What would resolve it.** Pick one of two positions and write it down:

- **(a) Honest position.** Plugins are trusted with the vault when running as the GM.
  The capability model constrains *accidental* overreach and gives role-scoped
  instances; it is not a defence against a hostile plugin. Then: delete "and unreachable
  by plugins" from the phase-1 DoD, retract ADR-0019's "A plugin is not more trusted
  than the user who installed it", and rewrite the `design.md` §8 risk row "**A plugin
  leaks GM-only data** through its own scope | High | The core never hands a plugin the
  vault" — because a plugin *does* get the vault, it just isn't handed it.
- **(b) Security position.** Plugin UI is a real origin boundary. That is precisely the
  frame tier ADR-0029 kept ("A real document and origin boundary"), and it is the thing
  ADR-0029 argued away. If (b), the DoD survives and the shadow-root decision needs
  re-arguing for *UI* isolation with a *separate origin* rather than a shadow root.

Note the asymmetry worth being honest about: (a) is defensible for a local, trusted-user
tool — it is what Obsidian plugins and VS Code extensions do. (b) is what the phase-1 DoD
currently claims. Right now the document set claims (b) and implements (a).

---

### 1.3 Phase 1's scope is incoherent — three milestones' acceptance criteria require phases 2 and 3 to exist

**The claim.** `docs/phase-1-plan.md`:

> **Explicitly not in phase 1:** dice rolling UI, encounters, the board, audio,
> player-facing character creation

`docs/design.md` §7 Phase 1: "Out of scope for phase 1: dice rolling UI, encounters, the
board, audio, player-facing character creation."

**What the plan actually requires.**

- **M10 "Done when"**: "An SRD monster in the demo encounter **becomes a token**; an SRD
  class **appears as a wizard option**." A token is the phase-3 tabletop (ADR-0016). A
  wizard option is the phase-2 character creation wizard. `srd-vault.md` §8 repeats it
  as a demo-campaign acceptance criterion ("A monster from the SRD becomes a token; a
  class from the SRD becomes a wizard option").
- **M12 "Done when — the six proofs in ADR-0012 all hold"**, and ADR-0012's proof 6 is:
  "The **encounter planner** uses it without special-casing." The encounter planner is
  phase 2 (`design.md` §7).
- **M5 "Done when"** contains three criteria that are M10's work: "The import **refuses
  a URL that resolves to a private address**, and **refuses to reconnect to a different
  address than it validated** (ADR-0031). The import is **disabled unless explicitly
  enabled**, and its fetch requires https." The SSRF defence is a security control for
  a feature that does not exist until M10.
- **M5's CI note** says CI runs "the byte-identical round-trip" against the committed
  vault. The byte-identical round-trip is the *editor's* contract (ADR-0021) and the
  editor is M7 — after M5.

**And the ADR that defines phase 1 contradicts the plan.** ADR-0004:

> The schema is considered validated when a complete character can be created,
> edited, **rolled** and re-read without any of it needing to be hand-fixed.

"Rolled" is dice. `phase-1-plan.md`'s own Definition of done quietly drops it: "a
complete character can be created, edited, have a formula evaluated, be seen by its owner
…". Meanwhile ADR-0008 §Neutral says:

> The dice engine does not need to be in phase 1. It is phase 2 work

— but M3 *is* the dice engine, and M12 requires a working dice-pool roll ("A dice-pool
workflow rolls correctly: rating dice vs opposition dice, highest count wins, with
position and escalation") which needs the engine in phase 1. So three documents
disagree about whether the dice engine is in phase 1, and one of them is the ADR that
settles the phase boundary.

**What would resolve it.** Strip the cross-phase criteria out of M5, M10 and M12 and put
them where the feature lives. Amend ADR-0004 to drop "rolled" (or say "rolled" means
"formula-evaluated"), and amend ADR-0008 to distinguish the dice *engine* (phase 1, per
M3) from the dice *runner* (phase 2). Then re-check the sequencing diagram, which
currently has no edges for M8 → M10 and no edge for M9 → M10 even though M10's criteria
need both.

---

### 1.4 The write-back contract contradicts itself about atomicity, and its conflict rule and recovery default are unimplementable

**The claim.** `docs/session-writeback.md` §2:

> - **Atomicity.** A session either lands completely or not at all. A crash halfway
>   through must not leave half the party at 3 HP and half at 27.
> - **One revert.** The whole session's changes undo with one action.

§2 write order, step 1:

> **Validate.** Every target note still exists and its current hash matches `base`.
> If any does not, the whole batch is aborted and the conflicts are reported.
> Nothing is written.

**What §3 says.** Same document, three sections later:

> A conflict does **not** mean the batch fails. It means the app and the human disagree,
> and only about the keys the session actually touched.

§3's per-key table then skips the conflicting key and writes the rest. So §2 and §3
describe **two different transactions for the same event**: all-or-nothing with abort,
versus partial-apply with a conflict report. §3 is the one that matches the machinery
M1 is specified to build ("**Per-key compare-and-swap** against a base content hash"),
so §2 is the wrong half — and §2 is also where "Atomicity" and "One revert" live.

Additional problems in §3 and §8:

- **The conflict table is not a decision procedure.** The "Session's base value" column
  holds "—" in rows 1 and 2 and the non-value "differs from base" in row 3. Rows 2 and 3
  are the same case stated twice. And no row covers the case where the human's current
  value happens to *equal* the value the session wants to write — under row 2 that reads
  as a conflict, which would report a conflict where there is none.
- **The failure-recovery default cannot be implemented.** §8: "*Default: the batch is
  applied to a **staging directory** and moved into place, so a partial apply is
  impossible rather than recoverable.*" A session batch touches `Notes/Characters/`,
  `Sessions/`, `Encounters/`, `Notes/World/…` — there is no single directory to move,
  and a cross-directory atomic move does not exist. Atomicity has to come from the
  journal (ADR-0015) plus an explicit resume/abort state, not from a rename.
- **Two different batch-revert operations.** ADR-0015 §Reverting: "**Revert a batch** —
  restore the state of a set of files **as of a timestamp**." `session-writeback.md`
  §2 step 4: "One journal entry per file, all sharing the batch id, so the batch reverts
  **atomically**" — i.e. by batch id. Only one of these is specified.
- **Idempotency cannot detect a re-apply.** §4 detects "the batch id already present" in
  the journal. But ADR-0015 says reverting *writes a new journal entry* — so after
  revert-then-re-apply the batch id is still present, and "already written back" is
  indistinguishable from "written back, undone, written back".

**What would resolve it.** Choose the transaction model — I would choose partial-apply
with per-key conflict reporting, because it is the only one consistent with M1's
compare-and-swap — then rewrite §2 to match §3, delete the staging-directory default
and replace it with per-file apply + journal-based recovery, define batch revert by id
(or by timestamp, not both), and make the idempotency key `(batchId, revision)` with an
explicit "reverted" tombstone.

---

### 1.5 The visibility model has no default and no property-level resolution — as specified, the shipped 5e pack hides every NPC's HP from players

**The claim.** `docs/design.md` §5.4, note level:

> | `public` | Anyone with the URL — including between sessions, if the server exposes a campfire view |
> | `party` | All players in the session |
> | `owner` | The player who owns this entity, plus the GM |
> | `gm` | Only the GM. Never serialised into a player response |

Property level, the same section:

> ```yaml
> # in the pack
> resources:
>   - { key: hp, label: Hit Points, visibility: owner }
> ```

**Why it is wrong / undefined.**

1. **No default.** No document states what `visibility` means when the field is absent.
   That is the common case: Rule 1 says "Anything you can write by hand, the app can
   read", and a hand-written note may omit `visibility`. §5.2 lists `visibility: party`
   as though it were always present. A security-relevant field with no default is a hole
   with a default of "whatever the code happens to do".
2. **The property-level rule has no "no owner" case.** `visibility: owner` resolves
   against the note's `owner` field, and §5.2 says `owner` is "absent for GM-created
   notes". The shipped 5e pack declares `hp` with `visibility: owner` (design §6; M4
   lists hp among the pack's resources). So on every GM-created NPC, `hp` has no owner —
   and under a literal reading of the model, no player can see an NPC's hit points.
   ADR-0002 §Decision says the opposite: "omit individual *properties* (a player's HP
   is visible to them and the GM, **not to the other players**)" — which implies an
   NPC's HP *is* party-visible. Both cannot hold with one `owner`-only property rule.
3. **There are no property-level grants at all**, only one deny. `session-writeback.md`
   §6 §Property-level is the entire property-level section: "a property marked `gm` is
   never serialised into a player response." There is no rule for a property with no
   pack visibility, no rule for `owner`-with-no-owner, and no interaction with
   `display:` masking (§5.4) — masking is described but never given an evaluation
   order relative to grants and denies.
4. **The "formalised" model omits workflow.** `session-writeback.md` §6 is introduced as
   "The visibility model, formalised". M8 requires: "Workflow gating: a note in `draft`
   or `submitted` is visible only to its owner and the GM, **regardless of its
   `visibility`**." Under §6's formal model — any grant applies, `gm` is the only deny —
   a `visibility: party` + `workflow: draft` note *is* visible to the party. The formal
   model and the implementation rule disagree, and the formal model is the one written
   down as authoritative.
5. **`visibility: public` is defined three different ways.** §5.4: "Anyone with the URL".
   `api.md` §2: "The server accepts **either** a session cookie or a bearer token **on
   every request**", with `unauthenticated` in the error table. ADR-0017's between-sessions
   list grants a player their character, their discoveries, and "**Their party's shared
   view** of anything marked `visibility: party`" — and does not mention `public` at
   all. So: is `public` visible to an unauthenticated caller, to any authenticated
   player, or to nobody outside a session? Unresolved.

**What would resolve it.** Write down the default. Add a property-level grant table
mirroring the note-level one, with an explicit "no owner → party" (or "no owner → gm")
rule, and an explicit statement of whether a pack's `visibility: owner` on a resource
means "the note's owner" or "the note's owner if it has one, otherwise the party". Add
workflow to the formal model as a second, grant-suppressing deny class. Decide what
`public` means relative to authentication, and state it in one place.

---

## 2. Contradictions table

"A" and "B" quote the competing statements. "Should win" is my judgement of which
document is more likely to reflect intent; in most rows the answer is "the newer,
more specific one", and the fix is to delete the stale text.

| # | File A says | File B says | Should win |
|---|---|---|---|
| 1 | `README.md` "Plugins": "Any plugin may contribute browser UI, which runs in a **sandboxed iframe** and talks to the core over a role-scoped bridge." | `README.md` item 7 and `design.md` §4 "Plugin surfaces (ADR-0029)": plugin UI is a **shadow root**; ADR-0029: "Plugin UI is isolated by shadow DOM, not by a frame." | ADR-0029. The README contradicts itself six lines apart and is the first thing a reader sees. |
| 2 | `design.md` §3 heading: "Architectural thesis — **four rules**" | Same section contains Rules 1–6 (Rules 5 and 6 "added by ADR-0031"); `design.md` §10 change log: "**Two new rules**". | The body. The heading is stale. |
| 3 | `README.md`: "**Six ideas** hold it together:" followed by **8** numbered items | — | The body. |
| 4 | `decisions/README.md`: "They cluster into **six themes**:" followed by **7** (Shape of the product, The architecture, Trust and access, The data model, Distribution, Craft, Scope) | — | The body. |
| 5 | `tech-stack.md` §1 principle 4: "**The vault is the database.** There is no second store. **No SQLite**, no Postgres, no in-memory model that shadows the files." | `tech-stack.md` §2 "What lives in SQLite, and what does not" and §13 ("SQLite is used — but only for derived caches outside the vault"); ADR-0030 | ADR-0030. Principle 4 is a flat contradiction of the same document, eight sections later. |
| 6 | ADR-0008 §Neutral: "The dice engine does not need to be in phase 1. It is **phase 2 work**." | `phase-1-plan.md` M3 (dice engine) and M12 (Blades: "A dice-pool workflow **rolls correctly**"); M12 is in phase 1. | The plan. Amend ADR-0008. |
| 7 | ADR-0004: "a complete character can be created, edited, **rolled** and re-read" | `phase-1-plan.md` Definition of done (no "rolled"); `design.md` §7 excludes dice rolling UI from phase 1. | The plan. Amend ADR-0004. |
| 8 | ADR-0010 §Status: "The reasoning below about **deriving URLs from the request** and shelled-out `git` survives" | `api.md` §8a: "The server **does not trust the request's origin by default**. It generates absolute URLs … from a **configured public base URL**, not from the `Host` header."; ADR-0031 | ADR-0031 / api.md. The URL-derivation reasoning is exactly what ADR-0031 reversed, so the status note preserves the wrong half. |
| 9 | ADR-0010 §Status: shelled-out `git` "survives" | `tech-stack.md` §13: go-git rejected, "The archive import replaced the git route (ADR-0025)". | tech-stack. |
| 10 | ADR-0020 §Consequences "Bad": "**The iframe model constrains plugin UI.** No shared DOM, no shared CSS variables for free, an extra bridge hop for every interaction." | ADR-0029: frames removed; "Custom properties on `:root` reach every shadow surface automatically" — and the iframe model's other complaints are the whole justification for the decision. | ADR-0029. ADR-0020's "Bad" list was never updated; it is now a list of objections to a model the project no longer uses. |
| 11 | ADR-0020 bundle and `plugin-api.md` §1: "`ui/index.html` optional, **iframe entry point**" | ADR-0029: plugin UI mounts into a shadow root the app provides; there is no iframe, so there is no "iframe entry point". | ADR-0029. The manifest's `entry:` field has no defined meaning under the shadow-root model. |
| 12 | ADR-0009 §Field changes: "`workflow` applies to **any note a player owns**, so a player-authored location or item can use the same gate." | ADR-0009's own Scope note: "The review workflow covers **characters only**."; `session-writeback.md` §1: "There is no proposal queue for world changes, and no player-facing 'propose an edit' affordance." | The Scope note — but it is an in-place edit to an accepted ADR, which the README's working agreement forbids. |
| 13 | ADR-0017 §Decision: "The ability to change the world. The world is the GM's; **players propose, the GM decides**." | ADR-0009 Scope note + `session-writeback.md` §1: no proposal queue for world changes at all. | session-writeback / ADR-0009. ADR-0017 was never amended. |
| 14 | `session-writeback.md` §2: "**Atomicity.** A session either lands completely or not at all" + step 1 "the whole batch is **aborted**… Nothing is written." | `session-writeback.md` §3: "A conflict does **not** mean the batch fails." | §3 (it matches M1's compare-and-swap). §2 must be rewritten. |
| 15 | `design.md` §5.4 + `session-writeback.md` §6: grant/deny model, no workflow dimension | `phase-1-plan.md` M8: "Workflow gating: a note in `draft` or `submitted` is visible only to its owner and the GM, **regardless of its `visibility`**." | M8's rule; the formal model needs a workflow deny. |
| 16 | ADR-0002: "a player's HP is visible to them and the GM, not to the other players" (⇒ NPC HP is party-visible) | `design.md` §6 pack example `hp: … visibility: owner` + §5.2 "owner … absent for GM-created notes" | Needs a third rule, not a winner. This is a real gap, not a stale sentence. |
| 17 | `design.md` §5.2: universal fields exist so they work with "Obsidian's Bases, whose property system **does not support nested objects**" | `session-writeback.md` §5: `discovered:` is a **list of objects**, and it is called "a **universal** field" — but it is not in §5.2's universal list at all. | The universal layer. `discovered` breaks its own layer's stated constraint and is missing from the schema it claims to belong to. |
| 18 | `session-writeback.md` §5: "**Packs may extend the vocabulary** but the field itself is universal." | `design.md` §5.2: universal fields are "the same across every system"; "Neither is pack-specific". | §5.2. Packs cannot extend a universal field's vocabulary without breaking the layer. |
| 19 | `design.md` §4 package layout (10 packages under `internal/`) | `tech-stack.md` §3 package layout (14 packages: adds `import`, `store`, `assets`, `session`; moves `journal` out of `vault/`) | tech-stack. Two competing "package layout" listings; M1/M10 create packages only tech-stack lists. |
| 20 | `tech-stack.md` §1 principle 5: "The only network calls in the product are the **archive import and the optional MCP endpoint**." | `design.md` Rule 6: "The only optional network features are archive import, the MCP endpoint, **a cloud transcription engine**, and an advisory 'a new version is available' notice."; ADR-0018 | design Rule 6. (MCP is also an *inbound* endpoint, not a network call the app makes.) |
| 21 | `design.md` §4 endpoints: "`/ws` WebSocket — live session state (**phases 2–4**)" | `phase-1-plan.md` M6 (phase 1): "The **REST and WebSocket contract** (`api.md`)" | The plan, but only if M6 also stops claiming phase-2/3 subject matter. Otherwise M6 builds a session protocol with no session. |
| 22 | `phase-1-plan.md` M1: "`internal/store` — The SQLite connection and schema: `note_fts` (FTS5), **and the session tables**" (one database) | ADR-0030 and `tech-stack.md` §2: two files, `search.db` and `session.db` | ADR-0030. Also: M1 builds session tables in phase 1 with no phase-1 consumer. |
| 23 | ADR-0025: "`--insecure`  allow plain http (default: https only)" | ADR-0031: "The URL must be `https`, unless `--allow-http-import` is passed."; M10 lists **both** names | One flag. Pick one and fix all three documents. |
| 24 | ADR-0031: "The import is **disabled by default** and must be enabled per installation." M10: "The import is **disabled unless explicitly enabled**." | `operations.md` §5 table: "| Archive import | allowed | allowed, with SSRF defence always on |" — i.e. allowed by default. `operations.md` §9: "The import is off unless you enabled it." | ADR-0031 + M10 + operations §9. operations §5's table row is wrong, and **no document names the flag or setting that enables it**. |
| 25 | ADR-0030: "The `vault-id` is stored in the vault's own `.demitable/config.yaml`" | `design.md` §5.8: `config.yaml` is "app settings: which packs are installed, defaults". `api.md` §2: "The GM secret lives in the vault's `.demitable/config.yaml`". | One file, three documented contents. Pick one and enumerate. |
| 26 | ADR-0011 §Decision: "**A pack is trusted by virtue of being local.**" | ADR-0011's own amendment and ADR-0025: "**Imported packs are untrusted input.**" | The amendment. The unqualified sentence is still in the Decision section. |
| 27 | ADR-0029: "the cross-browser harness **should exist before the plugin host is built**"; `shadow-dom-verification.md` §7: "Repeat these pages in Playwright's `firefox` and `webkit` projects **before committing to shadow DOM**" | `phase-1-plan.md` M7: "The same harness runs against Firefox and WebKit **before the plugin host is considered done**" — i.e. after the plugin host is built. And `shadow-dom-verification.md` header: "Raw harness and full transcripts **live outside the repository**." | ADR-0029. The harness is a phase-1 deliverable with no owner, no repository location, and the wrong position in the sequence. |
| 28 | ADR-0019: "**Only then** freeze it at `api: 1`." `plugin-api.md` header: "It is explicitly **not** frozen. `api: 1` is not declared here." | `plugin-api.md` §2 manifest: "`api: 1` # the plugin API major it targets"; ADR-0020's example manifest: "`api: 1`" | The freeze point. `api: 1` is in every example; nothing marks it provisional; and no milestone freezes it (M11 is the second plugin, and its "Done when" has no freeze criterion). |
| 29 | ADR-0020 / M6 / `plugin-api.md` §7: "The host ABI **is** Extism's, so a plugin written against the Extism PDK would work here unchanged." | `phase-1-plan.md` "Open items to resolve during M1–M6": "**Whether the WASM host ABI should follow Extism's shape exactly, or be simplified**" | Neither: it is undecided and the docs claim it is decided. The ABI as specified (`demitable_call`, `host_vault_read(ptr,len)`, `host_now()`) is not Extism's ABI (Extism's kernel functions are `alloc`/`load_u8`/`store_u8`/`output_set`/`http_request`/`log_info`/`time_now` with different signatures, and the guest export convention differs), so "the Extism PDK works unchanged" is false as written. |
| 30 | `srd-vault.md` §2.5 and ADR-0024 §"Two works, one licence": the upstream is **CC-BY-4.0**, "**no share-alike obligation**". | `srd-vault.md` §7: "**If the upstream is share-alike**, the enriched template inherits that, and it may constrain bundling the template into a released binary." | §2.5 — but §7's hedge is the only honest statement in the set, because ADR-0024 also says the corpus has "no licence file, no source URL, and no version pin anywhere". See §3.12. |
| 31 | `srd-vault.md` §3: "**The body is never modified.** Only the YAML frontmatter block is extended. This keeps the byte-identical round-trip contract (ADR-0021) intact **by construction**". | `srd-vault.md` §5 and M5 step 4: "**Converts relative markdown links to wikilinks.**" — a body modification. | §5. The round-trip contract is not intact "by construction"; the corpus is post-conversion and therefore never contains the relative-link syntax the corpus is supposed to test. |
| 32 | ADR-0021 §1: the golden corpus contains "CRLF, BOM, no trailing newline, emoji, **zero-width characters**, and a file that is nothing but a Dataview block." | M7: "**The golden corpus**: a directory of deliberately awkward markdown fixtures"; ADR-0024: "The committed vault is also the **golden corpus**"; `srd-vault.md` §6: the corpus is 1,692 pipeline-generated files. | Two different things are called "the golden corpus". A generated, uniform corpus cannot contain CRLF, BOM or zero-width characters. |
| 33 | `design.md` §1/Rule 4 and `README.md` rule 5: "**The vault never leaves the GM's machine.**" | ADR-0014: "Connecting an LLM to this server means **the campaign leaves the machine**"; ADR-0018: a cloud transcription engine receives the session audio. | Qualify the absolute. MCP ships in phase 1, so a phase-1 user can break the claim. |
| 34 | `design.md` §4 / ADR-0020: compiled-in plugins are trusted with **everything**; ADR-0019: "The core never hands a plugin the vault." | ADR-0027's warning text: plugin UI "can read and change what you see. That includes every note in the campaign, including ones marked GM-only." | See §1.2 — this needs a decision, not a winner. |
| 35 | `plugin-api.md` §3: "The complete list" of capabilities (14, incl. `session.roll`, `vault.assets`, `process.spawn`). | ADR-0020 / `plugin-api.md` §7: the WASM host ABI exposes six functions (`vault.read`, `vault.write`, `vault.query`, `http.fetch`, `log`, `now`). | Reconcile. Eight capabilities — including `process.spawn`, which phase 4's transcriber needs — are unreachable from a WASM plugin, and no capability→host-function mapping is given. |
| 36 | `plugin-api.md` §3: `dom.inline` is a **per-plugin** capability. | `plugin-api.md` §2: `provides:` is a **per-surface** list (one plugin can publish several surfaces). | A per-surface flag is missing: a plugin with `dom.inline` gets light DOM for *all* its surfaces. |
| 37 | `design.md` §5.3: "Every note gets a `uid` on creation — a **short ULID**." | ADR-0024: "Uids are **derived deterministically from the source path**, not randomly generated."; `tech-stack.md` §3: "we want the uid derivation to be ours, because it has to be deterministic for the SRD pipeline." | ADR-0024. A ULID is time-ordered and cannot be derived from a path, so the SRD vault's uids are a different species from the app's uids — with no stated rule for distinguishing them. |
| 38 | `design.md` §6: `max_hp` (formula) and `maxFormula: max_hp` | `phase-1-plan.md` M4: the 5e pack has resources "hp, **hp_max**, ac, …"; `design.md` §5.2's data has no `hp_max` at all. | Pick one. This is a wire format, and it already has two names. |
| 39 | `design.md` §4 package layout / `tech-stack.md` §8: "`internal/server` embeds `webdist` with `//go:embed`" | `tech-stack.md` §8 build block: "`sync: copy web/dist → internal/server/webdist`" and "The embed directory is kept in the source tree, gitignored… A committed `.gitkeep` keeps `go build` working on a fresh clone." | `go:embed` cannot embed an empty directory, and a fresh clone with only `.gitkeep` produces a binary that serves no frontend while still building. |
| 40 | `design.md` §9 #11: "**Deployment recipes** — drafted in `operations.md`: … **Still to write: the 'how to secure this' guide for a non-sysadmin GM, and the multi-campaign routing story.**" | `operations.md` preamble: "**It is written for a **GM who is not a sysadmin**.**" And `operations.md` §10: "**Multiple campaigns on one server.** The app serves one vault per process. Run two processes on two ports, or a reverse proxy that routes by path." | `operations.md`. The design says two things are unwritten that the operations guide has already written and decided. |
| 41 | `design.md` §9 #7: "**Plugin UI depth** — settled: **iframes** for surfaces, plus in-document web components under an explicit `dom.inline` capability (ADR-0027)." | ADR-0029 and `design.md` §4: plugin UI is a **shadow root**; the frame is retained only for embedding untrusted third-party pages. | ADR-0029. The living design document's own open-questions list still asserts the model that ADR-0029 superseded two sections earlier. |
| 42 | ADR-0009: "The server issues a **one-time join code** (`K7F2-9QX4`), **valid for 24 hours, single use**." The player exchanges it for a passphrase and it becomes a permanent account. | `api.md` §3: "The **session code** … is short, unambiguous (no `O`/`0`, `I`/`1`, `L`), shown on the GM's screen, and **rotates per session**." — illustrated with the *same* example value `K7F2-9QX4`. | Neither — the two documents never say whether the code spaces are disjoint. See §3.27. |
| 43 | `api.md` §3 seat table: "\| `player` \| An NPC played by the GM \| As the NPC's own visibility says \|" — a player seat may name a character the caller does not own. | `api.md` §3, three lines later: "**A seat naming a character the caller does not own is rejected** — the projection still applies to a seat, because a seat is not a privilege." | The second, but then the NPC row must be removed, and the guest case (a guest owns nothing, so by the rule a guest may name no character) has no seat kind at all — `spectator` is "None — the board and the public view only". |
| 44 | `api.md` §2: "**Two flows**, one authorization model" (join, login, gm) and "**The role is fixed at authentication.** The role determines the projection for every subsequent request." | `api.md` §3 adds a fourth flow (`POST /api/v1/sessions/:id/join` with a session code) and a principal — a **guest seat** — that has no role in the role model (`gm`, `player`, `observer`, per M8). | `api.md` §3. The authorisation model now has an identity that is not a role, and §2's summary of it is stale. |
| 45 | `.gitignore`: "Fetched by `tools/srdfetch`" | ADR-0024: "There is no URL, no release feed, and no scripted acquisition"; the fetch step was replaced by `tools/srdcheck`. | ADR-0024. The .gitignore documents a tool that no longer exists. |
| 46 | `.gitignore`: "The committed outputs are NOT ignored: `packs/dnd5e.yaml`, **`templates/dnd5e-srd/`**" | Every document: the enriched vault is at **`dev/srd-vault/`**. README: "`dev/` — development artefacts: the SRD vault and the demo campaign". | The documents. The .gitignore names a path that appears nowhere else. |
| 47 | `tech-stack.md` §5: "Preview | `marked` | **Consistent with Tiptap's own parser**, so a preview and the editor agree" | `marked` and `@tiptap/markdown` are different parsers with different behaviours; the whole editor contract exists because parsers disagree. | Neither. Drop the claim, or render the preview from the editor's own document. |
| 48 | `srd-vault.md` §3 spell row: "`level`, `school`, `classes[]`, `casting_time`, `range`, `components`, `duration`, **`classes`**, `damage`, `healing`" | — | `classes` listed twice. |
| 49 | `srd-vault.md` §6: "24 class-progression files carry **504 occurrences each** of `class"level"`, `class"pb"` and `class"feature"`"; M5: "24 of the class-progression files carry 504 malformed `class"…"` attributes **each**" | Measured: 21 occurrences of each per file, 504 of each across the 24 files. | The measurement. The per-file reading is 24× wrong; the corpus is 24× less malformed than the docs imply. |

---

## 3. Decisions claimed but not made

Blocking-ness: **H** = will stop implementation, **M** = will cause rework or a
late design argument, **L** = will be noticed by a user, not a developer.

### 3.1 How is the archive import enabled? — **H**
ADR-0031: "The import is **disabled by default** and must be enabled per installation."
M10: "The import is **disabled unless explicitly enabled**." operations.md §9: "The
import is off unless you enabled it." `operations.md` §5's table, however, says "| Archive
import | allowed | allowed |". **No document names the flag, config key, or environment
variable that enables it**, and M10's flag list does not contain one. This also collides
with the new-user story: the *only* way to get the flagship SRD content is the import
(ADR-0024), and the import is off by default. A new user's first command is refused for
reasons they cannot find.

### 3.2 How does the GM get a GM role? — **H**
ADR-0031's table: "| GM authentication | implicit when local | **always required** |".
api.md §2: `POST /api/v1/auth/gm { secret }` with the secret "in the vault's
`.demitable/config.yaml`". But every other endpoint requires a token (§2: "on every
request"), and the GM secret is printed to stdout, which a browser cannot read. There is
no documented flow by which the GM's own browser obtains a GM-scoped token — and "implicit
when local" is not implementable without deciding whether any LAN visitor is implicitly GM.
This is the trust boundary's front door.

### 3.3 The formula language's scope model — **H**
ADR-0003: "Identifiers resolve against **the entity's data** (`attributes.str`,
`resources.hp`) and against other formulas." But the pack examples need values that are
not in the entity's data:

- `max_hp: "10 + attributes.con + level * con_mod"` — where does `level` come from? The
  pack's `progression:` block declares `levels: { max: 20, label: Level }`, which is a
  shape, not a value, and §5.2's data has no `level` key.
- `attack_mod: "attributes.str_mod + proficiency + weapon.bonus"` — `weapon` is a
  *context*, not entity data. The dice workflow (`against: resources.ac`, `bonus:
  attack_bonus`) needs it too.

There are then three different context models for one language: the entity scope
(ADR-0003), the mount condition's "small note context" (`plugin-api.md` §2:
`note.type`, `note.data.kind`, `note.system`, `note.visibility`), and `/compute`'s
`context` (`api.md` §4). No rule says how they compose, and no ADR covers it.

### 3.4 The WASM host ABI — **H**
Listed as open in the plan's own open items, claimed as decided in ADR-0020, M6 and
`plugin-api.md` §7. See contradiction #29. Also undecided: the capability→host-function
mapping (14 capabilities, 6 host functions, different names: capability `http.client`
vs host `http.fetch`).

### 3.5 The `api: 1` freeze point and the compatibility policy — **H**
ADR-0019: freeze only after two plugins. M11 (MCP) is the second plugin and is the last
milestone. But (a) `api: 1` already appears in every example manifest, (b) no
milestone's "Done when" freezes it, and (c) "The core supports the current major and the
previous one for a deprecation window" is untestable at `api: 1`, where there is no
previous major. The deprecation mechanism — shims, what breaks, who guarantees them — is
entirely unspecified, and the whole plugin ecosystem depends on it.

### 3.6 Per-surface shadow-vs-light declaration — **H**
`dom.inline` is per-plugin (capability); `provides:` is per-surface. A plugin with two
surfaces cannot have one contained and one not. See contradiction #36. This changes the
manifest format, which is the thing ADR-0019 says must not be designed up front — so it
needs to be decided *by* the editor and MCP plugins, in M7 and M11, and it needs a
migration story for the manifests already written.

### 3.7 The shadow host contract — **H**
`plugin-api.md` §13: "**The shadow host contract.** Whether the app provides the shadow
root and the plugin mounts into it, or the plugin attaches its own. The first is simpler
and keeps the app in control of the root; the second lets a plugin use `mode: 'closed'`."
This determines the manifest's `entry:` semantics (see contradiction #11) and whether
`::part()` is available (R5 / `shadow-dom-verification.md` §5 relies on it). It is
"genuinely open" and it is a manifest-format question.

### 3.8 How the bridge reaches core capabilities, and how a plugin's own service is called — **H**
`plugin-api.md` §6's example calls `demitable.call('vault.read', { uid })` — a *core*
capability. §4 says plugins provide and consume services. The two call shapes are never
distinguished, and the bridge is described as the data boundary, so the bridge appears to
be a second, undocumented API to the core. Nothing states whether a plugin UI can call
its *own* server-side service, and if so, how it is addressed (plugin id + service id?).

### 3.9 Default `visibility`, property-level grants, workflow in the formal model, and `public` vs authentication — **H**
See §1.5. Four separate gaps in one model, all security-relevant, none with a stated
default.

### 3.10 The query language for `list_by` / `GET /api/v1/notes` — **H**
`design.md` §9 #9: "the query language for `list_by` needs defining. The MCP plugin and
the REST API should share that one language." `api.md` §10: "The filter and query
language for `list_by` is undefined." M11's "Done when" requires "A client can list
characters with computed values". The browser plugin (M9) needs the same language to
implement "filters by tag, faction, location, status, system". The cursor format is
separately undefined (`api.md` §10).

### 3.11 The Blades clock/harm "track" has no representation in the core schema — **H**
M12 requires: "**Clocks** work as a resource kind with **size, progress and
segmentation**" and "**Harm** works as a **condition track**, not a numeric pool."
ADR-0012 says the same and adds: "New resource *kinds* must be expressible."

But the core schema (`design.md` §5.2) has `data.resources` as scalar values
(`hp: 23`) and `data.conditions` as a list of flags. ADR-0003's `resources` are "tracked
pools with a kind …, min/max, and a formula for the maximum". A segmented track with
progress ticks is not a pool and not a flag. The design's schema invariants (§6.1) are all
of the form "a pack need not define X" — none covers *new kinds* of resource. So M12's
own gate ("If any of these need a change to the *core* rather than the pack, the pack
format was not generic enough and **phase 2 waits**") is likely to fire, and the design
gives no hint that a core schema change is the expected outcome. This should be an
explicit phase-1 deliverable: a `resource.kind: track` with segments, or an explicit
decision that Blades harm/clocks are expressible as compositions of the existing model.

### 3.12 The upstream corpus licence is asserted, not evidenced — **H (legal)**
ADR-0024 §Context: the corpus has "**no licence file, no source URL, and no version pin
anywhere in the corpus**". §"Two works, one licence": "The corpus is manually acquired,
so its terms cannot be auto-discovered from a licence file beside it — they are
**recorded in the repository**". So the entire redistribution strategy (publishing the
enriched vault as an archive; asserting "no share-alike obligation and nothing constrains
how the repository may be shared") rests on the project's own memory of a licence, with
no URL, no commit hash, no licence text and no licence file anywhere in the repo.
`srd-vault.md` §7 then hedges: "**If the upstream is share-alike**, the enriched template
inherits that" — contradicting §2.5's confident "no share-alike obligation".

Resolution: record the upstream repository URL, the commit hash, the licence file's
verbatim text and its SPDX identifier, and check them in `srdcheck` alongside the corpus
inventory. If the upstream's terms cannot be pinned, the "publish it as an archive"
decision is not safe to make.

### 3.13 The corpus is destroyed and cannot be re-obtained — **H (operational)**
ADR-0024: "After a successful build, `fixtures/` is deleted. It is never committed" and
"The corpus is version 2.28.1, and it is **only manually available**. **There is no URL,
no release feed, and no scripted acquisition.**" M5's "Done when": "`fixtures/` is gone,
and **the repository still contains everything needed to rebuild it**."

Those cannot both be true. The repository does not contain the corpus and the corpus
cannot be fetched. The pipeline is one-shot. The corpus is, in fact, present in this
working tree right now — so the first successful M5 build destroys the only copy of the
project's source data, and after that the "does the pipeline still reproduce the committed
vault?" check (a maintainer task) is unrunnable forever, and any enrichment bug becomes
unfixable without a manual re-acquisition that nobody has documented a source for.

Either keep the corpus (outside git, e.g. in a release asset or a documented mirror with
a pinned URL), or keep a pinned, checksummed copy of the *raw* corpus in a private
artefact store, or accept in writing that the pipeline is frozen after its first run.

### 3.14 ADR-0028 is missing — **M**
`docs/decisions/README.md` indexes 0001–0027, 0029, 0030, 0031. There is no 0028 and no
explanation. The gap sits exactly where the iframe→shadow transition would have been
decided, between ADR-0027 ("in-document components") and ADR-0029 ("shadow DOM
isolation"). Either a decision was made and the record lost, or the numbering skipped
because a draft was discarded. The README says "Every locked decision, in the order it
was made" — that is currently false. The README also says "An ADR is immutable; changing
a decision means writing a new one that supersedes it", and at least six ADRs carry
in-place amendments (0006, 0009, 0010, 0011, 0013, 0020, 0024, 0025, 0027) — the working
agreement is not being followed.

### 3.15 Nine decisions exist only as a change-log line — **M**
`design.md` §10 (2026-10-22) records, with no ADR anywhere: one session note per session
always; the world is the GM's (a narrowing of ADR-0009's Decision, which was therefore
edited in place); class features stay as sections in their class note; spell lists are
derived at read time; a demo campaign sits beside the SRD content; the editor is a
compiled-in plugin mounted in-document; the `when` condition reuses the formula language;
the WASM ABI is Extism's shape implemented on wazero; the core renders plugin config.
Several of these are consequential and format-defining (spell lists derived at read time
is a data-model decision; the demo campaign's existence is a phase-1 deliverable). They
have the reasoning nowhere.

### 3.16 Where the audit log lives — **M**
M8: "An **audit log** of authentication events, and — under `--expose` — of reads of
GM-only notes." ADR-0031 repeats it. But: is it in the vault (`.demitable/`), in SQLite
outside the vault, or on stdout (operations.md §4: "The app writes structured logs to
stdout and manages no log files of its own")? If it is in the vault, it syncs and it grows
against the "journal is excluded from the index" rule; if it is stdout, journald rotates
it away. And recording *every* read of a GM-only note puts a write on the hot path of
every projection.

### 3.17 Where the demo campaign's SRD references resolve — **M**
`srd-vault.md` §8: the demo campaign "lives **beside** it, in `dev/demo-campaign/`" and
"must contain … A played encounter … an SRD monster pulled in as a token". But the SRD
monsters live in `dev/srd-vault/`, a *different directory*. A wikilink cannot cross a
vault boundary, and the "every wikilink resolves" CI check cannot pass across two
directories. The demo campaign is only coherent if it is *merged into* the SRD vault (as
an import with `target: Notes/Content` would do for a user). "Beside" is the wrong word
and the test fixture's layout needs specifying.

### 3.18 Who hosts the published SRD archive — **M**
ADR-0024: the enriched vault "is **published as an importable archive**". ADR-0011/0025:
"no registry we operate", "Discovery is a README, a forum post, a link", "**Not from
us.**" So the flagship content has no distribution point, and `README.md` "Getting a
vault" does not mention the SRD at all — a new user gets `demitable init` and an empty
campaign. Either the project hosts it (a service it has disclaimed) or the new-user
story changes.

### 3.19 A markdown parser for the Go core — **H**
M1: the FTS5 body is preprocessed — "frontmatter stripped, **markdown removed**,
wikilinks resolved to plain text, code blocks and comments skipped". The CI suite
includes a **link checker** that must parse wikilinks out of bodies. Both run in Go.
`tech-stack.md` §3's Go dependency table contains **no markdown parser**, and §13
explicitly rejected the obvious option ("A JS markdown parser in Go | Would need a
JavaScript runtime in the core process"). A Go markdown/link parser is therefore needed
and unchosen. `marked` is a *browser* dependency and cannot serve the FTS indexer.

### 3.20 Sanitisation of rendered markdown — **H**
Nothing in the set mentions sanitisation, XSS, or escaping. The app renders arbitrary
user markdown (a GM's and players' files) into a document that also runs plugin UI, via
`marked` (tech-stack §5) and Tiptap. Plugin UI is explicitly trusted with the app's
privileges. A raw HTML block in a note is the obvious vector, and the corpus *contains*
raw HTML (srd-vault §6: "raw HTML tables with malformed attributes"). Some policy —
sanitise, strip, or sandbox the renderer — has to exist before the editor ships.

### 3.21 Whether search filtering leaks the existence of GM-only notes — **M**
`design.md` §5.9: "Both are filtered by role **before** results reach a player, so
search is part of the security boundary rather than a way around it." But FTS5 computes
BM25 (and therefore IDF, and therefore document frequencies) over the **whole** corpus
including GM-only notes. A term that appears only in GM-only notes changes the IDF and
hence the *ranking* of party-visible notes — a small but real side channel. No threat
model exists for search; the "search is part of the security boundary" claim is
unanalysed.

### 3.22 The `sp:` marker prefix survived the project rename — **L**
`design.md` §5.5 writes `<!-- sp:begin-session-log -->`; ADR-0006 §Mitigations says
`<!-- sp:begin-* -->`. The project was renamed from `sp-ng` to `demitable`
(`design.md` §10). These markers are written into **users' files** — a wire format
identifier that was not renamed. Changing them later is a breaking vault-format change,
which ADR-0031 says must never happen under a running campaign.

### 3.23 Graceful shutdown has no owner — **L**
`operations.md` §4: "It shuts down gracefully on `SIGTERM`: it stops accepting
connections, finishes in-flight writes, checkpoints the SQLite WAL, and exits."
`systemctl restart` is "safe" per the same section. No milestone mentions signal
handling, and no ADR does either. For a 24/7 server that is restarted to update, this
matters.

### 3.24 `demitable init` writing a `.gitignore` has no owner — **L**
`operations.md` §2: "The `demitable init` scaffolder **writes a `.gitignore`** that
excludes it [`.demitable/`]." M10's scaffolder work list does not include it; neither
does `design.md` §5.7. A small orphan requirement in the operations guide.

### 3.25 One vault per process vs one campaign per vault — **L**
`operations.md` §10 decides it: "**Multiple campaigns on one server.** The app serves one
vault per process." `design.md` §9 #2 still lists it as an open question with a
different default, and `phase-1-plan.md`'s open items list it again.

### 3.26 The exposure checklist is unowned, and does not cover the new seat credentials — **H**
`phase-1-plan.md` gained a "Phase-1 exit criteria for internet exposure" section while
this review was in progress, and it is good work: eleven checkable items, in the right
register ("This list is deliberately checkable rather than aspirational"). Two problems
remain.

First, **no milestone owns it.** It sits in the "Cross-cutting" section, outside every
M, so no milestone's "Done when" can fail because of it. Two of its items ("The import
endpoint refuses a URL that resolves to a private… address", "The import requires
https, is **off by default**") depend on M10's import, and one ("The GM authenticates
with a credential, not by being local") depends on M8.

Second, and more urgent: **it does not mention seats or session codes** (§3.27), which
are the newest credential in the system and which the checklist was written to
precede. An internet-exposed server with a rotating, human-readable, read-aloud session
code has an attack surface the checklist does not name, and ADR-0010's threat model
("the network the GM is on, plus one join code") predates it.

### 3.27 The seat / guest identity is a new principal with no ADR and no threat model — **H**
`api.md` §3 introduces seats: "An **account** is who you are *in this campaign* …
A **seat** is who is *at this table, right now*. It is session-scoped, it names the
character being played, and it may belong to an account or to a **guest who has none**."
`phase-1-plan.md` M8 carries it.

The *design* of the account/seat split is good — "They are separate because the cases
are genuinely separate", "a seat is **not** a privilege: the projection applies to it",
"the projection still applies to a seat, because a seat is not a privilege". That is
right, and it is the kind of distinction that is usually collapsed by accident.

But it is a **new authentication principal** and it has:

- **No ADR.** M8 cites "ADR-0009", which is about *accounts* and says nothing about
  seats, guests, or session codes. Under the README's working agreement, a decision
  that changes the trust model gets an ADR.
- **No place in the role model.** M8: "Role model: `gm`, `player`, and later
  `observer`." A guest seat is not a role, and `api.md` §2 still says "**Two flows**,
  one authorization model" and "The role determines the projection for every
  subsequent request". There are now four flows and a principal that is not a role.
- **A credential-confusion hazard.** The *account* join code is "**valid for 24 hours,
  single use**" (ADR-0009) and creates a permanent account with a passphrase. The
  *session* code "rotates per session" and is read aloud. Both are illustrated with the
  same example value, `K7F2-9QX4`, and both are "short, unambiguous, 8 characters". No
  document says the two code spaces are disjoint. If they are not, a session code that
  leaks in a voice chat could be redeemed against `POST /api/v1/auth/join` for a
  permanent account, and an account join code photographed for a 24-hour window could
  be mistaken for a session code.
- **An internal contradiction.** The seat table permits "| `player` | An NPC played by
  the GM |"; three lines later, "A seat naming a character the caller does not own is
  rejected". And a guest owns nothing, so by the rule a guest may name no character at
  all — which leaves `spectator` ("None — the board and the public view only") as the
  only seat kind a guest can hold, i.e. **a guest who joins with the session code
  cannot play a character unless the GM seats them in advance.** That is a defensible
  design, but it is not what the text says, and it is a surprising product behaviour to
  discover at the table.
- **An ambiguous acceptance criterion.** M8's "Done when" says "A wrong passphrase, a
  used join code and an expired join code are all rejected." With two join codes now in
  existence — one 24-hour single-use account code, one rotating session code — "a used
  join code" and "an expired join code" each refer to two different credentials, and
  neither is disambiguated.

### 3.28 The two join codes need one namespace, one lifetime and one threat model — **H**
Covered in §3.27, but separately actionable: state that account join codes and session
codes are drawn from disjoint alphabets or namespaces, that only the account code is
ever exchanged for a passphrase, that a session code is valid only for the session it
was minted for and only for the seat it names, and that the session code is rate-limited
and attempt-counted on an internet-exposed server. None of that exists today.

### 3.29 The formula language has no `!` — **L**
ADR-0003's grammar: `unary := '-' unary | 'not' unary | postfix`, and `logic_and` sits
*above* `unary`, so `not (a && b)` needs parentheses. Mount `when` conditions
(`plugin-api.md` §2) will want `not` over a conjunction. Not a blocker, but the grammar
is a frozen wire format and the omission is cheaper to fix now than later.

---

## 4. Untestable or unverifiable acceptance criteria in the phase-1 plan

For each: the quoted criterion, why it cannot be checked, and a rewrite that can.

### 4.1 M4 — "The 5e pack evaluates a level-5 fighter correctly: modifier +1, prof +3, max HP 38, AC 18, passive perception 13."

**Why it fails.** No stat line is given, and the four numbers are not derivable from one
another. With the pack's own formula `max_hp: "10 + attributes.con + level * con_mod"`:
`10 + con + 5·con_mod = 38` has no integer solution (con = 15.14). Under the reading
"first level is 10+con, then +con_mod per level" it requires CON 16. "modifier +1"
implies STR 12, "AC 18" implies armour (unarmoured AC = 10 + DEX), and "passive
perception 13" is consistent with either WIS 16 *without* proficiency or WIS 10 *with*
proficiency bonus. At least three different characters satisfy the sentence.

> **Rewrite.** "Given the fixture character `fixtures/testdata/fighter-l5.yaml`
> (STR 12, DEX 14, CON 16, WIS 10, CHA 8, level 5, chain mail + shield, proficient in
> perception, `hit_die: 10`, max-HP formula `10 + attributes.con + (level - 1) * con_mod`),
> the 5e pack evaluates: `str_mod` = +1, `proficiency` = +3, `max_hp` = 38, `ac` = 18,
> `passive_perception` = 13. The fixture is committed; the expected values live next to
> it."

The deeper problem: the *formula* is unspecified. "10 + con + level * con_mod" and
"10 + con + (level-1) * con_mod" are both 5e-legal and differ by 5 at level 5. The
acceptance criterion cannot be met until the pack's HP formula is fixed, and M12's
"Blades" proof does not constrain it.

### 4.2 M7 — "The editor is usable with no plugins installed beyond it"

"Usable" is not a predicate. There is no user, no task, no observable.

> **Rewrite.** "With the editor plugin as the only enabled plugin, a user can create a
> note, type prose containing a wikilink, a callout and a `%%comment%%`, save it, and
> see the file on disk byte-identical to the golden-corpus fixture it was derived from.
> The source view is reachable and shows the same markdown. (Automated: the E2E test
> `editor-solo.spec.ts` performs exactly this and asserts the file bytes.)"

### 4.3 M8 — "A property test asserts: for every note, every property and every role, the projection never contains a `gm`-visible field."

**Why it fails.** As written it is **false**: for role `gm` the projection deliberately
contains every `gm`-visible field (`design.md` §4: "The GM's own UI never goes through
the projection — it sees everything, which is the point"; and the GM's own UI *is* the
projection for a GM-role request). `tech-stack.md` §9 repeats the same broken predicate
("For every note, property and role, the projection never leaks a `gm` field"). A
property test that fails on the first generated case is not a gate.

> **Rewrite.** "For every note in the demo campaign and the SRD vault, every declared
> property, and every **non-GM** role (`player` with no owned notes; `player` owning
> note N), the serialised projection contains no field whose visibility is `gm`, and no
> field whose visibility is `owner` for a caller who does not own the note. The GM role
> is asserted separately: it *does* contain every `gm` field."

### 4.4 M9 — "A character's sheet shows the same numbers Obsidian shows in raw frontmatter."

"the same numbers" and "Obsidian shows" are both undefined; Obsidian does not compute
formulas, so "the same numbers" is really "the same numbers the frontmatter says, plus
computed ones".

> **Rewrite.** "For the fixture character `fighter-l5.yaml`, the sheet renders exactly
> the values `GET /api/v1/characters/<uid>?expand=1` returns, and every value that is
> also present in the note's frontmatter is rendered identically to the frontmatter
> value. The set of rendered fields equals the pack's `views.sheet` for that character."

### 4.5 M10 — "A fresh campaign can be created and used without touching the docs."

Not a predicate; "the docs" is not identified; "used" is not identified.

> **Rewrite.** "Running `demitable init /tmp/x && demitable --vault /tmp/x` produces a
> vault that contains one note of every type in §5.1, each with a `uid`, a `type`, and
> resolvable relative links, and that opens in the app with no errors logged. A
> scripted E2E test performs the init, the open, and the open of one note of each type,
> and asserts zero errors."

### 4.6 M10 — "The example campaign passes a round-trip check: open every note, save every note, and diff the vault — **zero unintended changes**."

"Unintended" is doing the work. A stronger and checkable version exists.

> **Rewrite.** "Opening and saving every note in `dev/example-campaign/` without
> modifying it leaves the vault **byte-identical** (`git diff --exit-code` on the
> example campaign is empty). This is the same gate as the golden-corpus round-trip,
> applied to the example campaign."

### 4.7 M10 — "An SRD monster in the demo encounter becomes a token; an SRD class appears as a wizard option."

Not a phase-1 criterion at all (see §1.3). Phase 1 *can* assert the data-level
precondition without the VTT and the wizard.

> **Rewrite.** "Every content note in the demo campaign's encounter is present in the
> vault's index with `content_type: monster` and a `system` naming an installed pack, and
> every class referenced by the demo character exists as a `content` note with
> `content_type: class`. (The token and the wizard are phase-2/3 acceptance criteria
> and live there.)"

### 4.8 M11 — "No GM-only filtering is applied — and the ADR says so explicitly, so the behaviour is documented rather than accidental."

This is a restatement, not a test. The testable version is about the *result*.

> **Rewrite.** "A request for `demitable://note/<uid-of-a-gm-only-note>` over MCP
> returns the note's full frontmatter and body, including fields whose pack visibility
> is `gm` and fields whose `display.party` mask is `\"??\"`. A request to the same note
> over `GET /api/v1/notes/<uid>` with a player token returns `404`."

### 4.9 M1 — "A vault of 5 000 notes indexes in under a second."

Testable, but it has no baseline and no statement of *what* is measured. A 5,000-note
vault does not exist in the repo, so the criterion can only be checked against a
synthetically generated one — which will not exercise `yaml.Node` round-trip retention
(the actual cost driver) or the FTS index.

> **Rewrite.** "A generated vault of 5,000 notes, each with 40 unknown frontmatter keys
> and a 2 KB body, is fully indexed (frontmatter parsed to `yaml.Node`, link graph
> built, FTS5 populated) in under 1 s wall clock, at p95 over 5 runs, on the CI
> runner. The generator is committed."

### 4.10 M1 — "A file changed on disk appears in the index within 200 ms of the change."

Testable, but it needs a definition of "appears" (the watcher fires; the re-parse
happens; the *query* reflects it) and a statement of which clock.

> **Rewrite.** "After `touch`-modifying a note, a `GET /api/v1/notes/<uid>` issued 200 ms
> later returns the new content. Measured from the fsnotify event, not from the write."

### 4.11 M3 — "A mixed d6 (five numbers, one critical face) rolls and logs correctly."

"Correctly" is doing the work.

> **Rewrite.** "A table `mixed-d6` with faces `1..5` (value = the number) and one face
> `critical` (value = 6, `tags: [critical]`) rolled 1,000 times produces: a total
> between 6 and 11 inclusive; exactly one face tagged `critical` in the log per roll;
> and a log entry per roll carrying the six face ids, the total, the roller and the
> timestamp."

### 4.12 M2 — "The malicious suite passes: `constructor.constructor(\"return 1\")()`, `a.__proto__`, deeply nested literals, and a self-referencing formula are all rejected with a useful error, not evaluated."

"a useful error" is not testable, and "deeply nested literals" has no stated depth.

> **Rewrite.** "The malicious suite passes. Each case in `formulas_test.go` asserts both
> (a) that evaluation returns an error, and (b) that the error message contains the
> formula name and the byte offset of the offending token. 'Deeply nested literals'
> means 1,000 nested `[`; it must be rejected with a depth-limit error, not a stack
> overflow."

### 4.13 M5 — the three import/SSRF criteria, and the CI note's round-trip.

Both are misplaced (§1.3). The SSRF criteria belong in M10; the round-trip belongs in
M7.

> **Rewrite (M5).** Remove them from M5; add to M10:
> "The import refuses a URL whose *resolved* address is in 127/8, 10/8, 172.16/12,
> 192.168/16, 169.254/16, ::1, fc00::/7 or fe80::/10; refuses a URL that resolves to a
> public address and then re-resolves to a private one at connect time; refuses
> `http://` unless `--allow-http-import` is passed; and is refused entirely unless
> enabled. Each case is a table-driven test with a stub resolver."

### 4.14 M6 — the missing WASM criterion.

M6's "Work" describes a whole tier (wazero, the Extism-shaped ABI, the host function
set, per-role instances) and its "Done when" has **no criterion that exercises a WASM
plugin at all**. The WASM tier therefore ships untested, and the tier's entire value
proposition is the sandbox.

> **Rewrite (M6).** "A WASM plugin that declares only `vault.read` and reads a note
> hidden from the calling role receives a `forbidden` error from the host function, not
> the note. A WASM plugin that declares no `http.client` cannot call `http.fetch` — the
> call returns an error JSON. A WASM plugin that declares no `dom.inline` mounts into a
> shadow-root host, and one that declares it mounts into a light-DOM host. A compiled-in
> and a WASM plugin with the same manifest see the same scoped view."

### 4.15 M7 — the cross-browser criterion, and the harness it depends on.

> **Rewrite (M7).** "The shadow-DOM harness (`web/test/shadow-harness.spec.ts`, committed)
> runs against Chromium, Firefox and WebKit in CI, and asserts all ten rules in
> `shadow-dom-verification.md` §4 in each engine. CI fails on any engine that does not
> pass." — plus: the harness must be *in the repository*, which today it is not
> ("Raw harness and full transcripts live outside the repository"), and it must be
> available **before** M6, per ADR-0029.

### 4.16 M0 — "serves the shell on one port and no second process is needed."

Weak, but it is the first milestone and it is checkable as written. Worth tightening to
assert the embed actually happened.

> **Rewrite.** "`go build ./cmd/demitable` produces a binary that, started with
> `--vault <tmp> --port 0 --no-browser`, serves `GET /` with the built Svelte bundle
> (asserted: the response contains the bundle hash) and `GET /api/health` with 200, from
> a single process, with no `node` in `PATH`."

---

## 5. Milestones that are really more milestones; the sequencing diagram

### 5.1 The diagram is wrong in at least six places

The diagram in `phase-1-plan.md` §"Sequencing and parallelism" encodes: M0 → M1; M1 →
{M7, M2}; M2 → M3 → M4; M4 → M10; M7 → M10; M10 → M11; M3/M4 → M6; M6 → M10, M8;
M8 → M9; M5 ← M1,M2,M4; M12 ← M2,M3,M4.

Defects:

1. **M6 → M7 is missing as an edge.** The text below the diagram says "**M6 must precede
   M7.** The plugin host exists so the editor can be written against it". The diagram
   shows M7 branching directly off M1. A reader following the picture builds the editor
   against a host that does not exist.
2. **M7 → M5 (or M5 → M7) is missing.** The prose says "**M5 sits early on purpose.**
   The SRD vault is the project's test corpus, and the editor's golden corpus does not
   exist until it does. Running M5 before M7 turns the editor milestone from 'build an
   editor and hope' into 'build an editor against 1,692 files that already break it'."
   But the diagram puts M5 *after* M4 (hence after M2, M3), and M7 off M1 — i.e. the
   diagram lets M7 run in parallel with M5, which defeats the stated purpose entirely.
   The stated rationale and the diagram describe different plans.
3. **M8 → M10 is missing.** M10's "Done when" includes the demo-campaign projection
   test, which needs the projection layer (M8).
4. **M9 → M10 is missing.** The same test is a browser-plugin test: "a player sees the
   visited location and not the unvisited one". M10 as written also says "An SRD
   monster … becomes a token" (M9/browser is the only phase-1 surface that could show
   it).
5. **M6 is shown as depending on M3/M4.** Why the plugin host needs the dice engine is
   not stated anywhere; the host needs the *capability model* and the *service
   registry* (core services, per `plugin-api.md` §4: `dice` is a core service the
   editor's manifest consumes). If M6 needs the `dice` service to exist, that edge is
   right, but then M6 cannot start before M3, which lengthens the critical path
   considerably. If it does not need it, the edge is wrong.
6. **The critical path is unstated.** By the diagram it is M0→M1→M2→M3→M4→M6→M10→M11,
   eight milestones long, with M7 and M9 hanging off it. That is worth writing down,
   because it is the number that determines whether phase 1 finishes.

### 5.2 Thirteen milestones that are thirty-five wearing a trenchcoat

Milestones that are actually several, and how to split:

| Milestone | Why it is really more than one | Suggested split |
|---|---|---|
| **M1** | Vault service + indexes + journal + assets + search + SQLite. The "Work" list has six distinct subsystems (`internal/vault`, `internal/assets`, `internal/search`, `internal/store`, `internal/journal`) and eleven "Done when" bullets. The journal alone (append-only log, content-addressed blobs, revert-one, revert-as-of-timestamp, key-level and line diffs, GC) is a project. | **M1a** vault read/index/watch + frontmatter patch; **M1b** journal + revert; **M1c** assets endpoint; **M1d** FTS5 search + store. |
| **M2** | Formula engine: lexer, Pratt parser, evaluator, scope resolution, cycle detection, sandbox, error reporting, plus a malicious-expression suite. | **M2a** parse + evaluate + dependency order; **M2b** sandbox hardening (the "paranoic" limits ADR-0025 demands). |
| **M3** | Dice engine: notation parser (7 notations), tables/faces, pools, opposed rolls, comparison rules, result typing, the dice log. | **M3a** numeric dice + notation; **M3b** tables/faces/symbols; **M3c** pools/opposed + the log. |
| **M4** | Pack model + loader + validator + linter + registry, *plus* authoring the whole 5e pack. | **M4a** loader/validator/linter; **M4b** the 5e pack. |
| **M6** | Plugin host: manifest parsing, service registry, capability model, per-role instances, mount points, config rendering, `when` conditions, the bridge, the WASM runtime and ABI, asset serving, the browser shell, the REST/WS contract. | **M6a** registry + capabilities + services; **M6b** shadow/light surface hosts + bridge + shell; **M6c** WASM runtime + ABI; **M6d** the API contract. |
| **M7** | Editor: Tiptap integration, six Obsidian syntax extensions, raw node, golden corpus, write-side re-parse verification, extension registry, source view, light-DOM mount, the shadow-DOM harness, and the cross-browser matrix. | **M7a** parse/serialize + raw node + corpus; **M7b** write-side verification + frontmatter panel; **M7c** harness + cross-browser; **M7d** source view + extension registry. |
| **M8** | Accounts (join codes, argon2id, cookies, bearer tokens, rate limiting, audit log) + the projection layer (note-level, property-level, masking, workflow gating, ownership) + the property test. | **M8a** accounts and tokens; **M8b** projection; **M8c** the property test and the exposure checklist. |
| **M9** | Browser plugin (typed views, table/card/board layouts, filters, backlinks, dashboard, character sheet) + search plugin + a relation view "decide at implementation time". | **M9a** typed views + filters; **M9b** sheet rendering; **M9c** search UI. |
| **M10** | Scaffolder + templates + archive import (fetch, SSRF, extract, validate, apply) + plugin warning + the demo campaign + the example campaign. | **M10a** scaffolder + templates; **M10b** import (fetch + SSRF); **M10c** extract + apply + plugin warning; **M10d** the two campaign fixtures. |
| **M11** | MCP: transport, four resource types, six tools, token gating, off-by-default. | **M11a** transport + resources; **M11b** tools. |

Milestones that are *too big to be milestones* rather than too small: **M5** is three
milestones (srdcheck, the enricher, the corpus/validation/link-checker suites) and
should be split, and its "Done when" should be rewritten to remove M7's and M10's work.

Milestones that should be **merged**: M11 (MCP) could fold into M10's tail if the
`api: 1` freeze is the only thing at stake — but no, keep it separate: it is the
*second* plugin and its whole job is to prove the API is not editor-shaped, and that
argument needs the MCP plugin to be genuinely independent, which argues for it being
last and separate.

### 5.3 What the plan is missing entirely

- **A milestone for the exposure checklist** — it now exists in the plan, but it is
  outside every milestone and does not cover the new seat credentials (§3.26, §3.27).
- **A milestone or criterion for the seat / guest identity** — the fourth auth flow, the
  second code space, and a principal that is not a role (§3.27).
- **A milestone for the formula language's contextual scope** (§3.3). M2 as written
  cannot deliver the 5e pack's `weapon.bonus` or `level`.
- **A milestone or criterion for the projection's property-level rules and the default
  `visibility`** (§1.5, §3.9). M8 says "Visibility evaluation per §5.4" — §5.4 does not
  contain the rules.
- **A milestone for sanitisation** (§3.20).
- **A Go markdown parser** (§3.19) — a dependency decision with no ADR.
- **A milestone for the cross-browser harness**, and for putting it in the repository
  (contradiction #27, §4.15).
- **A milestone for the `api: 1` freeze** and the compatibility shim mechanism (§3.5).
- **A milestone for the `dev/srd-vault/` + `dev/demo-campaign/` merge** (§3.17).
- **A milestone for graceful shutdown and the SIGTERM contract** (§3.23).
- **A reconciliation of the two exposure checklists** — the plan's new one and
  `operations.md` §9's, which are different lists.
- **Any account of the *second* demo campaign.** M10 requires both a "demo campaign"
  (to exercise the projection) and an "example campaign" (the acceptance test), with
  overlapping contents (a PC, an NPC, a location, a faction, an item, a quest, a session
  log, an encounter plan, a symbol table). Two fixtures, one purpose, duplicated
  maintenance. One fixture with two named views would do.

---

## 6. Scope reality check

**The short answer: 13 milestones for phase 1, with a ~35-milestone decomposition, is
not a part-time project finishing in a season. At a realistic part-time rate it is a
3–4 year phase 1 on its own, and the design's own risk table says so — without applying
the same scepticism to phase 1 that it applies to phase 3.**

Evidence from the documents themselves:

- ADR-0006 §Note: "**This ADR makes phase 1 noticeably larger.** That is accepted
  deliberately: the editor is the product's floor, and building it after phase 2 would
  mean retrofitting it into every feature."
- ADR-0009 §Bad: "**Phase 1 needs an account system**, even though the wizard is phase
  2… This is the single largest schedule consequence of choosing 'GM and players both'."
- ADR-0019 §Bad: "**Phase 1 grows.** The plugin host is new work on top of everything
  already scoped, and the editor becomes a plugin rather than a feature."
- ADR-0007 §Bad: "5e's vocabulary will leak into the schema if the invariants are not
  enforced."
- `design.md` §8: "| Scope explosion — phase 3 alone is a multi-month product | High |".

**My estimate for the 35-milestone decomposition above**, for one competent full-time
engineer, and for a part-time contributor (≈10 h/week):

| Block | Milestones | Full-time | Part-time |
|---|---|---|---|
| M0–M1 (vault, index, journal, assets, search, SQLite) | 5 | 8–10 weeks | 5–6 months |
| M2–M4 (formula, dice, packs + the 5e pack) | 6 | 8–10 weeks | 5–6 months |
| M5 (SRD pipeline, corpus, suites) | 3 | 4–6 weeks | 3 months |
| M6 (plugin host, four ways) | 4 | 10–12 weeks | 6–7 months |
| M7 (editor, four ways, plus cross-browser) | 4 | 12–16 weeks | 7–9 months |
| M8 (accounts, projection, property tests) | 3 | 6–8 weeks | 4 months |
| M9 (browser + search plugins) | 3 | 6–8 weeks | 4 months |
| M10 (scaffold, import, fixtures) | 4 | 8–10 weeks | 5 months |
| M11 (MCP) | 2 | 3–4 weeks | 2 months |
| M12 (Blades pack — the format gate) | 1 | 3–4 weeks | 2 months |
| **Total** | **35** | **~70–90 weeks** | **~3.5–4.5 years** |

The part-time figure is brutal, and I want to be precise about why: the *critical path*
alone (M0 → M1 → M2 → M3 → M4 → M6 → M7 → M10 → M11 → M12) is ten milestones, and
M6+M7 are ~22 of the ~35. At 10 h/week, M6+M7 alone is roughly a year. And the
"part-time" column assumes no context-switching tax, no life events, and one person
holding the whole design in their head — which, for a design this interconnected, is the
thing most likely to break.

**The four phases total** is therefore **~3–4 years of part-time work** for a
single-part-time-person project, and the design does not say so anywhere. The README
says nothing about timeline; `design.md` §7 says nothing about effort. The nearest thing
to an honest statement is ADR-0016's ("a VTT is not a feature list — it is an application
with a feature list") applied to phase 3 — but it is not applied to phase 1, which is
larger.

**The single largest piece of work hiding in a milestone, sized honestly:**

**M7 — the editor — is at least 12–16 weeks full-time and is not sized honestly anywhere.**
It is presented as one milestone. It contains:
- a Tiptap/ProseMirror integration with **six** custom Obsidian syntax extensions
  (wikilink with alias and heading forms, embed, block reference, callout, comment mark,
  and a raw node) — each of which needs a ProseMirror node/mark, a tokenizer, a
  serializer, and a *round-trip fixture*;
- a **byte-identical** round-trip contract over a 1,693-file corpus that is *known* to
  contain malformed HTML, and which the docs themselves predict will break the parser
  ("build an editor against 1,692 files that already break it");
- a **write-side verification** path (re-parse, compare, block the write, show a diff);
- a frontmatter form panel with `yaml.Node`-level fidelity;
- a source view (CodeMirror 6);
- a plugin extension registry;
- a light-DOM mount with the Tiptap CSS re-homing issue;
- a shadow-DOM harness with **ten** rules, replicated across **three** browser engines.

And its risk is *asymmetric*: `@tiptap/markdown` is **beta** with known round-trip
escaping bugs (ADR-0021 cites tiptap#8519), and ADR-0021 requires the editor to be
"able to swap it out". The design has a plan for the parser being inadequate — the raw
node — but a raw node for *everything the parser gets wrong* is a way of not having a
parser, and the corpus is adversarial on purpose.

The honest sizing: **the editor is a phase.** Not a milestone inside phase 1.

**Second-largest:** M1's journal (append-only, content-addressed blobs, revert-one,
revert-as-of-timestamp, key-level *and* line diffs, reference-aware GC — and a retention
rule that is self-contradictory: ADR-0015 says "Journals are kept for 90 days by
default" *and* "never automatically deleting a journal entry that a revert would still
need", which for a server that runs 24/7 for months means either nothing is ever
collectable or every write older than 90 days is unrevertable). The journal is a small
product on its own.

**What I would actually recommend** (and this is a scoping recommendation, not a
design one): cut phase 1 to a defensible core and move the rest.

- **Move the editor to phase 1.5**, keep Obsidian as the prose editor for phase 1
  (ADR-0001's original plan), and make the app's own editor a phase-1 *read-only*
  renderer plus a structured frontmatter panel. This removes the largest single item
  from phase 1 and lets the schema be validated against real usage earlier, which was
  ADR-0004's stated motivation ("risks spending months building UI against a schema
  that needs three rewrites after real use"). The full editor then gets a schema that
  has survived contact with users. **This is a change to ADR-0006 and ADR-0021 and
  should be an ADR, not a plan edit.**
- **Move MCP (M11) out of phase 1** into a phase 1.5. It is the "second plugin" that
  proves the API is not editor-shaped — but ADR-0019's method can be satisfied by any
  second non-UI plugin, and a *smaller* one would do. MCP drags an external protocol
  into phase 1 for a feature the docs themselves rate Medium risk and which is off by
  default.
- **Move the archive import (M10b/M10c) out of phase 1**, along with its SSRF defence.
  It is a security-sensitive, non-interactive, network-facing feature whose only
  phase-1 user is the maintainer publishing the SRD archive. Keeping it out of phase 1
  removes ~4 weeks and an entire class of security-critical code from the first
  release. (It does then need a different distribution path for the SRD — see §3.18.)
- **Move the Blades pack (M12) earlier, not later.** It is the *format gate*, and the
  format gate should run *before* the schema is frozen by the 5e pack and the editor,
  not after. Right now the order is: build 5e → build the editor → then find out
  whether the format was generic. The gate fires after the fact. Running M12 against
  a skeleton core (M2, M3, M4 only) would tell you within weeks whether clocks and
  harm tracks have a home in the schema — which is exactly the thing I suspect they do
  not (§3.11).

That is not a criticism of the design's reasoning, which is good; it is a criticism of
its *sequencing*, which optimises for "phase 1 proves the schema" but schedules the
proof last.

---

## 7. Claims the evidence does not support

| Claim | Where | What would have to be true | Verdict |
|---|---|---|---|
| "The SRD is 7,012 entities — 4,469 monsters, 2,417 items, 936 spells…" | ADR-0023, srd-vault §1, ADR-0024 | That those numbers are the SRD subset | **False.** They are the full-corpus counts, measured against the corpus in the repo. The SRD key set is 3,990, of which only 1,999 are also in the documented x-source set. |
| "4,469 monsters and 936 spells are a real bestiary and a real spell list" | srd-vault §8 | That the SRD contains 4,469 monsters | **False** — 658. 936 is the corpus spell count, of which 658 are in `srdKeys`. |
| "The corpus is 17,939 entities, of which only 7,012 are SRD; the other 10,927 belong to copyrighted sourcebooks" | ADR-0024 | That the corpus has 17,939 entities | **Unverified.** The index has 23,117 keys (17,428 non-fluff); the JSON holds ~26,900 records. No combination I measured produces 17,939. |
| "The upstream … is CC-BY-4.0 — the same terms … There is no share-alike obligation" | srd-vault §2.5, ADR-0024 | That the corpus's licence is known and pinned | **Asserted, not evidenced.** The corpus has "no licence file, no source URL, and no version pin anywhere" (ADR-0024). srd-vault §7 hedges the opposite way. |
| "A pack written against the Extism PDK would work here unchanged" / "the shape is battle-tested rather than ours" | plugin-api §7, phase-1-plan M6 | That the specified ABI *is* Extism's | **False as specified.** The export is `demitable_call`; the host functions are `host_vault_read(ptr,len)`, `host_now()`, etc. Extism's kernel ABI is different in both names and signatures. What is battle-tested is Extism's *shape*; the implementation is ours and untested. |
| "Frontmatter is tiny; 5,000 notes parse in milliseconds" | ADR-0030 | That `yaml.Node` round-trip retention is as cheap as plain unmarshalling | **Unverified, and probably false.** M1's "Design note" calls the `yaml.Node` round-trip "the single most important correctness detail in the whole project"; comment- and order-preserving YAML is materially more expensive than struct unmarshalling. This number is load-bearing for keeping metadata in memory. |
| "The app opens instantly on a large vault" | ADR-0030 §Good | A measurement | **Unverified.** No benchmark is cited. It follows from the FTS index being persisted — which is true — but the *indexing* cost on first run after a corpus change is not measured. |
| "The core never hands a plugin the vault" / "A plugin is not more trusted than the user who installed it" | design §4, ADR-0019, design §8 risk table | That compiled-in plugins are not "the program" | **False** for the tier that ships every first-party plugin (ADR-0020: "Everything — it *is* the program"). See §1.2. |
| "A frame was never a security boundary" | README item 7, design §4, ADR-0029 §Context | That no frame can be a boundary | **Over-broad.** The design *keeps* a Frame tier described as "A real document and origin boundary" (ADR-0029's own table). A `sandbox`ed iframe without `allow-same-origin` is a genuine boundary. The argument as stated refutes the old model (a *same-origin* frame) and is then used to dismiss a tier that is deliberately *not* same-origin. |
| "The vault never leaves the GM's machine" | README rule 5, design §1, Rule 4 | That MCP and cloud STT are off | **False in the shipped configuration.** ADR-0014 makes MCP a phase-1 feature; ADR-0018 permits a cloud engine. The absolute claim is in the README's first screen. |
| "Search is part of the security boundary rather than a way around it" | design §5.9, ADR-0013 §Neutral | That role-filtering is sufficient | **Unanalysed.** BM25/IDF is computed over the whole corpus, so a term's presence in GM-only notes changes the *ranking* of visible notes. No threat model exists. |
| "A file the app cannot see is a file the app cannot corrupt" | design §5.8 | That the app never writes what it indexes | **Loose.** The app writes the vault, by design; the sentence is about the *index* and should say so. |
| "A note's existence is not secret" (api.md §7) | api.md | That note uids are not enumerable | **Unverified.** `GET /api/v1/notes/:uid` returns `not_found` for hidden notes "because a note's existence is not secret" — but the uid space is derived from file paths in the SRD pipeline (deterministic), and `/api/v1/notes` is listable by any player, so if the SRD's ~3,990 content notes carry the `visibility: party` shown in `srd-vault.md` §3, the whole bestiary and spell list is enumerable. |
| "The import endpoint is a request-forgery primitive" and the SSRF defence | ADR-0031, M10 | — | **Well-founded.** The reasoning is correct and specific, and the defence list (resolve-then-connect, re-check at each redirect hop, caps) is the right shape. The *enablement* mechanism is missing (§3.1) and the flag is named two ways (§2 #23), but the substance is sound. |
| "The core can always enforce role-scoped visibility" | design §4 | That plugin UI cannot reach data outside the bridge | **False for plugin UI** (§1.2). It is true for the WASM *logic* path. |
| "a 5e pack copy" in the example campaign (M10) | phase-1-plan M10 | — | **A real conflict with ADR-0011/0019:** a vault pack shadows a built-in pack of the same id (ADR-0011), so shipping a copy of the 5e pack inside the example campaign *replaces* the built-in pack for that vault. Whether that is intended (a fixture of a shadowed pack) or an accident is not stated, and it interacts with the "one copy of the truth" principle for the 5e mechanics. |
| "The threat model is small and honest: the network the GM is on, plus **one join code**" | ADR-0010 §Good | That there is one join code and one credential type | **Out of date.** There are now two code types (a 24-hour single-use *account* code and a rotating *session* code), a third credential (the GM secret), a bearer token, and a guest identity that needs no credential at all but still receives a session-scoped token. The exposure checklist does not mention seats. |
| "A seat is **not** a privilege: the projection applies to it" | api.md §3 | That a seat carries no authority of its own | **Half true.** The projection does apply — but a guest seat *does* get a session-scoped bearer token and a view of the table without any account, and the table's own row for a player seat naming an NPC contradicts the sentence three lines below it. |

---

## 8. What is genuinely solid

Do not re-litigate these.

1. **The trust boundary is in the right place.** Server-side projection (ADR-0002) is
   the single best decision in the set, and the reasoning for it is correct: it makes
   "a GM-only note is not in the vault the players can see" *structural* rather than a
   convention, and it gives a native client the same guarantee for free ("a native app
   cannot leak GM-only notes, because it never holds them" — api.md §1). The one thing
   to fix is the *plugin* claim layered on top of it (§1.2), not the boundary itself.

2. **The three-layer split.** System = pack, content = notes, application = core +
   plugins (ADR-0023, ADR-0019). The reasoning — that a pack holding 7,012 entities
   would be "unqueryable in Obsidian, ungreppable, un-diffs" — is exactly right, and the
   resulting "one file, two consumers" (wizard option and VTT token) is a genuinely
   elegant consequence.

3. **ADR-0030's refinement of Rule 1.** Replacing "no database" with a *test* —
   "**Can it be rebuilt from the vault, in one pass, without the vault?**" — is the
   best piece of reasoning in the set. It is the only place where a slogan was replaced
   by a criterion, and it immediately caught and fixed the search index and the session
   store. (Its application to live session state is circular, §1.4-adjacent, but the
   *test* is the right instrument.)

4. **The byte-identical round-trip contract, and its three mechanisms.** ADR-0021's
   golden corpus + raw node + lossless-first, plus the write-side re-parse verification
   ("**The app cannot corrupt a vault with a round-trip bug**"), is the right answer to
   the hardest problem in phase 1. The framing — "Losing a user's unknown-to-us syntax
   is the worst failure this editor can have. It's a trust failure, not a bug" — is
   exactly the right prioritisation.

5. **`shadow-dom-verification.md` is exemplary.** It tested rather than reasoned, it
   recorded *ten* implementation rules derived from the testing, it is explicit about
   what was **not** verified (Firefox, Safari, native OS drags, real IMEs, 50 shadow
   roots, plugin↔plugin drags), and it caught a backwards privilege model in ADR-0027
   and corrected it in the open. The two incidental findings (Playwright's selector
   engine piercing shadow roots; a classic-script bundle clobbering `window.DOMParser`)
   are exactly the kind of thing you only learn by running the thing. **The one flaw is
   that the harness itself is not in the repository** (contradiction #27, §4.15), so the
   evidence cannot be re-derived.

6. **The archive-at-a-URL distribution decision.** ADR-0011 + ADR-0025: no registry we
   operate, no signing keys, no revocation, licence terms travel with the content, and
   the extraction is treated as the security-sensitive part it is. The SSRF analysis in
   ADR-0031 (resolve-then-connect, re-check at each hop, caps) is correct and specific.

7. **The `--expose` idea.** "One flag says 'this is exposed', and the settings that
   matter change together" (ADR-0031) is good operational design, and `operations.md`
   is a genuinely good document — written for "a GM who is not a sysadmin", with the
   systemd unit, the Caddy/nginx/tunnel recipes, the troubleshooting table and the
   "check before you expose it" checklist. The backup-first framing ("**Back up the
   vault. It is the only thing that cannot be regenerated.**") and the "Test the
   restore. A backup you have never restored is a hypothesis." are the right
   emphases.

8. **"The world is the GM's."** The narrowing in `session-writeback.md` §1 — one writer
   for world state, no proposal queue — is a real simplification with a stated reason
   (one writer means no merge), and it makes the between-sessions view honestly
   read-only. It is the kind of decision that is usually reached by accident and
   documented by accident; here it was made on purpose.

9. **The write journal, and the question it answers.** ADR-0015's framing — the user's
   actual question is not "did you corrupt my file" but "**what did you just do to my
   character?**" — is right, and the append-only-with-revert primitive is right. The
   "an undo is itself an event" rule is a nice touch. (The retention rule needs
   fixing: "Journals are kept for 90 days" and "never automatically deleting a journal
   entry that a revert would still need" are in tension — see §3.13/§4.)

10. **The write-back *preview*.** `session-writeback.md` §7 — the GM sees exactly what
    will change before it changes — is the right answer to the journal's question asked
    *before* rather than after, and the observation that the preview is rendered by a
    plugin while the data is a core service is a clean application of the core/plugin
    split.

11. **"No automatic update."** "An update that changes the vault format under a running
    campaign is not something an app should do to someone" (ADR-0031) is the right
    instinct, and `operations.md` §7 handles the consequence honestly.

12. **The local-first discipline.** "No core feature may require the internet" is
    stated as a hard requirement with a named list of exceptions, and the exceptions
    (import, MCP, cloud transcription, the version notice) are all opt-in and
    visible-at-the-point-of-enabling. The stance is consistent across ADR-0014, ADR-0018
    and ADR-0031, which is rarer than it should be.

13. **"The app is a core plus plugins", and the *method* for designing the plugin
    API.** ADR-0019's insight — "The plugin API is the highest-risk artifact in the
    project, because everything depends on it and changing it breaks everything.
    Designing it on paper is how that goes wrong" — and the method (write the editor
    first against a provisional API, then a non-UI plugin, then freeze) is exactly
    right. The failure is not the method; it is that the freeze point was never
    assigned to a milestone and the manifest's `entry:` field was frozen by accident
    (§3.5, §3.6).

14. **The decision to correct itself in the open.** ADR-0029's "**That was backwards,
    and this ADR corrects it**" and ADR-0027's rewritten warning text are better
    practice than the alternative. The problem in this document set is not that
    corrections happen; it is that they leave residue (ADR-0020's stale "Bad" list,
    the README's iframe sentence, ADR-0010's status note, `design.md` §9 #7's "iframes
    for surfaces") — which is a *process* fix (one pass to purge superseded text after
    each ADR) rather than a design fix.

15. **The account/seat distinction in `api.md` §3.** "An **account** is who you are
    *in this campaign* … A **seat** is who is *at this table, right now* … They are
    separate because the cases are genuinely separate", and "A seat is **not** a
    privilege: the projection still applies to a seat". That is a sharp distinction
    that most designs collapse, made for the right reason (a player may have an
    account and not be at the table; a guest may come to one session and never make an
    account; a player may sit at two tables playing different characters). It needs an
    ADR, a role, and a disjoint code space — but the *idea* is right and worth keeping.

16. **The exposure checklist.** `phase-1-plan.md`'s "Phase-1 exit criteria for internet
    exposure" — eleven checkable items, including "The app **starts and works with the
    network interface down**, and with DNS unavailable" and "There is **no automatic
    update** path" — is exactly the right standard, and the note under it ("This list
    is deliberately checkable rather than aspirational") is the discipline the rest of
    the plan lacks. It should be reconciled with `operations.md` §9 (which has a
    different, shorter list) and given an owner.

---

## 9. The fastest, cheapest improvements

If you do only these five things, the document set gets materially more correct:

1. **Recompute the SRD numbers from `fixtures/` and rewrite ADR-0023, ADR-0024,
   `srd-vault.md` §1/§2.5/§7/§8 and M5's criteria against the measurement.** The
   licence boundary is the one place where a wrong number is not a documentation bug.
2. **Decide the plugin-UI trust question (§1.2)** and propagate it to the phase-1 DoD,
   ADR-0019, ADR-0027, `plugin-api.md` §6/§12 and `design.md` §8. One paragraph, five
   documents.
3. **Rewrite `session-writeback.md` §2 to agree with §3**, delete the staging-directory
   default, and pick one batch-revert semantics. The write-back contract has two
   consumers and is scheduled to be built in phase 2; it must be right first.
4. **Fix the phase-1 scope leakage in M5, M10 and M12** and add the three missing
   edges to the sequencing diagram (M6→M7, M5→M7, M8/M9→M10). Then split M1, M6, M7
   and M10, and re-estimate.
5. **Write down the defaults**: default `visibility`, the property-level grant table,
   the formula language's contextual scope, the default `visibility` for the SRD's
   ~3,990 content notes (right now the *example* in `srd-vault.md` §3 puts
   `visibility: party` on a content note, and M5's enrichment list does not mention
   `visibility` at all — so either the entire bestiary and spell list is party-visible
   and enumerable, or its visibility is undefined). That is a spoiler leak in the
   flagship content, and it is one line of code to fix and one sentence to write.
6. **Give the seat/guest identity an ADR**, a role, and a disjoint code space from
   the account join code, and add seats to the exposure checklist (§3.27).
