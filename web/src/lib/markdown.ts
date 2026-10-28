// Client-side markdown rendering for projected note content.
//
// The server's `/render` endpoint is an escaped-text stub (sanitisation
// policy pending) and serves `n.Raw` to the GM — frontmatter included. So
// the webapp renders locally: strip frontmatter, render GFM, then
// sanitise. The trust boundary is unchanged: non-GM callers only ever
// receive the server-projected body, and secrets are styled here, never
// reconstructed — stripped text cannot come back through this module.

import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import remarkRehype from "remark-rehype";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import rehypeStringify from "rehype-stringify";
import { visit } from "unist-util-visit";
import type { Root as HastRoot, Element as HastElement, Text as HastText } from "hast";
import type { Root as MdastRoot, Text as MdastText, Parent as MdastParent } from "mdast";

// The sanitiser's default schema allows no classes and no data
// attributes; the visibility and secret affordances need both. Note that
// `className` values are checked individually, so each class this module
// emits is allowlisted here — anything else is dropped, which is the
// fail-closed behaviour we want.
const CLASS_ALLOW = [
  "wikilink",
  "dangling",
  "secret-block",
  "secret-block__head",
  "callout",
  "block-ref",
  "statblock",
  "statblock__head",
  "admonition",
  "admonition__head",
  "callout__label",
  "secret--revealed",
  "secret-reveal",
];
const CLASS_RE = [/^callout-/, /^admonition-/, /^language-/];

// A per-tag `className` allow-list (e.g. footnotes on `a`) shadows the
// `*` entry — findDefinition returns the first match — so the classes
// this module emits are merged into each tag's own tuple rather than
// added globally. Anything not listed is dropped, fail-closed.
function allowClasses(existing: readonly unknown[] | undefined): unknown[] {
  const vals: unknown[] = [];
  const rest: unknown[] = [];
  for (const e of existing ?? []) {
    if (Array.isArray(e) && e[0] === "className") vals.push(...e.slice(1));
    else rest.push(e);
  }
  return [...rest, ["className", ...vals, ...CLASS_ALLOW, ...CLASS_RE]];
}
const attrs = defaultSchema.attributes as Record<string, readonly unknown[] | undefined>;
const schema = {
  ...defaultSchema,
  tagNames: [...(defaultSchema.tagNames ?? []), "button"],
  attributes: {
    ...defaultSchema.attributes,
    "*": [...(attrs["*"] ?? []), "dataBlockId", "dataUid", "dataRevealBlock"],
    a: allowClasses(attrs["a"]),
    blockquote: allowClasses(attrs["blockquote"]),
    button: allowClasses(attrs["button"]),
    div: allowClasses(attrs["div"]),
    p: allowClasses(attrs["p"]),
    pre: allowClasses(attrs["pre"]),
    span: allowClasses(attrs["span"]),
  },
};

export function stripFrontmatter(src: string): string {
  const text = src.replace(/\r\n/g, "\n");
  if (!text.startsWith("---\n") && text !== "---") return src;
  const lines = text.split("\n");
  for (let i = 1; i < lines.length; i++) {
    if (lines[i].trim() === "---" || lines[i].trim() === "...") {
      return lines.slice(i + 1).join("\n");
    }
  }
  return src;
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

// --- remark: wikilinks ------------------------------------------------------
// [[Target]] and [[Target|alias]] become links when the caller can resolve
// the name to a uid, and inert styled spans otherwise. A dangling link is
// present but not followable — never a dead navigation.

const WIKILINK_RE = /\[\[([^\]|]+?)(?:\|([^\]]+?))?\]\]/g;

function remarkWikilinks(resolve?: (name: string) => string | null) {
  return (tree: MdastRoot) => {
    visit(tree, "text", (node: MdastText, index: number | undefined, parent: MdastParent | undefined) => {
      if (index === undefined || !parent || parent.type === "link") return;
      const value = node.value;
      WIKILINK_RE.lastIndex = 0;
      let m: RegExpExecArray | null;
      let last = 0;
      const out: Array<MdastText | any> = [];
      let changed = false;
      while ((m = WIKILINK_RE.exec(value)) !== null) {
        changed = true;
        if (m.index > last) out.push({ type: "text", value: value.slice(last, m.index) });
        const target = m[1].trim();
        const alias = (m[2] ?? target).trim();
        const uid = resolve?.(target) ?? null;
        out.push({
          type: "link",
          url: uid ? `#note:${uid}` : "#",
          data: {
            hProperties: uid
              ? { className: ["wikilink"], dataUid: uid }
              : { className: ["wikilink", "dangling"] },
          },
          children: [{ type: "text", value: alias }],
        });
        last = m.index + m[0].length;
      }
      if (!changed) return;
      if (last < value.length) out.push({ type: "text", value: value.slice(last) });
      parent.children.splice(index, 1, ...out as any);
      return index + out.length;
    });
  };
}

// --- rehype: callouts, block refs, statblocks --------------------------------

function hastText(el: HastElement): string {
  let s = "";
  const walk = (n: any) => {
    if (n.type === "text") s += (n as HastText).value;
    if (Array.isArray((n as any).children)) for (const c of (n as any).children) walk(c);
  };
  walk(el);
  return s;
}

function firstTextNode(el: HastElement): HastText | null {
  let found: HastText | null = null;
  const walk = (n: any) => {
    if (found) return;
    if (n.type === "text") { found = n as HastText; return; }
    if (Array.isArray(n.children)) for (const c of n.children) walk(c);
  };
  for (const c of el.children) walk(c);
  return found;
}

