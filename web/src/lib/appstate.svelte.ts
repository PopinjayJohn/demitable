// Shared UI state: surface, theme, proof, selection, player-eye.
export type Surface = "dashboard" | "editor" | "characters" | "player" | "tabletop" | "chat";
export type Proof = "none" | "greyscale" | "flat-hue" | "protanopia" | "deuteranopia" | "tritanopia";

class AppState {
  surface: Surface = $state<Surface>("dashboard");
  theme: "light" | "dark" = $state("light");
  proof: Proof = $state("none");
  selectedUid: string | null = $state(null);
  playerEye = $state(false);
  showInspector = $state(true);
  showJournal = $state(false);
  sessionId = $state("");
  encounterId = $state("");
  displayMode = $state(false);
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
