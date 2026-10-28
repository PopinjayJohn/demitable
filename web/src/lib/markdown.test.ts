import { describe, expect, it } from "vitest";
import { renderMarkdown, stripFrontmatter } from "./markdown";

describe("stripFrontmatter", () => {
  it("removes a leading frontmatter block", () => {
    expect(stripFrontmatter("---\nuid: x\n---\n# Hi")).toBe("# Hi");
  });
  it("leaves body text alone", () => {
    expect(stripFrontmatter("# Hi\n---\nrule")).toBe("# Hi\n---\nrule");
  });
});

describe("renderMarkdown", () => {
  it("renders GFM to HTML", () => {
    const html = renderMarkdown("# Title\n\n**bold** and a table:\n\n| a | b |\n|---|---|\n| 1 | 2 |\n");
    expect(html).toContain("<h1>");
    expect(html).toContain("<strong>bold</strong>");
    expect(html).toContain("<table>");
  });
  it("never includes frontmatter", () => {
    const html = renderMarkdown("---\nuid: x\nvisibility: gm\n---\nBody text");
    expect(html).not.toContain("uid");
    expect(html).toContain("Body text");
  });
  it("styles secret blocks and extracts the block id", () => {
    const html = renderMarkdown("> [!secret] hidden words ^blk1\n");
    expect(html).toContain("secret-block");
    expect(html).toContain('data-block-id="blk1"');
    expect(html).not.toContain("[!secret]");
    expect(html).toContain("hidden words");
  });
  it("strips scripts (sanitised)", () => {
    const html = renderMarkdown('hello <script>alert("x")</script>');
    expect(html).not.toContain("<script>");
  });
  it("resolves wikilinks when a resolver is given", () => {
    const html = renderMarkdown("See [[Ilia Vane]] and [[Nowhere]].", {
      resolveWiki: (n) => (n === "Ilia Vane" ? "u1" : null),
    });
    expect(html).toContain('data-uid="u1"');
    expect(html).toContain("dangling");
  });
  it("marks already-revealed secrets with the shared outline", () => {
    const html = renderMarkdown("> [!secret] hidden words ^blk1\n", { revealed: ["blk1"] });
    expect(html).toContain("secret--revealed");
    expect(html).toContain('data-block-id="blk1"');
    const html2 = renderMarkdown("> [!secret] hidden words ^blk1\n");
    expect(html2).not.toContain("secret--revealed");
  });
  it("adds a hover reveal button in GM view", () => {
    const html = renderMarkdown("> [!secret] hidden words ^blk1\n", { revealButtons: true });
    expect(html).toContain("secret-reveal");
    expect(html).toContain('data-reveal-block="blk1"');
    const html2 = renderMarkdown("> [!secret] hidden words ^blk1\n", { revealButtons: true, revealed: ["blk1"] });
    expect(html2).not.toContain("secret-reveal");
  });
  it("marks bare block ids", () => {
    expect(renderMarkdown("^abc123\n")).toContain("block-ref");
  });
  it("renders admonition fences with a title", () => {
    const html = renderMarkdown("```ad-tip\ntitle: Running it\nBody text\n```\n");
    expect(html).toContain("admonition");
    expect(html).toContain("Running it");
  });
});
