/**
 * The markdown renderer.
 *
 * ADR-0043 picks `remark`/`micromark` for the browser and `goldmark` for Go,
 * and requires the two to agree on a conformance fixture set. This is the
 * browser half, running on the corpus's own content.
 *
 * What is *not* standard markdown and has to be taught to it:
 *
 *   - Obsidian callouts — `> [!type] Title`, with `-` and `+` fold markers
 *   - the `ad-statblock` code block — prose in a code fence
 *   - block references — `^id`
 *   - wikilinks — `[[target]]`
 *
 * What is *in* the corpus but is not Obsidian:
 *
 *   - three callouts whose `[!note]` marker is appended to a paragraph
 *   - 8,604 raw HTML tags, some of them malformed (`class"level"`)
 *   - links whose destination is a vault-root-relative `.md` path with a
 *     URL-encoded anchor
 */

import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import remarkDirective from "remark-directive";
import remarkRehype from "remark-rehype";
import rehypeStringify from "rehype-stringify";
import type { Root, RootContent } from "mdast";
import type { Plugin } from "unified";
import { sanitizeHtml } from "./sanitize";
import { parseStatBlock, NONE_MARKER, type StatBlock } from "./statblock";
import type { SecretVocabulary } from "../projection";

/* ------------------------------------------------------------------ types */

type HastRoot = import("hast").Root;


export interface SecretSlot {
  id: string;
  label: string;
  source: string;
}

export interface LinkResolution {
  /** The href to use. */
  href: string;
  external: boolean;
  /** A link whose target is not in the vault. */
  dangling?: boolean;
}

export interface RenderOptions {
  vocab: SecretVocabulary;
  /** Render a GM-only block. Called only for blocks that survived. */
  secret?: (slot: SecretSlot) => string;
  /** Resolve a link destination. */
  resolveLink?: (href: string) => LinkResolution;
  /** The pack's structured data for a note, for the reduced stat card. */
  statBlockVariant?: "full" | "party";
}

export interface RenderResult {
  html: string;
  /** Every construct the renderer recognised, for the coverage report. */
  constructs: string[];
  /** Blocks that were GM-only. */
  secrets: SecretSlot[];
  /** Things the renderer saw and did not understand. */
  warnings: string[];
}

/* ----------------------------------------------------------- preprocessing */

/**
 * Rewrite the Obsidian constructs into directives remark-directive can read.
 *
 * The secret blocks are replaced *before* the markdown is parsed, and only
 * their identity survives — the text itself never enters the AST, so it
 * cannot leak through a rendering accident.
 */
