// Vault-wide browser data shared by the left sidebar and the center
// surfaces: the projected note list, visibility counts, and the
// needs-attention set. One loader, many readers — the dashboard's old
// private fetch moves here so the sidebar can show campaigns and
// attention from every surface.
import { loadDashboard, DASHBOARD_BUDGET_MS } from "./dashboard";
import type { NoteSummary } from "./api";

class VaultStore {
  notes = $state<NoteSummary[]>([]);
  attention = $state<NoteSummary[]>([]);
  counts = $state({ party: 0, gm: 0, owner: 0, private: 0, total: 0 });
  status = $state<string>("idle");
  elapsedMs = $state(0);
  error = $state("");

  get campaigns(): Array<[string, number]> {
    const m = new Map<string, number>();
    for (const n of this.notes) {
      const name = String((n as any).campaign ?? "No campaign");
      m.set(name, (m.get(name) ?? 0) + 1);
    }
    return [...m.entries()].sort((a, b) => b[1] - a[1]);
  }

  get players(): string[] {
    return [...new Set(
      this.notes.map((n) => String((n as any).player ?? "")).filter(Boolean),
    )].sort();
  }

  get characters(): NoteSummary[] {
    return this.notes.filter((n) => String((n as any).type ?? "") === "character");
  }

  get reviewQueue(): NoteSummary[] {
    return this.notes.filter((n) => {
      if (String((n as any).type ?? "") !== "character") return false;
      const w = String((n as any).workflow ?? "");
      return w === "draft" || w === "submitted";
    });
  }

  async load(asPrincipal = "", fetchImpl: typeof fetch = fetch): Promise<void> {
    this.status = "loading";
    this.error = "";
    try {
      const data = await loadDashboard(fetchImpl, DASHBOARD_BUDGET_MS, asPrincipal);
      this.notes = data.notes;
      this.attention = data.attention;
      this.counts = data.counts;
      this.elapsedMs = data.elapsedMs;
      this.status = "loaded";
    } catch (e) {
      this.error = e instanceof Error ? e.message : String(e);
      this.status = "failed";
    }
  }
}

export const vaultStore = new VaultStore();
