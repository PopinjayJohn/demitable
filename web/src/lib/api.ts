import { authHeaders, apiFetch } from "./auth";

export interface NoteSummary {
  uid: string;
  path?: string;
  name?: string;
  type?: string;
  campaign?: string;
  workflow?: string;
  visibility?: string;
  effective_visibility?: string;
  effectiveVisibility?: string;
  defaulted?: boolean;
  grants?: string[];
  [key: string]: unknown;
}

export interface NoteDoc {
  uid: string;
  path: string;
  name: string;
  content: string;
  hash: string;
  effectiveVisibility?: string;
  defaulted?: boolean;
}

export interface SearchParams {
  q?: string;
  query?: string;
  filter?: string;
  type?: string;
  cursor?: string;
  limit?: number;
  campaign?: string;
  sort?: string;
  [key: string]: unknown;
}

export function freeTextToO2(input: string): string {
  const text = input.trim();
  if (text === "") return "";
  // Escape backslashes and double quotes per the O2 string rules.
  const escaped = text.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
  return `name contains "${escaped}" or body contains "${escaped}"`;
}

function buildQuery(params?: SearchParams | string): string {
  if (!params) return "";
  if (typeof params === "string") params = { q: params };
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === "") continue;
    if (k === "q" || k === "query") {
      // Translate free text into a valid O2 expression client-side.
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

async function parseError(res: Response): Promise<Error> {
  let msg = `${res.status}`;
  try {
    const j = await res.json();
    if (j?.error?.message) msg = j.error.message;
    else if (j?.message) msg = j.message;
    else if (typeof j?.error === "string") msg = j.error;
  } catch {
    /* ignore */
  }
  const e = new Error(msg);
  (e as any).status = res.status;
  return e;
}

export async function fetchNotes(params?: SearchParams | string): Promise<NoteSummary[]> {
  const res = await apiFetch(`/api/v1/notes${buildQuery(params)}`);
  if (!res.ok) throw await parseError(res);
  const json = (await res.json()) as any;
  if (Array.isArray(json)) return json as NoteSummary[];
  return (json.items ?? []) as NoteSummary[];
}

export async function searchNotes(params: SearchParams = {}): Promise<{ items: NoteSummary[]; nextCursor?: string; count?: number; requests?: string[] }> {
  const url = `/api/v1/notes${buildQuery(params)}`;
  const res = await apiFetch(url);
  if (!res.ok) throw await parseError(res);
  const json = (await res.json()) as any;
  const items: NoteSummary[] = Array.isArray(json) ? (json as NoteSummary[]) : (json.items ?? []);
  return { items, nextCursor: json.next_cursor ?? json.nextCursor ?? json.cursor, count: json.count, requests: [url] };
}

// alias used by some callers
export const loadSearch = searchNotes;

export async function fetchNote(uid: string): Promise<NoteDoc> {
  const res = await apiFetch(`/api/v1/notes/${encodeURIComponent(uid)}`);
  if (!res.ok) throw await parseError(res);
  const json = (await res.json()) as any;
  return {
    uid: json.uid,
    path: json.path,
    name: json.name,
    content: json.content ?? "",
    hash: json.hash ?? "",
    effectiveVisibility: json.effective_visibility,
    defaulted: json.defaulted,
  };
}

export async function fetchNoteRender(uid: string): Promise<string> {
  const res = await apiFetch(`/api/v1/notes/${encodeURIComponent(uid)}/render`);
  if (!res.ok) throw await parseError(res);
  const json = (await res.json()) as any;
  return (json.html ?? json.content ?? "") as string;
}

// alias
export const fetchRender = fetchNoteRender;
export const renderNote = fetchNoteRender;

export async function saveNote(uid: string, content: string, hash?: string): Promise<NoteDoc> {
  const body: Record<string, unknown> = { content };
  if (hash) body.hash = hash;
  const res = await apiFetch(`/api/v1/notes/${encodeURIComponent(uid)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw await parseError(res);
  const json = (await res.json()) as any;
  return {
    uid: json.uid,
    path: json.path,
    name: json.name,
    content: json.content ?? content,
    hash: json.hash ?? "",
    effectiveVisibility: json.effective_visibility,
  };
}

export async function patchNote(uid: string, fields: Record<string, unknown>): Promise<NoteDoc> {
  const res = await apiFetch(`/api/v1/notes/${encodeURIComponent(uid)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ fields }),
  });
  if (!res.ok) throw await parseError(res);
  const json = (await res.json()) as any;
  return {
    uid: json.uid,
    path: json.path,
    name: json.name,
    content: json.content ?? "",
    hash: json.hash ?? "",
    effectiveVisibility: json.effective_visibility,
    defaulted: json.defaulted,
  };
}
