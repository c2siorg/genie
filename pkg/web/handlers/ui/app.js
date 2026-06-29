// Genie UI — vanilla JS, no framework. Pairs with index.html + styles.css.
//
// Session management uses HttpOnly cookies (handled automatically by the browser)
// and CSRF tokens (transmitted via X-CSRF-Token header or form field).
//
// State lives in memory; refresh loses it. Server session persists via HttpOnly
// cookie (browser auto-sends it, JS cannot access). CSRF tokens are generated
// on login/signup and refreshed on each API response.
//
// ─── Session & Auth Strategy ──────────────────────────────────────────────────
//
// 1. User logs in → server creates HttpOnly cookie + returns CSRF token
// 2. Client stores CSRF token in memory (state.csrfToken)
// 3. On subsequent requests:
//    - Browser auto-sends HttpOnly cookie with request
//    - Client includes CSRF token in X-CSRF-Token header
//    - Server validates both
// 4. On 401 Unauthorized (session expired):
//    - Redirect to login
//    - Clear client-side state
// 5. On 403 Forbidden (CSRF token invalid):
//    - Attempt refresh via a dedicated endpoint
//    - If refresh succeeds, retry original request
//    - If refresh fails, redirect to login

(function () {
  'use strict';

  // -----------------------------------------------------------------
  // state
  // -----------------------------------------------------------------

  const state = {
    base: '/v1',
    csrfToken: null,      // Current CSRF token (updated after each request)
    csrfTokenRefresh: null, // Pending refresh attempt (prevents double-refresh)
    user: null,           // Current user object
    documents: [],        // Uploaded documents (lost on refresh)
    activeStream: null,   // Current SSE connection controller
  };

  // -----------------------------------------------------------------
  // CSRF token management
  // -----------------------------------------------------------------

  /**
   * extractCSRFTokenFromResponse extracts the CSRF token from the response.
   * The server can return it via the X-CSRF-Token response header.
   *
   * If present, update state.csrfToken. The client must include this token
   * in all subsequent POST/PUT/DELETE requests.
   */
  function extractCSRFTokenFromResponse(resp) {
    const token = resp.headers.get('X-CSRF-Token');
    if (token) {
      state.csrfToken = token;
    }
  }

  /**
   * refreshCSRFToken fetches a new CSRF token from the server.
   *
   * This is called when the current token is invalid (403 Forbidden).
   * The endpoint should return a fresh token in the response header.
   *
   * Returns the new token if successful, or null if the refresh failed.
   */
  async function refreshCSRFToken() {
    // Prevent multiple concurrent refresh attempts (race condition).
    if (state.csrfTokenRefresh) {
      return state.csrfTokenRefresh;
    }

    const refreshPromise = (async () => {
      try {
        const resp = await fetch(state.base.replace(/\/$/, '') + '/csrf-refresh', {
          method: 'GET',
          credentials: 'include',  // Include cookies in the request
        });

        // If CSRF refresh fails (401 = session expired), clear state and redirect.
        if (resp.status === 401) {
          clearSession();
          leaveApp();
          return null;
        }

        // If CSRF refresh succeeded, extract the new token.
        if (resp.ok) {
          extractCSRFTokenFromResponse(resp);
          return state.csrfToken;
        }

        // Any other status is a hard failure.
        return null;
      } catch (err) {
        console.error('CSRF token refresh failed:', err);
        return null;
      } finally {
        state.csrfTokenRefresh = null;
      }
    })();

    state.csrfTokenRefresh = refreshPromise;
    return refreshPromise;
  }

  /**
   * clearSession removes all client-side session state.
   * Note: The HttpOnly cookie is cleared by the server (via logout endpoint).
   */
  function clearSession() {
    state.csrfToken = null;
    state.user = null;
    state.documents = [];
  }

  // -----------------------------------------------------------------
  // API request interceptor
  // -----------------------------------------------------------------

  /**
   * api(path, opts) is the main HTTP client. It handles:
   * - Adding CSRF token to POST/PUT/DELETE requests
   * - Extracting new CSRF tokens from responses
   * - Retrying on CSRF token expiry (403)
   * - Redirecting on session expiry (401)
   *
   * Options:
   *   method: GET, POST, PUT, DELETE (default: GET)
   *   json: object to send as JSON body
   *   headers: custom headers (merged with defaults)
   *   body: raw body (if json not used)
   *   credentials: 'include' to send cookies (always set by default)
   *
   * Example:
   *   const result = await api('/users/login', {
   *     method: 'POST',
   *     json: { email: 'user@example.com', password: '...' }
   *   });
   */
  async function api(path, opts = {}) {
    const method = opts.method || 'GET';
    const headers = Object.assign(
      {
        'Accept': 'application/json',
        'Content-Type': 'application/json',
      },
      opts.headers || {}
    );

    // Add CSRF token to unsafe methods (POST, PUT, DELETE, PATCH).
    if (['POST', 'PUT', 'DELETE', 'PATCH'].includes(method) && state.csrfToken) {
      headers['X-CSRF-Token'] = state.csrfToken;
    }

    // Build request body.
    let body = null;
    if (opts.json !== undefined) {
      body = JSON.stringify(opts.json);
    } else if (opts.body !== undefined) {
      body = opts.body;
    }

    // Always include credentials (cookies) in the request.
    const fetchOpts = Object.assign({}, opts, {
      method,
      headers,
      body,
      credentials: 'include',  // Send HttpOnly cookie with every request
    });

    // Delete opts.json so fetch() doesn't see it.
    delete fetchOpts.json;

    const url = state.base.replace(/\/$/, '') + path;

    try {
      const resp = await fetch(url, fetchOpts);

      // Extract new CSRF token from response header (if present).
      extractCSRFTokenFromResponse(resp);

      // Handle 401 Unauthorized (session expired).
      if (resp.status === 401) {
        clearSession();
        leaveApp();
        throw new Error('Your session expired. Please sign in again.');
      }

      // Handle 403 Forbidden (CSRF token invalid or missing).
      if (resp.status === 403) {
        // Attempt to refresh CSRF token once.
        const newToken = await refreshCSRFToken();
        if (newToken) {
          // Retry the original request with the new token.
          return api(path, opts);
        }
        // If refresh failed, treat as session expiry.
        clearSession();
        leaveApp();
        throw new Error('CSRF verification failed. Please sign in again.');
      }

      // Parse response body.
      const text = await resp.text();
      let body = text;
      try {
        body = JSON.parse(text);
      } catch (_) {
        /* keep as text */
      }

      // Handle other error status codes.
      if (!resp.ok) {
        const message = (body && body.error) || (typeof body === 'string' ? body : resp.statusText);
        throw new Error(message);
      }

      return body;
    } catch (err) {
      // Rethrow the error for the caller to handle.
      throw err;
    }
  }

  // -----------------------------------------------------------------
  // tiny helpers
  // -----------------------------------------------------------------

  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

  function show(el) {
    el.hidden = false;
    el.style.display = '';
  }

  function hide(el) {
    el.hidden = true;
    el.style.display = 'none';
  }

  function flash(node, type, message) {
    node.textContent = message || '';
    node.dataset.flash = type || '';
  }

  // -----------------------------------------------------------------
  // navigation
  // -----------------------------------------------------------------

  const views = {
    auth: $('#view-auth'),
    ask: $('#view-ask'),
    documents: $('#view-documents'),
    governance: $('#view-governance'),
    settings: $('#view-settings'),
  };

  function activateTab(name) {
    Object.entries(views).forEach(([k, el]) => {
      if (k !== 'auth') (k === name ? show(el) : hide(el));
    });
    $$('.tab').forEach(t => {
      const isActive = t.dataset.tab === name;
      t.classList.toggle('active', isActive);
      t.setAttribute('aria-selected', isActive ? 'true' : 'false');
      t.setAttribute('tabindex', isActive ? '0' : '-1');
    });
    if (name === 'documents') refreshDocs();
    if (name === 'governance') refreshGovernance();
  }

  // Tab click handler
  $('#tabs').addEventListener('click', e => {
    const tab = e.target.closest('.tab');
    if (tab) activateTab(tab.dataset.tab);
  });

  // Tab keyboard navigation (Arrow keys)
  $('#tabs').addEventListener('keydown', e => {
    if (!e.target.classList.contains('tab')) return;
    const tabs = $$('.tab');
    const current = tabs.indexOf(e.target);
    let next = -1;

    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      next = (current + 1) % tabs.length;
      e.preventDefault();
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      next = (current - 1 + tabs.length) % tabs.length;
      e.preventDefault();
    } else if (e.key === 'Home') {
      next = 0;
      e.preventDefault();
    } else if (e.key === 'End') {
      next = tabs.length - 1;
      e.preventDefault();
    }

    if (next >= 0) {
      activateTab(tabs[next].dataset.tab);
      tabs[next].focus();
    }
  });

  // Global Escape key handler
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape') {
      // Stop streaming if active
      if (state.activeStream) {
        state.activeStream.abort();
        e.preventDefault();
      }
      // Close report card on Escape
      const reportCard = $('#report-card');
      if (reportCard && !reportCard.hidden) {
        hide(reportCard);
        e.preventDefault();
      }
    }
  });

  // -----------------------------------------------------------------
  // auth
  // -----------------------------------------------------------------

  const authTabs = $$('.tab-inline');
  authTabs.forEach(t => {
    t.addEventListener('click', () => {
      authTabs.forEach(x => x.classList.toggle('active', x === t));
      const which = t.dataset.auth;
      $('#form-login').hidden = which !== 'login';
      $('#form-signup').hidden = which !== 'signup';
    });
  });

  $('#form-login').addEventListener('submit', async (e) => {
    e.preventDefault();
    const form = e.target;
    const btn = form.querySelector('button[type="submit"]');
    const errorEl = $('#form-login-error');
    const fd = new FormData(form);

    // Clear previous errors
    errorEl.textContent = '';
    hide(errorEl);
    clearFieldErrors(form);

    // Validate fields
    const email = fd.get('email').trim();
    const password = fd.get('password');

    if (!email) {
      showFieldError('email-error', 'Email is required');
      return;
    }
    if (!email.includes('@')) {
      showFieldError('email-error', 'Please enter a valid email');
      return;
    }
    if (!password) {
      showFieldError('password-error', 'Password is required');
      return;
    }

    // Show loading state
    setButtonLoading(btn, true);
    form.style.opacity = '0.6';
    form.style.pointerEvents = 'none';

    try {
      const out = await api('/users/login', {
        method: 'POST',
        json: { email, password },
      });

      // On successful login:
      // - Server sets HttpOnly cookie (browser auto-sends it)
      // - Response includes CSRF token
      // - Store user and token in state
      state.user = out.user;
      if (out.csrf_token) {
        state.csrfToken = out.csrf_token;
      }

      enterApp();
    } catch (err) {
      const message = getUserMessage(err);
      errorEl.textContent = message;
      show(errorEl);
      setButtonLoading(btn, false);
      form.style.opacity = '1';
      form.style.pointerEvents = 'auto';
    }
  });

  $('#form-signup').addEventListener('submit', async (e) => {
    e.preventDefault();
    const form = e.target;
    const btn = form.querySelector('button[type="submit"]');
    const errorEl = $('#form-signup-error');
    const fd = new FormData(form);

    // Clear previous errors
    errorEl.textContent = '';
    hide(errorEl);
    clearFieldErrors(form);

    // Validate fields
    const name = fd.get('name').trim();
    const email = fd.get('email').trim();
    const password = fd.get('password');

    if (!name) {
      showFieldError('name-error', 'Name is required');
      return;
    }
    if (!email) {
      showFieldError('signup-email-error', 'Email is required');
      return;
    }
    if (!email.includes('@')) {
      showFieldError('signup-email-error', 'Please enter a valid email');
      return;
    }
    if (!password) {
      showFieldError('signup-password-error', 'Password is required');
      return;
    }
    if (password.length < 8) {
      showFieldError('signup-password-error', 'Password must be at least 8 characters');
      return;
    }

    // Show loading state
    setButtonLoading(btn, true);
    form.style.opacity = '0.6';
    form.style.pointerEvents = 'none';

    try {
      const out = await api('/users', {
        method: 'POST',
        json: { email, name, password },
      });

      // On successful signup:
      // - Server sets HttpOnly cookie (browser auto-sends it)
      // - Response includes CSRF token
      // - Store user and token in state
      state.user = out.user;
      if (out.csrf_token) {
        state.csrfToken = out.csrf_token;
      }

      enterApp();
    } catch (err) {
      const message = getUserMessage(err);
      errorEl.textContent = message;
      show(errorEl);
      setButtonLoading(btn, false);
      form.style.opacity = '1';
      form.style.pointerEvents = 'auto';
    }
  });

  $('#logout').addEventListener('click', async () => {
    try {
      // Call logout API to clear server session.
      // The server will delete the HttpOnly cookie and invalidate the session.
      await api('/users/logout', { method: 'POST' });
    } catch (err) {
      // Even if logout fails, clear client state and redirect.
      console.error('Logout error (continuing):', err);
    }

    // Clear client-side state.
    clearSession();
    leaveApp();
  });

  function enterApp() {
    hide(views.auth);
    show($('#tabs'));
    show($('#user-chip'));
    if (state.user) {
      $('#user-chip .user-email').textContent = state.user.email;
    }
    activateTab('ask');
    refreshDocs();
    refreshHealth();
  }

  function leaveApp() {
    show(views.auth);
    hide($('#tabs'));
    hide($('#user-chip'));
    Object.entries(views).forEach(([k, el]) => {
      if (k !== 'auth') hide(el);
    });
  }

  // -----------------------------------------------------------------
  // documents
  // -----------------------------------------------------------------

  $('#form-upload').addEventListener('submit', async (e) => {
    e.preventDefault();
    const form = e.target;
    const btn = form.querySelector('button[type="submit"]');
    const file = $('#upload-file').files[0];
    const desc = $('#upload-desc').value;
    const cls = $('#upload-class').value;
    if (!file) {
      alert('Please select a file');
      return;
    }

    setButtonLoading(btn, true);
    form.style.opacity = '0.6';
    form.style.pointerEvents = 'none';

    const url = `/documents?description=${encodeURIComponent(desc)}&classification=${encodeURIComponent(cls)}`;
    try {
      const body = await file.arrayBuffer();
      const out = await api(url, {
        method: 'POST',
        body,
        headers: { 'Content-Type': file.type || 'application/octet-stream' },
      });
      state.documents.push({
        id: out.id,
        description: desc || '(no description)',
        classification: out.classification,
        kek_id: out.kek_id,
      });
      renderDocs();
      form.reset();
      setButtonLoading(btn, false);
      form.style.opacity = '1';
      form.style.pointerEvents = 'auto';
    } catch (err) {
      const message = getUserMessage(err);
      alert(message);
      setButtonLoading(btn, false);
      form.style.opacity = '1';
      form.style.pointerEvents = 'auto';
    }
  });

  async function refreshDocs() {
    // Genie has no GET /documents list endpoint yet; we mirror local uploads.
    renderDocs();
  }

  function renderDocs() {
    const tbody = $('#doc-list');

    // Hide skeletons
    $$('.skeleton-row', tbody).forEach(el => hide(el));

    // Remove old document rows
    $$('tr:not(.skeleton-row)', tbody).forEach(el => el.remove());

    if (state.documents.length === 0) {
      const tr = document.createElement('tr');
      tr.innerHTML = '<td colspan="4" class="muted">No uploads yet in this session.</td>';
      tbody.appendChild(tr);
    } else {
      for (const d of state.documents) {
        const tr = document.createElement('tr');
        tr.innerHTML = `
          <td><code>${escapeHTML(d.id.slice(0, 8))}…</code></td>
          <td>${escapeHTML(d.description)}</td>
          <td><span class="badge">${escapeHTML(d.classification)}</span></td>
          <td><code>${escapeHTML(d.kek_id || '')}</code></td>`;
        tbody.appendChild(tr);
      }
    }
    // Also re-populate the ask-doc dropdown.
    const sel = $('#ask-doc');
    const prev = sel.value;
    sel.innerHTML = '';
    if (state.documents.length === 0) {
      sel.innerHTML = '<option value="">No documents — upload one first</option>';
    } else {
      for (const d of state.documents) {
        const opt = document.createElement('option');
        opt.value = d.id;
        opt.textContent = `${d.description} — ${d.id.slice(0, 8)}…`;
        sel.appendChild(opt);
      }
      if (prev && state.documents.find(d => d.id === prev)) sel.value = prev;
    }
  }

  // -----------------------------------------------------------------
  // ask (sync + streaming)
  // -----------------------------------------------------------------

  function clearEvents() {
    $('#events').innerHTML = '';
  }

  function addEvent(kind, data, klass) {
    const li = document.createElement('li');
    if (klass) li.classList.add(klass);
    li.innerHTML = `<span class="event-kind">${escapeHTML(kind)}</span><span class="event-data"></span>`;
    li.querySelector('.event-data').textContent =
      typeof data === 'string' ? data : JSON.stringify(data);
    const list = $('#events');
    list.appendChild(li);
    list.scrollTop = list.scrollHeight;
  }

  function setBanner(text) {
    const el = $('#ai-disclosure');
    if (text) {
      el.textContent = text;
      show(el);
    } else {
      hide(el);
    }
  }

  function showReport(text) {
    $('#report-body').textContent = text;
    show($('#report-card'));
  }

  $('#btn-ask').addEventListener('click', async () => {
    const docID = $('#ask-doc').value;
    const q = $('#ask-question').value.trim();
    if (!docID) {
      alert('Please select a document first.');
      return;
    }
    if (!q) {
      alert('Please type a question.');
      return;
    }
    clearEvents();
    hide($('#report-card'));
    setBanner('');
    addEvent('request', 'POST /v1/ask');
    const btn = $('#btn-ask');
    setButtonLoading(btn, true);
    try {
      const out = await api('/ask', {
        method: 'POST',
        json: { question: q, document_id: docID },
      });
      if (out.ai_disclosure) setBanner(out.ai_disclosure);
      addEvent('trace', out.trace_id || '');
      addEvent('report', '(received)', 'event-report');
      showReport(out.report || '');
    } catch (err) {
      addEvent('error', err.message, 'event-error');
    } finally {
      setButtonLoading(btn, false);
    }
  });

  $('#btn-ask-stream').addEventListener('click', () => {
    const docID = $('#ask-doc').value;
    const q = $('#ask-question').value.trim();
    if (!docID) {
      alert('Please select a document first.');
      return;
    }
    if (!q) {
      alert('Please type a question.');
      return;
    }
    clearEvents();
    hide($('#report-card'));
    setBanner('');

    // EventSource doesn't allow POST or custom headers; we fall back to a
    // fetch-streamed reader and parse SSE frames manually.
    const ctrl = new AbortController();
    state.activeStream = ctrl;
    show($('#btn-stop'));
    addEvent('request', 'POST /v1/ask/stream');

    fetch(state.base.replace(/\/$/, '') + '/ask/stream', {
      method: 'POST',
      signal: ctrl.signal,
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
        'X-CSRF-Token': state.csrfToken || '',
      },
      body: JSON.stringify({ question: q, document_id: docID }),
      credentials: 'include',  // Include HttpOnly cookie
    })
      .then(async (resp) => {
        // Check for auth errors.
        if (resp.status === 401) {
          addEvent('error', 'Session expired. Please sign in again.', 'event-error');
          clearSession();
          leaveApp();
          return;
        }

        if (resp.status === 403) {
          addEvent('error', 'CSRF verification failed. Please sign in again.', 'event-error');
          clearSession();
          leaveApp();
          return;
        }

        if (!resp.ok || !resp.body) {
          addEvent('error', 'http ' + resp.status, 'event-error');
          return;
        }

        const reader = resp.body.getReader();
        const dec = new TextDecoder();
        let buf = '';
        while (true) {
          const { value, done } = await reader.read();
          if (done) break;
          buf += dec.decode(value, { stream: true });
          let idx;
          while ((idx = buf.indexOf('\n\n')) >= 0) {
            const frame = buf.slice(0, idx);
            buf = buf.slice(idx + 2);
            parseSSE(frame);
          }
        }
      })
      .catch((err) => {
        if (err.name === 'AbortError') return;
        addEvent('error', err.message || String(err), 'event-error');
      })
      .finally(() => {
        hide($('#btn-stop'));
        state.activeStream = null;
      });
  });

  $('#btn-stop').addEventListener('click', () => {
    if (state.activeStream) state.activeStream.abort();
  });

  function parseSSE(frame) {
    const lines = frame.split('\n');
    let event = 'message';
    let data = '';
    for (const line of lines) {
      if (line.startsWith('event:')) event = line.slice(6).trim();
      else if (line.startsWith('data:')) data += line.slice(5).trim();
    }
    if (event === 'ai_disclosure') return setBanner(data);
    if (event === 'report') {
      addEvent('report', '(received)', 'event-report');
      showReport(data);
      return;
    }
    if (event === 'agent.handle') {
      try {
        data = JSON.stringify(JSON.parse(data));
      } catch (_) {
        /* leave as string */
      }
    }
    addEvent(event, data);
  }

  // -----------------------------------------------------------------
  // governance
  // -----------------------------------------------------------------

  async function refreshGovernance() {
    try {
      const d = await api('/disclosures');
      $('#disclosures').textContent = JSON.stringify(d, null, 2);
    } catch (e) {
      $('#disclosures').textContent = 'error: ' + e.message;
    }

    const isAdmin = (state.user && state.user.roles || []).includes('admin');
    if (isAdmin) {
      try {
        const inv = await api('/ai-inventory');
        $('#inventory').textContent = JSON.stringify(inv, null, 2);
      } catch (e) {
        $('#inventory').textContent = 'error: ' + e.message;
      }
      try {
        const bom = await api('/aibom');
        $('#aibom').textContent = JSON.stringify(bom, null, 2);
      } catch (e) {
        $('#aibom').textContent = 'error: ' + e.message;
      }
      try {
        const inc = await api('/incidents?limit=20');
        $('#incidents').textContent = JSON.stringify(inc, null, 2);
      } catch (e) {
        $('#incidents').textContent = 'error: ' + e.message;
      }
    }
  }

  // -----------------------------------------------------------------
  // settings + health
  // -----------------------------------------------------------------

  $('#settings-base').value = state.base;
  $('#api-base').textContent = state.base;

  $('#settings-save').addEventListener('click', () => {
    const v = $('#settings-base').value.trim();
    if (!v) return;
    state.base = v;
    $('#api-base').textContent = v;
    refreshHealth();
  });

  async function refreshHealth() {
    const url = state.base.replace(/\/v1$/, '') + '/readyz';
    try {
      const resp = await fetch(url);
      const body = await resp.text();
      $('#health').textContent = `Readiness ${resp.status}: ${body.trim()}`;
    } catch (e) {
      $('#health').textContent = 'Readiness probe unreachable: ' + e.message;
    }
  }

  // -----------------------------------------------------------------
  // util
  // -----------------------------------------------------------------

  function escapeHTML(s) {
    return String(s).replace(/[&<>"']/g, c =>
      ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])
    );
  }

  // Error handling helpers
  const ERROR_MESSAGES = {
    'Failed to fetch': 'Network error. Check your connection and try again.',
    'Your session expired. Please sign in again.': 'Your session expired. Please sign in again.',
    'CSRF verification failed. Please sign in again.':
      'CSRF verification failed. Please sign in again.',
    '403': 'You do not have permission to perform this action.',
    '404': 'Not found. Please check your input.',
    '500': 'Server error. Our team has been notified. Please try again later.',
    'AbortError': 'Request cancelled.',
  };

  function getUserMessage(error) {
    const msg = error.message || String(error);
    return (
      ERROR_MESSAGES[msg] ||
      ERROR_MESSAGES[msg.split(' ')[0]] ||
      msg
    );
  }

  function showFieldError(fieldId, message) {
    const errorEl = $(fieldId);
    if (errorEl) {
      errorEl.textContent = message;
      errorEl.style.display = 'block';
      const input = errorEl.previousElementSibling;
      if (input && (input.tagName === 'INPUT' || input.tagName === 'TEXTAREA')) {
        input.setAttribute('aria-invalid', 'true');
      }
    }
  }

  function clearFieldErrors(form) {
    $$('.field-error', form).forEach(el => {
      el.textContent = '';
      el.style.display = 'none';
    });
    $$('input, textarea, select', form).forEach(el => {
      el.setAttribute('aria-invalid', 'false');
    });
  }

  function setButtonLoading(btn, loading) {
    if (!btn) return;
    btn.disabled = loading;
    btn.setAttribute('aria-busy', loading ? 'true' : 'false');
    if (loading) {
      btn.dataset.originalText = btn.textContent;
      btn.textContent = btn.dataset.label || btn.textContent;
    } else if (btn.dataset.originalText) {
      btn.textContent = btn.dataset.originalText;
    }
  }

  // Clear errors on field focus
  document.addEventListener(
    'focus',
    (e) => {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
        e.target.setAttribute('aria-invalid', 'false');
        const errorId = e.target.getAttribute('aria-describedby')?.split(' ').find(id => id.includes('-error'));
        if (errorId) {
          const errorEl = $(errorId);
          if (errorEl) {
            errorEl.textContent = '';
          }
        }
      }
    },
    true
  );

  // -----------------------------------------------------------------
  // boot
  // -----------------------------------------------------------------

  // On initial load, check if we have an active session and user state.
  // If the server still has a valid HttpOnly cookie, the next API call will
  // succeed. If the cookie expired, the api() function will get a 401 and
  // redirect to login automatically.
  //
  // Note: We cannot check the session directly from JavaScript (HttpOnly),
  // so we start in "logged out" state and let the first request determine
  // actual session status.
  if (state.user) {
    enterApp();
  }
})();
