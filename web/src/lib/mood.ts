// Mood derivation for P4: a pure function, no I/O, deterministic.
// Fixed vocabulary: tense, triumphant, sombre, calm. Unknown input -> calm.

export const MOOD_TENSE = "tense";
export const MOOD_TRIUMPHANT = "triumphant";
export const MOOD_SOMBRE = "sombre";
export const MOOD_CALM = "calm";
export const DEFAULT_MOOD = MOOD_CALM;
export const MOOD_VOCABULARY = [MOOD_TENSE, MOOD_TRIUMPHANT, MOOD_SOMBRE, MOOD_CALM];

export type Mood = string;

export interface EncounterState {
  active?: boolean;
  is_active?: boolean;
  in_combat?: boolean;
  combat?: boolean;
  round?: number;
  current_round?: number;
  round_number?: number;
  hp?: Record<string, number>;
  hp_fractions?: Record<string, number>;
  hpFractions?: Record<string, number>;
  side_hp?: Record<string, number>;
  sideHp?: Record<string, number>;
  health?: Record<string, number>;
  fractions?: Record<string, number>;
  conditions?: string[];
  statuses?: string[];
  effects?: string[];
  sides?: Array<Record<string, unknown>>;
  participants?: Array<Record<string, unknown>>;
  encounter?: unknown;
  [key: string]: unknown;
}

const combatTerms = new Set([
  "combat", "attack", "attacks", "attacked", "initiative", "damage", "hit", "hits",
  "strike", "strikes", "wound", "wounded", "battle", "fight", "fighting", "weapon",
  "spell", "cast", "bloodied", "enemy", "monster", "hostile",
]);
const restTerms = new Set([
  "rest", "camp", "heal", "healing", "recover", "recovery", "safe", "breather",
  "shortrest", "longrest", "short rest", "long rest",
]);
const victoryTerms = new Set([
  "victory", "victorious", "triumph", "triumphant", "defeated", "win", "won",
  "success", "cleared",
]);
const sombreTerms = new Set([
  "death", "died", "dead", "mourning", "mourn", "funeral", "grief", "loss", "lost",
  "tragedy", "tears", "fallen", "tpk",
]);
const tenseConditions = new Set([
  "bloodied", "stunned", "poisoned", "prone", "frightened", "charmed", "restrained",
  "paralyzed", "unconscious", "dying", "wounded", "staggered",
]);
const sombreConditions = new Set(["dead", "deceased", "mourning", "fallen"]);
const partyKeys = ["party", "players", "allies", "ally", "pc", "pcs", "heroes", "hero"];
const enemyKeys = ["enemy", "enemies", "monster", "monsters", "opposition", "hostile", "hostiles", "foe", "foes", "npc"];

function toBool(v: unknown): boolean {
  if (typeof v === "boolean") return v;
  if (typeof v === "number") return v !== 0;
  if (typeof v === "string") {
    const s = v.trim().toLowerCase();
    return s === "true" || s === "yes" || s === "1" || s === "active" || s === "in_combat";
  }
  return false;
}

function toFloat(v: unknown): number {
  if (typeof v === "number") return v;
  if (typeof v === "string") {
    const n = Number(v.trim());
    return isNaN(n) ? 0 : n;
  }
  return 0;
}

function toStrings(v: unknown): string[] {
  if (v == null) return [];
  if (Array.isArray(v)) return v.map((e) => String(e));
  if (typeof v === "string") return [v];
  return [String(v)];
}

function mergeHp(dst: Record<string, number>, src: unknown): void {
  if (!src || typeof src !== "object") return;
  if (Array.isArray(src)) return;
  for (const [k, val] of Object.entries(src as Record<string, unknown>)) {
    dst[k.trim().toLowerCase()] = toFloat(val);
  }
}

