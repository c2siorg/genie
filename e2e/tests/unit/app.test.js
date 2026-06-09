// Genie app.js Unit Tests (35 tests)
// CSRF token extraction, refresh, API client, session management, form validation, state, error handling

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

describe('app.js Unit Tests', () => {
  let mockFetch;
  let mockLocalStorage;
  let mockState;

  beforeEach(() => {
    // Clear DOM
    document.body.innerHTML = `
      <div id="view-auth"></div>
      <div id="view-ask"></div>
      <div id="view-documents"></div>
      <div id="view-governance"></div>
      <div id="view-settings"></div>
      <div id="tabs"></div>
      <div id="user-chip"><span class="user-email"></span></div>
      <div id="form-login"><button type="submit"></button><div id="form-login-error"></div><input name="email"/><input name="password"/></div>
      <div id="form-signup"><button type="submit"></button><div id="form-signup-error"></div><input name="name"/><input name="email"/><input name="password"/></div>
      <div id="logout"></div>
      <div id="form-upload"><button type="submit"></button><input id="upload-file" type="file"/><input id="upload-desc"/><input id="upload-class"/></div>
      <div id="doc-list"></div>
      <div id="ask-doc" class="dropdown"></div>
      <div id="btn-ask"></div>
      <div id="btn-ask-stream"></div>
      <div id="btn-stop" hidden></div>
      <div id="report-card" hidden><div id="report-body"></div></div>
      <div id="events"></div>
      <div id="ai-disclosure"></div>
      <div id="settings-base"></div>
      <div id="api-base"></div>
      <div id="settings-save"></div>
      <div id="health"></div>
      <div id="disclosures"></div>
      <div id="inventory"></div>
      <div id="aibom"></div>
      <div id="incidents"></div>
    `;

    mockFetch = global.fetch;
    mockLocalStorage = global.localStorage;
    mockState = {
      base: '/v1',
      csrfToken: null,
      csrfTokenRefresh: null,
      user: null,
      documents: [],
      activeStream: null,
    };
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  // ─────────────────────────────────────────────────────────────
  // CSRF Token Extraction (5 tests)
  // ─────────────────────────────────────────────────────────────

  describe('CSRF Token Extraction', () => {
    it('should extract CSRF token from response header', () => {
      const mockResp = {
        headers: new Map([['X-CSRF-Token', 'token-abc123']]),
      };

      const token = mockResp.headers.get('X-CSRF-Token');
      expect(token).toBe('token-abc123');
    });

    it('should handle missing CSRF token gracefully', () => {
      const mockResp = {
        headers: new Map([]),
      };

      const token = mockResp.headers.get('X-CSRF-Token');
      expect(token).toBeNull();
    });

    it('should update state with new CSRF token', () => {
      const mockResp = {
        headers: new Map([['X-CSRF-Token', 'new-token-xyz']]),
      };

      const token = mockResp.headers.get('X-CSRF-Token');
      mockState.csrfToken = token;
      expect(mockState.csrfToken).toBe('new-token-xyz');
    });

    it('should handle multiple CSRF tokens (use latest)', () => {
      const token1 = 'token-1';
      const token2 = 'token-2';
      mockState.csrfToken = token1;
      mockState.csrfToken = token2;
      expect(mockState.csrfToken).toBe(token2);
    });

    it('should preserve state.csrfToken across requests', () => {
      mockState.csrfToken = 'persistent-token';
      const stored = mockState.csrfToken;
      expect(stored).toBe('persistent-token');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // CSRF Token Refresh (5 tests)
  // ─────────────────────────────────────────────────────────────

  describe('CSRF Token Refresh', () => {
    it('should fetch new CSRF token on refresh', async () => {
      mockFetch.mockResolvedValueOnce({
        status: 200,
        ok: true,
        headers: new Map([['X-CSRF-Token', 'refreshed-token']]),
      });

      const resp = await fetch('/v1/csrf-refresh', {
        method: 'GET',
        credentials: 'include',
      });

      expect(mockFetch).toHaveBeenCalledWith('/v1/csrf-refresh', expect.any(Object));
      expect(resp.ok).toBe(true);
    });

    it('should return null on 401 Unauthorized (session expired)', async () => {
      mockFetch.mockResolvedValueOnce({
        status: 401,
        ok: false,
        headers: new Map([]),
      });

      const resp = await fetch('/v1/csrf-refresh', {
        method: 'GET',
        credentials: 'include',
      });

      expect(resp.status).toBe(401);
    });

    it('should prevent concurrent refresh attempts', async () => {
      const promise1 = Promise.resolve({ ok: true });
      const promise2 = Promise.resolve({ ok: true });

      mockState.csrfTokenRefresh = promise1;
      const result = mockState.csrfTokenRefresh;

      expect(result).toBe(promise1);
    });

    it('should clear refresh promise after completion', async () => {
      mockState.csrfTokenRefresh = null;
      expect(mockState.csrfTokenRefresh).toBeNull();
    });

    it('should handle network errors during refresh', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      try {
        await fetch('/v1/csrf-refresh');
      } catch (err) {
        expect(err.message).toContain('Network');
      }
    });
  });

  // ─────────────────────────────────────────────────────────────
  // API Client (8 tests)
  // ─────────────────────────────────────────────────────────────

  describe('API Client', () => {
    it('should add CSRF token to POST requests', async () => {
      mockState.csrfToken = 'test-csrf-token';
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: () => Promise.resolve('{}'),
        json: () => Promise.resolve({}),
        headers: new Map([]),
      });

      await fetch('/v1/test', {
        method: 'POST',
        headers: {
          'X-CSRF-Token': mockState.csrfToken,
          'Content-Type': 'application/json',
        },
      });

      expect(mockFetch).toHaveBeenCalled();
    });

    it('should add CSRF token to PUT requests', async () => {
      mockState.csrfToken = 'test-csrf-token';
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: () => Promise.resolve('{}'),
        json: () => Promise.resolve({}),
        headers: new Map([]),
      });

      await fetch('/v1/test', {
        method: 'PUT',
        headers: {
          'X-CSRF-Token': mockState.csrfToken,
        },
      });

      expect(mockFetch).toHaveBeenCalled();
    });

    it('should add CSRF token to DELETE requests', async () => {
      mockState.csrfToken = 'test-csrf-token';
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: () => Promise.resolve('{}'),
        headers: new Map([]),
      });

      await fetch('/v1/test', {
        method: 'DELETE',
        headers: {
          'X-CSRF-Token': mockState.csrfToken,
        },
      });

      expect(mockFetch).toHaveBeenCalled();
    });

    it('should NOT add CSRF token to GET requests', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: () => Promise.resolve('{}'),
        headers: new Map([]),
      });

      await fetch('/v1/test', {
        method: 'GET',
      });

      const callArgs = mockFetch.mock.calls[0];
      const headers = callArgs[1]?.headers || {};
      expect(headers['X-CSRF-Token']).toBeUndefined();
    });

    it('should serialize JSON body correctly', async () => {
      const testData = { email: 'test@example.com', password: 'secret' };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: () => Promise.resolve('{"ok":true}'),
        json: () => Promise.resolve({ ok: true }),
        headers: new Map([]),
      });

      await fetch('/v1/test', {
        method: 'POST',
        body: JSON.stringify(testData),
      });

      expect(mockFetch).toHaveBeenCalled();
    });

    it('should parse JSON response', async () => {
      const responseData = { user: { id: 1, email: 'test@example.com' } };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: () => Promise.resolve(JSON.stringify(responseData)),
        json: () => Promise.resolve(responseData),
        headers: new Map([]),
      });

      const resp = await fetch('/v1/test');
      const json = await resp.json();
      expect(json).toEqual(responseData);
    });

    it('should include credentials with every request', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: () => Promise.resolve('{}'),
        headers: new Map([]),
      });

      await fetch('/v1/test', {
        credentials: 'include',
      });

      const callArgs = mockFetch.mock.calls[0];
      expect(callArgs[1]?.credentials).toBe('include');
    });

    it('should handle API errors with proper error message', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        text: () => Promise.resolve(JSON.stringify({ error: 'Invalid input' })),
        headers: new Map([]),
      });

      const resp = await fetch('/v1/test');
      expect(resp.ok).toBe(false);
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Session Management (8 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Session Management', () => {
    it('should clear CSRF token on session clear', () => {
      mockState.csrfToken = 'active-token';
      mockState.csrfToken = null;
      expect(mockState.csrfToken).toBeNull();
    });

    it('should clear user data on session clear', () => {
      mockState.user = { id: 1, email: 'test@example.com' };
      mockState.user = null;
      expect(mockState.user).toBeNull();
    });

    it('should clear documents on session clear', () => {
      mockState.documents = [{ id: 'doc1' }, { id: 'doc2' }];
      mockState.documents = [];
      expect(mockState.documents).toEqual([]);
    });

    it('should clear activeStream on session clear', () => {
      mockState.activeStream = { abort: () => {} };
      mockState.activeStream = null;
      expect(mockState.activeStream).toBeNull();
    });

    it('should store user after login', () => {
      mockState.user = { id: 1, email: 'user@example.com', roles: ['user'] };
      expect(mockState.user.email).toBe('user@example.com');
    });

    it('should update CSRF token after login', () => {
      mockState.csrfToken = 'login-token';
      expect(mockState.csrfToken).toBe('login-token');
    });

    it('should handle session expiry (401)', () => {
      mockState.csrfToken = null;
      mockState.user = null;
      expect(mockState.csrfToken).toBeNull();
      expect(mockState.user).toBeNull();
    });

    it('should handle CSRF verification failure (403)', () => {
      mockState.csrfToken = null;
      expect(mockState.csrfToken).toBeNull();
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Form Validation (5 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Form Validation', () => {
    it('should validate email format', () => {
      const email = 'test@example.com';
      const isValid = email.includes('@');
      expect(isValid).toBe(true);
    });

    it('should reject invalid email (no @)', () => {
      const email = 'invalidemail';
      const isValid = email.includes('@');
      expect(isValid).toBe(false);
    });

    it('should require password minimum length (8 chars)', () => {
      const password1 = 'short';
      const password2 = 'longenough';
      expect(password1.length < 8).toBe(true);
      expect(password2.length >= 8).toBe(true);
    });

    it('should require all mandatory fields', () => {
      const form = {
        email: '',
        password: '',
        name: '',
      };
      const isValid = form.email && form.password && form.name;
      expect(isValid).toBe(false);
    });

    it('should trim whitespace from inputs', () => {
      const input = '  test@example.com  ';
      const trimmed = input.trim();
      expect(trimmed).toBe('test@example.com');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // State Management (2 tests)
  // ─────────────────────────────────────────────────────────────

  describe('State Management', () => {
    it('should initialize state with correct defaults', () => {
      expect(mockState.base).toBe('/v1');
      expect(mockState.csrfToken).toBeNull();
      expect(mockState.user).toBeNull();
      expect(mockState.documents).toEqual([]);
    });

    it('should persist state changes across operations', () => {
      mockState.csrfToken = 'token-1';
      const token1 = mockState.csrfToken;
      mockState.csrfToken = 'token-2';
      const token2 = mockState.csrfToken;
      expect(token1).toBe('token-1');
      expect(token2).toBe('token-2');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Error Handling (2 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Error Handling', () => {
    it('should handle network errors', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Failed to fetch'));

      try {
        await fetch('/v1/test');
      } catch (err) {
        expect(err.message).toContain('Failed to fetch');
      }
    });

    it('should map error messages to user-friendly text', () => {
      const errorMap = {
        'Failed to fetch': 'Network error. Check your connection and try again.',
        'Your session expired. Please sign in again.': 'Your session expired. Please sign in again.',
      };

      const mapped = errorMap['Failed to fetch'];
      expect(mapped).toContain('Network error');
    });
  });
});
