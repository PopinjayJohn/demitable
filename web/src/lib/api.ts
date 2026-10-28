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
  // Projected view fields (owner-scoped ones present only when allowed).
  type?: string;
  visibility?: string;
  workflow?: string;
  campaign?: string;
  kind?: string;
  player?: string;
  grants?: string[];
  revealed?: unknown;
  discovered?: string[];
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

export async function fetchNote(uid: string, opts: { as?: string } = {}): Promise<NoteDoc> {
  const q = opts.as ? `?as=${encodeURIComponent(opts.as)}` : "";
  const res = await apiFetch(`/api/v1/notes/${encodeURIComponent(uid)}${q}`);
  if (!res.ok) throw await parseError(res);
  const json = (await res.json()) as any;
  const view = (json.view ?? {}) as Record<string, unknown>;
  const pick = (k: string) => (json[k] !== undefined ? json[k] : view[k]);
  return {
    uid: json.uid,
    path: json.path,
    name: json.name,
    content: json.content ?? "",
    hash: json.hash ?? "",
    effectiveVisibility: json.effective_visibility,
    defaulted: json.defaulted,
    type: pick("type") as string | undefined,
    visibility: pick("visibility") as string | undefined,
    workflow: pick("workflow") as string | undefined,
    campaign: pick("campaign") as string | undefined,
    kind: pick("kind") as string | undefined,
    player: pick("player") as string | undefined,
    grants: (pick("grants") ?? undefined) as string[] | undefined,
    revealed: pick("revealed"),
    discovered: (pick("discovered") ?? undefined) as string[] | undefined,
  };
}

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

export async function createNote(input: Record<string, unknown>): Promise<any> {
  const res = await apiFetch(`/api/v1/notes`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!res.ok) throw await parseError(res);
  return await res.json();
}

export async function sendChatMessage(input: { campaign?: string; name?: string; content?: string; visibility?: string; player?: string; channel?: string } & Record<string, unknown>): Promise<any> {
  const body: Record<string, unknown> = {
    type: "message",
    ...input,
  };
  if (body.content === undefined && (input as any).body !== undefined) {
    body.content = (input as any).body;
  }
  return createNote(body);
}

export interface PlayerAccount {
  id: string;
  player: string;
  campaigns: string[];
}

export async function deleteNote(uid: string): Promise<void> {
  const res = await apiFetch(`/api/v1/notes/${encodeURIComponent(uid)}`, { method: "DELETE" });
  if (!res.ok) throw await parseError(res);
}

export async function fetchPlayers(): Promise<PlayerAccount[]> {
  const res = await apiFetch(`/api/v1/players`);
  if (!res.ok) throw await parseError(res);
  const j = await res.json();
  const items = (j.items ?? []) as any[];
  return items.map((a) => ({ id: String(a.id), player: String(a.player ?? a.id), campaigns: (a.campaigns ?? []) as string[] }));
}

export async function createPlayer(name: string, campaign = ""): Promise<{ name: string; campaign: string; join_code: string; expires_at: string }> {
  const res = await apiFetch(`/api/v1/players`, {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name, campaign }),
  });
  if (!res.ok) throw await parseError(res);
  return await res.json();
}

export async function fetchLinks(uid: string): Promise<any> {
  const res = await apiFetch(`/api/v1/notes/${encodeURIComponent(uid)}/links`);
  if (!res.ok) throw await parseError(res);
  return await res.json();
}

export async function fetchJournal(): Promise<any[]> {
  const res = await apiFetch(`/api/v1/journal`);
  if (!res.ok) throw await parseError(res);
  const j = await res.json();
  return Array.isArray(j) ? j : (j.items ?? j.entries ?? []);
}

export async function revertJournal(id: string): Promise<void> {
  const res = await apiFetch(`/api/v1/journal/${encodeURIComponent(id)}/revert`, { method: "POST" });
  if (!res.ok) throw await parseError(res);
}

export async function fetchCharacter(uid: string, expand = true): Promise<any> {
  const res = await apiFetch(`/api/v1/characters/${encodeURIComponent(uid)}${expand ? "?expand=1" : ""}`);
  if (!res.ok) throw await parseError(res);
  return await res.json();
}

export async function createCharacter(input: Record<string, unknown>): Promise<any> {
  const res = await apiFetch(`/api/v1/characters`, {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input),
  });
  if (!res.ok) throw await parseError(res);
  return await res.json();
}

export async function submitCharacter(uid: string): Promise<void> {
  const res = await apiFetch(`/api/v1/characters/${encodeURIComponent(uid)}/submit`, { method: "POST" });
  if (!res.ok) throw await parseError(res);
}

export async function reviewCharacter(uid: string, approve: boolean, rid?: string): Promise<void> {
  const path = rid
    ? `/api/v1/characters/${encodeURIComponent(uid)}/requests/${encodeURIComponent(rid)}/${approve ? "approve" : "reject"}`
    : `/api/v1/characters/${encodeURIComponent(uid)}/${approve ? "approve" : "reject"}`;
  const res = await apiFetch(path, { method: "POST" });
  if (!res.ok) throw await parseError(res);
}

export async function computeValues(formulas: string[], context: Record<string, unknown>): Promise<Record<string, unknown>> {
  const res = await apiFetch(`/api/v1/compute`, {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ formulas, context }),
  });
  if (!res.ok) throw await parseError(res);
  const j = await res.json();
  return (j.values ?? j) as Record<string, unknown>;
}

export async function rollDice(input: { workflow?: string; actor?: string; target?: string; shared?: boolean; expression?: string } & Record<string, unknown>): Promise<any> {
  const res = await apiFetch(`/api/v1/roll`, {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input),
  });
  if (!res.ok) throw await parseError(res);
  return await res.json();
}

export async function createSession(input: Record<string, unknown>): Promise<any> {
  const res = await apiFetch(`/api/v1/sessions`, {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input),
  });
  if (!res.ok) throw await parseError(res);
  return await res.json();
}

export async function fetchSessionLog(sid: string): Promise<any[]> {
  const res = await apiFetch(`/api/v1/sessions/${encodeURIComponent(sid)}/log`);
  if (!res.ok) throw await parseError(res);
  const j = await res.json();
  return Array.isArray(j) ? j : (j.items ?? j.log ?? j.entries ?? []);
}

export async function closeSession(sid: string): Promise<any> {
  const res = await apiFetch(`/api/v1/sessions/${encodeURIComponent(sid)}/close`, { method: "POST" });
  if (!res.ok) throw await parseError(res);
  try { return await res.json(); } catch { return {}; }
}
