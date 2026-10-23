/**
 * The projection — ADR-0033's visibility model, run in the browser.
 *
 * This exists so the prototype can show the *consequences* of the model in the
 * interface: the GM's browser can render a note through a player's eyes and
 * the difference is a real difference, not a drawing of one.
 *
 * It is not the shipping projector. It is the shipping projector *minus the
 * server*, so that the rules can be exercised and argued with.
 *
 * The rule order is ADR-0033's, and the first rule that applies wins.
 */

import type {
  Decision, Note, NoteProperty, Principal, Projection, Visibility,
} from "./model";

/* ------------------------------------------------------- secret extraction */

/**
 * ADR-0036: a secret is *any block marked GM-only*, recognised across the
 * community's syntaxes — a callout type, an `ad-*` code block, a `gm: true`
 * option on any code block, or an explicit marker block.
 *
 * The vocabulary is pack configuration, not a hardcoded list. This prototype
 * uses the pack's declared list.
 */
export interface SecretVocabulary {
  calloutTypes: string[];
  codeBlockKeys: string[];
  /** The `statblock` dialect's own YAML option that marks a block GM-only. */
  statblockGmKey: string;
}

export interface SecretBlock {
  id: string;
  label: string;
  /** The exact source text. Never leaves this module for a non-GM. */
  source: string;
}

/** A block marked GM-only, with a stable identity (ADR-0035, ADR-0036). */
interface MarkedBlock {
  id: string;
  label: string;
  source: string;
  start: number;
  end: number;
}

/** First eight hex digits of the SHA-256 of the block's normalised source. */
function hashId(source: string): string {
  // A tiny FNV-1a-based digest. The prototype only needs *stable* ids, and
  // the shipping implementation is specified as SHA-256 (ADR-0035).
  let h = 0x811c9dc5;
  const norm = source.replace(/\r\n/g, "\n").replace(/[ \t]+$/gm, "").trim();
  for (let i = 0; i < norm.length; i++) {
    h ^= norm.charCodeAt(i);
    h = Math.imul(h, 0x01000193) >>> 0;
  }
  return h.toString(16).padStart(8, "0").slice(0, 8);
}

/**
 * Find every GM-only block in a body. The vocabulary comes from the pack.
 */
