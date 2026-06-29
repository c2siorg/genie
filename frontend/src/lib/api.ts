// api.ts — the single HTTP client for the Genie React UI.
//
// Security model (must match the hardened backend + legacy app.js):
//   - Session lives in an HttpOnly cookie → every request sends credentials.
//   - CSRF token lives in MEMORY only (never localStorage — that's XSS-
//     exfiltratable). The server returns it via the `X-CSRF-Token` response
//     header; we capture it and echo it on mutating requests.
//   - 401 means the session expired → callers surface a re-login.

const API_BASE = "/v1";
const CSRF_HEADER = "X-CSRF-Token";
const MUTATING = new Set(["POST", "PUT", "DELETE", "PATCH"]);

let csrfToken: string | null = null;

export function getCsrfToken(): string | null {
  return csrfToken;
}

/** Test seam + login flow: set the in-memory CSRF token. */
export function setCsrfToken(token: string | null): void {
  csrfToken = token;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public body?: unknown,
  ) {
    super(message);
    this.name = "ApiError";
  }
  get isUnauthenticated(): boolean {
    return this.status === 401;
  }
}

export interface ApiOptions {
  method?: string;
  /** JSON body — serialized automatically. */
  json?: unknown;
  headers?: Record<string, string>;
  signal?: AbortSignal;
}

/**
 * apiFetch performs a request against the Genie API, applying the cookie+CSRF
 * security model and capturing any rotated CSRF token from the response.
 * Returns the parsed JSON body (typed by the caller) or throws ApiError.
 */
export async function apiFetch<T = unknown>(
  path: string,
  opts: ApiOptions = {},
): Promise<T> {
  const method = (opts.method ?? "GET").toUpperCase();
  const headers: Record<string, string> = {
    Accept: "application/json",
    ...(opts.headers ?? {}),
  };
  if (opts.json !== undefined) headers["Content-Type"] = "application/json";
  if (MUTATING.has(method) && csrfToken) headers[CSRF_HEADER] = csrfToken;

  const resp = await fetch(API_BASE + path, {
    method,
    headers,
    credentials: "include", // send the HttpOnly session cookie
    body: opts.json !== undefined ? JSON.stringify(opts.json) : undefined,
    signal: opts.signal,
  });

  // Capture a rotated CSRF token if the server sent one.
  const rotated = resp.headers.get(CSRF_HEADER);
  if (rotated) csrfToken = rotated;

  const text = await resp.text();
  const body = text ? safeJSON(text) : undefined;

  if (!resp.ok) {
    const msg =
      (body && typeof body === "object" && "error" in body
        ? String((body as Record<string, unknown>).error)
        : resp.statusText) || `HTTP ${resp.status}`;
    throw new ApiError(resp.status, msg, body);
  }
  return body as T;
}

function safeJSON(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}
