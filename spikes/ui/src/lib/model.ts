/**
 * The domain model the prototype works against.
 *
 * This is a *reading* of ADR-0033, ADR-0035, ADR-0040 and ADR-0048 — not the
 * shipping types. It is here so the prototype can run the real rules and show
 * the consequences in the interface rather than in a document.
 */

export type Visibility = "public" | "party" | "owner" | "private" | "gm";

export type Workflow = "draft" | "submitted" | "active" | "retired";

export type PrincipalKind = "gm" | "player" | "display" | "anonymous";

export interface Principal {
  id: string;
  name: string;
  kind: PrincipalKind;
  /** The campaign this principal belongs to. The display belongs to none. */
  campaign?: string;
}

/** A property-level visibility declaration (ADR-0033 rule 7). */
export interface PropertyVisibility {
  /** `owner` resolves against the note's `player`. */
  visibility?: Visibility;
  /**
   * `display` overrides the rendered value per audience. Applied last, after
   * grants and denies have decided *whether* a value is present.
   */
  display?: Record<string, string>;
}

export interface NoteProperty {
  key: string;
  label: string;
  value: unknown;
  type: "string" | "number" | "boolean" | "list" | "longtext";
  /** The pack may declare a visibility for this property. */
  visibility?: PropertyVisibility;
}

export interface Note {
  uid: string;
  type: string;
  name: string;
  /** The campaign dimension (ADR-0033). Absent means campaign-less. */
  campaign?: string;
  /** Absent means "use the default" (ADR-0033 rule 6). */
  visibility?: Visibility;
  workflow?: Workflow;
  /** The player account that owns this note. Was `owner` (ADR-0040). */
  player?: string;
  /** `pc` | `npc` | `creature` (ADR-0040). */
  kind?: "pc" | "npc" | "creature";
  /** Additional parties who may see the note. */
  grants?: string[];
  /** Secret block ids that have been revealed (ADR-0035). */
  revealed?: string[];
  path: string;
  /** Free-form pack-declared properties. */
  properties: NoteProperty[];
  /** The note body, markdown. */
  body: string;
  /** The pack's declared content type, for the browser's grouping. */
  contentType?: string;
}

/* ------------------------------------------------------------------ reasons */

export type Rule =
  | "scope"
  | "absolute-deny"
  | "workflow-gate"
  | "base-level"
  | "grant"
  | "default"
  | "private"
  | "property";

export interface Decision {
  rule: Rule;
  /** What the rule decided, in words a GM would understand. */
  verdict: "allow" | "deny" | "conditional";
  detail: string;
  /** The rule number in ADR-0033, for the inspector to cite. */
  ref: string;
}

export interface Projection {
  /** Whether the note reaches this principal at all. */
  visible: boolean;
  /** The chain of rules that produced the answer, in ADR-0033's order. */
  decisions: Decision[];
  /** The body with GM-only blocks stripped. Never contains secret text. */
  body: string;
  /** The properties the principal may see. */
  properties: NoteProperty[];
  /** Properties denied to this principal, for the GM's inspector only. */
  deniedProperties: { key: string; label: string; reason: string }[];
  /** Secret blocks that were stripped, for the GM's inspector only. */
  strippedSecrets: { id: string; label: string }[];
}

/* ------------------------------------------------------------------ helpers */

export function principalIsGM(p: Principal): boolean {
  return p.kind === "gm";
}

export function visibilityLabel(v: Visibility | undefined): string {
  switch (v) {
    case "public": return "Public";
    case "party": return "Party";
    case "owner": return "Owner";
    case "private": return "Private";
    case "gm": return "GM only";
    default: return "Unset";
  }
}

export function workflowLabel(w: Workflow | undefined): string {
  switch (w) {
    case "draft": return "Draft";
    case "submitted": return "Submitted";
    case "active": return "Active";
    case "retired": return "Retired";
    default: return "—";
  }
}
