/**
 * The `ad-statblock` dialect, parsed.
 *
 * The corpus's stat block is *prose in a code fence*: a `title:` option, a
 * token image, a size/type line, a list of labelled fields, a six-column
 * ability table, and then `## Traits` / `## Actions` sections. It is not a
 * schema, and it is not a table — it is something a human reads.
 *
 * So the parser is line-oriented and best-effort. It reports what it could
 * not understand rather than guessing.
 */

export interface StatField {
  label: string;
  value: string;
}

export interface StatAbility {
  name: string;
  score: string;
}

export interface StatSection {
  heading: string;
  entries: { name?: string; text: string }[];
}

export interface StatBlock {
  title: string;
  token?: { url: string; alt: string };
  subtitle?: string;
  fields: StatField[];
  abilities?: StatAbility[];
  sections: StatSection[];
  /** Things the parser saw and did not understand. */
  unparsed: string[];
}

const ABILITY_NAMES = ["STR", "DEX", "CON", "INT", "WIS", "CHA"];

/** The corpus's "no proficiency" marker. 5etools writes U+23E4. */
export const NONE_MARKER = "⏤";

export function parseStatBlock(source: string): StatBlock {
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const out: StatBlock = {
    title: "",
    fields: [],
    sections: [],
    unparsed: [],
  };

  let i = 0;

  // The option block: leading `key: value` lines, ended by a blank line.
  while (i < lines.length && lines[i].trim() !== "") {
    const m = lines[i].match(/^([A-Za-z_][\w-]*):\s*(.*)$/);
    if (!m) break;
    if (m[1] === "title") out.title = m[2].trim();
    else out.unparsed.push(`option \`${m[1]}\` — not part of the known layout`);
    i++;
  }
  i++; // the blank line

  // The token image.
  const img = lines[i]?.match(/^!\[([^\]]*)\]\(([^)]*)\)\s*$/);
  if (img) {
    out.token = { url: img[2], alt: img[1] };
    i++;
  }

  // The size / type line: `*Huge dragon (chromatic), Chaotic Evil*`.
  const sub = lines[i]?.match(/^\*([^*]+)\*\s*$/);
  if (sub) {
    out.subtitle = sub[1].trim();
    i++;
  }

  let currentSection: StatSection | undefined;

  for (; i < lines.length; i++) {
    const line = lines[i];

    if (line.trim() === "") continue;

    // A section heading: `## Actions`.
    const heading = line.match(/^#{1,6}\s+(.*)$/);
    if (heading) {
      currentSection = { heading: heading[1].trim(), entries: [] };
      out.sections.push(currentSection);
      continue;
    }

    // A labelled field: `- **Armor Class** 19`.
    const field = line.match(/^[-*]\s+\*\*(.+?)\*\*\s*:?\s*(.*)$/);
    if (field && !currentSection) {
      out.fields.push({ label: field[1].trim(), value: field[2].trim() });
      continue;
    }

    // The ability table.
    if (line.trimStart().startsWith("|")) {
      const header = line.trim().replace(/^\||\|$/g, "").split("|").map((c) => c.trim());
      if (header.length === 6 && ABILITY_NAMES.every((n, k) => header[k] === n)) {
        i++;
        if (lines[i]?.trimStart().startsWith("|")) i++; // the alignment row
        const row = lines[i]?.trim().replace(/^\||\|$/g, "").split("|").map((c) => c.trim()) ?? [];
        out.abilities = ABILITY_NAMES.map((name, k) => ({ name, score: row[k] ?? "" }));
        i++;
        continue;
      }
      out.unparsed.push("a table that is not the six-column ability grid");
      continue;
    }

    // An entry inside a section: `***Bite.*** *Melee Attack Roll:* ...`
    if (currentSection) {
      const entry = line.match(/^\*\*\*(.+?)\*\*\*\s*(.*)$/);
      if (entry) {
        currentSection.entries.push({ name: entry[1].trim(), text: entry[2].trim() });
        continue;
      }
      currentSection.entries.push({ text: line.trim() });
      continue;
    }

    out.unparsed.push(line.slice(0, 60));
  }

  return out;
}

/**
 * The fields a *player* may see, when the note is party-visible but the
 * mechanics are pack-scoped.
 *
 * This is the honest finding of the spike: the `ad-statblock` dialect is
 * prose, and prose has no per-field visibility. So the reduced stat card is
 * **not** derived from the prose block — it comes from the pack's structured
 * `data:` (ADR-0036). This function is the *fallback*, for a vault that has
 * no structured data yet, and it is deliberately conservative: the identity
 * of a creature, not its numbers.
 */
export const PARTY_SAFE_FIELDS = new Set([
  "Armor Class",
  "Hit Points",
  "Speed",
  "Proficiency Bonus",
  "Senses",
  "Languages",
  "Challenge",
]);

export function partySafeFields(block: StatBlock): StatField[] {
  return block.fields.filter((f) => PARTY_SAFE_FIELDS.has(f.label));
}