export function findSecrets(body: string, vocab: SecretVocabulary): SecretBlock[] {
  const lines = body.split("\n");
  const blocks: MarkedBlock[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // --- Obsidian callout: `> [!secret] Title` ---------------------------
    const callout = line.match(/^\s*>\s*\[!([^\]\s]+)\]([+-]?)\s*(.*)$/);
    if (callout && vocab.calloutTypes.includes(callout[1])) {
      const start = i;
      const label = callout[3] || callout[1];
      let j = i + 1;
      while (j < lines.length && /^\s*>/.test(lines[j])) j++;
      const source = lines.slice(start, j).join("\n");
      blocks.push({
        id: blockId(lines, start, j, source),
        label,
        source,
        start,
        end: j,
      });
      i = j;
      continue;
    }

    // --- Explicit marker block: <!-- secret:begin --> … <!-- secret:end -->
    if (/<!--\s*secret:begin\s*-->/.test(line)) {
      const start = i;
      let j = i + 1;
      while (j < lines.length && !/<!--\s*secret:end\s*-->/.test(lines[j])) j++;
      j = Math.min(j + 1, lines.length);
      const source = lines.slice(start, j).join("\n");
      blocks.push({
        id: blockId(lines, start, j, source),
        label: "Secret block",
        source,
        start,
        end: j,
      });
      i = j;
      continue;
    }

    // --- Fenced code block with a `gm: true` option ------------------------
    const fence = line.match(/^\s*(`{3,}|~{3,})\s*(\S*)\s*(.*)$/);
    if (fence) {
      const marker = fence[1];
      const info = fence[2];
      let j = i + 1;
      while (j < lines.length && !lines[j].trimStart().startsWith(marker)) j++;
      const bodyLines = lines.slice(i, Math.min(j + 1, lines.length));
      const source = bodyLines.join("\n");
      // Option block: leading `key: value` lines, ending at the first blank.
      const opts: Record<string, string> = {};
      for (const l of bodyLines.slice(1)) {
        if (l.trim() === "") break;
        const m = l.match(/^([A-Za-z_][\w-]*):\s*(.*)$/);
        if (!m) break;
        opts[m[1]] = m[2].trim();
      }
      const isSecret =
        (info && vocab.codeBlockKeys.includes(info)) ||
        opts[vocab.statblockGmKey] === "true" ||
        Object.entries(opts).some(
          ([k, v]) => vocab.codeBlockKeys.includes(k) && v === "true",
        );
      if (isSecret) {
        blocks.push({
          id: blockId(lines, i, Math.min(j + 1, lines.length), source),
          label: opts.title ?? info ?? "Secret block",
          source,
          start: i,
          end: Math.min(j + 1, lines.length),
        });
      }
      i = Math.min(j + 1, lines.length);
      continue;
    }

    i++;
  }

  return blocks.map((b) => ({ id: b.id, label: b.label, source: b.source }));
}

/** An explicit `^id` on the last line of the block wins; else a content hash. */
function blockId(lines: string[], start: number, end: number, source: string): string {
  for (let k = end - 1; k >= start; k--) {
    const m = lines[k]?.match(/\^([A-Za-z0-9][\w-]*)\s*$/);
    if (m) return m[1];
  }
  return hashId(source);
}

/**
 * Strip GM-only blocks from a body. This is the "the text never reaches a
 * non-GM client" rule, executed.
 *
 * The stripped region is replaced by nothing at all — not a placeholder. A
 * placeholder would tell the player that something was there.
 */
export function stripSecrets(body: string, vocab: SecretVocabulary): {
  body: string;
  stripped: { id: string; label: string }[];
} {
  const lines = body.split("\n");
  const blocks = findSecrets(body, vocab);
  if (blocks.length === 0) return { body, stripped: [] };

  const ranges: { start: number; end: number; id: string; label: string }[] = [];
  for (const b of blocks) {
    const linesOfBlock = b.source.split("\n").length;
    // find the line index of the block start
    const start = lines.findIndex((l, idx) => idx >= 0 && l === b.source.split("\n")[0]);
    if (start >= 0) {
      ranges.push({ start, end: start + linesOfBlock, id: b.id, label: b.label });
    }
  }
  ranges.sort((a, b) => a.start - b.start);

  const out: string[] = [];
  let cursor = 0;
  for (const r of ranges) {
    out.push(...lines.slice(cursor, r.start));
    cursor = r.end;
  }
  out.push(...lines.slice(cursor));

  // Collapse the blank lines a removal leaves behind, but keep the block
  // boundary: the paragraphs either side must not merge.
  const collapsed = out.join("\n").replace(/\n{3,}/g, "\n\n").replace(/^\n+/, "");
  return {
    body: collapsed,
    stripped: ranges.map((r) => ({ id: r.id, label: r.label })),
  };
}

/* ------------------------------------------------------------ the projector */

export interface ProjectorOptions {
  vocab: SecretVocabulary;
  /** The notes of the vault, for grant resolution and owner fallbacks. */
  notes: Note[];
}

/**
 * The visibility model, in ADR-0033's order. The first rule that applies
 * wins, and every rule that was consulted is recorded so the UI can explain
 * the answer (ADR-0033's consequences section asks for exactly this
 * inspector).
 */
export function project(
  note: Note,
  principal: Principal,
  opts: ProjectorOptions,
): Projection {
  const decisions: Decision[] = [];
  const isGM = principal.kind === "gm";

  /* Rule 1 — scope ------------------------------------------------------- */
  const inCampaign =
    note.campaign !== undefined &&
    (principal.kind === "display" ||
      (principal.campaign !== undefined && principal.campaign === note.campaign));
  if (note.campaign !== undefined) {
    if (inCampaign || isGM) {
      decisions.push({
        rule: "scope",
        verdict: "allow",
        ref: "ADR-0033 §Scope",
        detail: `The note is in campaign “${note.campaign}”, and ${
          isGM ? "the GM" : principal.name
        } is in it.`,
      });
    } else {
      decisions.push({
        rule: "scope",
        verdict: "deny",
        ref: "ADR-0033 §Scope",
        detail: `The note is in campaign “${note.campaign}”. ${
          principal.kind === "display"
            ? "The shared display sees party content only, so it is in scope only for party-visible notes."
            : `${principal.name} is not a member of that campaign.`
        }`,
      });
      return {
        visible: false, decisions, body: "", properties: [],
        deniedProperties: [], strippedSecrets: [],
      };
    }
  } else {
    decisions.push({
      rule: "scope",
      verdict: isGM ? "allow" : "conditional",
      ref: "ADR-0033 §Scope",
      detail: isGM
        ? "The note has no campaign, so it is a personal or global note. The GM sees it."
        : "The note has no campaign, so it is visible to the GM and to anyone explicitly granted.",
    });
  }

  /* Rule 2 — the absolute deny ------------------------------------------- */
  if (note.visibility === "gm") {
    if (isGM) {
      decisions.push({
        rule: "absolute-deny",
        verdict: "allow",
        ref: "ADR-0033 §2",
        detail:
          "`visibility: gm`. Absolute: no grant overrides it, and it is never serialised into a non-GM response. You are the GM, so it is yours.",
      });
    } else {
      decisions.push({
        rule: "absolute-deny",
        verdict: "deny",
        ref: "ADR-0033 §2",
        detail:
          "`visibility: gm`. Absolute: no grant overrides it, and it is never serialised into a non-GM response.",
      });
      return {
        visible: false, decisions, body: "", properties: [],
        deniedProperties: [], strippedSecrets: [],
      };
    }
  }

  /* Rule 3 — the workflow gate -------------------------------------------- */
  if (note.workflow === "draft" || note.workflow === "submitted") {
    const owner = note.player;
    const isOwner = owner !== undefined && owner === principal.id;
    if (!isGM && !isOwner) {
      decisions.push({
        rule: "workflow-gate",
        verdict: "deny",
        ref: "ADR-0033 §3",
        detail: `\`workflow: ${note.workflow}\`. The workflow gate is a grant-suppressing deny: visible only to its owner and the GM, regardless of visibility or grants.`,
      });
      return {
        visible: false, decisions, body: "", properties: [],
        deniedProperties: [], strippedSecrets: [],
      };
    }
    decisions.push({
      rule: "workflow-gate",
      verdict: "allow",
      ref: "ADR-0033 §3",
      detail: `\`workflow: ${note.workflow}\`, but the gate names ${isGM ? "the GM" : "its owner"} as visible.`,
    });
  }

  /* Rule 4/5/6 — base level, grants, default ----------------------------- */
  const declared = note.visibility;
  const effective: Visibility = declared ?? (note.campaign !== undefined ? "party" : "gm");

  if (declared === undefined) {
    decisions.push({
      rule: "default",
      verdict: effective === "gm" ? "deny" : "allow",
      ref: "ADR-0033 §6",
      detail:
        effective === "gm"
          ? "No `visibility`, and no `campaign`, so the default is `gm`."
          : "No `visibility`, but the note is in a campaign, so the default is `party`.",
    });
  } else {
    decisions.push({
      rule: "base-level",
      verdict: "allow",
      ref: "ADR-0033 §4",
      detail: `Declared \`visibility: ${declared}\`.`,
    });
  }

  let allowed = false;
  switch (effective) {
    case "public":
      allowed = true;
      break;
    case "party":
      allowed = inCampaign && principal.kind !== "display"
        ? true
        : principal.kind === "display";
      if (!allowed && principal.kind !== "display") {
        // a party note outside the campaign was already denied by scope
        allowed = true;
      }
      break;
    case "owner":
      allowed = isGM || (note.player !== undefined && note.player === principal.id);
      break;
    case "private":
      // ADR-0048: the note's player, and nobody else — not even the GM.
      allowed = note.player !== undefined && note.player === principal.id;
      if (isGM && !allowed) {
        decisions.push({
          rule: "private",
          verdict: "deny",
          ref: "ADR-0048 §private",
          detail: "`visibility: private`. The owner's personal journal. The GM is not a reader — this narrows `owner`, and never widens anything.",
        });
        return {
          visible: false, decisions, body: "", properties: [],
          deniedProperties: [], strippedSecrets: [],
        };
      }
      break;
    case "gm":
      allowed = isGM;
      break;
  }

  /* Rule 5 — grants ------------------------------------------------------- */
  if (!allowed && note.grants?.includes(principal.id)) {
    allowed = true;
    decisions.push({
      rule: "grant",
      verdict: "allow",
      ref: "ADR-0033 §5",
      detail: `Granted to ${principal.name} (\`grants: [${note.grants.join(", ")}]\`).`,
    });
  }

  /* The display principal (ADR-0048) -------------------------------------- */
  if (principal.kind === "display" && effective !== "party" && effective !== "public") {
    decisions.push({
      rule: "base-level",
      verdict: "deny",
      ref: "ADR-0048 §display",
      detail:
        "The shared display resolves to party-visible content only. It does not see the union of what the party members can see, because that union leaks.",
    });
    allowed = false;
  }

  if (!allowed) {
    decisions.push({
      rule: "base-level",
      verdict: "deny",
      ref: "ADR-0033 §4",
      detail: `\`visibility: ${effective}\` does not include ${principal.name}.`,
    });
    return {
      visible: false, decisions, body: "", properties: [],
      deniedProperties: [], strippedSecrets: [],
    };
  }

  /* Rule 7 — property level, and rule 8 — masking ------------------------- */
  const properties: NoteProperty[] = [];
  const deniedProperties: { key: string; label: string; reason: string }[] = [];
  for (const prop of note.properties) {
    const res = resolveProperty(prop, note, principal, isGM);
    if (res.visible) {
      properties.push(res.prop);
    } else {
      deniedProperties.push({ key: prop.key, label: prop.label, reason: res.reason });
    }
  }

  /* Secrets (ADR-0035) ---------------------------------------------------- */
  const revealed = new Set(note.revealed ?? []);
  const { body, stripped } = stripSecrets(note.body, opts.vocab);
  const strippedSecrets: { id: string; label: string }[] = [];
  let finalBody = body;
  if (stripped.length > 0 && !isGM) {
    for (const s of stripped) {
      if (revealed.has(s.id)) continue;
      strippedSecrets.push(s);
    }
    // A revealed secret is promoted in place (ADR-0035). Re-insert it.
    finalBody = reinsertRevealed(note.body, opts.vocab, revealed);
    strippedSecrets.length = 0;
  } else if (isGM) {
    for (const s of stripped) {
      if (!revealed.has(s.id)) strippedSecrets.push(s);
    }
  }

  return {
    visible: true,
    decisions,
    body: finalBody,
    properties,
    deniedProperties,
    strippedSecrets,
  };
}