function normalizeEncounter(input: unknown): {
  active: boolean;
  round: number;
  hp: Record<string, number>;
  conditions: string[];
} {
  const out = { active: false, round: 0, hp: {} as Record<string, number>, conditions: [] as string[] };
  if (input == null) return out;
  if (typeof input !== "object" || Array.isArray(input)) {
    // maybe a struct-like? no; primitive -> default
    return out;
  }
  const o = input as Record<string, unknown>;

  // helper get key case-insensitive
  const get = (keys: string[]): unknown => {
    for (const k of keys) {
      if (Object.prototype.hasOwnProperty.call(o, k)) return o[k];
      const lk = k.toLowerCase();
      for (const mk of Object.keys(o)) {
        if (mk.toLowerCase() === lk) return o[mk];
      }
    }
    return undefined;
  };

  const activeVal = get(["active", "is_active", "active_encounter", "in_combat", "combat"]);
  if (activeVal !== undefined) out.active = toBool(activeVal);

  const roundVal = get(["round", "current_round", "round_number"]);
  if (roundVal !== undefined) out.round = Math.trunc(toFloat(roundVal));

  for (const key of ["hp", "hp_fractions", "hpFractions", "side_hp", "sideHp", "health", "fractions", "hp_fraction"]) {
    const v = get([key]);
    if (v !== undefined) mergeHp(out.hp, v);
  }

  for (const key of ["conditions", "statuses", "effects", "condition_list", "status_list"]) {
    const v = get([key]);
    if (v !== undefined) {
      for (const s of toStrings(v)) out.conditions.push(s.trim().toLowerCase());
    }
  }

  // nested encounter
  const nested = get(["encounter", "encounter_state", "state"]);
  if (nested && typeof nested === "object" && !Array.isArray(nested)) {
    const inner = normalizeEncounter(nested);
    if (inner.active) out.active = true;
    if (inner.round !== 0) out.round = inner.round;
    for (const [k, v] of Object.entries(inner.hp)) out.hp[k] = v;
    out.conditions = out.conditions.concat(inner.conditions);
  }

  // sides/participants array
  const arrVal = get(["sides", "participants"]);
  if (Array.isArray(arrVal)) {
    for (const e of arrVal) {
      if (e && typeof e === "object" && !Array.isArray(e)) {
        const m = e as Record<string, unknown>;
        const sideGet = (keys: string[]) => {
          for (const k of keys) {
            if (Object.prototype.hasOwnProperty.call(m, k)) return m[k];
            const lk = k.toLowerCase();
            for (const mk of Object.keys(m)) if (mk.toLowerCase() === lk) return m[mk];
          }
          return undefined;
        };
        const sideRaw = sideGet(["side", "name", "team", "faction"]);
        const side = sideRaw != null ? String(sideRaw) : "";
        let frac: number | null = null;
        for (const key of ["hp", "hp_fraction", "fraction", "health", "hp_percent"]) {
          const fv = sideGet([key]);
          if (fv !== undefined) { frac = toFloat(fv); break; }
        }
        if (side && frac !== null) out.hp[side.trim().toLowerCase()] = frac as number;
      }
    }
  }

  return out;
}

function isPartyKey(k: string): boolean {
  return partyKeys.some((p) => k.includes(p));
}
function isEnemyKey(k: string): boolean {
  return enemyKeys.some((p) => k.includes(p));
}

function partyFraction(hp: Record<string, number>): number {
  const vals = Object.entries(hp).filter(([k]) => isPartyKey(k)).map(([, v]) => v);
  if (vals.length === 0) return 1;
  return Math.min(...vals);
}
function enemyFraction(hp: Record<string, number>): number {
  const vals = Object.entries(hp).filter(([k]) => isEnemyKey(k)).map(([, v]) => v);
  if (vals.length === 0) return 1;
  return Math.min(...vals);
}

function normalizeKeywords(input: unknown): string[] {
  if (input == null) return [];
  if (Array.isArray(input)) return input.map((e) => String(e));
  if (typeof input === "string") {
    return input.split(/[\s,]+/).map((s) => s.trim()).filter(Boolean);
  }
  return [String(input)];
}

export function deriveMood(encounter: unknown, keywords: unknown): Mood {
  const n = normalizeEncounter(encounter);
  const kws = normalizeKeywords(keywords).map((k) => k.trim().toLowerCase()).filter(Boolean);

  let hasSombre = false;
  let hasCombat = n.active || n.round > 0;
  let hasVictory = false;
  let hasRest = false;

  for (const k of kws) {
    if (sombreTerms.has(k)) hasSombre = true;
    if (combatTerms.has(k)) hasCombat = true;
    if (victoryTerms.has(k)) hasVictory = true;
    if (restTerms.has(k)) hasRest = true;
    for (const term of sombreTerms) if (k.includes(term)) hasSombre = true;
    for (const term of combatTerms) if (k.includes(term)) hasCombat = true;
    for (const term of victoryTerms) if (k.includes(term)) hasVictory = true;
    for (const term of restTerms) if (k.includes(term)) hasRest = true;
  }
  for (const c of n.conditions) {
    if (sombreConditions.has(c)) hasSombre = true;
    if (tenseConditions.has(c)) hasCombat = true;
  }
  const pf = partyFraction(n.hp);
  const ef = enemyFraction(n.hp);
  if (pf <= 0) hasSombre = true;
  if (pf < 1) hasCombat = true;
  if (n.active && ef <= 0 && Object.keys(n.hp).length > 0) hasVictory = true;
  if (!n.active && ef <= 0 && pf > 0 && Object.keys(n.hp).length > 0) hasVictory = true;

  if (hasSombre) return MOOD_SOMBRE;
  if (hasCombat) return MOOD_TENSE;
  if (hasVictory) return MOOD_TRIUMPHANT;
  if (hasRest) return MOOD_CALM;
  return DEFAULT_MOOD;
}

// aliases
export const deriveMoodTag = deriveMood;
export const mood = deriveMood;
export function deriveMoods(encounter: unknown, keywords: unknown): Mood[] {
  return [deriveMood(encounter, keywords)];
}
export function deriveMoodTags(encounter: unknown, keywords: unknown): Mood[] {
  return [deriveMood(encounter, keywords)];
}
export function isMoodVocabulary(s: unknown): boolean {
  return typeof s === "string" && MOOD_VOCABULARY.indexOf(s) !== -1;
}
