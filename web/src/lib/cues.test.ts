import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  isLocalCueSrc, cueBasename, cuesForMood, selectCue, projectCueForDisplay,
  cueSelectableBy, selectCueGuarded, createPlayback, type Cue,
} from "./cues";

describe("cue locality", () => {
  it("accepts local paths", () => {
    expect(isLocalCueSrc("/asset/music/tense.mp3")).toBe(true);
    expect(isLocalCueSrc("music/tense.mp3")).toBe(true);
  });
  it("rejects remote URLs", () => {
    expect(isLocalCueSrc("https://example.com/x.mp3")).toBe(false);
    expect(isLocalCueSrc("http://example.com/x.mp3")).toBe(false);
  });
});

describe("cue selection", () => {
  const cues: Cue[] = [
    { mood: "tense", src: "/asset/tense.mp3", title: "GM secret title" },
    { mood: "calm", src: "/asset/calm.mp3" },
  ];
  it("filters by mood", () => {
    expect(cuesForMood(cues, "tense").length).toBe(1);
    expect(selectCue(cues, "tense")?.src).toBe("/asset/tense.mp3");
  });
  it("returns null for unknown mood", () => {
    expect(selectCue(cues, "nope")).toBeNull();
  });
});

describe("display projection", () => {
  it("strips GM-chosen title, keeps mood/src/track", () => {
    const cue: Cue = { mood: "tense", src: "/asset/tense.mp3", title: "secret" };
    const d = projectCueForDisplay(cue)!;
    expect(d.mood).toBe("tense");
    expect(d.src).toBe("/asset/tense.mp3");
    expect(d.track).toBe("tense.mp3");
    expect((d as any).title).toBeUndefined();
  });
  it("rejects remote src", () => {
    expect(projectCueForDisplay({ mood: "tense", src: "https://x/y.mp3" })).toBeNull();
  });
});

describe("forged seat", () => {
  const cue: Cue = { mood: "tense", src: "/asset/tense.mp3" };
  it("GM can select", () => {
    expect(cueSelectableBy("gm", cue)).toBe(true);
    expect(() => selectCueGuarded("gm", [cue], "tense")).not.toThrow();
  });
  it("seat is rejected", () => {
    expect(cueSelectableBy("seat", cue)).toBe(false);
    expect(() => selectCueGuarded("seat", [cue], "tense")).toThrow();
  });
  it("display is rejected", () => {
    expect(cueSelectableBy("display", cue)).toBe(false);
  });
});

describe("playback network-down", () => {
  let fetchSpy: any;
  beforeEach(() => {
    fetchSpy = vi.fn(() => { throw new Error("network down"); });
    (globalThis as any).fetch = fetchSpy;
  });
  afterEach(() => {
    delete (globalThis as any).fetch;
  });
  it("does not call fetch", () => {
    const audio = { src: "", play: vi.fn(), pause: vi.fn(), removeAttribute: vi.fn() } as any;
    const handle = createPlayback(() => audio);
    const cue: Cue = { mood: "tense", src: "/asset/tense.mp3" };
    handle.play(cue);
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(audio.src).toBe("/asset/tense.mp3");
    expect(handle.current).toBe(cue);
  });
  it("stop clears", () => {
    const audio = { src: "", play: vi.fn(), pause: vi.fn(), removeAttribute: vi.fn() } as any;
    const handle = createPlayback(() => audio);
    handle.play({ mood: "calm", src: "/asset/calm.mp3" });
    handle.stop();
    expect(handle.current).toBeNull();
  });
});
