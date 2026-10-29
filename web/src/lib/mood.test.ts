import { describe, it, expect } from "vitest";
import { deriveMood, deriveMoods, deriveMoodTags, isMoodVocabulary, MOOD_VOCABULARY, DEFAULT_MOOD } from "./mood";

describe("mood vocabulary", () => {
  it("contains exactly the four tags", () => {
    expect([...MOOD_VOCABULARY].sort()).toEqual(["calm", "sombre", "tense", "triumphant"].sort());
  });
  it("every tag reachable", () => {
    expect(deriveMood({ active: true }, [])).toBe("tense");
    expect(deriveMood({ active: false, hp: { party: 1, enemy: 0 } }, [])).toBe("triumphant");
    expect(deriveMood(null, ["death"])).toBe("sombre");
    expect(deriveMood(null, [])).toBe("calm");
  });
});

describe("mood derivation", () => {
  it("maps fixture sessions", () => {
    const fixtures = [
      { enc: { active: true, round: 2 }, kw: ["attack"], want: "tense" },
      { enc: { active: false, hp: { party: 1, enemy: 0 } }, kw: [], want: "triumphant" },
      { enc: null, kw: ["mourning"], want: "sombre" },
      { enc: null, kw: [], want: "calm" },
    ];
    for (const f of fixtures) {
      expect(deriveMood(f.enc, f.kw)).toBe(f.want);
    }
  });
  it("unknown input maps to documented default", () => {
    expect(deriveMood("garbage", "more garbage")).toBe(DEFAULT_MOOD);
    expect(deriveMood(null, null)).toBe(DEFAULT_MOOD);
    expect(deriveMood({}, [])).toBe(DEFAULT_MOOD);
  });
  it("never returns empty", () => {
    expect(deriveMood(null, [])).not.toBe("");
  });
  it("hostile strings do not leak", () => {
    const got = deriveMood({ active: "nope", round: "abc" }, ["<script>", "'; drop"]);
    expect(got).toBe("calm");
  });
  it("aliases", () => {
    expect(deriveMoods(null, null)).toEqual([DEFAULT_MOOD]);
    expect(deriveMoodTags(null, ["victory"])).toEqual(["triumphant"]);
  });
  it("isMoodVocabulary", () => {
    expect(isMoodVocabulary("tense")).toBe(true);
    expect(isMoodVocabulary("nope")).toBe(false);
  });
});
