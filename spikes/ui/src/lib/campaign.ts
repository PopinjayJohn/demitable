/**
 * The demo campaign the prototype runs against.
 *
 * The *prose* is written for the spike. The *embedded corpus excerpts* are
 * verbatim — see `src/lib/corpus/` and `tools/extract-corpus.mjs`.
 *
 * This is not phase 1's demo campaign. Question 8 of the phase-0 brief writes
 * that one, and it is a deliverable. This is a fixture: it exists to exercise
 * the projection, and it is shaped to make the awkward cases visible.
 */

import dragonStatblock from "./corpus/dragon-statblock.md?raw";
import guardStatblock from "./corpus/guard-statblock.md?raw";
import guardQuote from "./corpus/guard-quote.md?raw";
import wizardTldr from "./corpus/wizard-tldr.md?raw";
import ammunitionNote from "./corpus/ammunition-note.md?raw";
import fireBolt from "./corpus/fire-bolt.md?raw";
import deckTable from "./corpus/deck-table.md?raw";

import type { Note, Principal } from "./model";
import type { SecretVocabulary } from "./projection";

/** The pack's secret vocabulary (ADR-0036). Configuration, not a hardcoded list. */
export const VOCABULARY: SecretVocabulary = {
  calloutTypes: ["secret", "gm-only", "hidden"],
  codeBlockKeys: ["ad-secret"],
  statblockGmKey: "gm",
};

export const PRINCIPALS: Principal[] = [
  { id: "gm", name: "The GM", kind: "gm" },
  { id: "p_8f2a", name: "Bram", kind: "player", campaign: "stormdark" },
  { id: "p_91bc", name: "Wren", kind: "player", campaign: "stormdark" },
  { id: "display", name: "The shared display", kind: "display" },
];

/* --------------------------------------------------------------- the notes */

const dragonBody = `# Adult Red Dragon

${dragonStatblock.split("\n").slice(5).join("\n")}

## Environment

hill, mountain`;

const guardBody = `# Guard

${guardQuote.split("\n").slice(5).join("\n")}

${guardStatblock.split("\n").slice(5).join("\n")}`;

const fireBoltBody = fireBolt.split("\n").slice(5).join("\n");

