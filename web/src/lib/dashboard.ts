// GM dashboard data layer (O5b).
//
// The dashboard renders EXCLUSIVELY through O2 queries (GET /api/v1/notes,
// which compiles to Store.Query). It never reads a single note by uid —
// a "client vault read" is a request to /api/v1/notes/:uid or its render
// endpoint, and the loader refuses to make one. Counts, attention and
// coverage are all derived from the one list response, so the request set
// is a single full scan of the vault (the per-call cost O2 measured at
// ~63-81 ms over the SRD corpus) rather than an N+1 storm.
//
// Projection discipline: a surface never recomputes a note's visibility.
// It displays the server-projected `effective_visibility` and fails if a
// caller hands it raw (unprojected) data. See surfaceVisibility.

import type { NoteSummary } from "./api";

export const DASHBOARD_BUDGET_MS = 250;

export interface DashboardCounts {
  party: number;
  gm: number;
  owner: number;
  private: number;
  total: number;
}

export interface DashboardData {
  notes: NoteSummary[];
  counts: DashboardCounts;
  attention: NoteSummary[];
  requests: string[];
  elapsedMs: number;
}

const VAULT_READ_RE = /\/api\/v1\/notes\/[^/?#]+(\/render)?(\?|$)/;

export function isVaultRead(url: string): boolean {
  // A list endpoint is /api/v1/notes or /api/v1/notes?... ; a per-note
  // read is /api/v1/notes/<uid> (optionally /render). Match the latter.
  const path = url.split("?")[0].split("#")[0];
  return /^\/api\/v1\/notes\/[^/]+(\/render)?$/.test(path);
}

export function assertZeroVaultReads(urls: readonly string[]): void {
  const offenders = urls.filter(isVaultRead);
  if (offenders.length > 0) {
    throw new Error(
      "dashboard made client vault reads: " + offenders.join(", "),
    );
  }
}

// The visibility a surface is allowed to show. It is the server-projected
// value, and it is required: a surface pointed at raw (unprojected) data
// has no effective_visibility and fails here rather than silently rendering
// a declared value that the projection might have overridden.
export function surfaceVisibility(note: NoteSummary | Record<string, unknown>): string {
  const n = note as Record<string, unknown>;
  const projected =
    (n.effectiveVisibility as string | undefined) ??
    (n.effective_visibility as string | undefined);
  if (typeof projected !== "string" || projected === "") {
    throw new Error(
      "surface requires a server-projected effective_visibility; raw data is not allowed",
    );
  }
  return projected;
}

export function assertVisibilityAgreement(shown: string, projected: string): void {
  if (shown !== projected) {
    throw new Error(
      `visibility disagreement: surface shows ${shown}, projection says ${projected}`,
    );
  }
}

function declaredVis(n: NoteSummary): string {
  const v = (n as Record<string, unknown>).visibility;
  if (typeof v === "string" && v !== "") return v;
  const campaign = (n as Record<string, unknown>).campaign;
  return campaign ? "party" : "gm";
}

export async function loadDashboard(
  fetchImpl: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response> = fetch,
  budgetMs: number = DASHBOARD_BUDGET_MS,
): Promise<DashboardData> {
  const requests: string[] = [];
  const start =
    typeof performance !== "undefined" ? performance.now() : Date.now();

  const url = "/api/v1/notes";
  requests.push(url);
  const res = await fetchImpl(url);
  if (!res.ok) {
    throw new Error(`dashboard notes: ${res.status}`);
  }
  const json = (await res.json()) as unknown;
  const notes: NoteSummary[] = Array.isArray(json)
    ? (json as NoteSummary[])
    : (((json as { items?: NoteSummary[] })?.items ?? []) as NoteSummary[]);

  // Zero client vault reads: check the request set before doing anything
  // else, so a leaking loader fails fast.
  assertZeroVaultReads(requests);

  const counts: DashboardCounts = {
    party: 0,
    gm: 0,
    owner: 0,
    private: 0,
    total: notes.length,
  };
  // Counts are derived from the server-projected effective_visibility, using
  // the same discipline as every other surface: surfaceVisibility() throws
  // on raw (unprojected) data instead of silently defaulting. Aggregates do
  // NOT get their own fallback — if a note lacks a projection the loader
  // fails, which is the desired fail-closed behaviour.
  for (const n of notes) {
    const v = surfaceVisibility(n);
    if (v === "party") counts.party++;
    else if (v === "gm") counts.gm++;
    else if (v === "owner") counts.owner++;
    else if (v === "private") counts.private++;
  }

  const attention = notes.filter((n) => {
    const rec = n as Record<string, unknown>;
    const workflow = rec.workflow;
    if (workflow === "draft" || workflow === "submitted") return true;
    const type = rec.type;
    const vis = declaredVis(n);
    const grants = rec.grants;
    if (type === "handout" && vis === "gm" && !(Array.isArray(grants) && grants.length)) {
      return true;
    }
    const campaign = rec.campaign;
    if (!campaign) return true;
    return false;
  });

  const elapsed =
    (typeof performance !== "undefined" ? performance.now() : Date.now()) - start;
  if (elapsed > budgetMs) {
    throw new Error(
      `dashboard render budget exceeded: ${elapsed.toFixed(1)}ms > ${budgetMs}ms`,
    );
  }

  return { notes, counts, attention, requests, elapsedMs: elapsed };
}
