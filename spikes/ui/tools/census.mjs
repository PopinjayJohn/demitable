#!/usr/bin/env node
// The corpus census — question 4.
//
// Walks every markdown file in the corpus and counts every construct the
// renderer has to handle. The numbers in `spikes/ui/census.md` come from this
// script; re-run it to check them.
//
//   node tools/census.mjs [--root ../../fixtures] [--json]
//
// The corpus is a test corpus and is gitignored. This script reads it; it
// never writes to it and never copies it into the spike.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, extname } from "node:path";

const argv = process.argv.slice(2);
const arg = (name, fallback) => {
  const i = argv.indexOf(name);
  return i >= 0 && argv[i + 1] ? argv[i + 1] : fallback;
};
const ROOT = arg("--root", "../../fixtures");
const AS_JSON = argv.includes("--json");

// ---------------------------------------------------------------- file walk

function walk(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    const p = join(dir, entry);
    const st = statSync(p);
    if (st.isDirectory()) walk(p, out);
    else if (extname(p) === ".md") out.push(p);
  }
  return out;
}

/** Split frontmatter from body. Returns { frontmatter, body }. */
function split(text) {
  if (!text.startsWith("---")) return { frontmatter: null, body: text };
  const lines = text.split("\n");
  for (let i = 1; i < lines.length; i++) {
    if (lines[i].trim() === "---") {
      return { frontmatter: lines.slice(1, i).join("\n"), body: lines.slice(i + 1).join("\n") };
    }
  }
  return { frontmatter: null, body: text };
}

const files = walk(ROOT).sort();
const docs = files.map((path) => {
  const text = readFileSync(path, "utf8");
  const { frontmatter, body } = split(text);
  return { path, rel: relative(ROOT, path), frontmatter, body };
});

// ------------------------------------------------------------------ counters

/** count(pattern, body) -> { n, files:Set } */
function counter() {
  return { n: 0, files: new Set() };
}
function bump(c, doc, k = 1) {
  c.n += k;
  c.files.add(doc.rel);
}

const T = {
  // container
  fence: counter(),
  fenceInfo: new Map(), // info string -> count
  statblock: counter(),
  statblockLayouts: new Map(),
  callout: counter(),
  calloutType: new Map(),
  calloutFold: new Map(),
  blockRef: counter(),
  blockRefId: new Map(),
  blockRefAnchor: counter(), // what the ^id is attached to
  rawHtml: counter(),
  rawHtmlTags: new Map(),
  table: counter(),
  tableCols: new Map(),
  tableAlign: new Map(),
  link: counter(),
  linkDest: new Map(),
  linkTitle: counter(),
  linkAnchor: counter(),
  image: counter(),
  wikilink: counter(),
  embed: counter(),
  task: counter(),
  ordered: counter(),
  bullet: counter(),
  heading: counter(),
  blockquote: counter(),
  inlineCode: counter(),
  bold: counter(),
  italic: counter(),
  boldItalic: counter(),
  entity: counter(),
  hardBreak: counter(),
  highlight: counter(),
  math: counter(),
  obsidianComment: counter(),
  htmlComment: counter(),
  thematicBreak: counter(),
  definitionList: counter(),
  // frontmatter
  fmKeys: new Map(),
  fmCombos: new Map(),
  cssClass: new Map(),
  obsidianUiMode: new Map(),
  tag: new Map(),
};

const FM_KEY = /^([A-Za-z_][\w-]*):\s*(.*)$/;