export const NOTES: Note[] = [
  {
    uid: "01HQDRAGON0000000000000",
    type: "monster",
    name: "Adult Red Dragon",
    campaign: "stormdark",
    // A monster is `gm` by default (ADR-0033's content visibility table).
    // The party has not met it.
    visibility: "gm",
    path: "compendium/bestiary/dragon/adult-red-dragon-xmm.md",
    contentType: "monster",
    properties: [
      { key: "size", label: "Size", value: "Huge", type: "string" },
      { key: "creature_type", label: "Creature Type", value: "Dragon (chromatic)", type: "string" },
      { key: "challenge", label: "Challenge", value: "17 (9,000 XP)", type: "string" },
    ],
    body: dragonBody,
  },
  {
    uid: "01HQGUARD000000000000000",
    type: "monster",
    name: "Guard",
    campaign: "stormdark",
    // Granted to the party: they have met one.
    visibility: "party",
    path: "compendium/bestiary/humanoid/guard-xmm.md",
    contentType: "monster",
    properties: [
      { key: "size", label: "Size", value: "Small or Medium", type: "string" },
      { key: "creature_type", label: "Creature Type", value: "Humanoid", type: "string" },
      { key: "challenge", label: "Challenge", value: "1/8 (25 XP)", type: "string" },
    ],
    body: guardBody,
  },
  {
    uid: "01HQFIREBOLT000000000000",
    type: "spell",
    name: "Fire Bolt",
    campaign: "stormdark",
    // A spell is `party` by default — a player must be able to read their
    // own spells.
    path: "compendium/spells/fire-bolt-xphb.md",
    contentType: "spell",
    properties: [
      { key: "level", label: "Level", value: "Cantrip", type: "string" },
      { key: "school", label: "School", value: "Evocation", type: "string" },
      { key: "casting_time", label: "Casting Time", value: "1 Action", type: "string" },
    ],
    body: fireBoltBody,
  },
  {
    uid: "01HQHOLLOW00000000000000",
    type: "location",
    name: "The Hollow Hill",
    campaign: "stormdark",
    visibility: "party",
    path: "campaign/stormdark/locations/hollow-hill.md",
    contentType: "location",
    properties: [
      { key: "region", label: "Region", value: "The Drowned March", type: "string" },
      { key: "discovered", label: "Discovered by", value: "The party", type: "string" },
      {
        key: "true_nature",
        label: "True Nature",
        value: "A sealed reliquary of the Cult of the Pale Star.",
        type: "longtext",
        // A property the pack marks GM-only. This is what a note's *content*
        // looks like when the pack decides, rather than the note's own field.
        visibility: { visibility: "gm" },
      },
    ],
    body: `# The Hollow Hill

A low grassy mound at the edge of the marsh, marked by three standing stones.
The party found the entrance on the second day of the search.

> [!secret] What is behind the door
> The door is not a door. It is the seal, and something on the other side is
> testing it from the inside. It will open on the third dawn whether the party
> is ready or not.

The stone is older than the road, and the road is older than the village.

${deckTable.split("\n").slice(5).join("\n")}`,
  },
  {
    uid: "01HQDUKE000000000000000",
    type: "handout",
    name: "The Duke's Letter",
    campaign: "stormdark",
    // A handout is normally `gm`, and its reveal is an event.
    visibility: "gm",
    grants: ["p_8f2a"],
    revealed: ["dukes-letter"],
    path: "campaign/stormdark/handouts/dukes-letter.md",
    contentType: "handout",
    properties: [
      { key: "from", label: "From", value: "Duke Alaric Vane", type: "string" },
      { key: "sealed", label: "Sealed", value: "Yes", type: "boolean" },
    ],
    body: `# The Duke's Letter

> [!secret] The letter, unsealed
> Alaric — bring the relic to the ford at the third dawn. Bring it alone, or
> the marsh takes the village.

*Received by Bram. Wren does not know.*`,
  },
  {
    uid: "01HQBRAM0000000000000000",
    type: "character",
    name: "Bram",
    campaign: "stormdark",
    kind: "pc",
    player: "p_8f2a",
    visibility: "owner",
    workflow: "active",
    path: "campaign/stormdark/characters/bram.md",
    contentType: "character",
    properties: [
      { key: "name", label: "Name", value: "Bram Oakhide", type: "string" },
      { key: "ancestry", label: "Ancestry", value: "Human", type: "string" },
      { key: "background", label: "Background", value: "Haunted One", type: "string" },
      { key: "level", label: "Level", value: "3", type: "number" },
      { key: "hp_max", label: "Hit Points", value: "24", type: "number" },
      { key: "hp_current", label: "Current Hit Points", value: "17", type: "number" },
      { key: "ac", label: "Armour Class", value: "15", type: "number" },
      { key: "speed", label: "Speed", value: "30 ft.", type: "string" },
      { key: "save_dc", label: "Saving Throw DC", value: "13", type: "number" },
      { key: "initiative", label: "Initiative", value: "+2", type: "number" },
      { key: "conditions", label: "Conditions", value: "Poisoned (1)", type: "list" },
      { key: "inspiration", label: "Inspiration", value: "Yes", type: "boolean" },
      {
        key: "carry",
        label: "Carrying Capacity",
        value: "135 lb.",
        type: "string",
        // `owner` resolves against the note's player. Bram has one, so this
        // is Bram's business — and the GM's.
        visibility: { visibility: "owner" },
      },
      {
        key: "motivation",
        label: "True Motivation",
        value: "Bram is looking for his sister, and will trade the relic for news of her.",
        type: "longtext",
        visibility: { visibility: "gm" },
      },
      {
        key: "gm_note",
        label: "GM Notes",
        value: "Bram has not told the party about the sister. He is lying about the dreams.",
        type: "longtext",
        visibility: { visibility: "gm" },
      },
      {
        key: "private_note",
        label: "Bram's own note",
        value: "I do not trust Wren. I have not told them why.",
        type: "longtext",
        // `private` — the note's player, and nobody else. Not even the GM.
        visibility: { visibility: "private" },
      },
    ],
    body: `# Bram Oakhide

*A human of the Drowned March, and the only one of the party who reads the
marsh the way the marsh reads itself.*

## Appearance

Broad, weather-beaten, with a hunter's habit worn soft at the elbows.
${ammunitionNote.split("\n").slice(5).join("\n")}`,
  },
  {
    uid: "01HQCAPTAIN0000000000000",
    type: "character",
    name: "Captain Edda Marr",
    campaign: "stormdark",
    kind: "npc",
    visibility: "gm",
    path: "campaign/stormdark/characters/captain-eda-marr.md",
    contentType: "character",
    properties: [
      { key: "name", label: "Name", value: "Captain Edda Marr", type: "string" },
      { key: "occupation", label: "Occupation", value: "Watch captain", type: "string" },
      { key: "level", label: "Level", value: "3", type: "number" },
      { key: "hp_max", label: "Hit Points", value: "22", type: "number" },
      {
        key: "carry",
        label: "Carrying Capacity",
        value: "210 lb.",
        type: "string",
        // `owner` on a property, but the note has **no player**. ADR-0033's
        // rule 7 says the property falls back to the note's visibility — which
        // here is `gm`. Without the fallback, the shipped pack hid every NPC's
        // hit points from the players.
        visibility: { visibility: "owner" },
      },
      {
        key: "motive",
        label: "Motive",
        value: "She has been paid to lose the party's trail, and she intends to do it badly.",
        type: "longtext",
        visibility: { visibility: "gm" },
      },
    ],
    body: `# Captain Edda Marr

The watch captain at the ford. Courteous, tired, and extremely careful about
the third stone from the left.`,
  },
  {
    uid: "01HQMARSHAL000000000000",
    type: "character",
    name: "Marshal Ivo Sarn",
    campaign: "stormdark",
    kind: "npc",
    // A party-visible NPC: the party has met him and can read his sheet.
    visibility: "party",
    path: "campaign/stormdark/characters/marshal-ivo-sarn.md",
    contentType: "character",
    properties: [
      { key: "name", label: "Name", value: "Marshal Ivo Sarn", type: "string" },
      { key: "occupation", label: "Occupation", value: "Marshal of the March", type: "string" },
      { key: "level", label: "Level", value: "3", type: "number" },
      { key: "hp_max", label: "Hit Points", value: "26", type: "number" },
      {
        key: "disposition",
        label: "Disposition",
        value: "Hostile, but unwilling to be the one who says so",
        type: "string",
        // `party`, masked for one player. The masked value is still a value:
        // its *presence* is not a leak.
        visibility: { visibility: "party", display: { p_91bc: "Unknown to Wren" } },
      },
      {
        key: "motive",
        label: "Motive",
        value: "He has been told the relic must not reach the ford, and he believes it.",
        type: "longtext",
        // A property the pack marks GM-only, on a note the party can see.
        // This is the case ADR-0033 rule 7 exists for.
        visibility: { visibility: "gm" },
      },
    ],
    body: `# Marshal Ivo Sarn

The marshal of the Drowned March, and the man who sent the party looking for
the door in the first place.`,
  },
  {
    uid: "01HQWIZARD00000000000000",
    type: "class",
    name: "Wizard — Class and Feature Progression",
    campaign: "stormdark",
    visibility: "party",
    path: "compendium/classes/wizard-xphb.md",
    contentType: "class",
    properties: [
      { key: "source", label: "Source", value: "SRD 5.2 and the Free Rules (2024)", type: "string" },
    ],
    // The `> [!tldr]` callout whose body is a raw HTML table.
    body: wizardTldr.split("\n").slice(5).join("\n"),
  },
  {
    uid: "01HQSESSION00000000000000",
    type: "session",
    name: "Session 3 — The Hollow Hill",
    campaign: "stormdark",
    visibility: "party",
    path: "campaign/stormdark/sessions/session-3.md",
    contentType: "session",
    properties: [
      { key: "date", label: "Date", value: "14th of Ashfall", type: "string" },
      { key: "party", label: "Party", value: "Bram, Wren", type: "string" },
    ],
    body: `# Session 3 — The Hollow Hill

The party tracked the sealed door to the third standing stone and found the
seal already broken from the inside. Nothing came out. The grass on the mound
had been flattened in a circle.

## What the party knows

- There is a door in the Hollow Hill, and it was open.
- The stones are older than the road.
- Edda Marr did not meet the party's eyes once.

## What they do not know

- The letter.
- The relic.
- What the seal was holding.`,
  },
  {
    uid: "01HQEMBERFALL00000000000",
    type: "location",
    name: "The Emberfall Gate",
    // A different campaign. The player is not a member, so it is out of
    // scope — a different leak from the one ADR-0033 fixed.
    campaign: "emberfall",
    visibility: "party",
    path: "campaign/emberfall/locations/emberfall-gate.md",
    contentType: "location",
    properties: [],
    body: `# The Emberfall Gate

A gate of black iron, still warm.`,
  },
  {
    uid: "01HQPERSONAL0000000000000",
    type: "note",
    name: "Session 2 — the shopping list",
    // No campaign. ADR-0033's default: a note without a campaign is `gm`.
    // The scaffolder's trap.
    path: "personal/shopping.md",
    contentType: "note",
    properties: [],
    body: `# Session 2 — the shopping list

- Rope, 50 ft.
- Chalk
- A second lamp`,
  },
  {
    uid: "01HQDRAFT000000000000000",
    type: "quest",
    name: "The reliquary of the Pale Star",
    campaign: "stormdark",
    // The workflow gate: a draft, visible only to its owner and the GM.
    workflow: "draft",
    visibility: "party",
    path: "campaign/stormdark/quests/reliquary.md",
    contentType: "quest",
    properties: [
      { key: "stage", label: "Stage", value: "Opening", type: "string" },
    ],
    body: `# The reliquary of the Pale Star

Three seals, one of them broken from the inside.`,
  },
];

export const CAMPAIGNS = [
  { id: "stormdark", name: "Stormdark", noteCount: 0 },
  { id: "emberfall", name: "Emberfall", noteCount: 0 },
];
