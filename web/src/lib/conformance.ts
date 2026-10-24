export interface ExtractedLink {
  Kind: string;
  Target: string;
  Text: string;
}

export interface ExtractedDoc {
  text: string;
  links: ExtractedLink[];
}

function stripCalloutMarker(line: string): string {
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

export function preprocessSource(src: string): string {
  const lines = src.replace(/\r\n/g, "\n").split("\n");
  const out: string[] = [];
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith("^") && trimmed.length > 1 && !/[ \t]/.test(trimmed)) continue;
    out.push(stripCalloutMarker(line));
  }
  return out.join("\n");
}

function isCommunityBlock(lang: string | null | undefined): boolean {
  const l = (lang || "").trim().toLowerCase();
  return ["ad-statblock", "statblock", "ad-monster", "ad-monster-block", "ad-item"].includes(l);
}

function stripYAMLPrefixes(s: string): string {
  const lines = s.split("\n");
  const out: string[] = [];
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

function resolveWikiLinks(s: string, links: ExtractedLink[]): string {
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
      links.push({ Kind: kind, Target: target, Text: text });
      out += text;
      i = end + 2;
      continue;
    }
    out += s[i];
    i++;
  }
  return out;
}

interface WalkState { out: string }
// In browser/Vite, we import at top instead; the above is fallback. We'll instead use dynamic import.
export async function extractMarkdownAsync(body: string): Promise<ExtractedDoc> {
  const { unified } = await import("unified");
  const { default: remarkParse } = await import("remark-parse");
  const { default: remarkGfm } = await import("remark-gfm");
  const preprocessed = preprocessSource(body);
  const state: WalkState = { out: "" };
  const links: ExtractedLink[] = [];
  const tree = unified().use(remarkParse).use(remarkGfm).parse(preprocessed);
  // walk synchronously using a sync version
  walkSync(tree, state, links, unified, remarkParse, remarkGfm);
  return { text: state.out, links };
}

function walkSync(node: any, state: WalkState, links: ExtractedLink[], unified: any, remarkParse: any, remarkGfm: any): void {
  // same as walk but pass parsers
  if (!node) return;
  if (node.type === "code") {
    if (isCommunityBlock(node.lang)) {
      const content = stripYAMLPrefixes(preprocessSource((node.value || "") + "\n"));
      const sub: WalkState = { out: "" };
      const subLinks: ExtractedLink[] = [];
      const tree = unified().use(remarkParse).use(remarkGfm).parse(content);
      walkSync(tree, sub, subLinks, unified, remarkParse, remarkGfm);
      links.push(...subLinks);
      if (sub.out !== "") {
        if (state.out.length > 0) state.out += " ";
        state.out += sub.out;
      }
    }
    return;
  }
  if (node.type === "html") return;
  if (node.type === "thematicBreak") return;
  if (node.type === "image") {
    for (const c of node.children || []) walkSync(c, state, links, unified, remarkParse, remarkGfm);
    return;
  }
  if (node.type === "link") {
    const txt = (node.children || []).map((c: any) => c.value || "").join("");
    if (txt === node.url) return;
    for (const c of node.children || []) walkSync(c, state, links, unified, remarkParse, remarkGfm);
    links.push({ Kind: "markdown", Target: node.url, Text: txt.trim() });
    return;
  }
  if (node.type === "text") {
    const resolved = resolveWikiLinks(node.value || "", links);
    if (resolved !== "") {
      if (state.out.length > 0) state.out += " ";
      state.out += resolved;
    }
    return;
  }
  if (node.type === "inlineCode") {
    if (node.value) {
      if (state.out.length > 0) state.out += " ";
      state.out += node.value;
    }
    return;
  }
  if (node.children) {
    for (const c of node.children) walkSync(c, state, links, unified, remarkParse, remarkGfm);
  }
}


export async function extractMarkdown(body: string): Promise<ExtractedDoc> {
  const { unified } = await import("unified");
  const { default: remarkParse } = await import("remark-parse");
  const { default: remarkGfm } = await import("remark-gfm");
  const preprocessed = preprocessSource(body);
  const state: WalkState = { out: "" };
  const links: ExtractedLink[] = [];
  const tree = unified().use(remarkParse).use(remarkGfm).parse(preprocessed);
  walkSync(tree, state, links, unified, remarkParse, remarkGfm);
  return { text: state.out, links };
}
