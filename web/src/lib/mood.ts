// Mood is SERVER-COMPUTED (P4 rework). This module no longer derives a mood
// from encounter state; it only carries the fixed vocabulary and a
// fail-closed fallback for display code that cannot reach the server.
// The canonical mapping lives in internal/mood (Go). Any client-side
// fallback MUST return the documented default (`calm`) and MUST NOT
// evaluate the mapping.

export const MOOD_TENSE = "tense";
export const MOOD_TRIUMPHANT = "triumphant";
export const MOOD_SOMBRE = "sombre";
export const MOOD_CALM = "calm";
export const DEFAULT_MOOD = MOOD_CALM;
export const MOOD_VOCABULARY = [MOOD_TENSE, MOOD_TRIUMPHANT, MOOD_SOMBRE, MOOD_CALM];

export type Mood = string;

export interface EncounterState {
  [key: string]: unknown;
}

// Fail-closed fallback: never evaluate the mapping client-side.
export function deriveMood(_encounter: unknown, _keywords: unknown): Mood {
  return DEFAULT_MOOD;
}
export const deriveMoodTag = deriveMood;
export const mood = deriveMood;
export function deriveMoods(_encounter: unknown, _keywords: unknown): Mood[] {
  return [DEFAULT_MOOD];
}
export function deriveMoodTags(_encounter: unknown, _keywords: unknown): Mood[] {
  return [DEFAULT_MOOD];
}
export function isMoodVocabulary(s: unknown): boolean {
  return typeof s === "string" && MOOD_VOCABULARY.indexOf(s) !== -1;
}
