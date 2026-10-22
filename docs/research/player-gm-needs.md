# What TTRPG players and GMs actually need — gaps in the Demitable design

**Research date:** 2026-10-22
**Design reviewed:** `docs/design.md` (Demitable), `docs/decisions/*` (0001–0031), `docs/session-writeback.md`, `docs/phase-1-plan.md`, `README.md`.

**How to read the evidence tags**

- **[EVIDENCED]** — I found a source describing the need, a workflow people describe in practice, or a complaint. URL given.
- **[INFERRED]** — the need follows from cited evidence plus the design's own structure, but I found no source describing it as a need.
- **[HYPOTHESIS]** — plausible and common-sounding, but unverified. Treat as a bet, not a finding.

**One framing note.** The design is unusually well-informed about the *GM's* side of the table. Every one of the four phases is built around what the GM does: build notes, run an encounter, run a board, record a session. The gaps below are concentrated in three places: (a) things that happen *at the table, in the room*, (b) things that happen to *players* rather than to the campaign, and (c) things that happen to a campaign *after* it has been running for two years.

---

## 1. Executive summary — the ten gaps that would hurt a real table most

| # | Gap | Severity | Phase affected | Evidence |
|---|---|---|---|---|
| G1 | **The second screen and the shared device.** Phase 3 assumes every player has their own phone. The most common in-person pattern is a GM screen plus *one* shared display showing only what the party sees, with players interacting through that one screen. | Critical | 3 | EVIDENCED |
| G2 | **Handouts / "reveal this to the table" is not a concept.** We have *assets* and a *player client*; a handout is neither. Every serious rival has one as a named feature. | Critical | 1–3 | EVIDENCED |
| G3 | **No chat, no in-game message channel.** The design has a dice log and no way to say anything to a player. | High | 1–3 | EVIDENCED |
| G4 | **Nothing captures the session *during* the session.** The write-back contract writes the session note at the *end*; real tables capture during, or not at all. | High | 1, 4 | EVIDENCED |
| G5 | **The player-facing surface is too thin.** No personal journal, no party roster, no player-authored notes on shared characters. | High | 1–2 | EVIDENCED |
| G6 | **No pre-session ritual.** No recap, no session zero, no safety tools, no house rules. | High | 1 | EVIDENCED |
| G7 | **"Blind" content is modelled at note and value level, not paragraph level.** No in-place reveal, no per-player private material attached to a shared note. | High | 1–3 | EVIDENCED |
| G8 | **Tables are GM-side only.** No blind roll, no player-initiated roll, no in-world party table. | Medium-High | 1–2 | EVIDENCED |
| G9 | **No portable character sheet.** Players expect a PDF/print sheet; we render in-app only. | Medium-High | 2 | EVIDENCED |
| G10 | **Campaign lifecycle is unmodelled.** One server = one vault = one campaign. Nothing for a player leaving, a character dying or retiring, a campaign ending, or a campaign handed to a new GM. | Medium-High | 1 | EVIDENCED (our own plan flags it as open) |

**Below the top ten, but real and specific:** accessibility / localisation / RTL (G11), Obsidian interop depth beyond file format (G12), Discord and voice-chat interop (G13), GM time sinks we still don't touch (G14), combat-tracker depth beyond HP and conditions (G15), and a set of small table staples — map pins, pings, hot-seat, session scheduling, an in-world calendar (G16).

---

## 2. Gap-by-gap detail

### G1 — The second screen, and the shared device

**What people actually do.** A very large share of tables are *in person*, and for those tables the canonical setup is not "everyone on their own phone". It is: the GM has a laptop with the full GM view, and a TV / monitor / projector on the table showing a *player* view. Players interact with that one screen (touch TV, shared keyboard/mouse, or the GM moves things for them). [EVIDENCED]

