// Authentication + principal helpers for the webapp (F5a).
//
// Tokens are bearer tokens issued by /api/v1/auth/*. The browser stores
// them in localStorage (native apps use their own store) and injects
// `Authorization: Bearer <token>` on every fetch. The cookie path is left
// to the server; the SPA prefers the explicit header so a principal
// switcher can flip identities without a page reload.

export interface PrincipalClaims {
  kind: string;
  id: string;
  session?: string;
  campaigns?: string[];
  exp?: number;
}

const TOKEN_KEY = "demitable_token";

export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setToken(token: string | null): void {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token);
    else localStorage.removeItem(TOKEN_KEY);
  } catch {
    /* ignore */
  }
}

export function clearToken(): void {
  setToken(null);
}

export function getPrincipal(): PrincipalClaims | null {
  const t = getToken();
  if (!t) return null;
  const parts = t.split(".");
  if (parts.length !== 2) return null;
  try {
    const raw = atob(parts[0].replace(/-/g, "+").replace(/_/g, "/"));
    const c = JSON.parse(raw) as PrincipalClaims;
    if (c.exp && Date.now() / 1000 > c.exp) return null;
    return c;
  } catch {
    return null;
  }
}

export function isGM(): boolean {
  return getPrincipal()?.kind === "gm";
}

export function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
  const h: Record<string, string> = { ...extra };
  const t = getToken();
  if (t) h["Authorization"] = `Bearer ${t}`;
  return h;
}

export async function apiFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
  const headers = authHeaders((init.headers as Record<string, string>) ?? {});
  return fetch(input, { ...init, headers, credentials: "same-origin" });
}

// --- auth flows ---

export async function gmLogin(secret: string): Promise<string> {
  const res = await apiFetch("/api/v1/auth/gm", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ secret }),
  });
  if (!res.ok) throw new Error(`gm login: ${res.status}`);
  const j = (await res.json()) as any;
  if (j.token) setToken(j.token);
  return j.token;
}

export async function loginPlayer(player: string, passphrase: string): Promise<string> {
  const res = await apiFetch("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player, passphrase }),
  });
  if (!res.ok) throw new Error(`login: ${res.status}`);
  const j = (await res.json()) as any;
  if (j.token) setToken(j.token);
  return j.token;
}

export async function joinCampaign(code: string): Promise<string> {
  const res = await apiFetch("/api/v1/auth/join", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });
  if (!res.ok) throw new Error(`join: ${res.status}`);
  const j = (await res.json()) as any;
  if (j.token) setToken(j.token);
  return j.token;
}

export async function joinSession(sessionId: string, code: string): Promise<string> {
  const res = await apiFetch(`/api/v1/sessions/${encodeURIComponent(sessionId)}/join`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });
  if (!res.ok) throw new Error(`session join: ${res.status}`);
  const j = (await res.json()) as any;
  if (j.token) setToken(j.token);
  return j.token;
}

export function logout(): void {
  clearToken();
}
