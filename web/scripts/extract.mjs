import fs from "node:fs";
import path from "node:path";
import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import { visit } from "unist-util-visit";

function stripCalloutMarker(line) {
  let prefixLen = 0;
  while (prefixLen < line.length) {
    const c = line[prefixLen];
    if (c === " " || c === "\t" || c === ">") { prefixLen++; continue; }
    break;
  }
  const t = line.slice(prefixLen);
  if (!t.startsWith("[!")) return line;
  const end = t.indexOf("]");
  if (end < 0) return line;
  let rest = t.slice(end + 1);
  rest = rest.replace(/^\+/, "").replace(/^-/, "");
  return line.slice(0, prefixLen) + rest;
}

function preprocessSource(src) {
  const lines = src.replace(/\r\n/g, "\n").split("\n");
  const out = [];
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith("^") && trimmed.length > 1 && !/[ \t]/.test(trimmed)) continue;
    out.push(stripCalloutMarker(line));
  }
  return out.join("\n");
}

function isCommunityBlock(lang) {
  const l = (lang || "").trim().toLowerCase();
  return ["ad-statblock", "statblock", "ad-monster", "ad-monster-block", "ad-item"].includes(l);
}

function stripYAMLPrefixes(s) {
  const lines = s.split("\n");
  const out = [];
  for (const line of lines) {
    const trimmed = line.trim();
    const idx = trimmed.indexOf(": ");
    if (idx > 0) {
      const key = trimmed.slice(0, idx);
      if (/^[A-Za-z0-9_-]+$/.test(key)) {
        out.push(trimmed.slice(idx + 2));
        continue;
      }
    }
    out.push(line);
  }
  return out.join("\n");
}

function resolveWikiLinks(s, links) {
  let out = "";
  let i = 0;
  while (i < s.length) {
    if (s[i] === "[" && i + 1 < s.length && s[i + 1] === "[") {
      const end = s.indexOf("]]", i + 2);
      if (end < 0) { out += s[i]; i++; continue; }
      const inner = s.slice(i + 2, end);
      let kind = "wikilink";
      if (out.length > 0 && out.endsWith("!")) {
        out = out.slice(0, -1);
        kind = "embed";
      }
      let target = inner, alias = "";
      const pipe = inner.indexOf("|");
      if (pipe >= 0) { target = inner.slice(0, pipe); alias = inner.slice(pipe + 1); }
      const hash = target.indexOf("#");
      if (hash >= 0) target = target.slice(0, hash);
      target = target.trim();
      alias = alias.trim();
      const text = alias === "" ? target : alias;
      links.push({ Kind: kind, Target: target, Text: text, Resolved: target });
      out += text;
      i = end + 2;
      continue;
    }
    out += s[i];
    i++;
  }
  return out;
}

function writeSeg(state, s) {
  if (s === "") return;
  if (state.out.length > 0) state.out += " ";
  state.out += s;
}

function walk(node, state, links) {
  if (!node) return;
  if (node.type === "code") {
    if (isCommunityBlock(node.lang)) {
      const lines = (node.value || "").split("\n");
      // preprocessSource on the fence content
      let content = preprocessSource((node.value || "") + "\n");
      content = stripYAMLPrefixes(content);
      const sub = { out: "" };
      const subLinks = [];
      const tree = unified().use(remarkParse).use(remarkGfm).parse(content);
      walk(tree, sub, subLinks);
      // append sub links
      links.push(...subLinks);
      writeSeg(state, sub.out);
    }
    return;
  }
  if (node.type === "html") return;
  if (node.type === "thematicBreak") return;
  if (node.type === "image") {
    for (const c of node.children || []) walk(c, state, links);
    return;
  }
  if (node.type === "link") {
    // autolink detection: text equals url
    const txt = (node.children || []).map(c => c.value || "").join("");
    if (txt === node.url) {
      // skip autolink entirely
      return;
    }
    for (const c of node.children || []) walk(c, state, links);
    links.push({ Kind: "markdown", Target: node.url, Text: txt.trim() });
    return;
  }
  if (node.type === "text") {
    writeSeg(state, resolveWikiLinks(node.value || "", links));
    return;
  }
  if (node.type === "strong" || node.type === "emphasis" || node.type === "delete" || node.type === "inlineCode") {
    if (node.type === "inlineCode") {
      writeSeg(state, node.value || "");
      return;
    }
    for (const c of node.children || []) walk(c, state, links);
    return;
  }
  if (node.children) {
    for (const c of node.children) walk(c, state, links);
  }
}

function extract(body) {
  const preprocessed = preprocessSource(body);
  const state = { out: "" };
  const links = [];
  const tree = unified().use(remarkParse).use(remarkGfm).parse(preprocessed);
  walk(tree, state, links);
  return { text: state.out, links };
}

const root = process.argv[2] || "fixtures";
const files = [];
function walkDir(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walkDir(p);
    else if (e.name.endsWith(".md")) files.push(p);
  }
}
walkDir(root);

const out = {};
for (const f of files) {
  const raw = fs.readFileSync(f, "utf8");
  let body = raw;
  if (body.startsWith("---")) {
    const m = body.match(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/);
    if (m) body = body.slice(m[0].length);
  }
  out[f] = extract(body);
}
process.stdout.write(JSON.stringify(out));
