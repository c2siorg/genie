import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";
import { apiFetch, ApiError, setCsrfToken, getCsrfToken } from "./api";

// Typed fetch args so strict-mode tsc lets us inspect mock.calls.
type Init = RequestInit & { headers: Record<string, string>; body?: string };
type FetchArgs = [input: string, init: Init];

function mockFetch(
  status: number,
  body: unknown,
  headers: Record<string, string> = {},
) {
  return vi.fn((..._args: FetchArgs) =>
    Promise.resolve(
      new Response(typeof body === "string" ? body : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json", ...headers },
      }),
    ),
  );
}

describe("apiFetch", () => {
  beforeEach(() => setCsrfToken(null));
  afterEach(() => vi.restoreAllMocks());

  it("always sends credentials (HttpOnly cookie) and the /v1 base", async () => {
    const f = mockFetch(200, { ok: true });
    vi.stubGlobal("fetch", f);
    await apiFetch("/auth/me");
    const [url, init] = f.mock.calls[0];
    expect(url).toBe("/v1/auth/me");
    expect(init.credentials).toBe("include");
  });

  it("captures a rotated CSRF token from the response header", async () => {
    vi.stubGlobal("fetch", mockFetch(200, {}, { "X-CSRF-Token": "tok-123" }));
    await apiFetch("/auth/me");
    expect(getCsrfToken()).toBe("tok-123");
  });

  it("echoes the CSRF token on mutating requests only", async () => {
    setCsrfToken("tok-abc");
    const f = mockFetch(200, {});
    vi.stubGlobal("fetch", f);

    await apiFetch("/commerce/order", { method: "POST", json: { x: 1 } });
    expect(f.mock.calls[0][1].headers["X-CSRF-Token"]).toBe("tok-abc");

    f.mockClear();
    await apiFetch("/auth/me"); // GET
    expect(f.mock.calls[0][1].headers["X-CSRF-Token"]).toBeUndefined();
  });

  it("serializes a JSON body and sets content-type", async () => {
    const f = mockFetch(200, {});
    vi.stubGlobal("fetch", f);
    await apiFetch("/x", { method: "POST", json: { a: 2 } });
    expect(f.mock.calls[0][1].body).toBe(JSON.stringify({ a: 2 }));
    expect(f.mock.calls[0][1].headers["Content-Type"]).toBe("application/json");
  });

  it("throws ApiError with isUnauthenticated on 401", async () => {
    vi.stubGlobal("fetch", mockFetch(401, { error: "unauthenticated" }));
    await expect(apiFetch("/auth/me")).rejects.toMatchObject({
      name: "ApiError",
      status: 401,
    });
    try {
      await apiFetch("/auth/me");
    } catch (e) {
      expect(e).toBeInstanceOf(ApiError);
      expect((e as ApiError).isUnauthenticated).toBe(true);
    }
  });

  it("surfaces a server error message from the body", async () => {
    vi.stubGlobal("fetch", mockFetch(400, { error: "bad gst" }));
    await expect(apiFetch("/merchant", { method: "POST", json: {} })).rejects.toThrow(
      "bad gst",
    );
  });
});