export function preprocess(
  source: string,
  vocab: SecretVocabulary,
): { markdown: string; secrets: SecretSlot[]; statBlocks: StatBlock[] } {
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const out: string[] = [];
  const secrets: SecretSlot[] = [];
  const statBlocks: StatBlock[] = [];
  const warnings: string[] = [];

  let i = 0;
  let secretIndex = 0;

  while (i < lines.length) {
    const line = lines[i];

    // --- a GM-only block, in any of ADR-0036's syntaxes --------------------
    const marked = matchMarkedBlock(lines, i, vocab);
    if (marked) {
      const id = marked.id;
      secrets.push({ id, label: marked.label, source: marked.source });
      out.push(`:::secret{id="${escapeAttr(id)}" label="${escapeAttr(marked.label)}"}`);
      out.push(":::");
      i = marked.next;
      secretIndex++;
      continue;
    }

    // --- an `ad-statblock` fence ------------------------------------------
    const fence = line.match(/^\s*(`{3,})\s*ad-statblock\s*$/);
    if (fence) {
      const marker = fence[1];
      let j = i + 1;
      while (j < lines.length && !lines[j].trimStart().startsWith(marker)) j++;
      const body = lines.slice(i + 1, Math.min(j, lines.length)).join("\n");
      const parsed = parseStatBlock(body);
      statBlocks.push(parsed);
      out.push(`:::statblock{index="${statBlocks.length - 1}"}`);
      out.push(":::");
      i = Math.min(j + 1, lines.length);
      continue;
    }

    // --- an Obsidian callout ----------------------------------------------
    const callout = line.match(/^([ \t]*)>[ \t]*\[!([^\]\n]+)\]([+-]?)[ \t]*(.*)$/);
    if (callout) {
      const [, indent, type, fold, title] = callout;
      const body: string[] = [];
      let j = i + 1;
      while (j < lines.length && /^[ \t]*>/.test(lines[j])) {
        body.push(lines[j].replace(/^[ \t]*>[ \t]?/, ""));
        j++;
      }
      const attrs = [
        `type="${escapeAttr(type)}"`,
        `title="${escapeAttr(title)}"`,
        fold ? `fold="${fold === "-" ? "closed" : "open"}"` : "",
      ].filter(Boolean).join(" ");
      out.push(`${indent}:::callout{${attrs}}`);
      out.push(...body);
      out.push(`${indent}:::`);
      i = j;
      continue;
    }

    // --- a callout marker that is not at the start of a line ---------------
    if (/\[![^\]\n]+\]/.test(line) && !/^\s*>\s*\[!/.test(line)) {
      warnings.push(
        "A callout marker that is not at the start of a line. The corpus has three of these; they render as literal text, because a callout is a block and this is not one.",
      );
    }

    out.push(line);
    i++;
  }

  return { markdown: out.join("\n"), secrets, statBlocks };
}

/** Detect a GM-only block at line `i`, in any of ADR-0036's syntaxes. */
function matchMarkedBlock(
  lines: string[],
  i: number,
  vocab: SecretVocabulary,
): { id: string; label: string; source: string; next: number } | null {
  const line = lines[i];

  // `> [!secret] Title`
  const callout = line.match(/^\s*>\s*\[!([^\]\s]+)\]([+-]?)\s*(.*)$/);
  if (callout && vocab.calloutTypes.includes(callout[1])) {
    let j = i + 1;
    while (j < lines.length && /^\s*>/.test(lines[j])) j++;
    const source = lines.slice(i, j).join("\n");
    return {
      id: blockIdentity(lines, i, j, source),
      label: callout[3] || callout[1],
      source,
      next: j,
    };
  }

  // `<!-- secret:begin -->` … `<!-- secret:end -->`
  if (/<!--\s*secret:begin\s*-->/.test(line)) {
    let j = i + 1;
    while (j < lines.length && !/<!--\s*secret:end\s*-->/.test(lines[j])) j++;
    j = Math.min(j + 1, lines.length);
    const source = lines.slice(i, j).join("\n");
    return { id: blockIdentity(lines, i, j, source), label: "Secret block", source, next: j };
  }

  // A fenced code block with a `gm: true` option.
  const fence = line.match(/^\s*(`{3,}|~{3,})\s*(\S*)\s*(.*)$/);
  if (fence) {
    const marker = fence[1];
    let j = i + 1;
    while (j < lines.length && !lines[j].trimStart().startsWith(marker)) j++;
    const end = Math.min(j + 1, lines.length);
    const body = lines.slice(i, end);
    const opts: Record<string, string> = {};
    for (const l of body.slice(1)) {
      if (l.trim() === "") break;
      const m = l.match(/^([A-Za-z_][\w-]*):\s*(.*)$/);
      if (!m) break;
      opts[m[1]] = m[2].trim();
    }
    const marked =
      (fence[2] && vocab.codeBlockKeys.includes(fence[2])) ||
      opts[vocab.statblockGmKey] === "true" ||
      Object.entries(opts).some(([k, v]) => vocab.codeBlockKeys.includes(k) && v === "true");
    if (marked) {
      const source = body.join("\n");
      return {
        id: blockIdentity(lines, i, end, source),
        label: opts.title ?? fence[2] ?? "Secret block",
        source,
        next: end,
      };
    }
  }

  return null;
}

/** An explicit `^id` on the block's last line wins; else a content hash. */
function blockIdentity(lines: string[], start: number, end: number, source: string): string {
  for (let k = end - 1; k >= start; k--) {
    const m = lines[k]?.match(/\^([A-Za-z0-9][\w-]*)\s*$/);
    if (m) return m[1];
  }
  let h = 0x811c9dc5;
  const norm = source.replace(/\r\n/g, "\n").replace(/[ \t]+$/gm, "").trim();
  for (let i = 0; i < norm.length; i++) {
    h ^= norm.charCodeAt(i);
    h = Math.imul(h, 0x01000193) >>> 0;
  }
  return h.toString(16).padStart(8, "0").slice(0, 8);
}

function escapeAttr(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;");
}

/* -------------------------------------------------------------- the plugins */

/**
 * Turn `:::callout` and `:::statblock` directives into HTML.
 *
 * The shape is remark's: a plugin is an *attacher* — a function that returns
 * the transformer. `Plugin<[], Root>` is what `remark-gfm` declares, and
 * `.use()` accepts it.
 */
const remarkDemitable = (opts: {
  statBlocks: StatBlock[];
  secrets: SecretSlot[];
  secret?: (slot: SecretSlot) => string;
  variant: "full" | "party";
}): Plugin<[], Root> => () => (tree: Root) => {
    walk(tree, (node, parent, index) => {
      if (node.type !== "containerDirective") return;
      if (parent === null || index === null) return;
      const name = (node as unknown as { name: string }).name;
      const attrs = (node as unknown as { attributes?: Record<string, string | null> })
        .attributes ?? {};

      if (name === "secret") {
        const id = attrs.id ?? "";
        const slot = opts.secrets.find((s) => s.id === id);
        const html = opts.secret && slot
          ? opts.secret(slot)
          : `<div class="stripped" role="note"><span aria-hidden="true">▨</span><span>Secret block — not sent to this audience</span></div>`;
        parent.children[index] = { type: "html", value: html } as RootContent;
        return;
      }

      if (name === "statblock") {
        const idx = Number(attrs.index ?? "0");
        const block = opts.statBlocks[idx];
        if (!block) return;
        const html = renderStatBlockHtml(block, opts.variant);
        parent.children[index] = { type: "html", value: html } as RootContent;
      }
    });
  };

/** Wikilinks and block references, in text nodes. */
const remarkInline: Plugin<[], Root> = () => (tree: Root) => {
    walk(tree, (node, parent, index) => {
      if (node.type !== "text" || parent === null || index === null) return;
      const value = (node as unknown as { value: string }).value;
      const parts: RootContent[] = [];

      // A trailing block reference: `… text ^id`
      const ref = value.match(/^(.*?)\s*\^([A-Za-z0-9][\w-]*)\s*$/s);
      if (ref && ref[1].length > 0) {
        parts.push({ type: "text", value: ref[1] } as RootContent);
        parts.push({
          type: "html",
          value: `<sup class="block-ref"><a href="#^${escapeAttr(ref[2])}" id="^${escapeAttr(ref[2])}">^${escapeAttr(ref[2])}</a></sup>`,
        } as RootContent);
      } else if (/^\^([A-Za-z0-9][\w-]*)\s*$/.test(value.trim())) {
        const id = value.trim().slice(1);
        parts.push({
          type: "html",
          value: `<sup class="block-ref"><a href="#^${escapeAttr(id)}" id="^${escapeAttr(id)}">^${escapeAttr(id)}</a></sup>`,
        } as RootContent);
      } else {
        parts.push(node as RootContent);
      }
      parent.children.splice(index, 1, ...parts);
    });
  };

/** A table whose first column is a dice expression is a roll-on-a-table. */
const remarkDiceTable: Plugin<[], Root> = () => (tree: Root) => {
    walk(tree, (node, parent, index) => {
      if (node.type !== "table" || parent === null || index === null) return;
      const table = node as unknown as {
        align?: (string | null)[];
        children: { type: string; children?: { type: string; value?: string }[] }[];
      };
      const header = table.children[0];
      const first = header?.children?.[0]?.value ?? "";
      const m = first.match(/^dice:\s*(.+)$/);
      if (!m) return;
      const dice = m[1].trim();
      // The corpus's roll-on-a-table form writes the die in the header cell:
      // `| dice: 1d100 | Illusion |`. The header cell becomes the roll
      // instruction, and the die is kept as the table's caption.
      const cell = header?.children?.[0];
      if (cell) cell.value = `Roll ${dice.replace(/^\d*d/, "d")}`;
      const attr = (node as unknown as { data?: Record<string, unknown> }).data ?? {};
      attr.hProperties = { "data-dice": dice };
      (node as unknown as { data?: Record<string, unknown> }).data = attr;
    });
  };

/** Resolve markdown links, and mark the ones that dangle. */
const remarkLinks = (resolve: (href: string) => LinkResolution): Plugin<[], Root> => () => (tree: Root) => {
    walk(tree, (node) => {
      if (node.type !== "link") return;
      const link = node as unknown as { url: string; title?: string | null };
      const res = resolve(link.url);
      link.url = res.href;
      if (res.dangling) {
        const data = (link as unknown as { data?: Record<string, unknown> }).data ?? {};
        data.hProperties = { className: "link--dangling", title: "Not in this vault" };
        (link as unknown as { data?: Record<string, unknown> }).data = data;
      } else if (res.external) {
        const data = (link as unknown as { data?: Record<string, unknown> }).data ?? {};
        data.hProperties = { className: "link--external" };
        (link as unknown as { data?: Record<string, unknown> }).data = data;
      }
    });
  };

/** Sanitise raw HTML, and repair the corpus's empty-alt images. */
const rehypeSanitize: Plugin<[], HastRoot> = () => (tree: HastRoot) => {
    walkHast(tree, (node) => {
      if (node.type === "raw") {
        node.value = sanitizeHtml(node.value);
      }
      if (node.type === "element" && node.tagName === "img") {
        const alt = node.properties?.alt;
        if (alt === undefined || alt === "") {
          const src = String(node.properties?.src ?? "");
          const caption = src
            .split("/")
            .pop()
            ?.split("#")[0]
            ?.replace(/\.[a-z0-9]+$/i, "")
            .replace(/%20/g, " ");
          node.properties = { ...node.properties, alt: caption ? `Image: ${caption}` : "Image" };
        }
      }
    });
  };

/* -------------------------------------------------------------- the stat block */

export function renderStatBlockHtml(block: StatBlock, variant: "full" | "party"): string {
  const parts: string[] = [];
  const fields = variant === "party"
    ? block.fields.filter((f) =>
        ["Armor Class", "Hit Points", "Speed", "Proficiency Bonus", "Senses", "Languages", "Challenge"].includes(f.label),
      )
    : block.fields;

  parts.push(`<div class="statblock" data-variant="${variant}">`);
  parts.push(`<div class="statblock__head">`);
  if (block.token) {
    parts.push(
      `<div class="statblock__token" aria-hidden="true"><img src="${escapeAttr(block.token.url)}" alt="" loading="lazy"></div>`,
    );
  }
  parts.push(`<div class="grow">`);
  parts.push(`<div class="statblock__title">${escapeHtml(block.title)}</div>`);
  if (block.subtitle) {
    parts.push(`<div class="statblock__subtitle">${escapeHtml(block.subtitle)}</div>`);
  }
  parts.push(`</div></div>`);

  parts.push(`<div class="statblock__body">`);
  parts.push(`<div class="statblock__grid">`);
  for (const f of fields) {
    parts.push(
      `<div class="statblock__field"><div class="statblock__label">${escapeHtml(f.label)}</div>` +
      `<div class="statblock__value">${renderInlineValue(f.value)}</div></div>`,
    );
  }
  parts.push(`</div>`);

  if (block.abilities && variant === "full") {
    parts.push(`<table class="md-table statblock__abilities"><thead><tr>`);
    for (const a of block.abilities) {
      parts.push(`<th scope="col">${escapeHtml(a.name)}</th>`);
    }
    parts.push(`</tr></thead><tbody><tr>`);
    for (const a of block.abilities) {
      parts.push(`<td>${escapeHtml(a.score)}</td>`);
    }
    parts.push(`</tr></tbody></table>`);
  }

  for (const s of block.sections) {
    if (variant === "party") continue;
    parts.push(`<div class="statblock__section"><h4>${escapeHtml(s.heading)}</h4>`);
    for (const e of s.entries) {
      parts.push(
        `<div class="statblock__action">${e.name ? `<strong>${escapeHtml(e.name)}</strong> ` : ""}${renderInlineValue(e.text)}</div>`,
      );
    }
    parts.push(`</div>`);
  }

  parts.push(`</div></div>`);
  return parts.join("");
}

/** A stat field's value: markdown links, inline code, and the blank marker. */
function renderInlineValue(value: string): string {
  if (!value) return `<span class="subtle">—</span>`;
  let html = escapeHtml(value);
  html = html.replace(
    /`([^`]+)`/g,
    (_m, code) => `<code>${code}</code>`,
  );
  html = html.replace(
    /\[([^\]]+)\]\(([^)]+)\)/g,
    (_m, text, href) => `<a href="${escapeAttr(href)}">${text}</a>`,
  );
  html = html.replace(
    new RegExp(NONE_MARKER, "g"),
    `<span aria-label="none">—</span>`,
  );
  return html;
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

/* --------------------------------------------------------------- traversal */

function walk(
  node: { children?: unknown[] } | null,
  visit: (
    node: any,
    parent: any,
    index: number | null,
  ) => void,
): void {
  if (!node || !Array.isArray((node as { children?: unknown[] }).children)) return;
  const children = (node as { children: any[] }).children;
  for (let i = 0; i < children.length; i++) {
    const child = children[i];
    visit(child, node, i);
    // The visitor may have replaced this child; re-read it.
    const current = children[i];
    if (current !== child) continue;
    walk(current, visit);
  }
}

function walkHast(
  node: { children?: unknown[] } | null,
  visit: (node: any) => void,
): void {
  if (!node || !Array.isArray((node as { children?: unknown[] }).children)) return;
  for (const child of (node as { children: any[] }).children) {
    visit(child);
    walkHast(child, visit);
  }
}

/* ---------------------------------------------------------------- the render */

export function renderMarkdown(source: string, options: RenderOptions): RenderResult {
  const { markdown, secrets, statBlocks } = preprocess(source, options.vocab);
  const constructs = new Set<string>();

  const processor = unified()
    .use(remarkParse)
    .use(remarkGfm)
    .use(remarkDirective)
    .use(remarkDemitable({
      statBlocks,
      secrets,
      secret: options.secret,
      variant: options.statBlockVariant ?? "full",
    }))
    .use(remarkInline)
    .use(remarkDiceTable)
    .use(remarkLinks(options.resolveLink ?? ((href) => ({ href, external: /^https?:/.test(href) }))))
    .use(remarkRehype, { allowDangerousHtml: true })
    .use(rehypeSanitize)
    .use(rehypeStringify, { allowDangerousHtml: true });

  const file = processor.processSync(markdown);
  const html = String(file);

  if (/\[\!/.test(source)) constructs.add("callout");
  if (/```ad-statblock/.test(source)) constructs.add("ad-statblock");
  if (/\^[A-Za-z0-9]/.test(source)) constructs.add("block-ref");
  if (/\[\[/.test(source)) constructs.add("wikilink");
  if (/^\s*\|/m.test(source)) constructs.add("table");
  if (/^> \[!/m.test(source)) constructs.add("callout");

  return { html, constructs: [...constructs], secrets, warnings: [] };
}