function rehypeCallouts(options?: { revealed?: string[]; revealButtons?: boolean }) {
  const revealed = new Set(options?.revealed ?? []);
  const revealButtons = options?.revealButtons ?? false;
  return (tree: HastRoot) => {
    visit(tree, "element", (node: any) => {
      if (node.tagName !== "blockquote") return;
      const el = node as HastElement;
      const first = el.children.find((c) => c.type === "element") as HastElement | undefined;
      if (!first || first.tagName !== "p") return;
      const text = hastText(first);
      const m = text.match(/^\s*\[!([\w-]+)\]\s*([+-]?)\s*/);
      if (!m) return;
      const kind = m[1].toLowerCase();
      const tn = firstTextNode(first);
      if (tn) tn.value = tn.value.replace(/^\s*\[![\w-]+\]\s*[+-]?\s*/, "");
      const props = (el.properties ??= {}) as Record<string, unknown>;
      if (kind === "secret") {
        // GM-only block: an outline plus hatch so it reads as "concealed"
        // rather than "warning" under greyscale — secrets are not errors.
        // No label is injected; the border pattern carries the meaning.
        // Already-revealed blocks get the shared-colour outline instead.
        const all = hastText(el);
        const idm = all.match(/\^([\w-]+)\s*$/);
        const isRevealed = !!idm && revealed.has(idm[1]);
        props.className = ["secret-block", ...(isRevealed ? ["secret--revealed"] : [])];
        if (idm) {
          (props as any).dataBlockId = idm[1];
          const last = [...el.children].reverse().find((c) => c.type === "element") as HastElement | undefined;
          const ltn = last ? firstTextNode(last) : null;
          if (ltn) ltn.value = ltn.value.replace(/\s*\^[\w-]+\s*$/, "");
        }
        // An in-body reveal affordance for the GM: hidden until the
        // block is hovered or focused, so reading prose stays clean.
        // One-way only — the server has no un-reveal.
        if (revealButtons && idm && !isRevealed) {
          el.children.push({
            type: "element",
            tagName: "button",
            properties: { className: ["secret-reveal"], dataRevealBlock: idm[1] },
            children: [{ type: "text", value: "Reveal" }],
          } as any);
        }
      } else {
        props.className = ["callout", `callout-${kind}`];
        el.children.unshift({
          type: "element",
          tagName: "div",
          properties: { className: ["callout__label"] },
          children: [{ type: "text", value: kind }],
        } as any);
      }
    });
  };
}

function rehypeBlockRefs() {
  return (tree: HastRoot) => {
    visit(tree, "element", (node: any) => {
      if (node.tagName !== "p") return;
      const el = node as HastElement;
      if (/^\^[\w-]+$/.test(hastText(el).trim())) {
        (el.properties ??= {} as any).className = ["block-ref"];
      }
    });
  };
}

function rehypeStatblocks() {
  return (tree: HastRoot) => {
    visit(tree, "element", (node: any, index: number | undefined, parent: any) => {
      if (node.tagName !== "pre" || !parent || index === undefined) return;
      const el = node as HastElement;
      const code = el.children.find((c) => c.type === "element") as HastElement | undefined;
      if (!code || code.tagName !== "code") return;
      const cls = ((code.properties?.className as string[] | undefined) ?? []).join(" ");
      const stat = cls.match(/language-(ad-)?statblock/);
      const admon = !stat && cls.match(/language-ad-([\w-]+)/);
      if (!stat && !admon) return;
      const text = hastText(code);
      const title = text.match(/^title:\s*(.+)$/m)?.[1]?.trim() ?? (admon ? admon[1] : "Stat block");
      // The title is chrome, not content: lift it to the head and drop
      // the raw line so it does not render twice.
      for (const child of code.children) {
        if (child.type === "text") {
          const v = (child as HastText).value.replace(/^title:\s*.+$/m, "").replace(/^\n/, "");
          if (v !== (child as HastText).value) {
            (child as HastText).value = v;
            break;
          }
        }
      }
      (el.properties ??= {} as any).className = [stat ? "statblock" : "admonition", ...(admon ? [`admonition-${admon[1]}`] : [])];
      el.children.unshift({
        type: "element",
        tagName: "div",
        properties: { className: [stat ? "statblock__head" : "admonition__head"] },
        children: [{ type: "text", value: title }],
      } as any);
    });
  };
}

export interface RenderOptions {
  resolveWiki?: (name: string) => string | null;
  /** Block ids already revealed to the table: green outline, no button. */
  revealed?: string[];
  /** Append a hover reveal button to unrevealed secrets (GM view). */
  revealButtons?: boolean;
}

export function renderMarkdown(src: string, opts: RenderOptions = {}): string {
  const body = stripFrontmatter(src);
  try {
    const file = unified()
      .use(remarkParse)
      .use(remarkGfm)
      .use(remarkWikilinks as any, opts.resolveWiki)
      .use(remarkRehype)
      .use(rehypeCallouts as any, { revealed: opts.revealed, revealButtons: opts.revealButtons })
      .use(rehypeBlockRefs as any)
      .use(rehypeStatblocks as any)
      .use(rehypeSanitize, schema as any)
      .use(rehypeStringify)
      .processSync(body);
    return String(file);
  } catch {
    return `<pre>${escapeHtml(body)}</pre>`;
  }
}