/** Property-level visibility (ADR-0033 rule 7) and masking (rule 8). */
function resolveProperty(
  prop: NoteProperty,
  note: Note,
  principal: Principal,
  isGM: boolean,
): { visible: boolean; prop: NoteProperty; reason: string } {
  const declared = prop.visibility?.visibility;

  // No declaration: inherits the note's effective visibility.
  if (!declared) {
    return { visible: true, prop, reason: "inherits the note's visibility" };
  }

  if (declared === "gm") {
    if (isGM) return { visible: true, prop, reason: "GM only" };
    return {
      visible: false,
      prop,
      reason: "`visibility: gm` on the property. Absolute — never serialised into a non-GM response.",
    };
  }

  if (declared === "private") {
    const isOwner = note.player !== undefined && note.player === principal.id;
    return {
      visible: isOwner,
      prop,
      reason: isOwner
        ? "private to the note's player"
        : "`visibility: private` — the note's player, and nobody else.",
    };
  }

  if (declared === "owner") {
    // The rule ADR-0033 rule 7 was written to fix: if the note has no player,
    // the property falls back to the note's visibility.
    if (note.player === undefined) {
      const noteLevel = note.visibility ?? (note.campaign !== undefined ? "party" : "gm");
      if (noteLevel === "gm" && !isGM) {
        return {
          visible: false,
          prop,
          reason:
            "`owner` on a property, but the note has no player — so it falls back to the note's visibility, which is `gm`.",
        };
      }
      return {
        visible: true,
        prop,
        reason: "`owner` on a property, but the note has no player — so it falls back to the note's visibility.",
      };
    }
    const isOwner = note.player === principal.id;
    if (!isOwner && !isGM) {
      return {
        visible: false,
        prop,
        reason: "`owner` on the property — visible to the note's player and the GM.",
      };
    }
  }

  // Masking (rule 8): applied last, after grants and denies have decided
  // whether a value is present.
  const mask = prop.visibility?.display?.[principal.id] ?? prop.visibility?.display?.["*"];
  if (mask !== undefined && !isGM) {
    return {
      visible: true,
      prop: { ...prop, value: mask },
      reason: "masked for this audience",
    };
  }

  return { visible: true, prop, reason: `declared \`visibility: ${declared}\`` };
}

/** Put revealed secrets back into the body, in place. */
function reinsertRevealed(
  body: string,
  vocab: SecretVocabulary,
  revealed: Set<string>,
): string {
  const lines = body.split("\n");
  const blocks = findSecrets(body, vocab);
  if (blocks.length === 0) return body;

  // Locate the blocks by their source's first line.
  const marks: { start: number; end: number; source: string }[] = [];
  for (const b of blocks) {
    const first = b.source.split("\n")[0];
    const idx = lines.indexOf(first);
    if (idx >= 0) marks.push({ start: idx, end: idx + b.source.split("\n").length, source: b.source });
  }
  marks.sort((a, b) => a.start - b.start);

  const keep = new Set<string>();
  for (const b of blocks) if (revealed.has(b.id)) keep.add(b.source);

  const out: string[] = [];
  let cursor = 0;
  for (const m of marks) {
    out.push(...lines.slice(cursor, m.start));
    if (keep.has(m.source)) out.push(...m.source.split("\n"));
    cursor = m.end;
  }
  out.push(...lines.slice(cursor));
  return out.join("\n").replace(/\n{3,}/g, "\n\n");
}
