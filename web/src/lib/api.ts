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

export async function fetchNotes(): Promise<NoteSummary[]> {
  const res = await fetch("/api/v1/notes");
  if (!res.ok) throw new Error(`notes: ${res.status}`);
  const json = (await res.json()) as any;
  // The server returns a paged envelope {count, items}; accept a bare
  // array too so the client does not silently render nothing.
  if (Array.isArray(json)) return json as NoteSummary[];
  return (json.items ?? []) as NoteSummary[];
}

export async function fetchNote(uid: string): Promise<NoteDoc> {
  const res = await fetch(`/api/v1/notes/${encodeURIComponent(uid)}`);
  if (!res.ok) throw new Error(`note: ${res.status}`);
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

export async function saveNote(uid: string, content: string, hash?: string): Promise<NoteDoc> {
  const res = await fetch(`/api/v1/notes/${encodeURIComponent(uid)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content, hash }),
  });
  if (!res.ok) throw new Error(`save: ${res.status}`);
  const json = (await res.json()) as any;
  return {
    uid: json.uid,
    path: json.path,
    name: json.name,
    content: json.content ?? content,
    hash: json.hash ?? "",
  };
}

export async function patchNote(uid: string, fields: Record<string, unknown>): Promise<NoteDoc> {
  const res = await fetch(`/api/v1/notes/${encodeURIComponent(uid)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ fields }),
  });
  if (!res.ok) throw new Error(`patch: ${res.status}`);
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
