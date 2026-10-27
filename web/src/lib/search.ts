import type { NoteSummary, SearchParams } from "./api";
import { assertZeroVaultReads } from "./dashboard";
import { authHeaders } from "./auth";

export interface SearchData {
  items: NoteSummary[];
  nextCursor?: string;
  count?: number;
  requests: string[];
}

function freeTextToO2(input: string): string {
  const text = input.trim();
  if (text === "") return "";
  const escaped = text.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
  return `name contains "${escaped}" or body contains "${escaped}"`;
}

function buildQuery(params?: SearchParams | string): string {
  if (!params || typeof params === "function") return "";
  if (typeof params === "string") params = { q: params };
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params as Record<string, unknown>)) {
    if (v === undefined || v === null || v === "") continue;
    if (k === "q" || k === "query") {
      sp.set("q", freeTextToO2(String(v)));
      continue;
    }
    if (k === "filter") {
      sp.set("q", String(v));
      continue;
    }
    sp.set(k, String(v));
  }
  const s = sp.toString();
  return s ? "?" + s : "";
}

export async function loadSearch(
  paramsOrFetch?: SearchParams | string | ((input: RequestInfo | URL, init?: RequestInit) => Promise<Response>),
  fetchImpl?: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>,
  _budget?: number,
): Promise<SearchData> {
  let params: SearchParams | string | undefined;
  let fi = fetchImpl;
  if (typeof paramsOrFetch === "function") {
    fi = paramsOrFetch as any;
    params = undefined;
  } else {
    params = paramsOrFetch as any;
  }
  const f = fi ?? fetch;
  const url = `/api/v1/notes${buildQuery(params)}`;
  const res = await f(url, { headers: authHeaders(), credentials: "same-origin" });
  if (!res.ok) throw new Error(`search: ${res.status}`);
  const json = (await res.json()) as any;
  const items: NoteSummary[] = Array.isArray(json) ? json : (json.items ?? []);
  const requests = [url];
  assertZeroVaultReads(requests);
  return {
    items,
    nextCursor: json.next_cursor ?? json.nextCursor ?? json.cursor,
    count: json.count,
    requests,
  };
}

export { searchNotes } from "./api";
export type { SearchParams };
