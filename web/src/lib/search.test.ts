/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi } from "vitest";
import { freeTextToO2 } from "./api";

describe("B3 free-text to O2", () => {
  it("translates plain text", () => {
    expect(freeTextToO2("salt")).toBe('name contains "salt" or body contains "salt"');
  });

  it("escapes quotes", () => {
    expect(freeTextToO2('a"b')).toBe('name contains "a\\"b" or body contains "a\\"b"');
  });

  it("empty -> empty", () => {
    expect(freeTextToO2("")).toBe("");
  });
});

describe("B3 loadSearch request shaping", () => {
  it("sends a valid O2 expression, not raw text", async () => {
    const { loadSearch } = await import("./search");
    const fetchMock = vi.fn(async () => ({
      ok: true,
      json: async () => ({ items: [], count: 0 }),
    })) as unknown as typeof fetch;
    await loadSearch("salt", fetchMock as any);
    const url = (fetchMock as any).mock.calls[0][0] as string;
    expect(url).toContain("q=name+contains");
    expect(url).not.toContain("q=salt");
  });
});
