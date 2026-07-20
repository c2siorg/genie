// Thin client for Genie's JSON API. Mirrors the fetch/SSE contract the vanilla
// app.js used, so the backend is unchanged by the UI rewrite.

export type User = { email: string; name?: string; roles?: string[] };

const LS_BASE = 'genie.apibase.v1';
const LS_SESSION = 'genie.session.v1';

export function loadBase(): string {
  if (typeof window === 'undefined') return '/v1';
  return localStorage.getItem(LS_BASE) || '/v1';
}
export function saveBase(v: string) {
  localStorage.setItem(LS_BASE, v);
}

export function loadSession(): { token: string; user: User } | null {
  if (typeof window === 'undefined') return null;
  try {
    const c = JSON.parse(localStorage.getItem(LS_SESSION) || 'null');
    if (c && c.token && c.user) return c;
  } catch {
    /* ignore */
  }
  return null;
}
export function saveSession(token: string | null, user: User | null) {
  if (token && user) localStorage.setItem(LS_SESSION, JSON.stringify({ token, user }));
  else localStorage.removeItem(LS_SESSION);
}

type ApiOpts = RequestInit & { json?: unknown };

export async function api(base: string, token: string | null, path: string, opts: ApiOpts = {}) {
  const headers: Record<string, string> = { Accept: 'application/json', ...(opts.headers as Record<string, string>) };
  if (token) headers['Authorization'] = 'Bearer ' + token;
  const init: RequestInit = { ...opts };
  if (opts.json !== undefined) {
    headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(opts.json);
    delete (init as ApiOpts).json;
  }
  init.headers = headers;
  const url = base.replace(/\/$/, '') + path;
  const resp = await fetch(url, init);
  const text = await resp.text();
  let body: unknown = text;
  try {
    body = JSON.parse(text);
  } catch {
    /* keep as text */
  }
  if (!resp.ok) {
    const b = body as { error?: string };
    throw new Error((b && b.error) || (typeof body === 'string' ? body : resp.statusText));
  }
  return body;
}
