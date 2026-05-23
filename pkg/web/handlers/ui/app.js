// Genie UI — vanilla JS, no framework. Pairs with index.html + styles.css.
//
// State lives in localStorage so refreshes survive. Streaming uses the
// browser's EventSource against /v1/ask/stream — same SSE shape as the curl
// example in the README.

(function () {
  'use strict';

  // -----------------------------------------------------------------
  // state
  // -----------------------------------------------------------------

  const LS_KEY = 'genie.session.v1';
  const LS_BASE = 'genie.apibase.v1';

  const state = {
    base: localStorage.getItem(LS_BASE) || '/v1',
    token: null,
    user: null,
    documents: [],
    activeStream: null,
  };

  try {
    const cached = JSON.parse(localStorage.getItem(LS_KEY) || 'null');
    if (cached && cached.token && cached.user) {
      state.token = cached.token;
      state.user = cached.user;
    }
  } catch (_) { /* ignore */ }

  // -----------------------------------------------------------------
  // tiny helpers
  // -----------------------------------------------------------------

  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

  function show(el) { el.hidden = false; }
  function hide(el) { el.hidden = true; }

  function persistSession() {
    if (state.token && state.user) {
      localStorage.setItem(LS_KEY, JSON.stringify({ token: state.token, user: state.user }));
    } else {
      localStorage.removeItem(LS_KEY);
    }
  }

  async function api(path, opts = {}) {
    const headers = Object.assign({ 'Accept': 'application/json' }, opts.headers || {});
    if (state.token) headers['Authorization'] = 'Bearer ' + state.token;
    if (opts.json !== undefined) {
      headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(opts.json);
      delete opts.json;
    }
    const url = state.base.replace(/\/$/, '') + path;
    const resp = await fetch(url, Object.assign({}, opts, { headers }));
    const text = await resp.text();
    let body = text;
    try { body = JSON.parse(text); } catch (_) { /* keep as text */ }
    if (!resp.ok) {
      const message = (body && body.error) || (typeof body === 'string' ? body : resp.statusText);
      throw new Error(message);
    }
    return body;
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
    Object.entries(views).forEach(([k, el]) => { if (k !== 'auth') (k === name ? show(el) : hide(el)); });
    $$('.tab').forEach(t => t.classList.toggle('active', t.dataset.tab === name));
    if (name === 'documents') refreshDocs();
    if (name === 'governance') refreshGovernance();
  }

  $('#tabs').addEventListener('click', e => {
    if (e.target.matches('.tab')) activateTab(e.target.dataset.tab);
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
    const fd = new FormData(e.target);
    try {
      const out = await api('/users/login', { method: 'POST', json: { email: fd.get('email'), password: fd.get('password') } });
      state.token = out.token; state.user = out.user; persistSession();
      enterApp();
    } catch (err) { alert('Login failed: ' + err.message); }
  });

  $('#form-signup').addEventListener('submit', async (e) => {
    e.preventDefault();
    const fd = new FormData(e.target);
    try {
      const out = await api('/users', { method: 'POST', json: { email: fd.get('email'), name: fd.get('name'), password: fd.get('password') } });
      state.token = out.token; state.user = out.user; persistSession();
      enterApp();
    } catch (err) { alert('Sign-up failed: ' + err.message); }
  });

  $('#logout').addEventListener('click', () => {
    state.token = null; state.user = null; persistSession();
    leaveApp();
  });

  function enterApp() {
    hide(views.auth);
    show($('#tabs'));
    show($('#user-chip'));
    $('#user-chip .user-email').textContent = state.user.email;
    activateTab('ask');
    refreshDocs();
    refreshHealth();
  }

  function leaveApp() {
    show(views.auth);
    hide($('#tabs'));
    hide($('#user-chip'));
    Object.entries(views).forEach(([k, el]) => { if (k !== 'auth') hide(el); });
  }

  // -----------------------------------------------------------------
  // documents
  // -----------------------------------------------------------------

  $('#form-upload').addEventListener('submit', async (e) => {
    e.preventDefault();
    const file = $('#upload-file').files[0];
    const desc = $('#upload-desc').value;
    const cls = $('#upload-class').value;
    if (!file) return;
    const url = `/documents?description=${encodeURIComponent(desc)}&classification=${encodeURIComponent(cls)}`;
    try {
      const body = await file.arrayBuffer();
      const out = await api(url, { method: 'POST', body, headers: { 'Content-Type': file.type || 'application/octet-stream' } });
      state.documents.push({ id: out.id, description: desc || '(no description)', classification: out.classification, kek_id: out.kek_id });
      renderDocs();
      e.target.reset();
    } catch (err) { alert('Upload failed: ' + err.message); }
  });

  async function refreshDocs() {
    // Genie has no GET /documents list endpoint yet; we mirror local uploads.
    renderDocs();
  }

  function renderDocs() {
    const tbody = $('#doc-list');
    tbody.innerHTML = '';
    if (state.documents.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="muted">No uploads yet in this session.</td></tr>';
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

  function clearEvents() { $('#events').innerHTML = ''; }

  function addEvent(kind, data, klass) {
    const li = document.createElement('li');
    if (klass) li.classList.add(klass);
    li.innerHTML = `<span class="event-kind">${escapeHTML(kind)}</span><span class="event-data"></span>`;
    li.querySelector('.event-data').textContent = typeof data === 'string' ? data : JSON.stringify(data);
    const list = $('#events');
    list.appendChild(li);
    list.scrollTop = list.scrollHeight;
  }

  function setBanner(text) {
    const el = $('#ai-disclosure');
    if (text) { el.textContent = text; show(el); } else { hide(el); }
  }

  function showReport(text) {
    $('#report-body').textContent = text;
    show($('#report-card'));
  }

  $('#btn-ask').addEventListener('click', async () => {
    const docID = $('#ask-doc').value;
    const q = $('#ask-question').value.trim();
    if (!docID || !q) return alert('Need a document and a question.');
    clearEvents();
    hide($('#report-card'));
    setBanner('');
    addEvent('request', 'POST /v1/ask');
    try {
      const out = await api('/ask', { method: 'POST', json: { question: q, document_id: docID } });
      if (out.ai_disclosure) setBanner(out.ai_disclosure);
      addEvent('trace', out.trace_id || '');
      addEvent('report', '(received)', 'event-report');
      showReport(out.report || '');
    } catch (err) {
      addEvent('error', err.message, 'event-error');
    }
  });

  $('#btn-ask-stream').addEventListener('click', () => {
    const docID = $('#ask-doc').value;
    const q = $('#ask-question').value.trim();
    if (!docID || !q) return alert('Need a document and a question.');
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
        'Authorization': 'Bearer ' + state.token,
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
      },
      body: JSON.stringify({ question: q, document_id: docID }),
    }).then(async (resp) => {
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
    }).catch((err) => {
      if (err.name === 'AbortError') return;
      addEvent('error', err.message || String(err), 'event-error');
    }).finally(() => {
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
      try { data = JSON.stringify(JSON.parse(data)); } catch (_) { /* leave as string */ }
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
    } catch (e) { $('#disclosures').textContent = 'error: ' + e.message; }

    const isAdmin = (state.user && state.user.roles || []).includes('admin');
    if (isAdmin) {
      try {
        const inv = await api('/ai-inventory');
        $('#inventory').textContent = JSON.stringify(inv, null, 2);
      } catch (e) { $('#inventory').textContent = 'error: ' + e.message; }
      try {
        const bom = await api('/aibom');
        $('#aibom').textContent = JSON.stringify(bom, null, 2);
      } catch (e) { $('#aibom').textContent = 'error: ' + e.message; }
      try {
        const inc = await api('/incidents?limit=20');
        $('#incidents').textContent = JSON.stringify(inc, null, 2);
      } catch (e) { $('#incidents').textContent = 'error: ' + e.message; }
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
    localStorage.setItem(LS_BASE, v);
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
    return String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  // -----------------------------------------------------------------
  // boot
  // -----------------------------------------------------------------

  if (state.token && state.user) enterApp();
})();