for (const doc of docs) {
  const b = doc.body;

  // --- fenced code blocks -------------------------------------------------
  const fenceRe = /^([ \t]*)(`{3,}|~{3,})[ \t]*(.*)$/gm;
  const fences = [];
  let m;
  while ((m = fenceRe.exec(b)) !== null) {
    fences.push({ indent: m[1].length, marker: m[2], info: m[3].trim(), start: m.index });
  }
  // pair them up: an info-bearing fence opens a block, the next bare fence closes
  const stack = [];
  const blocks = [];
  for (const f of fences) {
    if (stack.length === 0) {
      stack.push(f);
    } else if (!f.info && f.marker === stack[0].marker) {
      const open = stack.pop();
      blocks.push({ open, close: f, body: b.slice(open.start + open.marker.length + open.indent, f.start) });
    } else {
      // an info-bearing fence inside an open block: nested fence, ignore
    }
  }
  bump(T.fence, doc, fences.length);
  for (const f of fences) {
    if (f.info) T.fenceInfo.set(f.info, (T.fenceInfo.get(f.info) ?? 0) + 1);
  }
  for (const blk of blocks) {
    if (blk.open.info === "ad-statblock") {
      bump(T.statblock, doc);
      const sig = layoutSignature(blk.body);
      T.statblockLayouts.set(sig, (T.statblockLayouts.get(sig) ?? 0) + 1);
    }
  }

  // --- callouts -----------------------------------------------------------
  const callRe = /^([ \t]*)>[ \t]*\[!([^\]\n]*)\]([+-]?)[ \t]*(.*)$/gm;
  while ((m = callRe.exec(b)) !== null) {
    bump(T.callout, doc);
    T.calloutType.set(m[2], (T.calloutType.get(m[2]) ?? 0) + 1);
    const fold = m[3] || "none";
    T.calloutFold.set(fold, (T.calloutFold.get(fold) ?? 0) + 1);
  }

  // --- block references ---------------------------------------------------
  const brefRe = /\^([A-Za-z0-9][\w-]*)/g;
  while ((m = brefRe.exec(b)) !== null) {
    bump(T.blockRef, doc);
    T.blockRefId.set(m[1], (T.blockRefId.get(m[1]) ?? 0) + 1);
  }

  // --- links, images ------------------------------------------------------
  const linkRe = /(!?)\[((?:[^\]\\]|\\.)*)\]\(\s*(<[^>]*>|[^\s)]*)(?:\s+("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|\([^)]*\)))?\s*\)/g;
  while ((m = linkRe.exec(b)) !== null) {
    const isImg = m[1] === "!";
    let dest = m[3];
    if (dest.startsWith("<") && dest.endsWith(">")) dest = dest.slice(1, -1);
    const hasTitle = Boolean(m[4]);
    if (isImg) {
      bump(T.image, doc);
    } else {
      bump(T.link, doc);
      if (hasTitle) bump(T.linkTitle, doc);
      const kind =
        dest.startsWith("http") ? "http" :
        dest.startsWith("#") ? "fragment" :
        /\.md(#|$)/.test(dest) ? "relative.md" : "other";
      T.linkDest.set(kind, (T.linkDest.get(kind) ?? 0) + 1);
      if (dest.includes("#")) bump(T.linkAnchor, doc);
    }
  }

  // --- wikilinks ----------------------------------------------------------
  const wlRe = /(!?)\[\[([^\]\n]*)\]\]/g;
  while ((m = wlRe.exec(b)) !== null) {
    if (m[1] === "!") bump(T.embed, doc);
    else bump(T.wikilink, doc);
  }

  // --- lists --------------------------------------------------------------
  for (const line of b.split("\n")) {
    if (/^\s*[-*+] \[[ xX]\]/.test(line)) bump(T.task, doc);
    if (/^\s*\d+[.)] /.test(line)) bump(T.ordered, doc);
    if (/^\s*[-*+] /.test(line)) bump(T.bullet, doc);
    if (/^#{1,6} /.test(line)) bump(T.heading, doc);
    if (/^\s*>/.test(line)) bump(T.blockquote, doc);
    if (/^\s*([-*_])(\s*\1){2,}\s*$/.test(line)) bump(T.thematicBreak, doc);
  }

  // --- tables -------------------------------------------------------------
  const lines = b.split("\n");
  for (let i = 0; i < lines.length - 1; i++) {
    const hdr = lines[i];
    const sep = lines[i + 1];
    if (!hdr.trimStart().startsWith("|")) continue;
    if (!/^[ \t]*\|?[-: |]+\|?[ \t]*$/.test(sep)) continue;
    bump(T.table, doc);
    const cols = hdr.trim().replace(/^\||\|$/g, "").split("|").length;
    T.tableCols.set(cols, (T.tableCols.get(cols) ?? 0) + 1);
    const align = sep
      .trim().replace(/^\||\|$/g, "").split("|")
      .map((p) => {
        p = p.trim();
        if (p.startsWith(":") && p.endsWith(":")) return "C";
        if (p.endsWith(":")) return "R";
        if (p.startsWith(":")) return "L";
        return "N";
      }).join("");
    T.tableAlign.set(align, (T.tableAlign.get(align) ?? 0) + 1);
    i++;
  }

  // --- inline -------------------------------------------------------------
  const inlineCount = (re) => {
    const ms = b.match(re);
    return ms ? ms.length : 0;
  };
  const addInline = (key, re) => {
    const n = inlineCount(re);
    if (n) bump(T[key], doc, n);
  };
  addInline("inlineCode", /`[^`\n]+`/g);
  addInline("bold", /\*\*[^*\n]+\*\*/g);
  addInline("boldItalic", /\*\*\*[^*\n]+\*\*\*/g);
  addInline("italic", /(?<!\*)\*[^*\n]+\*(?!\*)/g);
  addInline("entity", /&[a-zA-Z]+;|&#\d+;/g);
  addInline("hardBreak", /  \n/g);
  addInline("highlight", /==[^=]+==/g);
  addInline("math", /\$\$|\$[^$\n]+\$/g);
  addInline("obsidianComment", /%%[\s\S]*?%%/g);
  addInline("htmlComment", /<!--[\s\S]*?-->/g);
  addInline("definitionList", /^\s*:\s+\S.*$/gm);

  // --- raw html -----------------------------------------------------------
  const tagRe = /<\/?([a-zA-Z][\w-]*)/g;
  while ((m = tagRe.exec(b)) !== null) {
    bump(T.rawHtml, doc);
    T.rawHtmlTags.set(m[1].toLowerCase(), (T.rawHtmlTags.get(m[1].toLowerCase()) ?? 0) + 1);
  }

  // --- frontmatter --------------------------------------------------------
  if (doc.frontmatter != null) {
    const keys = [];
    let key = null;
    for (const line of doc.frontmatter.split("\n")) {
      const km = line.match(FM_KEY);
      if (km && !/^\s/.test(line)) {
        key = km[1];
        keys.push(key);
        T.fmKeys.set(key, (T.fmKeys.get(key) ?? 0) + 1);
        if (key === "obsidianUIMode") {
          T.obsidianUiMode.set(km[2].trim(), (T.obsidianUiMode.get(km[2].trim()) ?? 0) + 1);
        }
        continue;
      }
      const im = line.match(/^\s*-\s*(.*)$/);
      if (im && key) {
        const v = im[1].trim().replace(/^["']|["']$/g, "");
        if (key === "cssclasses") T.cssClass.set(v, (T.cssClass.get(v) ?? 0) + 1);
        if (key === "tags") T.tag.set(v, (T.tag.get(v) ?? 0) + 1);
      } else if (line.trim() === "") {
        key = null;
      }
    }
    const combo = keys.join(",");
    T.fmCombos.set(combo, (T.fmCombos.get(combo) ?? 0) + 1);
  }
}

/** The shape of a stat block body: the sequence of bold labels, plus features. */
function layoutSignature(body) {
  const labels = [];
  const feats = new Set();
  for (const line of body.split("\n")) {
    const lm = line.match(/^\s*[-*]\s+\*\*(.+?)\*\*/);
    if (lm) labels.push(lm[1].trim());
    if (/^\s*\|/.test(line)) feats.add("table");
    if (/^\s*!\[/.test(line)) feats.add("image");
    if (/^\s*>/.test(line)) feats.add("quote");
    if (/^#{1,6}\s/.test(line)) feats.add("heading");
    if (/^\s*\d+\./.test(line)) feats.add("ordered");
  }
  return JSON.stringify({ labels, feats: [...feats].sort() });
}

// ------------------------------------------------------------------- output

const sorted = (map) => [...map.entries()].sort((a, b) => b[1] - a[1]);
const shape = (c) => ({ occurrences: c.n, files: c.files.size });

if (AS_JSON) {
  const out = {
    root: ROOT,
    files: files.length,
    constructs: {
      fence: shape(T.fence),
      statblock: shape(T.statblock),
      callout: shape(T.callout),
      blockRef: shape(T.blockRef),
      table: shape(T.table),
      rawHtml: shape(T.rawHtml),
      link: shape(T.link),
      image: shape(T.image),
      wikilink: shape(T.wikilink),
      embed: shape(T.embed),
      task: shape(T.task),
      ordered: shape(T.ordered),
      bullet: shape(T.bullet),
      heading: shape(T.heading),
      blockquote: shape(T.blockquote),
      inlineCode: shape(T.inlineCode),
      bold: shape(T.bold),
      italic: shape(T.italic),
      boldItalic: shape(T.boldItalic),
      entity: shape(T.entity),
      hardBreak: shape(T.hardBreak),
      highlight: shape(T.highlight),
      math: shape(T.math),
      obsidianComment: shape(T.obsidianComment),
      htmlComment: shape(T.htmlComment),
      thematicBreak: shape(T.thematicBreak),
      definitionList: shape(T.definitionList),
      linkTitle: shape(T.linkTitle),
      linkAnchor: shape(T.linkAnchor),
    },
    fenceInfo: Object.fromEntries(sorted(T.fenceInfo)),
    calloutTypes: Object.fromEntries(sorted(T.calloutType)),
    calloutFold: Object.fromEntries(sorted(T.calloutFold)),
    blockRefIds: Object.fromEntries(sorted(T.blockRefId)),
    statblockLayouts: Object.fromEntries(
      [...T.statblockLayouts.entries()].sort((a, b) => b[1] - a[1]).map(([k, v]) => [k, v]),
    ),
    tableCols: Object.fromEntries(sorted(T.tableCols)),
    tableAlign: Object.fromEntries(sorted(T.tableAlign)),
    linkDest: Object.fromEntries(sorted(T.linkDest)),
    rawHtmlTags: Object.fromEntries(sorted(T.rawHtmlTags)),
    frontmatterKeys: Object.fromEntries(sorted(T.fmKeys)),
    frontmatterCombos: Object.fromEntries(sorted(T.fmCombos)),
    cssClasses: Object.fromEntries(sorted(T.cssClass)),
    obsidianUiMode: Object.fromEntries(sorted(T.obsidianUiMode)),
    distinctTags: T.tag.size,
  };
  console.log(JSON.stringify(out, null, 2));
  process.exit(0);
}

const row = (label, c) =>
  console.log(`| ${label} | ${c.n.toLocaleString()} | ${c.files.size.toLocaleString()} |`);

console.log(`# Corpus census — ${files.length} markdown files under ${ROOT}\n`);
console.log("| Construct | Occurrences | Files |");
console.log("|---|---:|---:|");
for (const [label, key] of [
  ["Fenced code blocks (all)", "fence"],
  ["`ad-statblock` blocks", "statblock"],
  ["Obsidian callouts", "callout"],
  ["Block references `^id`", "blockRef"],
  ["Markdown tables", "table"],
  ["Raw HTML tags", "rawHtml"],
  ["Markdown links", "link"],
  ["  …with a title attribute", "linkTitle"],
  ["  …with an anchor", "linkAnchor"],
  ["Images", "image"],
  ["Wikilinks `[[…]]`", "wikilink"],
  ["Embeds `![[…]]`", "embed"],
  ["Task list items", "task"],
  ["Ordered list items", "ordered"],
  ["Bullet list items", "bullet"],
  ["Headings", "heading"],
  ["Blockquote lines", "blockquote"],
  ["Inline code spans", "inlineCode"],
  ["Bold spans", "bold"],
  ["Italic spans", "italic"],
  ["Bold-italic spans", "boldItalic"],
  ["HTML entities", "entity"],
  ["Two-space hard breaks", "hardBreak"],
  ["`==highlight==`", "highlight"],
  ["Math `$…$`", "math"],
  ["`%%obsidian comment%%`", "obsidianComment"],
  ["HTML comments", "htmlComment"],
  ["Thematic breaks", "thematicBreak"],
]) {
  row(label, T[key]);
}

console.log("\n## Fence info strings\n");
for (const [k, v] of sorted(T.fenceInfo)) console.log(`- \`${k}\` — ${v}`);

console.log("\n## Callout types\n");
for (const [k, v] of sorted(T.calloutType)) console.log(`- \`[!${k}]\` — ${v}`);
console.log("\n## Callout fold markers\n");
for (const [k, v] of sorted(T.calloutFold)) console.log(`- \`${k}\` — ${v}`);

console.log(`\n## Block references\n`);
console.log(`- ${T.blockRef.n} occurrences, ${T.blockRefId.size} distinct ids`);
console.log(`- \`statblock\` — ${T.blockRefId.get("statblock") ?? 0}`);

console.log(`\n## Stat block layouts — ${T.statblockLayouts.size} distinct\n`);
for (const [k, v] of [...T.statblockLayouts.entries()].sort((a, b) => b[1] - a[1])) {
  const sig = JSON.parse(k);
  console.log(`- **${v}** — \`${sig.labels.join(" › ")}\` [${sig.feats.join(", ")}]`);
}

console.log(`\n## Table shapes\n`);
for (const [k, v] of sorted(T.tableCols)) console.log(`- ${k} columns — ${v}`);
console.log("\n### Alignment\n");
for (const [k, v] of sorted(T.tableAlign)) console.log(`- \`${k}\` — ${v}`);

console.log(`\n## Link destinations\n`);
for (const [k, v] of sorted(T.linkDest)) console.log(`- ${k} — ${v}`);

console.log(`\n## Raw HTML tags\n`);
for (const [k, v] of sorted(T.rawHtmlTags)) console.log(`- \`${k}\` — ${v}`);

console.log(`\n## Frontmatter\n`);
for (const [k, v] of sorted(T.fmKeys)) console.log(`- \`${k}\` — ${v}`);
console.log("\n### Key combinations\n");
for (const [k, v] of sorted(T.fmCombos)) console.log(`- \`${k || "(none)"}\` — ${v}`);
console.log("\n### cssclasses\n");
for (const [k, v] of sorted(T.cssClass)) console.log(`- \`${k}\` — ${v}`);
console.log("\n### obsidianUIMode\n");
for (const [k, v] of sorted(T.obsidianUiMode)) console.log(`- \`${k}\` — ${v}`);
console.log(`\n- distinct tags — ${T.tag.size}`);
