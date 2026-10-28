// Shared UI state for the global frame (header + sidebar + center +
// contextual rail + footer). Center surfaces publish their right-rail
// content as a snippet so context stays local to the surface that owns
// the data; the frame only decides where it renders.
import type { Snippet } from "svelte";

export type Group = "wiki" | "vtt" | "creator";
export type WikiView = "home" | "explorer" | "viewer" | "editor" | "player" | "chat";
export type Surface = "dashboard" | "editor" | "characters" | "player" | "tabletop" | "chat";
export type Proof = "none" | "greyscale" | "flat-hue" | "protanopia" | "deuteranopia" | "tritanopia";

function storedBool(key: string, fallback: boolean): boolean {
  try {
    const v = localStorage.getItem(key);
    return v === null ? fallback : v === "1";
  } catch {
    return fallback;
  }
}

function loadSections(): Record<string, boolean> {
  try {
    const raw = localStorage.getItem("demitable:sections");
    if (raw) return JSON.parse(raw) as Record<string, boolean>;
  } catch { /* ignore */ }
  return {};
}

class AppState {
  group: Group = $state<Group>("wiki");
  wikiView: WikiView = $state<WikiView>("home");
  theme: "light" | "dark" = $state("light");
  proof: Proof = $state("none");
  selectedUid: string | null = $state(null);
  selectedCharacterUid: string | null = $state(null);
  /** Campaign filter for the wiki browser. Null = all campaigns. */
  campaignFilter: string | null = $state(null);
  /** Vault path-prefix filter, set from breadcrumbs. Null = whole vault. */
  pathFilter: string | null = $state(null);
  playerEye = $state(false);
  /** The account id the GM is borrowing eyes from. Empty = no target. */
  eyeTarget = $state("");
  showInspector = $state(true);
  showJournal = $state(false);
  showSessionJournal = $state(false);
  sessionId = $state("");
  encounterId = $state("");
  displayMode = $state(false);

  // Frame chrome state.
  sidebarCollapsed = $state(storedBool("demitable:sidebar", false));
  railPinned = $state(storedBool("demitable:rail", true));
  /** Collapsed sidebar sections by id. Persisted on toggle. */
  sidebarSections: Record<string, boolean> = $state(loadSections());

  // Right-rail content published by the active center surface.
  // Null means the view has no context: the rail collapses.
  rail: Snippet | null = $state(null);

  // Footer status, written by the owners of each fact.
  wsConnected = $state(false);
  saveStatus = $state("idle");
  diceLog: any[] = $state([]);
  rollStatus = $state("");

  toggleSidebar() {
    this.sidebarCollapsed = !this.sidebarCollapsed;
    try {
      localStorage.setItem("demitable:sidebar", this.sidebarCollapsed ? "1" : "0");
    } catch { /* ignore */ }
  }

  /** Compat mapping for the pre-frame surface ids. */
  get surface(): Surface {
    if (this.group === "vtt") return "tabletop";
    if (this.group === "creator") return "characters";
    if (this.wikiView === "explorer" || this.wikiView === "home" || this.wikiView === "viewer") return "dashboard";
    if (this.wikiView === "editor") return "editor";
    if (this.wikiView === "player") return "player";
    return "chat";
  }

  go(group: Group, view?: WikiView) {
    this.group = group;
    if (group === "wiki" && view) this.wikiView = view;
  }
}

export const appState = new AppState();

export function applyThemeProof() {
  try {
    document.documentElement.dataset.theme = appState.theme === "dark" ? "dark" : "light";
    if (appState.proof === "none") delete document.documentElement.dataset.proof;
    else document.documentElement.dataset.proof = appState.proof;
    const app = document.getElementById("app");
    if (app) {
      if (appState.proof === "none") app.removeAttribute("data-proof");
      else app.setAttribute("data-proof", appState.proof);
    }
  } catch { /* ignore */ }
}

// GM-only blocks in a markdown source: [!secret] callouts, ad-secret fences,
// gm:true statblocks. The block id is the ^id on any line of the block
// (conventionally the last); without one an s1, s2… fallback is used.
export interface SecretRef { id: string; label: string; }
export function findSecrets(src: string): SecretRef[] {
  const out: SecretRef[] = [];
  const lines = src.split("\n");
  let n = 0;
  const isQuote = (t: string) => /^\s*>/.test(t);
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    const m = t.match(/^\s*(?:>\s*)?\[!(secret[^\]]*)\]/i) || t.match(/```ad-secret/i) || (t.match(/```statblock/i) && /gm\s*:\s*true/i.test(src)) || (t.startsWith("```") && /secret/i.test(t));
    if (!m) continue;
    // Scan the whole block for a ^blockid: the rest of a callout, or
    // the rest of the fence.
    let id = "";
    const fenced = t.startsWith("```");
    for (let j = i; j < lines.length; j++) {
      if (j > i) {
        if (fenced && /^\s*```/.test(lines[j])) break;
        if (!fenced && !isQuote(lines[j].trim()) && lines[j].trim() !== "") break;
      }
      const idm = lines[j].match(/\^([\w-]+)/);
      if (idm) { id = idm[1]; break; }
    }
    out.push({ id: id || `s${++n}`, label: typeof (m as any)?.[1] === "string" ? (m as any)[1] : "secret" });
    if (id) n++;
  }
  return out;
}

// Revealed block ids from a note's `revealed` field, which is either a
// list of ids or a list of {id, …} objects depending on the writer.
export function revealedIds(v: unknown): string[] {
  if (!Array.isArray(v)) return [];
  return v.map((r) => (typeof r === "string" ? r : String((r as any)?.id ?? ""))).filter(Boolean);
}