The Foundry community has a dedicated, heavily-commented guide to exactly this: two accounts (a GM account and a *single shared player account* given Owner permission over all player characters), a TV mounted vertically or laid horizontally on the table like a real tabletop, and a pile of modules whose whole job is to make the shared display behave (Lock View to lock pan/zoom, Monk's Common Display to mirror the GM's view, Hide Player UI to strip chrome, Gaming Table Player to point the view, Material Deck / TouchVTT for input). [EVIDENCED — https://www.foundryvtt-hub.com/guide/using-foundry-for-in-person-gaming]

Roll20 documents the same thing as an official use case ("Using Roll20 while Playing In-Person": connect a TV/monitor to the GM's computer and use a separate browser window/tab for the player account). [EVIDENCED — https://wiki.roll20.net/Using_Roll20_while_Playing_In-Person] D&D Beyond's Maps has a presenter view that gets dragged to a TV and fullscreened. [EVIDENCED — https://www.facebook.com/groups/392678574938393/posts/2196266487912917] Players on r/DungeonAndDragons ask for exactly one shared screen, because "my party is just not all technically skilled enough to feel comfortable using VTTs individually". [EVIDENCED — https://www.reddit.com/r/DungeonsAndDragons/comments/153a0mt/what_vttprogram_do_you_use_for_one_screen_for_the/]

The in-person case is not a niche. D&D Beyond's own Journals announcement is full of posts describing a hybrid table: "I DM in-person games with my laptop but use digital maps on a monitor screen on the table", "We usually put physical minis on the monitor screen with the digital maps". [EVIDENCED — https://www.dndbeyond.com/posts/2175-never-lose-the-story-again-introducing-journals-on]

**Why we miss it.** The design's player client is a PWA that "players open a URL on their phone, join with a session token, see the board" (design.md §7, Phase 3). ADR-0016 scopes the player view to "pan, zoom, and their own character's sheet beside the board". There is no concept of a *shared* player view, a display mode with no chrome, a GM-controlled "point the table at this", or a way for the party to interact with one screen.

Worse, our model is per-player-projection: a player client's response is filtered for *that* player. A single shared screen has no single player. A shared-device scenario needs a session whose token maps to a *role at the table*, not to a person.

**Design implication.** Phase 3 needs a **display mode** distinct from the player client:
- A GM-controlled **shared view** URL (or mode) that renders a player-safe projection with no GM chrome — map, initiative, dice log, no sidebar, no settings, no way to pan into fog.
- A **hot-seat / shared-account** role: one session token, one player-shaped view, any player can pick up the device, with a quick "I am Ilia / I am Bram" switch that re-projects the sheet without revealing another player's private values. Note the security consequence: a shared device means the *projection* must be role-scoped per character, not per device.
- A **"point the table at this"** control: GM moves the shared view's centre, optionally locks pan/zoom (the Lock View / Gaming Table Player pattern).
- For the shared screen, the dice log and initiative sidebar need a **presentational** mode — big, legible from 2 m, and free of buttons a player can accidentally press.

This is not scope creep. It is the *other* half of the player client, and it is a different rendering target, not a different product. It should be in the phase-3 MVP, because retrofitting it means reworking the projection layer's shape (device vs person vs role).

**Not a second VTT.** Building a second VTT would be foolish (see §5). Building the display mode for the VTT we already have is not.

---

### G2 — Handouts: "reveal this to the table" is not a concept we have

**What people actually do.** A handout is a prop the GM pushes to the players mid-session: a letter, a map, a wanted poster, a contract, a quest board, a key, a set of ritual instructions. It is a *distinct* thing from a file in a folder, because it has a **reveal event** and a **recipient set**.

**How existing tools handle it.** All three major rivals have it as a named, first-class feature with a panel:
- **Foundry** — Journal Entries are explicitly "a method for GMs to organize and maintain hand-outs for their players", with page types Text, **Image, Video, PDF**; a "Show Players" action that can target **specific players** rather than everyone; and map notes that pin a journal to a scene. [EVIDENCED — https://foundryvtt.com/article/journal]
- **Roll20** — Handouts are a first-class Journal object alongside Characters, with a GM-only GM Notes field, a per-player "In Player's Journals" grant, and drag-to-token or right-click "show to players" reveal. [EVIDENCED — https://help.roll20.net/hc/en-us/articles/360039675133-Journal, https://wiki.roll20.net/Handout]
- **Alchemy** — a dedicated Handouts Panel; a handout has a name, a visual (image *or* video), a "open in full-screen" pan/zoom flag, metadata tags, a content tab, and a **Sharing tab that picks which players receive it**; players see shared handouts in the same panel in-game, and are notified when a handout's permissions change. [EVIDENCED — https://help.alchemyrpg.com/en/articles/9821627-creating-handouts]

Alchemy's own documentation of the *use cases* is worth reading as a spec: "Creating a quest board players can pull up in a local tavern", "Revealing a letter the players received inviting them to a local festival", "Presenting a map of the world players can easily pull up to reference during a game". [EVIDENCED — same source]

**Why we miss it.** The design mentions handouts exactly once, as a category of *asset*: "Maps, portraits, token images and handouts are vault attachments… subject to the same visibility rules as the notes that reference them" (design.md §4 Assets, ADR-0026). That is a **storage** concept, not a **reveal** concept. An attachment is visible to a player if a note visible to that player references it. But a handout's whole point is that the note referencing it is often GM-only, and the handout must become visible *at a chosen moment* to a chosen subset of players.

There is also no place for a handout to *live* as a durable player-visible artefact ("the letter from the Duke, which the party still refers to three sessions later"). If the handout is only an attachment to a GM-only note, the player can never reference it again.

**Design implication.** Add a **handout** as a first-class, visibility-managed object:
- A handout is a note (probably `type: handout`) with its own `visibility` — but unlike a note, its visibility is **mutable at runtime** and *revealing it is an event*, not a static property. The reveal should be journalled (ADR-0015 write journal) so "when did the party learn about the Duke's letter?" is answerable.
- Granting a handout to a *subset* of players is required (Alchemy and Foundry both do it; so do we — our `visibility` model has `owner` and `party` but no "these three players" set). A `visibility: group` or a per-note `grants: [p_8f2a, p_91bc]` value would cover it.
- A handout needs a **body** (the letter's text) as well as an image/PDF, and the body should be able to hold *hidden* parts (see G7).
- A handout should be **referencable by players after the reveal** — which means it must appear in the between-sessions projection, and be findable via search by the players who received it.
- Phase 2/3 need a "push to the table" action: reveal to all, reveal to one, reveal to the shared display, and (nice) "the player who received it may now read it privately on their own device".

**Scale check.** This is not a large feature. It is roughly: one note type, one runtime-mutable visibility grant, one journal event, one panel. It is high-value because it is a *moment* in play, and moments are what a tool is judged on.

---

### G3 — No chat, no in-game message channel

**What people actually do.** Players and GMs talk to each other *in the tool* during play. Roll20's chat is not a side feature; it is where dice results, whispers, handouts, and character actions live, and it has a dedicated "whisper" (`/w`) mechanic that lets a GM speak privately to one player or to a character (whispering to a *character* reaches whoever controls it — a trick people use to build in-character "languages"). [EVIDENCED — https://wiki.roll20.net/Text_Chat, https://help.roll20.net/hc/en-us/articles/360039675093-Text-Chat] Alchemy has a chat panel on the GM's left rail, and images can be pasted into the journal from chat. [EVIDENCED — https://help.alchemyrpg.com/en/articles/9821054-chat-within-alchemy]

Players also ask for a place to type *at the table*: "Too bad no DM or Player has ever had the ability to access Notepad while in VTT." [EVIDENCED — https://www.dndbeyond.com/posts/2175-never-lose-the-story-again-introducing-journals-on]

**Why we miss it.** The design has a "shared **dice log** using the pack's workflows" (design.md §7, Phase 2) and a dice log that is "written into the session note" (§6.2). It has no message channel at all — no table chat, no GM→player private message, no player→player whisper, no out-of-character channel.

This matters more than it looks, because of **the trust boundary**. Our design is unusually strict about projection. But strictness without a private channel pushes GM→player communication out of the app and into the real world (the GM leans over and whispers), which is exactly the workflow the tool should be improving on.

**Design implication.** A minimal chat, and no more:
1. **Table channel** — in-character, visible to everyone, archived into the session note (or into a `chat` artefact). Dice results post here.
2. **GM → one player** — a private message the GM sends, visible to the GM and that player only. This is a *projection* case we already know how to do: a note with `visibility: owner` and `owner: p_8f2a`. No new mechanism needed, which is a nice validation of the model.
3. **Player → GM** — a private message to the GM, useful for "can I ask a rules question mid-session?" and for the "3am lore question" workflow (see G14).
4. **Player → player** — optional. Roll20's whisper is beloved, but it is a *nice-to-have*; the risk of confusion is real. [HYPOTHESIS on its necessity; EVIDENCED that Roll20 has it and people use it.]
5. **OOC channel** — a distinctly-marked channel for "I need a bathroom" / "sorry, connection dropped" / table logistics. Roll20 separates these by convention (chat bubbles, `/ooc`).

**Scope discipline.** Do not build Slack. The test: can a GM hand a note to one player, and can a player ask the GM something privately, without leaving the app? If yes, the chat is done.

---

### G4 — Nothing captures the session *during* the session

**What people actually do.** At most tables, the notes are taken *by a player*, during the session, and often badly. D&D Beyond says this plainly in their own launch post: "most DMs don't have time to take extensive notes during a session. They're already busy trying to manage too many things! And we know that at most tables, if you're lucky, there's one player who steps up and takes almost all the notes. But that's a lot of burden on that player, and the whole campaign loses out if that player misses a session." [EVIDENCED — https://www.dndbeyond.com/posts/2175-never-lose-the-story-again-introducing-journals-on]

Their fix is a **single-key quick jot**: press J, type, done — explicitly designed so you "don't have to pause the table, alt-tab to a new program, switch contexts, or promise yourself you'd write it down later." [EVIDENCED — same source] That is the workflow people describe in glowing terms: a capture surface so cheap it fits inside the flow of play.

**Why we miss it.** Our write-back contract (session-writeback.md §2) applies **all seven kinds of change as one batch at the end of the session**, atomically, with a preview. That is a *correct* design for campaign state — HP, quest status, faction standing. It is the **wrong** design for capture.

The problem: what the GM wants to capture at minute 40 of a session is not "Ilia's HP is 23". It is "the Duke's steward was named Harlan" or "the party promised the ferryman 50gp" — a *fragment*, a half-formed fact, a callback, a promise. Under the current contract there is no place to put it, because anything written mid-session either (a) waits for the end-of-session batch, in which case it is not captured, or (b) is written into the vault directly, which breaks the atomicity and preview guarantees the design is rightly proud of.

Phase 4 does not close this gap either: transcription is explicitly "record, transcribe after, review, log" and "Live mode is a stretch goal, not the baseline" (design.md §7). So the *only* in-session capture the design has is the dice log. Everything else waits for after.

**Design implication.** Add a **third tier** between the canonical vault and the disposable session tier: a **scratch log**.

- A live, append-only, timestamped capture surface during a session: a text field bound to a hotkey, plus optional tags ("NPC", "promise", "clue", "correction").
- It lives in the **session tier** (SQLite, disposable) — not the vault — so it does not break the write-back atomicity.
- At session end it is **reconciled** into the batch: the GM sees the scratch log alongside the diff and either folds entries into the batch or discards them.
- The important trick: a scratch entry can be *promoted* to a durable artefact ("this is a new NPC — make a note") and be **resolved** from the scratch list, so the write-back can create the NPC note in the same atomic batch.
- This is also the natural feed for phase 4: the transcript proposes structure; the scratch log is what the GM actually wrote. Both are candidate inputs to the same session summary.

This single addition removes the largest practical obstacle to "just write it down", and it costs one append-only table plus a reconciliation step.

---

### G5 — The player-facing surface is too thin

**What players expect of their character, beyond the sheet.** Progression and inventory (we have those). But also: **their own notes and journal**, **the party**, **relationships**, and **seeing the world's state change in response to them**. D&D Beyond is *currently shipping* this: Journals, with a DM journal first and then a "communal **Player Journal**. Everyone will be able to contribute to a shared record of the campaign (while DMs will still have their own private journal space). Later, collaborative editing will let players expand entries together." [EVIDENCED — https://www.dndbeyond.com/posts/2175-never-lose-the-story-again-introducing-journals-on]

In the comment thread, players specifically ask for a **personal** player journal, not just a shared one: "Make a personal journal for players. Sometimes you are journaling your character's story to help roleplay" — and, after being told a shared player journal is coming, "The video suggested a shared collaborative player's journal, but I am requesting a personal player journal instead." [EVIDENCED — same source] Another: "I need to keep secret notes for various triggers" (from a DM). A third describes exactly the hybrid we need to support: "I have a note taker, but as a DM I need to keep secret notes for various triggers and remember key details. I typically use a google doc or physical notepads for this." [EVIDENCED — same source]

Kanka has the same shape from the GM side: "Journals — Plan your session or write a session recap in the eyes of a character" and quest tracking with "starting conditions, rewards, and more". [EVIDENCED — https://kanka.io/features] World Anvil and D&D Beyond both have per-character and per-campaign "notes" that live beside the sheet. [EVIDENCED — https://www.dndbeyond.com/forums/d-d-beyond-general/d-d-beyond-feedback/178816-campaign-notes]

**Why we miss it.** ADR-0017 gives a player, between sessions: their own character (editable), read-only campaign access filtered by discovery, and "their party's shared view of anything marked `visibility: party`". That is genuinely ahead of most tools — but it stops at the *sheet*.

What is missing:
- **A personal player journal** — the player's own prose about their character, their expectations, their plans, their guesses about the GM's intentions. Not shared. Not GM-visible by default (or GM-visible but not editable). D&D Beyond's own comment thread says players want the *personal* one.
- **The party roster** — who else is at the table, their public characters, their names. Currently a player can see "anything marked `visibility: party`" but there is no *party* concept, no roster, no "this is the party" view. Note: `owner` + `workflow` + `kind: pc` are enough to derive a roster, but nothing in the design says so.
- **Player-authored notes on shared characters** — Roll20's per-character "Player Notes" and "In Player's Journals" folders let a player attach private notes to a *shared* object (a character, a handout). Our model has no per-player attachment to a note someone else owns.
- **"How are we doing?"** — a party-level view of progress: current quest status, what the party has accomplished, the loose threads the GM has flagged. Our quest notes are GM-owned with `visibility`, and there is no notion of a "the party is currently doing X" summary.
- **The world reacting to them.** Our write-back does update world notes (item, faction, location) and quest notes. That is the mechanism. But the *player-facing* view of "the world changed because of you" is not specified — ADR-0017 lists locations visited, NPCs met, factions known, items carried, quests party to. It does not list "faction standing" or "a location you burned down". [INFERRED that this gap matters]

**Design implication.**
- Add a **player journal** as a first-class owned artefact: `owner: p_8f2a`, `visibility: owner` (or a new `visibility: owner-and-gm` if the player wants the GM to read it), editable by the owner, never in the GM's batch-write path.
- Define a **party** concept derived from `kind: pc` + `workflow: active` + the session roster, exposed as a view rather than a new note type. A derived view is cheap and matches the design's "derived cache" rule.
- Support **per-player attachments to shared notes** — i.e. a note can carry an "annotation" with its own owner and visibility. This is a natural extension of the discovery/grant model rather than a new mechanism.

---

### G6 — No pre-session ritual: recap, session zero, safety tools, house rules

**What people actually do.** "The session starts" is not a single event; it is a ritual with several parts, and a real table has a tool (or a habit) for each:

- **Recap.** "As a DM, keeping track of important details and giving a short recap at the start of each session is already part of the job." [EVIDENCED — https://www.dndbeyond.com/posts/2175-never-lose-the-story-again-introducing-journals-on] A DM with 40 sessions needs the recap *generated*, not remembered.
- **Session zero.** A pre-campaign meeting to agree tone, expectations, boundaries, and to build characters together. [EVIDENCED — https://slyflourish.com/safety_tools.html, https://www.rpgkc.org/resources/safety-tools]
- **Safety tools.** X-card, Lines & Veils, Script Change, Stars & Wishes, the Luxton Technique. These are now mainstream rather than fringe, and the TTRPG Safety Toolkit is the community's canonical bundle. [EVIDENCED — https://www.rpgkc.org/resources/safety-tools, https://melestrua.net/2022/07/29/safety-tools-in-rpgs] The lines/veils spreadsheet is explicitly a session-zero artefact to be "filled in with players". [EVIDENCED — https://melestrua.net/2022/07/29/safety-tools-in-rpgs]
- **House rules.** "Every table plays their game system a little differently from the base rules" — Foundry's first-run guide says to make sure your players are aware of your rules settings. [EVIDENCED — https://www.youtube.com/watch?v=qZRQymndcVY] House rules are a *shared artefact* the GM writes once and everyone refers to.
- **Attendance and scheduling.** Who is coming tonight affects what to prep. D&D Beyond tracks player attendance; Multiloop ships session scheduling as a headline feature. [EVIDENCED — https://www.dndbeyond.com/games, https://multiloop.app/compare]

**Why we miss it.** The design has a `session` note type with a date, attendees, scene beats, NPCs introduced, decisions, loot, loose threads (session-writeback.md §1). That is a *record* of a session, not a *ritual* before one. There is no recap artefact, no "the table's charter", no safety-tool record, no house-rules surface, no schedule.

**Design implication.**
- **Auto-recap.** At session end, the write-back already produces scene beats, decisions and loose threads. Derive a **player-facing recap** from them (this is phase 4's LLM's natural job) and make it a shareable artefact. This is the highest-value item in this section: it removes the single most-repeated pre-session task.
- **A campaign charter note.** One note per campaign holding: table tone, house rules, safety tools and their agreed lines/veils, expectations, and the agreed content warnings. It is a *party-visible* note (like D&D Beyond's shared journal) that players can read between sessions. The design's `visibility: party` handles the visibility; what is missing is the *template* and the idea that this is a first-class note type.
- **Session scheduling / attendance** is plausibly a phase-2 item: a `session` note with a planned date and an attendance list. Note the multi-campaign complication (G10) here: scheduling is inherently per-campaign, which means the campaign identity must become explicit (see G10).

**Cost note.** This is one note type, one template, and one derived recap. It is very cheap relative to its effect on the table's mood.

---

### G7 — "Blind" content: paragraph-level secrecy and per-player private material

**What people actually do.** Two distinct things that we currently collapse:

1. **Secret blocks inside a player-visible note.** Foundry has a first-class "Secret" paragraph format in journal text: "only be visible to the GM or Owner of the Journal Entry… Secret text blocks can be toggled as visible to players by clicking the 'reveal' button, allowing players to see the content of the text if they are presently viewing the Page." [EVIDENCED — https://foundryvtt.com/article/journal] This is the classic "the letter says X, but the GM's copy also says Y" pattern.
2. **Per-player private material attached to a shared object.** Roll20's Handout has "GM Notes (GM Only)" *and* the ability to place a specific player in the "In Player's Folder" so that player can *view and edit* that handout — "particularly useful for allowing players journals or secret notes for correspondence with the GM." [EVIDENCED — https://wiki.roll20.net/Handout, https://help.roll20.net/hc/en-us/articles/360039675133-Journal]

**Why we miss it.** Our visibility model is deliberately elegant: note-level (`public`/`party`/`owner`/`gm`), property-level (packs can declare a resource private), and value **masking** (show "8" to the owner, "??" to the party). §5.4. That is a good model for *numbers*. It has no way to express:
- a paragraph in a player-visible note that only the GM sees, and which can be **revealed to the players in place** ("the letter's real contents");
- material private to *one* player but attached to a *shared* note (a player's private notes on a party-wide handout, a player's secret to the GM about another player);
- a **reveal** as a time-stamped event that both the GM and the players can see happening.

The "absolute GM deny" rule (session-writeback.md §6) also means a GM-only note can never be revealed — which is correct for a note, and wrong for a paragraph that is *meant* to be revealed later.

**Design implication.**
- Add a **"revealable secret"** to the content model — a block, with its own `visibility: gm` until revealed, that can be *promoted* to `party` (or to a chosen subset) as a journaled event. It lives inside a note whose own visibility is `party`. This is a *different* mechanism from note-level visibility, and it should be named differently in the schema so the two do not get confused. Something like a `secrets:` list in frontmatter, or an HTML-comment-delimited block in the body (which round-trips safely through the Tiptap editor's raw-node escape hatch — see §5.6).
- Add a **grant set**: `grants: [p_8f2a, p_91bc]` on a note, meaning "these players may see this". Our four-level visibility vocabulary cannot express "these three". This is also what G2 (handouts to a subset) needs, so one mechanism serves both.
- Be explicit about the interaction with the absolute GM deny: **`visibility: gm` on a note remains absolute**; the revealable-secret is a *different* field. Preserving that distinction is what keeps the deny meaningful.

---

### G8 — Tables are GM-side only

**What people actually do.** Three different table behaviours, all common:

1. **Blind rolls.** The result is hidden from the players; they see "???". Foundry has a module whose entire purpose is this: "Players see a blind roll indicator (usually displayed as '???'). Only GMs see the actual table results; the roll is completely hidden from all players." [EVIDENCED — https://foundryvtt.com/packages/gm-only-tables] Its "Display roll formula to chat" setting even lets the GM hide the formula from players. [EVIDENCED — https://foundryvtt.com/article/roll-tables]
2. **A player rolls.** A table a *specific* player is asked to roll on — a wild-magic surge, a gamble, a "you may roll to pick your own result". Foundry's "Rolltable Requester" module "displays a chat card with a button asking a player to click it to roll on the table which is named." [EVIDENCED — https://foundryvtt.com/packages/rolltable-requester] The wild-magic example in the wild: "I have a wild magic sorcerer in two of my campaigns… I consult 3 different tables" [EVIDENCED — https://www.reddit.com/r/FoundryVTT/comments/t7r8u3/a_rollable_table_that_the_player_can_roll_but]
3. **Party-facing tables.** Rumour tables, "what's rumoured in this town", loot tables, "pick one of these three rumours" — tables that are *in-world artefacts* the party uses, sometimes with a rule that only one player may roll (to prevent metagaming). [EVIDENCED that random rumour tables are a standard GM practice — https://www.roleplayingtips.com/rptn/random-encounter-tables; the "only one player may roll" variant is [HYPOTHESIS], I found the practice but not the specific one-player-only rule in a source]

**Why we miss it.** Our `type: table` (design.md §6.3) is a note with `data.faces` that a GM rolls on. It has *visibility* (so a table can be party-visible) but no *roll semantics* beyond the result type. There is no notion of:
- a roll whose **result** is hidden from players (only the fact that a roll happened),
- a roll **initiated by a player** rather than the GM,
- a roll that is **authorised for one player** and refuses others,
- a table that is a **world object** rather than a GM tool (a rumour table the party reads).

**Design implication.** Extend the table/roll model with three orthogonal attributes on the roll, not on the table:
- `audience: everyone | gm-only | blind` (blind = players see "???" plus a timestamp, GM sees the result),
- `initiator: gm | player` (a player-initiated roll produces a "request" the GM can grant, and the result posts to chat),
- `eligible: [p_8f2a]` (only that player's request is accepted).

This is small, but it is the difference between a random-table feature and a table that participates in play. It also needs a *chat surface* (G3) to be usable.

---

### G9 — No portable character sheet

**What people actually do.** Players want a sheet they can carry, print, or read on a phone with no app. D&D Beyond's Export to PDF has been a long-standing, heavily-used feature with an entire support article ("Export Sheet") and a long forum thread on how to print sheets. [EVIDENCED — https://dndbeyond-support.wizards.com/hc/en-us/articles/7747238449556-Export-Sheet, https://www.dndbeyond.com/forums/d-d-beyond-general/general-discussion/55693-how-do-you-print-character-sheets] Players export, delete the fields that change with level, and print; one user describes a routine: "I usually save the entire character sheet to a pdf, the[n] go in and delete things like the save/attack values and HP… as those things change when you level… then I print and write those values on with pencil. Otherwise you end up reprinting your character sheet every level." [EVIDENCED — same thread] Printable sheets are described as "a beloved classic that brings its own unique pleasures". [EVIDENCED — https://litrpgreads.com/blog/rpg/download-the-new-2024-dnd-character-sheets-official-pdfs-now-available] GMs keep PDFs of every player's sheet: "I like to keep PDFs of all my players character sheets because some classes and spells have really unique or situational features." [EVIDENCED — https://www.reddit.com/r/DMAcademy/comments/120kp7p/do_you_look_at_your_players_character_sheets]

**Why we miss it.** design.md §7, Phase 2: "Character sheets. Two representations on purpose: `data:` frontmatter — structured, queryable, machine-written. A rendered sheet in the note body — human-editable, holds freeform notes, survives without the app." The *note body* is a survival mechanism, but it is not a portable, printable, offline, phone-readable sheet. And a PDF is a different artefact: paginated, fixed, no app required, and — crucially — **shareable with a human who is not a player of the app**.

**Design implication.**
- A **print/export view** that renders the pack's `views.sheet` into a paginated, print-styled document. This is a *view*, not a new document format — the pack already declares the sheet layout, so a print stylesheet over the same renderer gets you most of the way. `@media print` plus a "sheet only" route is likely 90% of the work.
- **PDF export** is worth doing if the renderer can print to PDF (headless browser, or the browser's own print-to-PDF — which is free and needs no server-side dependency, and keeps the single-binary promise).
- Because the sheet is pack-driven, the export is system-agnostic for free. A Blades sheet exports as a Blades sheet.
- Also exportable: the **character's public sheet** (for a player to show a human) and a **static HTML snapshot** of a note, which is useful for sharing lore with a non-player.

**This is a small, high-credibility feature.** "Of course it exports a PDF" is the kind of thing a table checks before it checks anything else.

---

### G10 — Campaign lifecycle: multiple campaigns, players in and out, handover, ending

**What people actually do.** This is the long tail that a 5-year campaign hits and a 3-session demo never does.

- **One GM, several campaigns.** Common. [EVIDENCED — https://www.reddit.com/r/DnD/comments/y14tem/hi_dms_what_medium_do_you_use_to_organise_your/] Our design assumes one server per vault per campaign: `demitable --vault <dir>`, accounts in `.demitable/players.yaml` **inside the vault**, and a `visibility` model with no campaign dimension. So a GM running three campaigns runs three servers, on three ports, with three sets of player accounts and three separate join flows. A player in two of those groups has two accounts on two servers.
- **One vault, several campaigns** is *also* the Obsidian convention. PhD20's guide to organising D&D campaigns in Obsidian is built around one vault holding multiple campaign folders, plus a shared "atlas" folder for setting material used by several campaigns. [EVIDENCED — https://phd20.com/blog/organizing-obsidian-dnd-campaigns/, https://phd20.com/blog/organizing-an-atlas-for-dnd-in-obsidian/] A Reddit DM describes exactly this: "I run multiple campaigns set in the same world… My notebook has 3 sections: DM Screen, Monomyth, and…" [EVIDENCED — https://www.reddit.com/r/DnD/comments/y14tem/hi_dms_what_medium_do_you_use_to_organise_your/]
  → **This directly contradicts our assumption.** Our own plan already flags it as an open item: "Whether one campaign may span several vaults" (phase-1-plan.md, Open items). The reverse question — whether one *vault* may hold several *campaigns* — is the one that matters, and the Obsidian community's answer is yes, commonly.
- **A player leaves.** The character's backstory is entangled with other characters; the table has to decide whether the PC becomes an NPC, retires, or dies. [EVIDENCED — https://www.reddit.com/r/DnD/comments/xrauux/what_do_you_do_with_a_character_when_their_player/, https://rpg.stackexchange.com/questions/131764/a_player_left_the_group_at_a_moments_notice_and_i_am_at_a_loss_for_what_to_do, https://www.enworld.org/threads/who-%E2%80%9Cowns%E2%80%9D-a-pc-after-the-player-stops-using-them.702898/] QuestKeepers' FAQ markets exactly this as a feature: "Players come and go, you know? That's why you'll be able to change players in and out as they join or exit your table, while all campaign notes [remain]." [EVIDENCED — https://www.kickstarter.com/projects/mettleurgy/questkeepers-the-ideal-note-taking-experience-for-ttrpgs/faqs]
- **A character dies or retires.** We have `workflow: retired` on a note, and `status: active` with a free-ish vocabulary, which is a good start. But a *PC → NPC conversion* (the departing player's character becoming a GM-run NPC) has no path: `kind` would change from `pc` to `npc`, and `owner` would have to be cleared. Nothing in the design says so. [INFERRED]
- **A player returns after 18 months.** Our `discovered` records are append-only and never removed ("A character who once met an NPC still knows them") — good, this survives. But the *account* is the problem: the player set a passphrase, and if they forgot it, the GM has no recovery path other than editing `players.yaml` by hand. [INFERRED]
- **The campaign ends / is archived.** A campaign that ends should be *readable and exportable* forever — it is a story record. Our design is good here in principle (plain files, git-friendly) but has no concept of an "archived" campaign state, no way to stop a server from serving a finished campaign, and no "export this campaign as an archive" beyond ADR-0025's *import* direction. [INFERRED]
- **The campaign is handed to a new GM.** "Can the DM be replaced mid-game?" is a real question people ask. [EVIDENCED — https://www.quora.com/In-D-D-can-the-DM-be-replaced-mid-game, https://rpg.stackexchange.com/questions/27163/running-a-campaign-alternating-gms-every-session] A new GM needs: the campaign bible, the running plot threads, the NPCs with their secrets, and the rules the old GM used. The `visibility: gm` notes are exactly the right material — but handing them over means handing over the vault, which means the accounts file (`.demitable/players.yaml`) with other people's password hashes travels with it. That is a **privacy problem the design has not considered**. [INFERRED]

**Design implication.**
- Introduce an explicit **`campaign:` field** on notes (universal, like `type` and `system`), so one vault can hold several campaigns and one server can serve them. The server would then be per-*vault*, not per-*campaign*; the player joins a *campaign*, and the join code names the campaign. This resolves the Obsidian convention conflict **and** the multi-campaign-per-GM problem in one move.
  - The cost: `visibility: party` currently means "all players in the session". It must become "all players in *this campaign*", which is a stricter and more correct rule — and it fixes a real bug: with one server and two campaigns, "party" would otherwise leak across.
  - The cost: the write-back, search index, and dashboard all need a campaign scope. Manageable, and it is a *schema* change, which the design says is the expensive kind ("structure is expensive to change later"). **This is the argument for deciding it in phase 1.**
- **Account lifecycle:** add `disabled` / `left` to a player record, and define what happens to characters they own (keep visible, or converted to `kind: npc` with `owner` cleared — the GM's call, with a one-click action).
- **Export/import of a finished campaign** as a *bundle* (vault + packs + a manifest) — ADR-0025 already has the archive mechanism for importing; an export direction is the natural complement, and it is what "the table ended" and "new GM takes over" both need.
- **Handover hygiene:** a documented "give this vault to another GM" procedure that strips `players.yaml` (or replaces it with a fresh one) and keeps the GM's notes. Given ADR-0009's "the vault never leaves the GM's machine" posture, this deserves an explicit answer.

---

### G11 — Accessibility, localisation, and RTL: currently zero

**What people actually do.**
- **Screen readers.** Roll20 has been shipping incremental screen-reader accessibility work for years ("Improved screen reader accessibility for the Powers section of the character sheet", "Added accessibility support to the search boxes… so screen readers now identify them as search boxes"). [EVIDENCED — https://help.roll20.net/hc/en-us/articles/20509479555991-2023-Change-Log, https://help.roll20.net/hc/en-us/articles/360037772613-Change-Log]
- **Colour-blindness.** This is a live, ongoing failure in the incumbents. A Roll20 forum thread on the 2024 sheet: "Anyone who is color blind will see those as 'grey' and the numbers (or other data) will be unreadable." [EVIDENCED — https://app.roll20.net/forum/post/11793851/announcing-the-new-character-sheet-for-d-and-d-2024/] Another thread is explicitly framed as "an issue of Accessibility for visually disabled people". [EVIDENCED — https://app.roll20.net/forum/permalink/11009797/] The general guidance from the accessible-games community is blunt: "Don't just use color for presenting information. Consider using sound, shapes and animations… ensure there is another visual element present to inform the player, such as an icon, image or border." [EVIDENCED — https://caniplaythat.com/2020/01/29/color-blindness-accessibility-guide]
- **Dyslexia.** "Stylized fonts in games can be inaccessible to dyslexic players. A choice of alternative fonts or formatting can help… be careful to make the text itself not too visually complex." [EVIDENCED — same source] Sans-serif is recommended over serif for legibility, with larger line height. [EVIDENCED — https://www.youtube.com/watch?v=NQ1KLc0NAS0]
- **Blind and low-vision players.** There is a real, published body of practice: the Fate Accessibility Toolkit, assembled by disabled contributors, and guides like "GMing for the Visually Impaired". [EVIDENCED — https://fate-srd.com/fate-accessibility-toolkit, https://therpggazette.wordpress.com/2025/06/04/gming-for-the-visually-impaired-a-guide/]
- **Localisation.** The TTRPG scene is heavily international, and players are explicit that playing in one's own language matters. [EVIDENCED — https://www.reddit.com/r/rpg/comments/1n3pfxn/nonnative_englishspeakers_whats_the_rpg_ecosystem/, https://www.facebook.com/groups/CriticalRoleFanClub/posts/745403182328126/] D&D Beyond has a long-running "Translating D&D Beyond" feedback thread, and note the argument in it: D&D "uses language and terms to refer to specific mechanics that everyone can recognize" — i.e. *mechanical terms* are a localisation problem distinct from *UI text*. [EVIDENCED — https://www.dndbeyond.com/forums/d-d-beyond-general/d-d-beyond-feedback/47203-translating-d-d-beyond-feedback] Foundry's answer is community translation infrastructure: the Foundry Hub Weblate project, and a "Localization" API so modules can be translated per-application. [EVIDENCED — https://www.foundryvtt-hub.com/news/lets-translate-all-the-packages-were-proud-to-announce-our-new-project-foundry-hub-weblate/, https://foundryvtt.com/article/localization/] The Forge supports English, French, Japanese, Spanish and German. [EVIDENCED — https://blog.forge-vtt.com/internationalization/]
- **Right-to-left.** No evidence found that any TTRPG tool ships RTL. I am treating RTL as a **hypothesis** — but it is cheap to prepare for and expensive to retrofit, and the design's own stack (Go + Svelte) makes it a CSS/`dir` attribute problem rather than a data problem.

**Why we miss it.** `grep` over `docs/` and `README.md` finds **no** mention of accessibility, screen readers, colour-blindness, dyslexia, i18n, l10n, localisation, or RTL. The only "accessibility" mention in the whole repo is ADR-0021's passing note that "Accessibility and mobile input are better in a rich-text editor" — and the only "guest language" is about *WebAssembly* guest languages, not human ones. (I checked this specifically.)

**Design implication.** These are cheap now and expensive later, and several of them are *table stakes for a system-agnostic tool* that claims to serve non-English tables:
- **Colour-independence as a rule.** Fog of war, token disposition, condition icons, HP bars, and initiative highlighting must never encode state in colour alone. Every status needs a shape/icon; the initiative tracker needs a text label. This is a *phase-3* requirement, and it is the one most likely to be got wrong by a team that has never thought about it.
- **Semantic HTML, ARIA, and keyboard operability** for the whole app. A sheet is a form; a form should be a form. Svelte can do this properly.
- **Font options** — a dyslexic-friendly face, a sans-serif default, adjustable line height and text size. One CSS variable set.
- **i18n plumbing** — string extraction, a locale file format, and a decision about *mechanical* terms. This one has a specific, elegant answer available to us: **the pack is data**, so pack labels (`STR`, `Hit Points`, `Saving Throw`) can be *data* too, and a pack can carry its own translations. A French 5e pack ships French labels. The design's pack format already has `label:` everywhere — making those translatable is a small extension and a strong differentiator for a system-agnostic tool.
- **RTL** — `dir="auto"` on prose, logical CSS properties, and mirrored board controls.

**Priority judgement.** Do the cheap parts now (semantic HTML, colour-independence in the pack's visual vocabulary, font options, string extraction) because they are structural. Do not attempt a full translation programme — that is a community effort (Weblate exists for exactly this), and the pack-as-data design already lets the community do it without us.

---

### G12 — Obsidian interop: file format is not the same as ecosystem participation

**What the Obsidian TTRPG community actually does.** This is a large, self-organising community with its own conventions, and it is the most directly comparable population to our target user.

- **Plugin ecosystem.** A whole TTRPG plugin family: Initiative Tracker, Fantasy Statblocks, Dice Roller, Calendarium (fantasy calendars), Leaflet (interactive maps), TTRPG Tools - Maps, TTRPG Tools - Soundboard, DM20 Protocol, GM Map, Atlas VTT, and the Campaign Codex scaffolder. [EVIDENCED — https://github.com/Obsidian-TTRPG-Community/initiative-tracker, https://phd20.com/blog/obsidian-community-plugins-dnd, https://community.obsidian.md/plugins/dnd-campaign-manager]
- **They are already building the VTT.** "Atlas VTT turns your Obsidian vault into a free, open-source virtual tabletop: battle maps, tokens, fog of war, dice, initiative and a separate player view, right beside your notes." [EVIDENCED — https://atlas-vtt.dev/, https://github.com/atlas-vtt/atlas-vtt] And the GM Map plugin: "Show a TTRPG map to your players on a second screen with DM-controlled fog of war, statblock-linked tokens, and DM-only markers… Open a second Obsidian window (or split the workspace) and run GM Map: Open Player view. Move the Player view window to your second monitor and go full-screen." [EVIDENCED — https://community.obsidian.md/plugins/gm-map]
  → **This is the single most important competitive datapoint in this report.** Our phase 3 will be compared to a free, open-source, Obsidian-native VTT that already does fog, tokens, dice, initiative and a separate player view, in the same vault, with no install. Our answer has to be *better*, and the differentiators have to be things a plugin cannot do: the campaign semantics (encounters, quests, discoveries), the write-back, the pack system, and the projection/secret model. That is a strong argument for **not** trying to out-map Atlas VTT, and for making phase 3 about the *campaign-aware* board rather than the *map* board.
- **Conventions.** The community's own advice is consistent and specific: query by a `type` property rather than by folder ("Query by `type` property, not by folder, so reorganizing folders never breaks queries"); keep every note's `type`; prefer flat frontmatter for anything Bases might query; use kebab-case; write a "Vault Conventions" note; avoid mixing conventions. [EVIDENCED — https://obsidianmate.com/article/obsidian-properties-complete-guide, https://www.obsidianstats.com/posts/2026-08-11-ttrpg-plugins]
  → **Our schema already satisfies this**, and the design says so explicitly (design.md §5.2: the universal layer exists "so they can be sorted, filtered and grouped by any tool (including Obsidian's Bases, whose property system does not support nested objects)"). That is a genuine and underrated strength. The `type:` vs `sp_type:` open item (phase-1-plan.md) is already leaning the right way for the right reason ("collides with Obsidian's Dataview conventions, which expect bare `type`").
- **Bases vs Dataview.** Bases is now a core plugin and is generally preferred for speed: "Data view has a tendency to slow down your vault performance, especially on mobile, it's kind of incredible to me how snappy bases are." [EVIDENCED — https://www.youtube.com/watch?v=1Mgsdpy7awE] Bases edits write back to frontmatter; Dataview additionally reads inline `key:: value` fields that Bases does not. [EVIDENCED — https://obsidianmate.com/article/obsidian-properties-complete-guide]
- **Machine-readable conventions are becoming a thing.** Plugins now generate an `AGENTS.md` "so the agent follows vault conventions like wikilinks and YAML frontmatter". [EVIDENCED — https://www.obsidianstats.com/tags/front-matter] → **Our MCP endpoint (ADR-0014) is the natural counterpart to this.** An MCP server that exposes the vault *with its conventions* to an LLM is a differentiator we already have, and the community is visibly moving toward it.
- **Template-driven creation.** Templater is a de-facto prerequisite ("Powers entity creation from templates… the plugin uses built-in templates (still works, but Templater gives you customizable prompts)"). [EVIDENCED — https://community.obsidian.md/plugins/dnd-campaign-manager] Our design's §5.7 scaffolder already says it "is also a template engine — every note type has a template with prompts, used by 'new note' and by Templater-style tooling." Good.

**Why we still miss something.** The design treats Obsidian as a *file-format* client: "Dataview / Bases blocks — left untouched, shown as source, never executed" (§5.6). That is *correct* for an editor (we must not execute a user's query language in a rich-text editor), but it means the community's most valuable views — a live Base of all NPCs, a Dataview query of "all quests with status active" — will be **stale** unless the user rebuilds them by hand in Obsidian, and will not reflect anything the app writes back.

Concretely: a user writes a Base over `type: character` and `faction:`. Our write-back changes `faction:` per-key, so Bases *will* pick it up (Bases reads frontmatter). Good. But a user writes a Dataview block over `data.attributes.str` — nested — which Bases cannot read and Dataview can, and our `data:` is nested by design (§5.2). That works in Dataview. But a user writes a query over a *field we don't have*, like `relationships` (we have no relationship model — G16) or `faction_standing`, and gets nothing.

**Design implication.**
- **Publish the schema as a documented, versioned contract** — a `schema.md` in the vault, or a URL, listing every universal field, its type, and its vocabulary. The community's "Vault Conventions" note is an informal version of this; making it machine-readable is cheap and is exactly what Bases/Templater/AI tooling want.
- **Ship starter Bases and Dataview queries** with the scaffolder, so a new vault comes with working views ("all PCs", "open quests", "NPCs by location", "sessions by date"). The Campaign Codex plugin already does this for a vault scaffold. [EVIDENCED — https://community.obsidian.md/plugins/dnd-campaign-manager]
- **Consider executing read-only Dataview/Bases queries in the app's browser plugin** — not in the editor, but in the *browser* (which is a plugin, and can be told "never execute" as a safety property while still offering the community's saved queries). At minimum, ship a query-friendly schema and document it.
- **Do not** try to be an Obsidian plugin. We are a *client* of the format. But we should be a *good* one: write clean, Bases-compatible frontmatter, preserve unknown keys, and never reorder the user's frontmatter.

---

### G13 — Discord and voice-chat interop

**What people actually do.** Most online tables run voice on Discord and the board on a VTT, in parallel, and are perfectly happy that way. "Why use Discord for audio in Roll20? Because most people run audio [on Discord] and roll20 for video and the gameboard." [EVIDENCED — https://www.facebook.com/groups/714169855634812/posts/1748095588908895/] Roll20 built a Discord Activity so the VTT can live inside a Discord voice channel. [EVIDENCED — https://help.roll20.net/hc/en-us/articles/25838862126359-Roll20-Discord-Activity] D&D Beyond ships **Avrae**, a Discord bot for dice and macros, as a first-class part of its tooling. [EVIDENCED — https://dndbeyond.com/] Avrae is a beloved part of the ecosystem. [EVIDENCED — https://www.rpgpub.com/threads/internet-videoconferencing-apps-for-rpgs.8630/]

**Why we miss it.** Nothing in the design mentions Discord, voice, or any external client. The design does have an **MCP endpoint** (ADR-0014) and the discipline that "the server must not require a browser" (Rule 5) — which is exactly the shape needed for a non-browser client. But there is no statement of intent about interop with the *voice* layer, and no HTTP/WS surface designed for a bot.

**Design implication.** Small and mostly about *intent*:
- Do **not** build voice chat. Campfire, Discord, and Roll20's built-in voice are all good enough; building another is the clearest possible scope mistake.
- Do make the **API and WebSocket surfaces** usable by a third-party client, and say so. A thin `POST /api/v1/roll` plus a `GET /api/v1/character/{uid}` is enough for a Discord bot to do `/sheet` and `/roll`.
- Optionally: a **"post this roll to a webhook"** integration, so the dice log can also land in the Discord channel the table already uses. Kanka has exactly this ("Premium campaigns can define workflows that send updates to Discord and other tools through webhooks when changes to entries are made") [EVIDENCED — https://kanka.io/features]. This is a nice, cheap integration: the *table* is in Discord, our app is the campaign memory.
- Also relevant to phase 4: if the table's voice is on Discord, our **local desktop audio capture** will not capture it. The design's phase-4 recorder captures "system audio + mic" on the GM's machine — which captures Discord's output if the GM has it on their machine, but not if the GM is on Discord and the players are in the room. [INFERRED] This is a concrete phase-4 risk: **the recorder's audio source assumption is wrong for hybrid tables**, which is exactly the table shape G1 describes. Worth resolving in the phase-4 ADR.

---

### G14 — The GM's actual time sinks

Let me map the design against the list in the brief.

| Time sink | Does the design address it? | Notes |
|---|---|---|
| **Session prep** | Partially. The vault, templates, the encounter planner, the pack system, and the "keep the tool fast" posture all help. | [INFERRED] |
| **Building NPCs on the fly** | Partially. design.md §7 Phase 2: the GM path "creates NPCs, which use a lighter path: a statline plus a couple of traits, because the NPC side is where a GM actually feels the volume." That is the right instinct. But: is there a **one-key "make me an NPC right now"** that produces a *statline*, a name, a voice and a hook? The tables (design.md §6.3) cover names. Random NPC generators are a real category of tool ([EVIDENCED — https://toolsandtaverns.com/npc-generator, https://www.monstershuffler.com]), and a common GM practice is "cheat sheets and quick stats" rather than full stat blocks ([EVIDENCED — https://forums.giantitp.com/showthread.php?369044/]). | **Gap: a fast NPC scratchpad.** |
| **A 3am lore question** | Not addressed. No quick-lookup surface, no "ask the GM" channel, no rules-reference. | **Gap.** |
| **Handouts** | Not addressed as a concept (G2). | **Gap.** |
| **Maps** | Deliberately out of scope: "The GM brings a map image" (ADR-0016). | **Correct call.** [EVIDENCED that map authoring is a separate product — Foundry's scene editor.] |
| **Rewards / loot** | Partially. `type: item` notes exist; write-back records "loot gained or lost" (session-writeback.md §1) and Kanka-style inventories are the obvious model. But there is no loot *distribution* mechanic and no "quest reward" concept. | **Gap: inventory model + rewards.** |
| **Session notes** | Strongly addressed — and then undermined by G4 (nothing to capture *during*). | **Fix via G4.** |
| **Campaign continuity across years** | Partially. `discovered` records are append-only and survive; session notes are one-file-per-session; search is indexed. But: no *recap*, no *index of open threads*, and no performance story for a 5-year vault. | **Gap: "what is still open?" view.** |
| **On-the-fly random generation** | Covered well. `type: table` + symbol dice + packs + "roll for a rumour" is explicitly named in §6.3. | **Good.** Needs the blind/player-initiated semantics (G8). |
| **Between-session work** | Not addressed. No downtime, no rewards, no quest review, no "what to prep next" queue. | **Gap.** |

**On the "3am lore question" specifically.** [EVIDENCED] This is a real, named category: a player asks something mid-week that the GM does not want to answer sloppily at the table. The nearest existing tool behaviour is the GM's "secret notes for various triggers" (D&D Beyond Journals comment) and the general advice to "always feel free to reach out if they have questions" (StartPlaying: "They should always feel like they can reach out if they have questions"). [EVIDENCED — https://startplaying.games/blog/posts/how-to-keep-players-coming-back-new-gm-guide]

**Design implication.**
- **A quick-lookup surface for the GM**: a single hotkey that searches the vault (and, if a pack ships rules text, the pack) and shows a *result card* on the GM's screen only. This is what the "3am lore question" needs at the table: the answer, plus a link to the note, plus a "show to player" action.
- **A "table inbox"**: the GM can see, in one place, questions and requests players have left (this falls out of G3's player→GM private message, plus ADR-0009's "approval is async and the GM may be away for days" — which the ADR already flags: "Players will expect the GM to have seen their changes. Approval is async and the GM may be away for days. The UI must make the state of a proposal obvious." [EVIDENCED — docs/decisions/0017] ).
- **A "still open" view**: quests with `status: active` and no session update in N sessions; NPCs with a promise attached and no payoff; loose threads recorded in the last session's scratch log. This is the *continuity* feature that worldbuilders love and that our one-file-per-session decision explicitly traded away ("reading the whole campaign is a search problem rather than a scroll" — session-writeback.md §1). We made the right trade; we now need the index that makes it cheap.
- **An inventory model.** Our character `data:` has `resources`, `conditions`, `skills` but no inventory. Kanka's "Every entry can have its own inventory… a character's possessions, a shop's sale inventory, a quest's reward for completion" is the reference model. [EVIDENCED — https://kanka.io/features] Our `type: item` notes exist, but nothing binds them to a character. **This is a real gap**: "players want progression and inventory" and we have no inventory.
- **Downtime.** Systems with downtime mechanics (5e PHB/XGtE) expect the between-session period to be spent, and a tool that tracks it is useful. [EVIDENCED — https://gnomestew.com/downtime-activities/, https://adventurers-guild.fandom.com/wiki/Downtime_Activities] [INFERRED that this matters enough to build for; a pack-level "downtime activity" definition plus a per-character counter would be enough, and the pack format already supports declaring things like this.]

---

### G15 — Initiative and combat tracking: beyond HP and conditions

**What the design has.** ADR-0016: "Tokens: portrait, name, size, HP, conditions, player/GM ownership, a visibility flag for the GM's hidden tokens. Fog of war… Initiative tracker, with the pack's initiative workflow. Dice log, shared and streamed. Measurement and distance."

**What real tables need on top.**
- **Concentration.** Universally tracked in 5e; a concentration check on damage ("adding the concentrating condition to your character and then you take damage prompts you to make a concentration check and gives the DC" is called out as a feature every digital sheet should have). [EVIDENCED — https://www.youtube.com/watch?v=rPek96Ki5os]
- **Death saves / dying states.** Standard 5e; a tracker that cannot model a dying PC misrepresents the most dramatic moment of a session. [EVIDENCED — https://www.youtube.com/watch?v=rPek96Ki5os] [INFERRED for other systems — the pack declares it.]
- **Hidden combatants.** Foundry's combat tracker lets the GM edit "whether their entries are visible to players in the tracker", and Monk's Combat Details goes further: "you can hide creatures from players while you are creating the encounter… and you can set this up so that enemies aren't revealed in the combat tracker until they've had their first turn." [EVIDENCED — https://foundryvtt.com/article/combat, https://foundryvtt.com/packages/monks-combat-details]
- **Concealment / disposition.** Foundry's Combat Tracker Extensions adds "hide/obscure initiative/combatant based on type/disposition/visibility/ownership". [EVIDENCED — https://foundryvtt.com/packages/combat-tracker-extensions]
- **Multiple concurrent encounters.** "More than one encounter can be tracked, and scenes can have multiple encounters being tracked at once, allowing the Gamemaster to manage multiple fights or encounters happening together with their own separate turn orders." [EVIDENCED — https://foundryvtt.com/article/combat]
- **Group initiative.** [EVIDENCED — https://foundryvtt.com/packages/combat-tracker-extensions]
- **"It's not your turn" enforcement.** Foundry's "Not Your Turn!" module locks the controls until it is the player's turn — because with a shared screen (G1) or a distracted player, this prevents accidental out-of-turn actions. [EVIDENCED — https://www.foundryvtt-hub.com/package/notyourturn/]
- **Turn notifications.** "During combat the player will receive an on screen alert notification, and sound effect when their turn is up next, and another one when it is their turn to move." [EVIDENCED — https://foundryvtt.com/packages/monks-combat-details]
- **Reactions / opportunity attacks / leaving a threatened area.** Foundry's "Ultimate 5e Combat Automation Suite" tracks concentration, opportunity attacks, and "people leaving threatened areas". [EVIDENCED — https://www.youtube.com/watch?v=5XlEK_Y-QJI]

**Design implication.** The pack format is the right place for most of this: **conditions, resources and workflows are pack-declared**. So:
- `concentration` and death-save/dying states are **pack-level resources and conditions** plus a small amount of core support (a resource with a save workflow, a condition with a counter). The pack format already has `resources` with `kind: health` and `conditions`; adding a `kind: concentration` and a "condition with a counter" is a schema addition in phase 2, not a phase-3 rewrite.
- **Concealment/disposition** and **hidden-until-activated** are core tracker features (not pack-level) and belong in the phase-3 MVP. Hidden-until-activated is a *real* play need (a horror reveal) and it is cheap.
- **Multiple concurrent encounters** and **group initiative** are later; note them in the pack format so they are not precluded.
- **"Not your turn"** locking matters more under the shared-screen model (G1) than under the per-phone model. It should be in the MVP if G1 is.
- **Turn notifications** (visual + optional sound) are a small, high-impact item for remote tables.

---

### G16 — Smaller table staples

Each of these is small; collectively they are the difference between "a tool we tolerate" and "a tool we use".

- **Map pins / notes on the map.** Foundry's Map Notes let a Journal Entry or page be pinned to a scene, clickable, with player permissions; Roll20's Map Pins "Add custom images and notes… Create pins directly from the Place Pin tool". [EVIDENCED — https://foundryvtt.com/article/map-notes, https://www.facebook.com/roll20app/posts/1507035381430227/] Our design has maps as attachments referenced from an encounter note, and no per-map annotations. An "on the map, there is a note" concept (an encounter note with an `at: {x, y}`) is cheap and very useful at the table ("the door is here").
- **Pings / placeables.** Roll20's ping and Foundry's placeables let a player say "look here" on the map. [EVIDENCED — Roll20 map ping tool; Foundry placeables.] Cheap and the single best way to make a 2D board social rather than tactical.
- **Hot seat.** Foundry's Hot Seat module is "primarily intended to be used to facilitate single screen VTT at home situations". [EVIDENCED — https://www.foundryvtt-hub.com/package/foundry-hot-seat-observer/] Same requirement as G1.
- **Session scheduling / attendance.** Who is coming next week changes what to prep. D&D Beyond tracks attendance; Multiloop ships scheduling. [EVIDENCED — https://www.dndbeyond.com/games, https://multiloop.app/compare]
- **An in-world calendar and time.** Kanka supports calendars, eras, moons, and "automatically calculating a character's age based on the calendar"; Obsidian's Calendarium plugin does the same; Obsidian's TTRPG plugin list includes Calendarium as a first-class choice. [EVIDENCED — https://kanka.io/features, https://phd20.com/blog/obsidian-community-plugins-dnd] Our `event` note type exists (timeline events) and session notes have a date, but there is no in-world date model. [INFERRED that this matters for long campaigns, especially with aging characters and moon/season flavour.]
- **Relationships between entities.** We have `faction:` and `location:` frontmatter fields and a *phase-1 stretch goal* for "a relation view (who knows whom, faction membership)" (phase-1-plan.md M9), but no relationship *schema*. Kanka's Relations — with per-relation privacy ("Need a relation to be kept secret from your players? Easy, just set the relation to private!") — is the reference model. [EVIDENCED — https://kanka.io/features] This matters for more than the graph view: a PC's relationship to an NPC is a *player-facing* fact, and it is one of the things players most want to see (see G5). [INFERRED]
- **A "party" as an object.** Kanka's "Party" is one of its 20 entity types. [EVIDENCED — https://kanka.io/features] We have nothing; the party is derived at best. [INFERRED that a real party object — shared inventory, party gold, party reputation — is wanted. Downtime activities in some systems are *per party*.]

---

## 3. What we got right

These are things the design already covers that are **less common than they should be**. Several of them are genuinely ahead of every tool in the comparison set, and they should be protected rather than traded away for a feature list.

1. **Markdown-as-the-database with Obsidian compatibility, as a first-class product decision.** Kanka, World Anvil, LegendKeeper, Notion, D&D Beyond and Multiloop all hold data in their own format. Our data is plain files the user owns. [EVIDENCED — https://multiloop.app/compare, https://kanka.io/features]
2. **A real trust boundary with an absolute GM deny.** Our "any grant applies; `gm` is an absolute deny" rule is stronger and easier to reason about than Foundry's per-document permission ladder, which includes a **Limited** level that "may see the map pin… will not see it in the sidebar. Double-clicking the map note pin will allow the Journal Entry Image to be displayed, but not the text" — a genuinely confusing, leaky model. [EVIDENCED — https://foundryvtt.com/article/journal] The finding that D&D Beyond "nuked their Campaign page by suddenly setting a 2048 character limit to Public and Private Notes" is a cautionary tale for any hosted note feature. [EVIDENCED — https://www.reddit.com/r/DnD/comments/1m8i69o/dndbeyond_just_nuked_their_campaign_page_by/]
3. **404, not 403, for invisible assets.** "A 403 confirms the path exists, which is itself a leak." Nobody else does this. [EVIDENCED as a design statement; the reasoning is sound.]
4. **Write semantics for concurrent Obsidian editing.** Per-key frontmatter writes, marked body sections, optimistic concurrency, and *no file locks* — because locks break Obsidian. No competitor has to solve this; it is the actual hard problem of being Obsidian-compatible, and we have designed it properly. [EVIDENCED as a design statement]
5. **The golden corpus + raw-node escape hatch + re-parse verification.** "The app cannot corrupt a vault with a round-trip bug." This is the kind of engineering discipline that is invisible until it saves you. [EVIDENCED as a design statement]
6. **`discovered` as a universal, character-level field.** *What this specific character has seen and met* is a first-class, append-only concept. No competitor models this. Foundry tracks world state; Kanka tracks world relations; neither tracks per-character knowledge. This is the foundation of ADR-0017's between-sessions view, and it is a genuine differentiator. [EVIDENCED — docs/session-writeback.md §5]
7. **The write-back contract's ergonomics.** A preview before writing, per-key conflict resolution with the human's value winning, idempotency keyed on `batchId`/`sessionId`, and one-action revert of a whole session. "A double-written character sheet — damage applied twice — is the kind of bug that destroys trust in a campaign tool." That is exactly right, and it is better than the write-back behaviour of most tools.
8. **One note per session, always.** "The arc of a campaign is fragmented across many files, and reading the whole campaign is a search problem rather than a scroll. That is the right trade: a note is a thing you can link, quote, version and diff." This is a better decision than a single running note, and better than a hosted tool's per-campaign blob.
9. **The pack as data, with a linter and a deliberately different second pack as a gate.** ADR-0012 requires a Blades pack before phase 2 is validated. That is the right way to keep "system-agnostic" from being a slogan. [EVIDENCED as a design statement]
10. **Pack-declared sheet layout.** `views.sheet` means the sheet is *data*, so a print/PDF export (G9), a different client's rendering, and per-system layouts all fall out for free.
11. **Tables as ordinary notes + symbol dice.** One mechanism covers numeric dice, rune dice, tarot, oracle cards, icon dice, the 5e Divine Roller, wild magic surges, *and* rumour tables. [EVIDENCED — design.md §6.3]
12. **Between-sessions player access at all.** Kanka's journals are GM-side; D&D Beyond is *shipping* player journals in 2026 as a new feature. We have had it in the design since ADR-0017, with a read-only projection that is strictly better than a shared document. [EVIDENCED — https://www.dndbeyond.com/posts/2175-never-lose-the-story-again-introducing-journals-on]
13. **No automatic updates, no operated registry.** "An update that changes the vault format under a running campaign is not something an app should do to someone." Given D&D Beyond's 2048-character-limit episode and Roll20's price-increase backlash, this is a principled position most tools do not take. [EVIDENCED — https://www.reddit.com/r/DnD/comments/1m8i69o/, https://app.roll20.net/forum/permalink/10040922/]
14. **Local transcription by default, with player consent as a design requirement.** Given that a session contains "unrehearsed player speech and the GM's unreleased secrets", this is the correct default. [EVIDENCED as a design statement]
15. **"Assistive by default" for mood-driven music.** "The system suggests a cue; the GM taps to accept. Full auto is a setting people will want and will immediately complain about. A table that fades to the wrong track mid-dialogue is worse than no music." Very well-judged. [EVIDENCED as a design statement]
16. **The PWA player client.** "Nobody installs anything. This is the single biggest adoption lever in the project." Agreed, and corroborated by the observation that most modern VTTs "require all of your players to create an account" and that players complain about needing accounts at all. [EVIDENCED — https://www.youtube.com/watch?v=ptzr70aVc4M]
17. **Search filtered by role before results reach a player** — "so search is part of the security boundary rather than a way around it." Almost nobody does this; D&D Beyond's own comment thread worries about enumeration. [EVIDENCED — docs/design.md §5.9; the concern is mirrored in our own ADR-0017]
18. **The MCP endpoint.** An MCP server over the vault, off by default and token-gated, is ahead of every tool in the comparison set, and the Obsidian community is visibly moving toward agent-readable vault conventions (`AGENTS.md`). [EVIDENCED — https://www.obsidianstats.com/tags/front-matter]

---

## 4. Things we should deliberately NOT build

Scope discipline is part of the deliverable. Each of these is something a new tool would be foolish to omit, *or* foolish to build — this list is the second kind.

1. **A second VTT, or map authoring.** ADR-0016's exclusion is right and should be defended. The strongest supporting evidence: the Obsidian community built a free open-source VTT (Atlas VTT) with maps, tokens, fog, dice, initiative and a player view — and the reaction was "it's not Foundry level (yet) but I love the..." [EVIDENCED — https://atlas-vtt.dev/, https://www.facebook.com/groups/392678574938393/posts/2240056760200556/] Map authoring is a separate application (Foundry's scene editor is a project of its own). **Build the campaign-aware board, not the better map editor.**
2. **Voice or video chat.** Discord, Campfire, and Roll20's built-in voice are all good enough. Building voice is the clearest possible scope mistake. Do the *interop* (G13) instead.
3. **Dynamic lighting, walls, line-of-sight, 3D.** Already excluded (ADR-0016). Agree. Vision rules that vary per system will need pack support eventually, but not in the MVP.
4. **A rules compendium or an SRD search UI.** The SRD is a *vault template*, per our own §6.4 — that is the correct call and should be defended. A rules-reference browser is a solved problem (D&D Beyond, SRD sites) and is a data problem, not a product problem.
5. **An AI GM or AI-generated content.** Phase 4's framing — "AI earns its place not in generating content, but in *filing* it into a schema" — is the right one and should be protected. The AI generates encounters; the *campaign* remembers them. Also: generating content at the table is the classic failure mode, and it changes the game in a way players did not consent to.
6. **A pack/plugin marketplace or registry.** ADR-0011's reasoning (a registry we run is a supply-chain surface of our own making) is sound. Discovery via README, forum post, and a shared link is fine. *However* — see G12: ship a **schema and starter Bases**, not a registry.
7. **A force-directed graph editor.** Already rejected in phase-1-plan.md M9 ("do not build a force-directed graph editor in phase 1"). Agree, and keep it rejected. A relation *schema* (G16) is worth having; a graph editor is not.
8. **Native mobile apps.** Rule 5 is right. A native client later is a benefit of the API contract, not a day-one requirement.
9. **Full-blown chat.** Do table chat, GM→player, player→GM, and an OOC marker. Do not build threads, reactions, DMs-as-a-product, or chat search. The test in G3: *can a GM hand a note to one player, and can a player ask the GM something privately, without leaving the app?*
10. **A full print/PDF designer.** Export the pack's sheet layout to a paginated document and be done. Do not build a template editor for print.
11. **A full calendar/moon-phase simulator.** A calendar *model* (era, date, optional moon) at the level of "the session happened on 3rd of Highwind, Year 512" is worth having. A fantasy-calendar simulator with moon cycles and seasonal weather is a hobby.
12. **Analytics, leaderboards, or a dice-log dashboard.** Nobody asked for this, and it is a competitive-mode feature, not a table feature.
13. **Automatic map generation, AI handouts, or AI-generated NPCs at the table.** These produce generic content at the exact moment the table is least forgiving, and they desynchronise from the vault. Random *tables* (deterministic, authored, in the vault) are the right version of this idea and we already have them.
14. **A social layer (friend requests, DMs, activity feeds).** The table is a group of four to six people who already know each other. Multiloop, Kanka and World Anvil all have some version of this; it is scope for a different product.
15. **Sync/hosting.** The design is explicit that the internet is optional and the user brings their own sync (git, Syncthing, Dropbox). Do not build a sync service; it contradicts ADR-0025/0031's philosophy and creates a data-loss liability.

---

## 5. A note on sequencing

If forced to choose three changes, they are:

1. **G1 (second screen + shared device)**, because it changes the *shape* of the projection layer and the player client, and both are expensive to change later.
2. **G2 + G7 (handouts, grants, revealable secrets)**, because they are the vocabulary of *reveal*, and reveal is the phase-3 interaction. They also cost one new note type and one new grant form.
3. **G4 (the in-session scratch log)**, because it is the difference between a tool that records what happened and a tool that *captures* what happened — and because it is the input phase 4's summariser needs most.

**G10 (a `campaign:` field) is the one to decide early even if it is not built early**, because it is a schema change and "structure is expensive to change later". The Obsidian community's one-vault-many-campaigns convention makes this more likely to bite than we assumed.

---

## 6. Sources

**Primary documentation**
- Foundry VTT — Journal Entries: https://foundryvtt.com/article/journal
- Foundry VTT — Combat: https://foundryvtt.com/article/combat
- Foundry VTT — Map Notes: https://foundryvtt.com/article/map-notes
- Foundry VTT — Rollable Tables: https://foundryvtt.com/article/roll-tables
- Foundry VTT — Localization: https://foundryvtt.com/article/localization/
- Foundry VTT — FAQ: https://foundryvtt.com/article/faq
- Foundry VTT modules — GM Only Tables: https://foundryvtt.com/packages/gm-only-tables ; Rolltable Requester: https://foundryvtt.com/packages/rolltable-requester ; Combat Tracker Extensions: https://foundryvtt.com/packages/combat-tracker-extensions ; Monk's Combat Details: https://foundryvtt.com/packages/monks-combat-details ; ReAcTiOnN's Combat Rules: https://foundryvtt.com/packages/reactionn-combat-rules
- Foundry Hub — Using Foundry for In-Person Gaming: https://www.foundryvtt-hub.com/guide/using-foundry-for-in-person-gaming
- Foundry Hub — Weblate translation project: https://www.foundryvtt-hub.com/news/lets-translate-all-the-packages-were-proud-to-announce-our-new-project-foundry-hub-weblate/ ; https://foundry-hub.github.io/weblate/
- Foundry VTT Community Wiki — Local play / dual screens: https://foundryvtt.wiki/en/setup/hosting/Local-play
- The Forge — Internationalization: https://blog.forge-vtt.com/internationalization/
- Roll20 — Using Roll20 while Playing In-Person: https://wiki.roll20.net/Using_Roll20_while_Playing_In-Person
- Roll20 — Journal (Handouts, per-player folders): https://help.roll20.net/hc/en-us/articles/360039675133-Journal ; https://wiki.roll20.net/Journal ; https://wiki.roll20.net/Handout
- Roll20 — Text Chat / whispers: https://wiki.roll20.net/Text_Chat ; https://help.roll20.net/hc/en-us/articles/360039675093-Text-Chat
- Roll20 — Discord Activity: https://help.roll20.net/hc/en-us/articles/25838862126359-Roll20-Discord-Activity
- Roll20 — Change Log (screen-reader work): https://help.roll20.net/hc/en-us/articles/360037772613-Change-Log ; https://help.roll20.net/hc/en-us/articles/20509479555991-2023-Change-Log
- Roll20 — colour-blindness thread: https://app.roll20.net/forum/permalink/11009797/ ; 2024 sheet thread: https://app.roll20.net/forum/post/11793851/announcing-the-new-character-sheet-for-d-and-d-2024/
- Roll20 — Send to Tabletop: https://help.roll20.net/hc/en-us/articles/39019410535703-Send-to-Tabletop
- Alchemy — Creating Handouts: https://help.alchemyrpg.com/en/articles/9821627-creating-handouts
- Alchemy — Chat Within Alchemy: https://help.alchemyrpg.com/en/articles/9821054-chat-within-alchemy
- Alchemy — Gamemaster Orientation: https://help.alchemyrpg.com/en/articles/9714185-gamemaster-orientation
- D&D Beyond — Introducing Journals on D&D Beyond: https://www.dndbeyond.com/posts/2175-never-lose-the-story-again-introducing-journals-on
- D&D Beyond — Maps VTT: https://www.dndbeyond.com/games
- D&D Beyond — Export Sheet: https://dndbeyond-support.wizards.com/hc/en-us/articles/7747238449556-Export-Sheet ; printing thread: https://www.dndbeyond.com/forums/d-d-beyond-general/general-discussion/55693-how-do-you-print-character-sheets
- D&D Beyond — Campaign Notes feedback: https://www.dndbeyond.com/forums/d-d-beyond-general/d-d-beyond-feedback/178816-campaign-notes
- D&D Beyond — Translating D&D Beyond: https://www.dndbeyond.com/forums/d-d-beyond-general/d-d-beyond-feedback/47203-translating-d-d-beyond-feedback
- D&D Beyond — "DM can be replaced mid-game": https://www.quora.com/In-D-D-can-the-DM-be-replaced-mid-game
- Kanka — Features: https://kanka.io/features ; Premium campaigns: https://kanka.io/premium
- Kanka vs World Anvil: https://kanka.io/kanka-vs-worldanvil
- Multiloop — comparison page (competitor, read for feature claims): https://multiloop.app/compare
- Obsidian — Bases core plugin: https://obsidian.md/ ; Canvas: https://obsidian.md/canvas
- Obsidian Properties guide (type-not-folder, Vault Conventions, Bases vs Dataview): https://obsidianmate.com/article/obsidian-properties-complete-guide
- Obsidian — Bases vs Dataview (performance): https://www.youtube.com/watch?v=1Mgsdpy7awE
- Obsidian — Bases made simple tutorial: https://www.youtube.com/watch?v=34NAe62IH-M

**Obsidian TTRPG community**
- Obsidian TTRPG Tutorials: https://obsidianttrpgtutorials.com/
- Obsidian Hub — for TTRPG: https://publish.obsidian.md/hub/04+-+Guides%2C+Workflows%2C+%26+Courses/for+TTRPG
- PhD20 — Awesome Obsidian community plugins for D&D: https://phd20.com/blog/obsidian-community-plugins-dnd
- PhD20 — Organizing Campaigns for D&D in Obsidian: https://phd20.com/blog/organizing-obsidian-dnd-campaigns/ ; Organising an Atlas: https://phd20.com/blog/organizing-an-atlas-for-dnd-in-obsidian/
- Obsidian-TTRPG-Community — Initiative Tracker: https://github.com/Obsidian-TTRPG-Community/initiative-tracker
- Obsidian plugin — Campaign Codex: https://community.obsidian.md/plugins/dnd-campaign-manager
- Obsidian plugin — GM Map (second screen): https://community.obsidian.md/plugins/gm-map
- Atlas VTT (open-source Obsidian VTT): https://atlas-vtt.dev/ ; https://github.com/atlas-vtt/atlas-vtt ; https://www.obsidianstats.com/plugins/atlas-vtt
- Obsidian stats — TTRPG plugins 2026: https://www.obsidianstats.com/posts/2026-08-11-ttrpg-plugins
- Obsidian stats — plugins generating AGENTS.md: https://www.obsidianstats.com/tags/front-matter
- GM Assistant — Using Obsidian for D&D and TTRPG Notes: https://gmassistant.app/blog/using-obsidian-for-ttrpg-notes-part-1/index.html
- Sly Flourish — Using Obsidian for Lazy RPG Prep: https://slyflourish.com/obsidian.html
- Gnome Stew — Campaign Management in Obsidian: https://gnomestew.com/campaign-management-in-obsidian
- James Goldie — Better tabletop maps with Obsidian Canvas: https://jamesgoldie.dev/writing/tabletop-maps-obsidian-canvas
- DM20 Protocol MCP server: https://glama.ai/mcp/servers/Polloinfilzato/dm20-protocol

**Safety tools / session zero**
- RPGKC — Safety Tools: https://www.rpgkc.org/resources/safety-tools
- Melestrua's Musings — Safety Tools in RPGs: https://melestrua.net/2022/07/29/safety-tools-in-rpgs
- Sly Flourish — D&D Safety Tools: https://slyflourish.com/safety_tools.html
- StartPlaying — Safety Tools explained: https://startplaying.games/blog/posts/startplaying-safety-tools-explained-how-to
- Camp Dragon — Exploring TTRPG Safety Tools: https://www.campdragononline.com/blog/exploring-ttrpg-safety-tools-x-card-session-zero-and-more

**GM workflow / time sinks**
- Sly Flourish — How long does it take to prep: https://slyflourish.com/how_long_to_prep.html
- r/rpg — prep time thread: https://www.reddit.com/r/rpg/comments/1cqutet/what_is_your_prep_time_to_be_a_gm_for_a_game/
- Dungeonsolvers — D&D session prep time: https://www.dungeonsolvers.com/how-long-prep-dnd-sessions
- Roleplaying Tips — Random Encounter Tables: https://www.roleplayingtips.com/rptn/random-encounter-tables
- Tools and Taverns — NPC generator: https://toolsandtaverns.com/npc-generator
- Monstershuffler — NPC/stat block generator: https://www.monstershuffler.com
- Giantitp — creating NPCs quickly: https://forums.giantitp.com/showthread.php?369044/Creating-NPC-and-enemies-How-do-you-do-it-quickly=
- StartPlaying — keeping players coming back (async GM availability, check-ins): https://startplaying.games/blog/posts/how-to-keep-players-coming-back-new-gm-guide
- Roleplaying Tips — Conducting GM Surveys / mid-campaign check-in: https://www.roleplayingtips.com/players-characters/conducting-gm-surveys ; https://www.openheartgames.com/blog/the-mid-campaign-check-in
- Gnome Stew — Downtime Activities: https://gnomestew.com/downtime-activities/ ; Adventurer's Guild Wiki: https://adventurers-guild.fandom.com/wiki/Downtime_Activities
- r/DMAcademy — "how do you all actually keep track of your campaign": https://www.reddit.com/r/DMAcademy/comments/1tyrxbm/how_do_you_all_actually_keep_track_of_your/

**Player expectations / character**
- r/DnD — campaign journals: https://www.reddit.com/r/DnD/comments/5p5lfn/does_anyone_keep_campaign_journals
- r/rpg.stackexchange — taking notes as a player: https://rpg.stackexchange.com/questions/198657/how-to-best-take-notes-as-a-player-in-an-ongoing-campaign
- r/DMAcademy — reading players' character sheets: https://www.reddit.com/r/DMAcademy/comments/120kp7p/do_you_look_at_your_players_character_sheets
- r/rpg.stackexchange — what info do you need from players' sheets: https://rpg.stackexchange.com/questions/137592/what_information_do_i_actually_need_from_my_players-character-sheets
- Pathbuilder 2e (character-builder UX benchmark): https://pathbuilder2e.com/ ; App Store reviews: https://apps.apple.com/us/app/pathbuilder-2e/id6740821064
- Demiplane forums — Pathbuilder lacks GM management: https://forums.demiplane.com/t/uphill-battle-vs-pathbuilder/1855

**Player in / out, handover**
- r/DnD — what to do with a character when their player leaves: https://www.reddit.com/r/DnD/comments/xrauux/what_do_you_do_with_a_character_when_their_player/
- rpg.stackexchange — player left at a moment's notice: https://rpg.stackexchange.com/questions/131764/a_player_left_the_group_at_a_moments_notice_and_i_am_at_a_loss_for_what_to_do
- EN World — who owns a PC after the player stops using them: https://www.enworld.org/threads/who-%E2%80%9Cowns%E2%80%9D-a-pc-after-the-player-stops-using-them.702898/
- rpg.stackexchange — alternating GMs: https://rpg.stackexchange.com/questions/27163/running-a-campaign-alternating-gms-every-session
- QuestKeepers Kickstarter FAQ — players joining/leaving: https://www.kickstarter.com/projects/mettleurgy/questkeepers-the-ideal-note-taking-experience-for-ttrpgs/faqs
- Roleplaying Tips — when a friend leaves your game: https://www.roleplayingtips.com/running-games/when-a-friend-leaves-your-game/

**Accessibility**
- Can I Play That — Color-Blindness Accessibility Guide: https://caniplaythat.com/2020/01/29/color-blindness-accessibility-guide
- Fate Accessibility Toolkit: https://fate-srd.com/fate-accessibility-toolkit ; Evil Hat product page: https://evilhat.com/product/fate-accessibility-toolkit/
- The RPG Gazette — GMing for the Visually Impaired: https://therpggazette.wordpress.com/2025/06/04/gming-for-the-visually-impaired-a-guide/
- r/rpg — accessibility options players want: https://www.reddit.com/r/rpg/comments/14zsiyq/what_accessibility_options_do_you_like_or_want_in
- rpg.stackexchange — making RPGs more accessible: https://forum.rpg.net/index.php?threads%2Fmaking-rpgs-more-accessible-for-those-with-disabilities.851467%2F=
- Can I Play That — dyslexic-friendly fonts note (in the colour-blindness guide above)

**Localisation / non-English play**
- r/rpg — non-native English speakers and the RPG ecosystem: https://www.reddit.com/r/rpg/comments/1n3pfxn/nonnative_englishspeakers_whats_the_rpg_ecosystem/
- r/rpg.stackexchange — running a non-English game: https://rpg.stackexchange.com/questions/199909/running-a-non-english-game
- EN World — playing D&D in a foreign language: https://www.enworld.org/threads/have-you-played-d-d-in-a-foreign-language.77157/

**Discord / community complaints**
- Roll20 forum — why use Discord for audio: https://www.facebook.com/groups/714169855634812/posts/1748095588908895/
- r/Roll20 — "anyway to see players' whispers as a GM": https://app.roll20.net/forum/post/11623132/anyway-to-see-players-whispers-to-each-other-as-a-gm
- Roll20 forum — price increase backlash: https://app.roll20.net/forum/permalink/10040922/
- r/DnD — D&D Beyond 2048-character limit on campaign notes: https://www.reddit.com/r/DnD/comments/1m8i69o/dndbeyond_just_nuked_their_campaign_page_by/
- r/FoundryVTT — mobile/phone usability: https://www.reddit.com/r/FoundryVTT/comments/1nvu91g/how_do_you_handle_vtt_when_most_players_only_have ; learning-curve thread: https://www.reddit.com/r/rpg/comments/1q39et9/what_is_the_learning_curve_of_foundry_vtt/
- r/VTT — Alchemy comparison: https://www.reddit.com/r/VTT/comments/1vehahb/alchemy_foundry_or_roll20 ; Numtini's Corner — Alchemy: https://www.numtini.com/2025/01/14/alchemy-vtt-changing-gold-into-lead
- r/DungeonsAndDragons — one screen for the whole table: https://www.reddit.com/r/DungeonsAndDragons/comments/153a0mt/what_vttprogram_do_you_use_for_one_screen_for_the/
- r/FoundryVTT — VTT for two screens on the same computer: https://www.reddit.com/r/DMAcademy/comments/1l9tm9u/vtt_for_two_screens_on_the_same_computer
- 14 VTT comparison (accounts required): https://www.youtube.com/watch?v=ptzr70aVc4M
- Star Playing — comprehensive VTT list: https://startplaying.games/blog/posts/a-comprehensive-list-of-virtual-tabletops
- GM Craft Tavern — Foundry vs Roll20 vs Owlbear 2026: https://gmcrafttavern.com/foundry-vs-roll20-owlbear-2026
- Cannibal Halfling — system split: campaign managers: https://cannibalhalflinggaming.com/2023/03/29/system-split-campaign-managers
- Saga20 — D&D note-taking apps compared: https://saga20.com/blog/dnd-note-taking-apps-compared
- Litrpgreads — printable sheets as "a beloved classic": https://litrpgreads.com/blog/rpg/download-the-new-2024-dnd-character-sheets-official-pdfs-now-available
- "7 FREE D&D tools" (concentration check, notes-by-date): https://www.youtube.com/watch?v=rPek96Ki5os
