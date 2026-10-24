import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import { visit } from "unist-util-visit";
import { toString } from "mdast-util-to-string";
import fs from "node:fs";
import path from "node:path";

const root = process.argv[2] || "fixtures";
const files = [];
function walk(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p);
    else if (e.name.endsWith(".md")) files.push(p);
  }
}
walk(root);

const out = {};
for (const f of files) {
  const raw = fs.readFileSync(f, "utf8");
  let body = raw;
  if (body.startsWith("---")) {
    const m = body.match(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/);
    if (m) body = body.slice(m[0].length);
  }
  const tree = unified().use(remarkParse).use(remarkGfm).parse(body);
  const text = toString(tree);
  const links = [];
  visit(tree, "link", (node) => {
    links.push({ Kind: "markdown", Target: node.url, Text: toString(node) });
  });
  out[f] = { text, links };
}
process.stdout.write(JSON.stringify(out));
