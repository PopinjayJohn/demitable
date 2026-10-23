#!/usr/bin/env node
/**
 * Extract the corpus excerpts the prototype renders.
 *
 * The corpus (`fixtures/`) is a test corpus: gitignored, not a deliverable.
 * But the prototype has to render *real* content, not a paraphrase of it. So
 * this script slices a handful of real files at recorded line ranges and
 * writes them into `src/lib/corpus/`, with a provenance header naming the
 * source file and the lines taken.
 *
 *   node tools/extract-corpus.mjs [--root ../../fixtures]
 *
 * Everything it writes is a verbatim excerpt — no edits, no reformatting.
 * The header comment is the only thing added, and it is added on a line the
 * renderer never sees (the excerpts are loaded as data, not as markdown).
 */

import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const ROOT = process.argv.includes("--root")
  ? process.argv[process.argv.indexOf("--root") + 1]
  : "../../fixtures";
const OUT = join(here, "..", "src", "lib", "corpus");

/**
 * Each entry: an output name, the corpus file it comes from, and the
 * 1-based inclusive line range.
 */
const EXTRACTS = [
  {
    out: "dragon-statblock.md",
    from: "compendium/bestiary/dragon/adult-red-dragon-xmm.md",
    lines: [37, 89],
    why: "A GM-only monster stat block, in the `ad-statblock` dialect: a title option, a token image, a size/type line, the labelled field list, the six-column ability table, and `## Traits` / `## Actions` sections inside the fence.",
  },
  {
    out: "guard-statblock.md",
    from: "compendium/bestiary/humanoid/guard-xmm.md",
    lines: [33, 61],
    why: "A second stat block, smaller and of a different layout — the party-visible monster in the side-by-side comparison.",
  },
  {
    out: "guard-quote.md",
    from: "compendium/bestiary/humanoid/guard-xmm.md",
    lines: [28, 30],
    why: "A `> [!quote]` callout with a title, and the empty `> ` continuation line the corpus uses.",
  },
  {
    out: "wizard-tldr.md",
    from: "compendium/classes/wizard-xphb.md",
    lines: [14, 43],
    why: "A `> [!tldr]` callout whose body is a raw HTML table — the class-progression table. It carries the corpus's malformed `class\"level\"` attributes, and closes with the `^class-progression` block reference.",
  },
  {
    out: "ammunition-note.md",
    from: "compendium/items/1-ammunition-xdmg.md",
    lines: [25, 30],
    why: "The corpus's three malformed callouts: the `[!note]` marker is appended to the end of a paragraph instead of starting a line.",
  },
  {
    out: "fire-bolt.md",
    from: "compendium/spells/fire-bolt-xphb.md",
    lines: [21, 35],
    why: "A party-visible spell note — a labelled bullet list, inline code for dice, a link with a title attribute, and a prose paragraph.",
  },
  {
    out: "deck-table.md",
    from: "compendium/decks/deck-of-illusions-xdmg.md",
    lines: [19, 58],
    why: "A `dice:`-headed two-column table — the corpus's roll-on-a-table form, which a markdown table with a `dice:` header column has to render as a d100 range.",
  },
];

mkdirSync(OUT, { recursive: true });

for (const e of EXTRACTS) {
  const src = readFileSync(join(ROOT, e.from), "utf8").split("\n");
  const [a, b] = e.lines;
  const excerpt = src.slice(a - 1, b).join("\n");
  const header =
    `<!-- EXCERPT — do not edit.\n` +
    `     Source: ${e.from} (lines ${a}–${b} of the D&D SRD corpus, CC-BY-4.0).\n` +
    `     ${e.why}\n` +
    `     Regenerate with: node tools/extract-corpus.mjs -->\n`;
  writeFileSync(join(OUT, e.out), header + excerpt + "\n", "utf8");
  console.log(`${e.out.padEnd(26)} ${String(e.out.length).padStart(0)} ← ${e.from}:${a}-${b}`);
}
