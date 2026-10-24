import { describe, it, expect } from "vitest";
import { preprocessSource } from "./conformance";

describe("conformance preprocessing", () => {
  it("strips callout markers", () => {
    const out = preprocessSource("> [!note] Title\n> body\n");
    expect(out).toContain("Title");
    expect(out).not.toContain("[!note]");
  });

  it("skips block references", () => {
    const out = preprocessSource("text\n^id\n");
    expect(out).not.toContain("^id");
  });
});
