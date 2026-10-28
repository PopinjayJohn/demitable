// Shared UI state for the global frame (header + sidebar + center +
// contextual rail + footer). Center surfaces publish their right-rail
// content as a snippet so context stays local to the surface that owns
// the data; the frame only decides where it renders.
import type { Snippet } from "svelte";

export type Group = "wiki" | "vtt" | "creator";
export type WikiView = "browser" | "editor" | "player" | "chat";
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

class AppState {
  group: Group = $state<Group>("wiki");
  wikiView: WikiView = $state<WikiView>("browser");
  theme: "light" | "dark" = $state("light");
  proof: Proof = $state("none");
  selectedUid: string | null = $state(null);
  selectedCharacterUid: string | null = $state(null);
  /** Campaign filter for the wiki browser. Null = all campaigns. */
  campaignFilter: string | null = $state(null);
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
    if (this.wikiView === "browser") return "dashboard";
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
// gm:true statblocks. Block id is ^id on the fence/callout or generated.
export interface SecretRef { id: string; label: string; }
export function findSecrets(src: string): SecretRef[] {
  const out: SecretRef[] = [];
  const lines = src.split("\n");
  let n = 0;
  for (const ln of lines) {
    const t = ln.trim();
    const m = t.match(/^\s*(?:>\s*)?\[!(secret[^\]]*)\]/i) || t.match(/```ad-secret/i);
    const gm = t.match(/```statblock/i) && /gm\s*:\s*true/i.test(src);
    if (m || (t.startsWith("```") && /secret/i.test(t))) {
      const idm = ln.match(/\^([\w-]+)/);
      out.push({ id: idm ? idm[1] : `s${++n}`, label: typeof m?.[1] === "string" ? m[1] : "secret" });
    } else if (gm) {
      const idm = ln.match(/\^([\w-]+)/);
      out.push({ id: idm ? idm[1] : `s${++n}`, label: "statblock" });
    }
  }
  return out;
}
