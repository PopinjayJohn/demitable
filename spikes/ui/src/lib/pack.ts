/**
 * A pack, in the shape ADR-0034 and ADR-0023 describe.
 *
 * The character sheet is a **form panel rendered from the pack's
 * `views.sheet`** (docs/tech-stack.md §5). This is that declaration, for the
 * prototype's character panel.
 *
 * Two things are worth noticing, and both are consequences of ADR-0033:
 *
 *  - the pack declares a **visibility per field**, and a field with no
 *    declaration inherits the note's;
 *  - the field list is *the same* for the GM and the player. The player gets
 *    the same form with the fields they may not see removed — not a second,
 *    smaller form. A second form is a second thing to keep in step.
 */

export interface PackField {
  key: string;
  label: string;
  type: "string" | "number" | "boolean" | "list" | "longtext" | "group";
  /** A pack-declared visibility. Absent means: inherit the note's. */
  visibility?: { visibility?: "public" | "party" | "owner" | "private" | "gm"; display?: Record<string, string> };
  /** Grouped fields, for the character sheet's sections. */
  group?: string;
}

export interface PackGroup {
  id: string;
  label: string;
}

export const PACK_ID = "dnd5e";
export const PACK_NAME = "D&D 5e (SRD 5.2)";

export const SHEET_GROUPS: PackGroup[] = [
  { id: "identity", label: "Identity" },
  { id: "combat", label: "Combat" },
  { id: "state", label: "State" },
  { id: "gm", label: "GM only" },
];

export const SHEET_FIELDS: PackField[] = [
  { key: "name", label: "Name", type: "string", group: "identity" },
  { key: "ancestry", label: "Ancestry", type: "string", group: "identity" },
  { key: "background", label: "Background", type: "string", group: "identity" },
  { key: "level", label: "Level", type: "number", group: "combat" },
  { key: "ac", label: "Armour Class", type: "number", group: "combat" },
  { key: "hp_max", label: "Hit Points", type: "number", group: "combat" },
  { key: "hp_current", label: "Current Hit Points", type: "number", group: "combat" },
  { key: "speed", label: "Speed", type: "string", group: "combat" },
  { key: "save_dc", label: "Saving Throw DC", type: "number", group: "combat" },
  { key: "initiative", label: "Initiative", type: "number", group: "combat" },
  { key: "conditions", label: "Conditions", type: "list", group: "state" },
  { key: "inspiration", label: "Inspiration", type: "boolean", group: "state" },
  {
    key: "carry",
    label: "Carrying Capacity",
    type: "string",
    group: "state",
    visibility: { visibility: "owner" },
  },
  {
    key: "disposition",
    label: "Disposition",
    type: "string",
    group: "state",
    // `party`, with a mask for one player. Masking is applied *last*, after
    // grants and denies have decided whether a value is present — so a masked
    // value is still a value, and its absence is not a leak.
    visibility: { visibility: "party", display: { p_91bc: "Unknown to Wren" } },
  },
  {
    key: "motivation",
    label: "True Motivation",
    type: "longtext",
    group: "gm",
    visibility: { visibility: "gm" },
  },
  {
    key: "gm_note",
    label: "GM Notes",
    type: "longtext",
    group: "gm",
    visibility: { visibility: "gm" },
  },
  {
    key: "private_note",
    label: "Own note",
    type: "longtext",
    group: "state",
    visibility: { visibility: "private" },
  },
];

/** A pack's labels are data, and therefore translatable (ADR-0034). */
export const PACK_LABELS = {
  en: { hp: "Hit Points", ac: "Armour Class", speed: "Speed", initiative: "Initiative" },
  fr: { hp: "Points de Vie", ac: "Classe d'Armure", speed: "Vitesse", initiative: "Initiative" },
};
