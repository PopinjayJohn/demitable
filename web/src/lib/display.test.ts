import { describe, expect, it } from "vitest";
import { displayName, parseHomeLink } from "./display";

describe("displayName", () => {
  it("strips wikilinks, keeping aliases", () => {
    expect(displayName("[[The Salt Debt]]")).toBe("The Salt Debt");
    expect(displayName("[[The Salt Debt|Salt]]")).toBe("Salt");
    expect(displayName("plain")).toBe("plain");
  });
});

describe("parseHomeLink", () => {
  it("reads a home: link from raw frontmatter", () => {
    expect(parseHomeLink('---\ntype: campaign\nhome: "[[Saltmere]]"\n---\nbody')).toBe("Saltmere");
  });
  it("accepts unquoted and aliased links", () => {
    expect(parseHomeLink("---\nhome: [[Saltmere]]\n---\n")).toBe("Saltmere");
    expect(parseHomeLink('---\nhome: "[[Saltmere|Home]]"\n---\n')).toBe("Saltmere");
  });
  it("returns null without the field, without frontmatter, or malformed", () => {
    expect(parseHomeLink("---\ntype: campaign\n---\nbody")).toBeNull();
    expect(parseHomeLink("no frontmatter here")).toBeNull();
    expect(parseHomeLink("---\nhome: just a string\n---\n")).toBeNull();
    expect(parseHomeLink("---\nhome: \"[[ ]]\"\n---\n")).toBeNull();
  });
});
