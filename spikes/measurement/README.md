# The measurement spikes — phase 0, questions 6, 7 and 9

> Three disposable spikes. The **answers** are the deliverable; the code is
> thrown away at the end of phase 0.
>
> - **Question 6** — the search spike. *Does FTS5 over the real corpus put
>   the right note first?* → [§1](#1-the-search-spike-question-6)
> - **Question 7** — the watcher spike. *Does the watcher see everything, on
>   every platform?* → [§2](#2-the-watcher-spike-question-7)
> - **Question 9** — the baselines. *What do the numbers actually cost?* →
>   [§3](#3-the-baselines-question-9)
>
> The proposed ADR changes are in [`adr-proposal.md`](adr-proposal.md). The
> orchestrator lands them; this spike does not edit `docs/decisions/`.
>
> ## The machine everything was measured on
>
> | | |
> |---|---|
> | CPU | Intel Xeon E5-1650 v3 @ 3.50 GHz — 6 physical cores / 12 threads, **8 online** (4 offline) |
> | RAM | 16 GiB |
> | Kernel | Linux 7.0.14-6-pve x86_64 (Proxmox) |
> | Filesystems | `/tmp` = **tmpfs** (RAM); `/home` = **ZFS** (`data/subvol-120-disk-0`) |
> | Go | go1.27.1 linux/amd64, `CGO_ENABLED=0` |
> | SQLite | `modernc.org/sqlite` v1.61.0 (SQLite 3.54.1), FTS5 compiled in |
> | fsnotify | `github.com/fsnotify/fsnotify` v1.10.1 |
> | Parser | `github.com/yuin/goldmark` v1.8.6 + GFM |
>
> The corpus lives on ZFS and the worktree on tmpfs. **Anything that depends
> on file-system behaviour was measured on both** — see §2.

---

## 1. The search spike (question 6)

> *Does FTS5 over the real corpus put the right note first?*

Code: [`search/`](search/) and the shared machinery in
[`corpus/`](corpus/). The synthetic campaign with real visibility fields is
[`testdata/campaign/`](testdata/campaign/). Raw output: [`results/`](results/).

### The answer, in three sentences

**FTS5 finds the right note for 12 of 15 real GM queries, but the *literal*
index finds it for 14 of 15 — and the three the ranked index misses
("stealth", "sleight of hand", "insight") are all rule-book terms that live
only in a note's section heading.** The GM-only notes matter: with the
ADR-0030 pack defaults applied to the corpus, the presence of the 1,169
GM-only notes changes the top result of 3 of 8 real queries and moves a
given note by up to 16 places, and one note's BM25 score doubles, so
ADR-0047's two-index partition is required rather than optional. The
operator set ADR-0047 declares is exactly the set the observed GM requests
need — no more, no less.

### 1.1 What was asked, and what came back

Fifteen queries a GM would type, over the 1,693-note SRD corpus. The
"expected" column is what a human would say is the right note; where the
corpus is genuinely ambiguous ("goblin" names three notes; the corpus has
both a Druid *class* and a Druid *monster*) any of the acceptable notes
counts.

| Query | Literal index | Ranked index (BM25) | Merged (ADR-0013) |
|---|---|---|---|
| goblin | 1st | 1st | 1st |
| goblin warrior | 1st | 1st | 1st |
| hobgoblin | 1st | 1st | 1st |
| thunderwave | 1st | 1st | 1st |
| magic missile | 1st | 1st | 1st |
| fire bolt | 1st | 1st | 1st |
| longbow | 1st | 1st | 1st |
| shillelagh | 1st | 1st | 1st |
| barbarian | 1st | 1st | 1st |
| druid | 1st | 1st | 1st |
| **stealth** | **1st** (Skills) | **MISS** (not in the top 10) | **1st** |
| **sleight of hand** | **1st** (Skills) | **#4** | **1st** |
| **insight** | **1st** (Skills) | **#2** | **1st** |
| darkvision | #7 | 1st | #7 |
| gob (prefix) | 1st | 1st | 1st |
| **first place** | **14/15** | **12/15** | **14/15** |

The three misses are the same failure: the corpus keeps **every skill in one
note** (`rules/skills.md`, 20 `## Skill` headings), and BM25's length
normalisation punishes it. "stealth" appears **14 times** in
`compendium/items/mithral-armor-xdmg.md` and **once** in
`rules/skills.md`, so the armors win on the numbers and the human answer is
the Skills note.

```
=== "stealth" ===   (ranked index, BM25, top 6)
  1. Mithral Armor           compendium/items/mithral-armor-xdmg.md   bm25 -4.0887
  2. Adamantine Armor        compendium/items/adamantine-armor-xdmg.md bm25 -4.0687
  3. +1 Armor                compendium/items/1-armor-xdmg.md          bm25 -4.0463
  4. +3 Armor                compendium/items/3-armor-xdmg.md          bm25 -4.0463
  5. +2 Armor                compendium/items/2-armor-xdmg.md          bm25 -4.0443
  6. Armor of Acid Resistance                                            bm25 -3.9492
  — rules/skills.md is not in the top 10.
```

Adding the note's **headings** as a boosted column moves "sleight of hand"
to #4 and "insight" to #2, and the **literal index over the headings**
puts the Skills note first for all three. That is the concrete form of the
gap: **ADR-0013's literal index covers names, aliases and wikilink targets.
It has to cover section headings as well**, because in a rule-book note the
heading *is* the name.

A second observation, on the same query: the top three goblin notes are
separated by **0.13 BM25 points out of 9.4 — 1.4%**. The order among
equally-relevant notes is decided by noise, not by relevance, and the UI
must not present it as if it were a ranking.

### 1.2 The preprocessing rules

Written down, implemented in
[`corpus/plaintext.go`](corpus/plaintext.go), and exercised over the whole
corpus.

1. **Frontmatter is removed.** Parsed by `internal/vault/frontmatter`; it
   never reaches the indexer.
2. **Two line-level Obsidian constructs are removed before parsing**,
   because they are not markdown and goldmark splits them across text
   nodes: a **callout marker** (`[!note]`, `[!ad-monster]`, with an
   optional fold marker `+`/`-`) at the start of a blockquote line, and a
   **block reference** (a line that is exactly `^blockid`).
3. **The body is parsed with goldmark + GFM, and only prose is kept:**

   | Construct | Rule |
   |---|---|
   | Text, String | kept |
   | Inline code span | kept, minus the backticks |
   | Fenced code block | **skipped** — except the community content fences, see 5 |
   | Indented code block | skipped |
   | HTML block, raw HTML | skipped (an HTML comment is noise) |
   | Link | the **text** is kept, the destination is dropped |
   | Image | the **alt text** is kept, the URL is dropped |
   | Autolink `<https://…>` | dropped (a URL is not a GM's search term) |
   | Heading | kept, minus the `#` markers |
   | Emphasis / strong / strikethrough | kept, minus the markers |
   | Blockquote | kept, minus the `>` markers and the callout marker |
   | List | kept, minus the bullet or number |
   | Table (GFM) | kept, minus the pipes and minus the delimiter row |
   | Thematic break | skipped |
4. **Obsidian extensions are resolved in the text pass:** `[[Target]]`,
   `[[Target|Alias]]`, `[[Target#Section]]` and `![[Target]]` become plain
   text; the target is resolved through the note-name map when it resolves
   (`[[ilias-note]]` indexes as "Ilia Vane"), and the last path segment is
   used when it does not.
5. **The community's content fences are not code.** `ad-statblock`,
   `statblock` and `ad-monster` blocks are re-parsed as markdown and walked
   like the rest of the document, after their YAML-ish `key: value` header
   lines are stripped. Every other fence is skipped.

**Rule 5 is the one that changes ADR-0030.** ADR-0030 says "code blocks and
comments skipped". In the SRD corpus the `ad-statblock` fence spans the
*whole* stat block including the creature's Actions and Bonus Actions
(`compendium/bestiary/fey/goblin-warrior-xmm.md` lines 45–76), and it
appears in **355 notes — 21% of the corpus**. Applying ADR-0030's rule as
written drops **26.2% of the corpus's indexable tokens**:

```
notes                     1693
tokens, raw markdown      413267
tokens after stripping     249937  (60.5% of raw)
tokens with ad-statblock  315440  (+26.2%)
notes affected by keeping 355 (21.0%)
```

A search for "hit points" or "armor class" — a GM's most common lookup —
returns nothing from the bestiary under ADR-0030's rule as written.

### 1.3 Rank inflation from GM-only notes — the IDF side channel

**The corpus cannot exercise this.** Its frontmatter has four keys
(`obsidianUIMode`, `cssclasses`, `aliases`, `tags`), all Obsidian-native:
there is no `visibility`, no `campaign`, no `type`. So the split has to be
manufactured, and it is manufactured two ways.

**(a) The real corpus, split by ADR-0033's pack defaults.** Every note is
put in one campaign (`srd`), and the content type implied by the file's
directory is given ADR-0033's declared default: `monster`, `item`,
`optionalfeature`, `variantrule` and the reference tables are `gm`;
`class`, `species`, `background`, `feat`, `spell` and the rules are
`party`. That gives **1,693 notes, of which 1,169 are GM-only and 524 are
party-visible**. The player is a member of the campaign.

**(b) A synthetic campaign** — nine notes with real `visibility`,
`campaign`, `player` and `workflow` fields, in
[`testdata/campaign/`](testdata/campaign/): "The Duke's Letter" (party),
"Duke Reynard" (gm), "Ilia Vane" (owner), "Sela Thorn" (owner),
"Goblins of the Wastes" (gm), and four party or public notes.

**The measurement.** Two indexes over the same notes: the GM index (all
1,693) and the player index (the 524 a player may see). The same query,
the same weights, the rank and the score of each player-visible note
compared.

| Query | GM index top-1 | Player index top-1 | Top-1 changed? | Max rank displacement | Mean displacement | Largest score change |
|---|---|---|---|---|---|---|
| stealth | Mithral Armor (**GM-only**) | Skills | **yes** | — | — | — |
| insight | Ioun Stone, Insight (**GM-only**) | Skills | **yes** | 4 | 3.00 | −4.0% |
| sleight of hand | Manacles (**GM-only**) | Skills | **yes** | **10** | 7.44 | −1.9% |
| barbarian | Barbarian | Barbarian | no | 4 | 1.00 | +9.6% |
| darkvision | Darkvision | Darkvision | no | 6 | 2.00 | **−100.6%** |
| thunderwave | Thunderwave | Thunderwave | no | 9 | 6.50 | +1.3% |
| magic missile | Magic Missile | Magic Missile | no | **16** | 7.25 | −4.4% |
| goblin | Goblin Minion (**GM-only**) | *(no hits at all)* | **yes** | — | — | — |

**Does it matter? Yes, and by a lot.**

- **3 of 8 queries change their top result.** In "insight" the GM's first
  hit is `Ioun Stone, Insight` — a magic item the player may not see — and
  the player's first hit is the Skills note. In "goblin" the player's index
  has *no goblin note at all*, because every goblin in the SRD is a monster
  and every monster is `gm`.
- **A note moves by up to 16 positions.** The Wizard note is 20th for
  "magic missile" in the GM index and 4th in the player index.
- **The score of a note can double.** For "darkvision", the Darkvision
  spell scores −3.84 in the GM index and −7.70 in the player index, a
  change of **100.6%**, because removing 1,169 documents roughly halves
  the corpus and therefore doubles the IDF of a term that appears in 31 of
  them. GM-only notes do not merely *reorder* the results — they *deflate*
  the visible notes' scores, and by an amount that depends on the hidden
  content.

**So ADR-0047's partition is load-bearing, not defensive.** A single index
with a filter after the ranking leaks in two ways at once: the hidden note
occupies a slot, and its presence changes the statistics of the visible
ones. The cost of the partition is that every indexed note is written twice
and the FTS file doubles — measured in §3.

**The residual channel ADR-0047 acknowledges is real and is smaller.** The
player index still contains the owner-scoped notes of *other* players (in
the synthetic campaign: "Sela Thorn", owner `p_8f2a`, is invisible to
`p_91bc` but present in `p_91bc`'s player index). Solving it exactly would
mean a per-player index; ADR-0047's decision to acknowledge rather than
solve it is the right one, and the measurement agrees that it is a much
narrower channel than GM-only content.

### 1.4 The operator set, observed

The corpus's frontmatter has four keys, all Obsidian-native, and none of
them a universal field — so **the corpus can exercise a list-membership
operator over `tags` and `aliases`, and nothing else**. The rest is
observed from what a GM asks for, against ADR-0040's reserved field set and
ADR-0033's content declarations.

| The GM says | The smallest predicate that answers it | Operators | Exercisable against the corpus? |
|---|---|---|---|
| all the characters in this campaign | `type == "character" and campaign == "stormdark"` | `==` `and` | yes |
| the characters I own | `player == "p_8f2a" and type == "character"` | `==` `and` | yes |
| notes that are still in draft | `workflow == "draft"` | `==` | yes |
| the GM-only notes | `visibility == "gm"` | `==` | yes |
| notes tagged npc | `tags contains "npc"` | `contains` | yes |
| anything that mentions Ilia | `name contains "Ilia"` | `contains` | no |
| the notes that mention a given player | `grants contains "p_8f2a"` | `contains` | no |
| spells of level 3 or higher | `data.attributes.level >= 3` | `>=` | no |
| spells of level exactly 3 | `data.attributes.level == 3` | `==` | no |
| spells below level 3 | `data.attributes.level < 3` | `<` | no |
| spells at most level 3 | `data.attributes.level <= 3` | `<=` | no |
| monsters with armour class over 15 | `data.attributes.armor_class > 15` | `>` | no |
| notes that are not characters | `type != "character"` | `!=` | yes |
| characters and NPCs | `type == "character" or type == "npc"` | `==` `or` | yes |
| characters, or NPCs that are tagged boss | `type == "character" or (type == "npc" and tags contains "boss")` | `==` `or` `and` `contains` `()` | no |
| notes of one of several types | `type in ["character", "npc", "faction"]` | `in` `==` | yes |
| notes that are not yet revealed | `not (revealed contains "s_42")` | `not` `contains` `()` | no |
| the note with this uid | `uid == "01HQ8R..."` | `==` | yes |
| notes of this kind, but not that one | `type == "character" and name != "Ilia Vane"` | `==` `and` `!=` | no |
| notes whose tag list has the whole tag `monster/cr/1-4` | `tags contains "monster/cr/1-4"` | `contains` | yes |
| notes whose preview mode mentions `prev` | `obsidianUIMode contains "prev"` | `contains` | yes |

**Tally:** `==` 10 · `contains` 5 · `and` 4 · `!=` 2 · `or` 2 · `()` 2 ·
`>` 1 · `>=` 1 · `<` 1 · `<=` 1 · `in` 1 · `not` 1.

**Operators ADR-0047 declares that no utterance needed: none.**
**Operators an utterance needed that ADR-0047 does not declare: none.**

ADR-0047's operator set is exactly right. Two observations the corpus does
add:

- **`contains` has two semantics, and the corpus pins both down.**
  `tags` is a YAML *list*, so `contains` is list membership; the values are
  hierarchical (`monster/cr/1-4`), so a substring reading would make
  `contains "monster/cr"` a hit and `contains "monster/cr/1-4"` a different
  one. `obsidianUIMode` is a *scalar*, so `contains` there is substring.
  One operator, two meanings, decided by the field's type — which is what
  ADR-0040's type registry is for.
- **The comparison operators (`>`, `>=`, `<`, `<=`) and `not` are
  unexercisable by the corpus**, which has no numeric and no boolean field.
  They can only be tested against a pack-declared field. That is a gap in
  the corpus, not in ADR-0047.

### 1.5 Two things the search *box* needs, that are not ADR-0047's language

ADR-0047 is the language for `list_by` — a filter over structured fields.
The **search box** is a different thing, and it needs two things the
measurement shows:

**(a) The query has to be normalised before it reaches FTS5.** FTS5's
default is a phrase query. "the duke's letter" as a phrase matches
**nothing** in the corpus; over the synthetic campaign it matches the
Duke's Letter note only by accident. The query has to be lowercased,
split, stripped of its possessive ("duke's" → "duke"), and stripped of
stopwords ("the") — and then matched as a disjunction. After that,
"the duke's letter" finds "The Duke's Letter" first.

**(b) The disjunction needs a coordination factor.** Over the corpus,
"the duke's letter" normalises to `duke OR letter`; "duke" matches **0**
notes and "letter" matches 2, so the query silently degrades to a
single-term search and returns the Cultist notes — the only two notes in
the SRD that happen to contain the word "letter":

```
query terms: [duke letter]
  OR:  1. Cultist            bm25 -3.8606
       2. Cultist Fanatic    bm25 -3.6548
  term "duke"   matches 0 notes
  term "letter" matches 2 notes
```

Ranking by *how many* query terms a note matched, before the BM25 score,
is what fixes this. It is the standard coordination factor, and ADR-0013
does not mention it.

**(c) A BM25 score is not comparable across queries.** BM25's IDF is
`log((N − n + 0.5) / (n + 0.5))`, so a term present in more than half the
documents has a **negative** IDF and the score collapses. Measured on
synthetic vaults where the term is in 60% of the notes:

| notes | matches | best score | worst score | relative spread |
|---|---|---|---|---|
| 10 | 6/10 | −0.0000015564 | −0.0000007583 | 51.3% |
| 30 | 18/30 | −0.0000015826 | −0.0000007974 | 49.6% |
| 100 | 60/100 | −0.0000016944 | −0.0000008074 | 52.4% |
| 300 | 180/300 | −0.0000016965 | −0.0000008108 | 52.2% |
| 1000 | 600/1000 | −0.0000016968 | −0.0000008113 | 52.2% |
| 3000 | 1000/3000 | −0.0000016969 | −0.0013303 | 21.6% |

The absolute score is ~1.5 × 10⁻⁶ for a common term and ~1–13 for a rare
one — **six orders of magnitude apart, and independent of corpus size.** The
*relative* ordering is still meaningful (52% separation), but the score is
not a quality measure and the UI must not present it as one. In the
nine-note synthetic campaign, the scores for "duke" are **−0.000002** —
decided in the sixth decimal place.

### 1.6 What changed as a result

- **ADR-0013 (search)** — the literal index must cover **section
  headings**, not only names, aliases and wikilink targets; the FTS5 table
  needs columns for the title, the aliases and the headings, because
  BM25 can only boost a *column*.
- **ADR-0030 (SQLite caches)** — the preprocessing rule "code blocks
  skipped" must except the community's content fences, or 26.2% of the
  corpus's tokens are lost; and the `note_fts` schema as written has
  nowhere to put the title and alias boost ADR-0013 requires.
- **ADR-0047 (query language)** — the operator set is confirmed, but the
  search *box* needs query normalisation and a coordination factor, and
  the search box is a separate language from `list_by`.
- **ADR-0047 / ADR-0033 (the partition)** — confirmed as load-bearing:
  3 of 8 top-1 results change, one note moves 16 places, one score
  doubles.

See [`adr-proposal.md`](adr-proposal.md).

---

## 2. The watcher spike (question 7)

> *Does the watcher see everything, on every platform?*

Code: [`watcher/`](watcher/). The driver is
[`watcher/cmd/watcher/`](watcher/cmd/watcher/), with the commands `rescan`,
`atomic`, `fdcount`, `chmod`, `macos` and `limits`.

### The answer, in three sentences

**The polling watcher sees an Obsidian-style atomic save — the temp-file
rename over the original — in 577 µs on Linux, and the watch survives it.**
**It holds zero file descriptors per note**, and the fsnotify watcher holds
one per *directory* on Linux (0.0000 per note), so the macOS kqueue landmine
is avoided by not using fsnotify there at all. **A full rescan of the
1,693-note corpus costs 9 ms stat-only or 52 ms with content hashes**, so a
30-second rescan interval is a **0.03% / 0.17% duty cycle** — the interval is
set by measurement rather than by guess.

### 2.1 The atomic save

The acceptance test writes a note the way Obsidian does: a temporary file in
the same directory, then a rename over the original. The rename is atomic on
POSIX, so a reader never sees a half-written file — and, critically, **the
inode of the path changes**.

```
mode dir   watching [/tmp/opencode/wt-test]
  SAW THE TARGET PATH after 577µs
  event: CREATE   .demitable-save-499323348    419µs after the rename
  event: WRITE    .demitable-save-499323348    520µs after the rename
  event: RENAME   .demitable-save-499323348    565µs after the rename
  event: CREATE   atomic-test.md               577µs after the rename
  RESULT: the change WAS seen on the target path
  content on disk after the save: "# Original\n\nthe REPLACED body, written by a temp file and renamed over the top"
  second atomic save:
  SAW THE TARGET PATH again after 222µs
  RESULT: the second change WAS seen — the watch survived
```

Two things matter here. The watcher sees the change **on the target path**,
not only on the temporary file — an implementation that only watched the
inode, or that ignored `RENAME`, would miss it. And **the watch survives
the rename**: a second atomic save is seen too, 222 µs later.

### 2.2 File descriptors

The macOS landmine is that fsnotify's kqueue backend opens **one file
descriptor per file**, so a 50,000-note vault means EMFILE. Measured here:

| Strategy | fds | per note |
|---|---|---|
| The polling watcher | **+0** for 1,693 notes | **0.0000** |
| fsnotify (inotify), watching 31 directories | +0 | **0.0000** |
| fsnotify, watching 1,693 *files* | +0 | **0.0000** |

`RLIMIT_NOFILE` on this machine is 524,288.

Linux's inotify needs no descriptor per watched path, which is why the third
row is also zero. **That is exactly why the macOS number cannot be inferred
from the Linux one**: the same fsnotify call opens a descriptor per *file*
on macOS's kqueue backend and none on Linux's inotify. The `macos` command
in the driver exists to be run on a Mac, where it prints the same table and
warns if the ratio exceeds 0.5 fds per note.

**What was verified here is the polling watcher's number — zero — and the
mechanism by which the macOS risk is avoided: by not using fsnotify there.**

### 2.3 The rescan interval

A full rescan of the corpus, five runs each:

| | Median | Per note |
|---|---|---|
| stat only (size, mtime, inode) | **8.999 ms** | 5.3 µs |
| stat + SHA-256 | **51.779 ms** | 30.6 µs |

Duty cycle at each candidate interval:

| Interval | stat only | stat + hash |
|---|---|---|
| 1 s | 0.90% | 5.18% |
| 5 s | 0.18% | 1.04% |
| 15 s | 0.06% | 0.35% |
| **30 s** | **0.03%** | **0.17%** |
| 60 s | 0.015% | 0.086% |

**30 seconds is chosen by measurement.** The phase-1 plan says "a full
rescan every 30–60 s"; the upper end is indistinguishable from the lower
end on duty cycle, and 30 s is the safer of the two.

### 2.4 What the removal looks like

The phase-1 plan says the watcher must "reconcile a removal reported as
`CHMOD` by stat-ing the path". On Linux, with the directory watched, the
removal came through as **`REMOVE`** — directly — even while the file was
still open elsewhere:

```
deleting a file that is open elsewhere:
  event while the file is still open: REMOVE   chmod-test.md
closing the descriptor:
  stat of the path says it is gone -> the watcher must treat this as a REMOVAL
```

So the `CHMOD` case is a kqueue artefact rather than a general one. The
rule stands — stat the path on a `CHMOD` — but on Linux it is a belt to
braces rather than the mechanism.

### 2.5 What this spike did not do

- It did not run on macOS. The `macos` command is written so the same
  binary can be run there, and it prints the three assertions that matter,
  but **the macOS numbers are not measured**.
- It did not test the native IME, a real drag gesture, or a vault being
  edited by Obsidian while the watcher runs.
- It did not measure the watcher against a vault with **writes** happening
  concurrently with rescans — only single-threaded changes.

---

## 3. The baselines (question 9)

> *What do the numbers actually cost?*

### The answer, in three sentences

**The phase-1 criterion fails: 5,000 notes take 1.23 s to index, not the
1 s the plan allows.** **The index is not the cost — it is 12% of it**
(144 ms of 1.23 s); the other 88% is walking the tree, reading 5,000
files, parsing each one's frontmatter and goldmark-parsing 4.35 MB of
markdown. **The real corpus is not the problem: 1,693 notes index in
0.30 s**, comfortably inside any budget.

### 3.1 The criterion, and what it actually measures

The phase-1 plan says:

> A generated vault of 5,000 notes, each with 40 unknown frontmatter keys
> and a 2 KB body, is fully indexed — frontmatter parsed to `yaml.Node`,
> link graph built, FTS5 populated — **in under 1 s wall clock, p95 over 5
> runs**, on the CI runner.

The generator is committed at [`gen/`](gen/) and is driven by
[`gen/cmd/genvault/`](gen/cmd/genvault/), because the plan says it must be
committed.

The measured breakdown, five runs each, on the machine above:

| | 5,000 generated notes | 1,693 real notes |
|---|---|---|
| **load** — walk, read, frontmatter, markdown, link graph | **1,077–1,102 ms** | **206–218 ms** |
| **insert** — the FTS5 insert | **144–151 ms** | **83–111 ms** |
| **wall clock** | **1,228–1,248 ms** | **294–317 ms** |
| index size (db + wal + shm) | 6,799,360 B | 4,251,648 B |
| peak RSS | **78.9 MiB** | — |

### 3.2 The finding

**The criterion fails by about 23%, and it fails for a reason nobody
expected.** The insert is 144 ms; the *load* is 1,080 ms. The plan's
criterion implicitly assumes the index is the expensive part, because that
is the part a reader thinks of as "indexing".

It is not. **88% of the cost is parsing.** Reading 5,000 files, splitting
each one's frontmatter, and goldmark-parsing 4.35 MB of markdown to
extract prose, headings, links and the community's content fences is an
order of magnitude more expensive than putting the result into FTS5.

That has two consequences for phase 1:

- **The criterion has to move, or the work has to be parallelised.** The
  obvious lever is that the corpus's own shape makes the load cheap: a
  note's body only has to be parsed once, and the result cached. But the
  plan says the caches are rebuildable from the vault, so a cold index pays
  this cost every time.
- **The criterion should be restated in terms of the thing that is
  expensive.** "Index 5,000 notes in under 1 s" is the wrong measurement.
  "Parse 5,000 notes in under 1 s, and insert them in under 250 ms" says
  where the budget goes, and it is the shape of the thing that has to be
  fast.

**The real corpus is comfortably inside budget.** 1,693 notes in 0.30 s, and
the corpus is what a real GM's vault resembles. The 5,000-note case is the
plan's own stress case, and it is a synthetic one: 40 unknown frontmatter
keys and a 2 KB body per note.

### 3.3 Queries

A 15-query sweep against the indexed corpus takes **239–246 ms** — but that
is dominated by the corpus load the command performs to build the link
resolver, not by the queries. **A single FTS5 query is sub-millisecond**,
and the two-index partition does not measurably affect single-query latency.

### 3.4 What this spike did not do

- It did not measure a **cold** index with a cold page cache.
- It did not measure **incremental** indexing — one note changed, the index
  updated — which is what the watcher actually drives in production.
- It did not measure **memory at steady state**, only peak RSS during the
  index build.
- It did not measure the **search + projection** combination, only search.
