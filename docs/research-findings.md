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
| [3](https://github.com/PopinjayJohn/demitable/issues/3) | Handouts — "reveal this to the table" | **Phase 1** — ADR-0033, `handout` note type, `grants`, the reveal event. **Phase 2** — the reveal UI |
| [4](https://github.com/PopinjayJohn/demitable/issues/4) | No chat, no in-game message channel | **Phase 3** — new milestone: table chat. GM→player is a `visibility: owner` note, so it needs no new mechanism |
| [5](https://github.com/PopinjayJohn/demitable/issues/5) | Nothing captures the session *during* the session | **Phase 1** — M1's session store gains a scratch log (it is session-tier and disposable). **Phase 2** — the reconciliation step at session end |
| [6](https://github.com/PopinjayJohn/demitable/issues/6) | The player-facing surface is too thin | **Phase 2** — new milestone: the player journal, the party roster, per-player annotations |
| [7](https://github.com/PopinjayJohn/demitable/issues/7) | No pre-session ritual | **Phase 2** — new milestone: the auto-recap, the campaign charter, house rules, safety tools |
| [8](https://github.com/PopinjayJohn/demitable/issues/8) | "Blind" content at paragraph level | **Phase 1** — ADR-0033, secret blocks, the deferred-grant distinction. **Phase 3** — the reveal UI |
| [9](https://github.com/PopinjayJohn/demitable/issues/9) | Tables are GM-side only | **Phase 2** — new milestone: blind rolls, player-initiated rolls, eligible-player rolls |
| [10](https://github.com/PopinjayJohn/demitable/issues/10) | No portable character sheet | **Phase 2** — new milestone: the print and PDF export of the pack's `views.sheet` |
| [11](https://github.com/PopinjayJohn/demitable/issues/11) | No campaign dimension | **Phase 1** — ADR-0033, the `campaign` field, and `party` scoped to a campaign. **M1, M4, M8** |
| [12](https://github.com/PopinjayJohn/demitable/issues/12) | Accessibility, localisation, RTL | **Phase 1** — ADR-0034, the pack's translatable labels, the colour-independence rule, the default. **Phase 3** — the board's visual vocabulary |
| [13](https://github.com/PopinjayJohn/demitable/issues/13) | Obsidian interop depth | **Phase 1** — M10, the published schema and starter Bases. **Phase 2** — the query language |
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
