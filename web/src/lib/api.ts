export interface NoteSummary {
  uid: string;
  path?: string;
  name?: string;
  [key: string]: unknown;
}

export interface NoteDoc {
  uid: string;
  path: string;
  name: string;
  content: string;
  hash: string;
}

export async function fetchNotes(): Promise<NoteSummary[]> {
  const res = await fetch("/api/v1/notes");
  if (!res.ok) throw new Error(`notes: ${res.status}`);
  return (await res.json()) as NoteSummary[];
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
  };
}
