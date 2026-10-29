import { describe, it, expect } from "vitest";
import { deriveMood, deriveMoods, deriveMoodTags, isMoodVocabulary, MOOD_VOCABULARY, DEFAULT_MOOD } from "./mood";

describe("mood vocabulary", () => {
  it("contains exactly the four tags", () => {
    expect([...MOOD_VOCABULARY].sort()).toEqual(["calm", "sombre", "tense", "triumphant"].sort());
  });
});

describe("client mood fallback is fail-closed", () => {
  it("never evaluates the mapping", () => {
    expect(deriveMood({ active: true }, [])).toBe(DEFAULT_MOOD);
    expect(deriveMood(null, ["death"])).toBe(DEFAULT_MOOD);
    expect(deriveMood("garbage", "more garbage")).toBe(DEFAULT_MOOD);
    expect(deriveMood(null, null)).toBe(DEFAULT_MOOD);
    expect(deriveMoods(null, null)).toEqual([DEFAULT_MOOD]);
    expect(deriveMoodTags(null, ["victory"])).toEqual([DEFAULT_MOOD]);
  });
  it("isMoodVocabulary", () => {
    expect(isMoodVocabulary("tense")).toBe(true);
    expect(isMoodVocabulary("nope")).toBe(false);
  });
});
