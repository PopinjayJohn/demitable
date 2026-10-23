# Research findings: where each one lands

> Every finding from the three research passes, and where it is assigned.
> Sources: [player and GM needs](research/player-gm-needs.md) ·
> [implementation risks](research/implementation-risks.md) ·
> [adversarial review](research/adversarial-review.md).
>
> Issues: [#18](https://github.com/PopinjayJohn/demitable/issues/18) (gaps) ·
> [#36](https://github.com/PopinjayJohn/demitable/issues/36) (implementation) ·
> [#37](https://github.com/PopinjayJohn/demitable/issues/37) (adversarial).

Phase 2 and 3 have no milestone plans yet. Their rows name **the milestone
that must be written**, so the work is not lost when those plans are made.

---

## 1. Player and GM gaps

| # | Gap | Lands in |
|---|---|---|
| [2](https://github.com/PopinjayJohn/demitable/issues/2) | The second screen and the shared device (hot seat) | **Phase 3** — new milestone: the display mode. Changes the projection layer's *shape*, so the **seat-scoped projection** decision is recorded and the shape is settled in phase 3's plan |
| [3](https://github.com/PopinjayJohn/demitable/issues/3) | Handouts — "reveal this to the table" | **Phase 1** — ADR-0033, the `handout` note type, `grants`, the reveal event. **Phase 2** — the reveal UI |
| [4](https://github.com/PopinjayJohn/demitable/issues/4) | No chat, no in-game message channel | **Phase 3** — new milestone: table chat. GM→player is a `visibility: owner` note, so it needs no new mechanism |
| [5](https://github.com/PopinjayJohn/demitable/issues/5) | Nothing captures the session *during* the session | **Phase 1** — M1's session store gains a scratch log (it is session-tier and disposable). **Phase 2** — the reconciliation step at session end |
| [6](https://github.com/PopinjayJohn/demitable/issues/6) | The player-facing surface is too thin | **Phase 2** — new milestone: the player journal, the party roster, per-player annotations |
| [7](https://github.com/PopinjayJohn/demitable/issues/7) | No pre-session ritual | **Phase 2** — new milestone: the auto-recap, the campaign charter, house rules, safety tools |
| [8](https://github.com/PopinjayJohn/demitable/issues/8) | "Blind" content at paragraph level | **Phase 1** — ADR-0035, the `> [!secret]` callout, block identity, server-side stripping, the `revealed` field, and the WebSocket `secret.reveal` command. **Phase 2** — the GM's reveal control and the durable write-back. **Phase 3** — the shared display showing a reveal |
| [9](https://github.com/PopinjayJohn/demitable/issues/9) | Tables are GM-side only | **Phase 2** — new milestone: blind rolls, player-initiated rolls, eligible-player rolls |
| [10](https://github.com/PopinjayJohn/demitable/issues/10) | No portable character sheet | **Phase 2** — new milestone: the print and PDF export of the pack's `views.sheet` |
| [11](https://github.com/PopinjayJohn/demitable/issues/11) | No campaign dimension | **Phase 1** — ADR-0033, the `campaign` field, and `party` scoped to a campaign. **M1, M4, M8** |
| [12](https://github.com/PopinjayJohn/demitable/issues/12) | Accessibility, localisation, RTL | **Phase 1** — ADR-0034, the pack's translatable labels, the colour-independence rule, the default. **Phase 3** — the board's visual vocabulary |
| [13](https://github.com/PopinjayJohn/demitable/issues/13) | Obsidian interop depth | **Phase 1** — M10, the published schema and starter Bases. **Phase 2** — the query language |
| — | **Support for the community's stat block and admonition formats** (ADR-0036) | **Phase 1** — M4, the bestiary reads both stat block dialects. **Phase 1** — M7, the editor renders callouts, `ad-*` blocks and `statblock` blocks natively. The corpus is already in these dialects, so this is reading what we have |
| [14](https://github.com/PopinjayJohn/demitable/issues/14) | Discord and voice interop; the phase-4 audio risk | **Phase 4** — new milestone: the recorder's audio-source model, and webhook delivery of the dice log |
| [15](https://github.com/PopinjayJohn/demitable/issues/15) | GM time sinks — inventory, quick lookup, "still open" | **Phase 1** — M4, the `inventory` field. **Phase 2** — new milestone: the quick-lookup surface, the still-open view, downtime |
| [16](https://github.com/PopinjayJohn/demitable/issues/16) | Combat depth | **Phase 2** — pack-level resources and conditions for concentration and dying states. **Phase 3** — concealment, hidden-until-activated, not-your-turn |
| [17](https://github.com/PopinjayJohn/demitable/issues/17) | Table staples | **Phase 3** — the board: map pins, pings, hot seat, the party view. **Phase 2** — the relation schema, the in-world calendar |

## 2. Implementation and adversarial findings

| # | Finding | Lands in |
|---|---|---|
| [20](https://github.com/PopinjayJohn/demitable/issues/20) | The round-trip contract is unsatisfiable by re-serializing markdown | **DONE** — ADR-0021 rewritten; the editor is CodeMirror 6 with block widgets |
| [21](https://github.com/PopinjayJohn/demitable/issues/21) | The SRD counts describe the copyrighted corpus | **DONE** — the filter is `srdKeys` membership; the numbers corrected |
| [22](https://github.com/PopinjayJohn/demitable/issues/22) | The plugin trust model claims one position and implements another | **DONE** — ADR-0019 and ADR-0027 amended; the phase-1 DoD withdraws the claim |
| [23](https://github.com/PopinjayJohn/demitable/issues/23) | The write-back contract describes two transactions | **Phase 2** — the transaction model must be settled before the write-back is built. **Not yet done** |
| [24](https://github.com/PopinjayJohn/demitable/issues/24) | The visibility model has no default and no property-level resolution | **DONE** — ADR-0033 completes it: default, property-level grants, the no-owner fallback, workflow gating, one definition of `public` |
| [25](https://github.com/PopinjayJohn/demitable/issues/25) | Phase 1's criteria require phases 2 and 3 | **DONE** — the criteria were moved to the phases that own them |
| [26](https://github.com/PopinjayJohn/demitable/issues/26) | wazero has no fuel; `Runtime.Close` does not stop a guest | **Phase 1** — M6, the plugin host: `WithCloseOnContextDone(true)`, a per-call deadline, a memory cap, and the ADR's socket claim corrected |
| [27](https://github.com/PopinjayJohn/demitable/issues/27) | SQLite's `busy_timeout` defaults to 0 | **Phase 1** — M1, the store: the DSN pragmas and a bounded pool |
| [28](https://github.com/PopinjayJohn/demitable/issues/28) | fsnotify's macOS backend opens one fd per file | **Phase 1** — M1, the watcher: a polling watcher as the primary mechanism on macOS and the universal backstop |
| [29](https://github.com/PopinjayJohn/demitable/issues/29) | Obsidian's property types are vault-wide by name | **Phase 1** — M1, the writer: type-aware frontmatter emission |
| [30](https://github.com/PopinjayJohn/demitable/issues/30) | The frontmatter writer is not byte-identical | **DONE** — ADR-0021: frontmatter is patched surgically and never re-encoded |
| [31](https://github.com/PopinjayJohn/demitable/issues/31) | The plugin ABI has no versioning scheme | **Phase 1** — M6, the ABI handshake and the golden ABI test. `api: 1` freezes at M11 |
| [32](https://github.com/PopinjayJohn/demitable/issues/32) | The shadow-DOM verification is Chromium-only | **Phase 1** — M7, the cross-browser harness, committed to the repository |
| [33](https://github.com/PopinjayJohn/demitable/issues/33) | A vault is untrusted input; no sanitisation | **Phase 1** — M1 and M7: parser budgets, the sanitisation policy, and the Go markdown parser the FTS indexer needs |
| [34](https://github.com/PopinjayJohn/demitable/issues/34) | The corpus is destroyed by the first build | **DONE** — ADR-0024: a pinned, checksummed copy is retained outside git, and `srdcheck` verifies the upstream's licence |
| [35](https://github.com/PopinjayJohn/demitable/issues/35) | Phase 1 is a 3–4 year part-time project | **DONE** — the plan is restructured, the editor is much cheaper with CodeMirror, and the deferrals are recorded in §"Phase 1.5" |

## 3. The scope discipline list

[ADR-0032](https://github.com/PopinjayJohn/demitable/blob/main/docs/decisions/0032-scope-discipline.md)
records the fifteen things we will not build, so that a proposal to add one
has to supersede a recorded decision rather than re-argue it.

---

## 4. The phase-0 findings

Phase 0 answered ten questions by building disposable spikes. Its findings
are the first ones that came from **running** something rather than reading
something, and they are the ones with measurements attached.

| # | Finding | Lands in |
|---|---|---|
| **P0-a** | The frontmatter extractor was never run, and it had a `for` loop with no post statement in its column-to-byte conversion — so every byte span the writer computed was wrong. **Phase 1's M1a would have shipped it** | **Phase 1** — fixed, and the round-trip contract is now a test: patching one field of each of the 1,692 real files leaves every other byte unchanged, in 1,692 of 1,692 cases ([ADR-0049](decisions/0049-frontmatter-io-contract.md)) |
| **P0-b** | **The SRD corpus cannot validate our schema.** Its 1,692 frontmatter blocks hold only four top-level keys, all Obsidian-native; not one of our twelve reserved fields appears anywhere, and there is no comment, block scalar, anchor, merge key, flow collection, nested mapping, wikilink in a property or CRLF file in the whole corpus | **Phase 1** — the extractor carries twelve fixtures of its own for the constructs the corpus lacks, and the corpus is a *test corpus*, not a deliverable |
| **P0-c** | A **one-item list** was written as `key: - item`, which is not YAML — the parser then refused the app's own output | **Phase 1** — a list or map is never inlined, and the block's own sequence style is reproduced ([ADR-0049](decisions/0049-frontmatter-io-contract.md)) |
| **P0-d** | **The schema has no state between "the party knows this" and "the GM knows this".** `gm` is an absolute deny, so an undiscovered location had to be `gm` — invisible for ever — and the Discovery grant in `session-writeback.md` §6 was **unreachable text** | **Phase 1** — a sixth visibility value, `discovered`; an unknown value fails closed ([ADR-0050](decisions/0050-schema-gaps-found-by-writing-a-campaign.md)) |
| **P0-e** | **The reserved layer had no visibility of its own**, so a party-visible character note published the *names* of places the party had not found | **Phase 1** — `discovered` and `grants` are `owner`-scoped; everything else inherits ([ADR-0050](decisions/0050-schema-gaps-found-by-writing-a-campaign.md)) |
| **P0-f** | **Obsidian's `%%comments%%` leak.** They are hidden in preview but were serialised into the projection — the most likely place for a GM's aside to be written by accident | **Phase 1** — stripped from non-GM projections ([ADR-0050](decisions/0050-schema-gaps-found-by-writing-a-campaign.md)) |
| **P0-g** | ADR-0033's own handout example — `visibility: gm` *and* `grants` in one block — is **invisible to the player it was granted to**, because rule 2 is absolute. A reveal that leaves the note `gm` reveals nothing | **Phase 1** — a handout's two states are one state or the other ([ADR-0050](decisions/0050-schema-gaps-found-by-writing-a-campaign.md)) |
| **P0-h** | `campaign: stormdark` is a slug, and **nothing resolves a slug** — identity must not move with renames, and resolution goes through the vault index | **Phase 1** — `campaign:` holds a link to the campaign note, and `campaign` is a note type ([ADR-0050](decisions/0050-schema-gaps-found-by-writing-a-campaign.md)) |
| **P0-i** | Three documents disagreed about `discovered`'s shape: ADR-0040 (simple-typed), `session-writeback.md` §5 (a list of records), and the extractor (which flattens maps to strings on write) | **Phase 1** — a flat list of note references, and the loss of the `how`/`at` granularity is stated rather than hidden ([ADR-0050](decisions/0050-schema-gaps-found-by-writing-a-campaign.md)) |
| **P0-j** | **`srdcheck` had nothing to verify.** The documents promised it would check the upstream URL, the commit hash and the licence text, and none of those values was recorded. Looking for the upstream found that **its canonical repository is no longer publicly resolvable** — only unofficial forks remain, all MIT-licensed at the repository level, which covers the tool and not the data | **Phase 2** — the pin is a hash of the *artifact*, not a commit in a fork; the licence never rested on the upstream ([corpus-provenance.md](corpus-provenance.md)) |
| **P0-k** | The extractor reported **every** top-level boolean as an issue, so a GM's own `dark_mode: true` was reported as unclean — the extractor was asking a question about the schema it has no way to see the answer to | **Phase 1** — booleans are reported only for fields the app owns ([ADR-0050](decisions/0050-schema-gaps-found-by-writing-a-campaign.md)) |
| **P0-l** | **Hiding was treated as partly a rendering concern and partly a serialisation concern** — and only the second one matters. The design said a GM aside was "hidden in preview"; the projection serialised it anyway | **Phase 1** — the projection is the single serialisation boundary for secrecy, and every hiding mechanism is stated in those terms ([threat-model.md](threat-model.md)) |
| **P0-m** | The phase-1 exposure checklist was **unowned and seat-blind** — it predated the session code and the GM secret, and it was authored *alongside* the threat posture rather than derived from it | **Phase 1** — the threat model is the source of truth, the checklist is derived from it with an owner, and it gains five rows ([ADR-0051](decisions/0051-threat-model-and-not-defended-list.md)) |
| **P0-n** | **The phase-1 Definition of Done claimed a GM-only note was "provably unreachable by players *and by plugins*"** — which is false. A compiled-in plugin is the program, and a plugin's UI runs in the app's document with the app's privileges | **Phase 1** — the DoD now claims unreachability by a *player* and says plainly what it does not claim ([ADR-0051](decisions/0051-threat-model-and-not-defended-list.md)) |
| **P0-o** | Two stale documents: `operations.md` said archive import was "allowed" when it is off unless enabled, and ADR-0025 still used `--insecure` where ADR-0031 and the phase-1 plan say `--allow-http-import` | **Phase 1** — both corrected |

---

## What is still open

- **The write-back transaction model** ([#23](https://github.com/PopinjayJohn/demitable/issues/23)).
  It has two consumers and is scheduled for phase 2, so it must be settled
  before it is built. The research recommends partial-apply with per-key
  conflict reporting, because that is the only model consistent with M1's
  compare-and-swap, and journal-based recovery rather than a staging
  directory, because a cross-directory atomic move does not exist.
- **The phase 2 and 3 milestone plans.** Sixteen gaps are assigned to them by
  name above, but no plan exists. The phase-2 plan is the next piece of work.
- **The seat/guest identity** (from the adversarial review). The
  account-versus-seat distinction is designed and recorded in `api.md` §3, and
  the seat-scoped projection is decided, but the seat has **no ADR, no role in
  the role model, and no disjoint code space** from the account join code. The
  exposure checklist does not mention seats. It needs its own ADR before the
  session work starts.
