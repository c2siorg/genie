'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { api, loadBase, loadSession, saveBase, saveSession, type User } from '@/lib/api';

type Tab = 'ask' | 'documents' | 'governance' | 'settings';
type Doc = { id: string; description: string; classification: string; kek_id?: string };
type Ev = { kind: string; data: string; klass?: 'event-report' | 'event-error' };

const DISCLOSURE = 'AI-generated, informational only; you retain final authority over any financial decision.';

export default function GenieConsole() {
  const [base, setBase] = useState('/v1');
  const [token, setToken] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const [tab, setTab] = useState<Tab>('ask');
  const [authMode, setAuthMode] = useState<'login' | 'signup'>('login');

  const [docs, setDocs] = useState<Doc[]>([]);
  const [docId, setDocId] = useState('');
  const [question, setQuestion] = useState('');
  const [events, setEvents] = useState<Ev[]>([]);
  const [report, setReport] = useState<string | null>(null);
  const [disclosure, setDisclosure] = useState('');
  const [streaming, setStreaming] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  const [gov, setGov] = useState<Record<string, string>>({});
  const [settingsBase, setSettingsBase] = useState('/v1');
  const [health, setHealth] = useState('');

  // Hydrate from localStorage after mount (static export prerenders with no window).
  useEffect(() => {
    const b = loadBase();
    setBase(b);
    setSettingsBase(b);
    const s = loadSession();
    if (s) {
      setToken(s.token);
      setUser(s.user);
    }
  }, []);

  const call = useCallback(
    (path: string, opts?: Parameters<typeof api>[3]) => api(base, token, path, opts),
    [base, token],
  );

  const signedIn = !!token && !!user;

  const refreshHealth = useCallback(async () => {
    const url = base.replace(/\/v1$/, '') + '/readyz';
    try {
      const resp = await fetch(url);
      const body = await resp.text();
      setHealth(`Readiness ${resp.status}: ${body.trim()}`);
    } catch (e) {
      setHealth('Readiness probe unreachable: ' + (e as Error).message);
    }
  }, [base]);

  useEffect(() => {
    if (signedIn) refreshHealth();
  }, [signedIn, refreshHealth]);

  // ---- auth ----
  async function onLogin(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    try {
      const out = (await call('/users/login', {
        method: 'POST',
        json: { email: fd.get('email'), password: fd.get('password') },
      })) as { token: string; user: User };
      setToken(out.token);
      setUser(out.user);
      saveSession(out.token, out.user);
      setTab('ask');
    } catch (err) {
      alert('Login failed: ' + (err as Error).message);
    }
  }

  async function onSignup(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    try {
      const out = (await call('/users', {
        method: 'POST',
        json: { email: fd.get('email'), name: fd.get('name'), password: fd.get('password') },
      })) as { token: string; user: User };
      setToken(out.token);
      setUser(out.user);
      saveSession(out.token, out.user);
      setTab('ask');
    } catch (err) {
      alert('Sign-up failed: ' + (err as Error).message);
    }
  }

  function logout() {
    setToken(null);
    setUser(null);
    saveSession(null, null);
  }

  // ---- documents ----
  async function onUpload(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const fileInput = form.elements.namedItem('upload-file') as HTMLInputElement;
    const file = fileInput?.files?.[0];
    if (!file) return;
    const desc = (form.elements.namedItem('upload-desc') as HTMLInputElement).value;
    const cls = (form.elements.namedItem('upload-class') as HTMLSelectElement).value;
    const url = `/documents?description=${encodeURIComponent(desc)}&classification=${encodeURIComponent(cls)}`;
    try {
      const body = await file.arrayBuffer();
      const out = (await call(url, {
        method: 'POST',
        body,
        headers: { 'Content-Type': file.type || 'application/octet-stream' },
      })) as { id: string; classification: string; kek_id?: string };
      setDocs((d) => [...d, { id: out.id, description: desc || '(no description)', classification: out.classification, kek_id: out.kek_id }]);
      form.reset();
    } catch (err) {
      alert('Upload failed: ' + (err as Error).message);
    }
  }

  // ---- ask (sync + stream) ----
  function resetRun() {
    setEvents([]);
    setReport(null);
    setDisclosure('');
  }
  const addEv = (e: Ev) => setEvents((prev) => [...prev, e]);

  async function onAsk() {
    if (!docId || !question.trim()) return alert('Need a document and a question.');
    resetRun();
    addEv({ kind: 'request', data: 'POST /v1/ask' });
    try {
      const out = (await call('/ask', { method: 'POST', json: { question: question.trim(), document_id: docId } })) as {
        ai_disclosure?: string;
        trace_id?: string;
        report?: string;
      };
      if (out.ai_disclosure) setDisclosure(out.ai_disclosure);
      addEv({ kind: 'trace', data: out.trace_id || '' });
      addEv({ kind: 'report', data: '(received)', klass: 'event-report' });
      setReport(out.report || '');
    } catch (err) {
      addEv({ kind: 'error', data: (err as Error).message, klass: 'event-error' });
    }
  }

  function onAskStream() {
    if (!docId || !question.trim()) return alert('Need a document and a question.');
    resetRun();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    setStreaming(true);
    addEv({ kind: 'request', data: 'POST /v1/ask/stream' });

    const parseSSE = (frame: string) => {
      let event = 'message';
      let data = '';
      for (const line of frame.split('\n')) {
        if (line.startsWith('event:')) event = line.slice(6).trim();
        else if (line.startsWith('data:')) data += line.slice(5).trim();
      }
      if (event === 'ai_disclosure') return setDisclosure(data);
      if (event === 'report') {
        addEv({ kind: 'report', data: '(received)', klass: 'event-report' });
        setReport(data);
        return;
      }
      if (event === 'agent.handle') {
        try {
          data = JSON.stringify(JSON.parse(data));
        } catch {
          /* leave as string */
        }
      }
      addEv({ kind: event, data });
    };

    fetch(base.replace(/\/$/, '') + '/ask/stream', {
      method: 'POST',
      signal: ctrl.signal,
      headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json', Accept: 'text/event-stream' },
      body: JSON.stringify({ question: question.trim(), document_id: docId }),
    })
      .then(async (resp) => {
        if (!resp.ok || !resp.body) {
          addEv({ kind: 'error', data: 'http ' + resp.status, klass: 'event-error' });
          return;
        }
        const reader = resp.body.getReader();
        const dec = new TextDecoder();
        let buf = '';
        for (;;) {
          const { value, done } = await reader.read();
          if (done) break;
          buf += dec.decode(value, { stream: true });
          let idx: number;
          while ((idx = buf.indexOf('\n\n')) >= 0) {
            parseSSE(buf.slice(0, idx));
            buf = buf.slice(idx + 2);
          }
        }
      })
      .catch((err: Error) => {
        if (err.name === 'AbortError') return;
        addEv({ kind: 'error', data: err.message || String(err), klass: 'event-error' });
      })
      .finally(() => {
        setStreaming(false);
        abortRef.current = null;
      });
  }

  function stopStream() {
    abortRef.current?.abort();
  }

  // ---- governance ----
  const refreshGovernance = useCallback(async () => {
    const next: Record<string, string> = {};
    try {
      next.disclosures = JSON.stringify(await call('/disclosures'), null, 2);
    } catch (e) {
      next.disclosures = 'error: ' + (e as Error).message;
    }
    const isAdmin = ((user?.roles) || []).includes('admin');
    if (isAdmin) {
      for (const [key, path] of [
        ['inventory', '/ai-inventory'],
        ['aibom', '/aibom'],
        ['incidents', '/incidents?limit=20'],
      ] as const) {
        try {
          next[key] = JSON.stringify(await call(path), null, 2);
        } catch (e) {
          next[key] = 'error: ' + (e as Error).message;
        }
      }
    }
    setGov(next);
  }, [call, user]);

  useEffect(() => {
    if (signedIn && tab === 'governance') refreshGovernance();
  }, [signedIn, tab, refreshGovernance]);

  function saveSettings() {
    const v = settingsBase.trim();
    if (!v) return;
    setBase(v);
    saveBase(v);
    refreshHealth();
  }

  const isAdmin = ((user?.roles) || []).includes('admin');

  return (
    <>
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark" aria-hidden>◈</span>
          <span className="brand-text">
            <span className="brand-name">Genie</span>
            <span className="brand-sub">Governed AI for money</span>
          </span>
        </div>
        {signedIn && (
          <nav className="tabs">
            {(['ask', 'documents', 'governance', 'settings'] as Tab[]).map((t) => (
              <button key={t} className={'tab' + (tab === t ? ' active' : '')} onClick={() => setTab(t)}>
                {t[0].toUpperCase() + t.slice(1)}
              </button>
            ))}
          </nav>
        )}
        {signedIn && (
          <div className="user">
            <span className="user-avatar" aria-hidden />
            <span className="user-email">{user?.email}</span>
            <button className="link" onClick={logout}>Sign out</button>
          </div>
        )}
      </header>

      <main>
        {!signedIn && (
          <section className="view">
            <div className="hero">
              <aside className="hero-aside">
                <p className="eyebrow">RBI FREE-AI aligned · MARA</p>
                <h1 className="hero-title">A governed control room for money.</h1>
                <p className="hero-lede">
                  Ask a question, watch a pipeline of specialist agents work it, and see every
                  hop — because each one passes the same governance gate before it runs.
                </p>
                <ul className="trust">
                  <li><span className="trust-dot" /> Every request traced, gated, and disclosed</li>
                  <li><span className="trust-dot" /> Statements encrypted end-to-end before storage</li>
                  <li><span className="trust-dot" /> Live AIBOM &amp; incident inventory for auditors</li>
                </ul>
              </aside>

              <div className="card auth">
                <h2>Welcome back</h2>
                <p className="muted">Sign in to your Genie account, or create a new one.</p>
                <div className="tabs-inline">
                  <button className={'tab-inline' + (authMode === 'login' ? ' active' : '')} onClick={() => setAuthMode('login')}>Sign in</button>
                  <button className={'tab-inline' + (authMode === 'signup' ? ' active' : '')} onClick={() => setAuthMode('signup')}>Sign up</button>
                </div>

                {authMode === 'login' ? (
                  <form className="stack" onSubmit={onLogin}>
                    <label>Email <input type="email" name="email" required autoComplete="email" placeholder="you@bank.example" /></label>
                    <label>Password <input type="password" name="password" required autoComplete="current-password" placeholder="••••••••" /></label>
                    <button type="submit" className="primary">Sign in</button>
                  </form>
                ) : (
                  <form className="stack" onSubmit={onSignup}>
                    <label>Name <input type="text" name="name" autoComplete="name" placeholder="Priya Sharma" /></label>
                    <label>Email <input type="email" name="email" required autoComplete="email" placeholder="you@bank.example" /></label>
                    <label>Password
                      <input type="password" name="password" required autoComplete="new-password" minLength={8} placeholder="At least 8 characters" />
                      <small className="muted">At least 8 characters.</small>
                    </label>
                    <button type="submit" className="primary">Create account</button>
                  </form>
                )}
                <p className="muted small api-line">Talking to <code>{base}</code></p>
              </div>
            </div>
          </section>
        )}

        {signedIn && tab === 'ask' && (
          <section className="view">
            <div className="grid">
              <div className="card console">
                <div className="card-head">
                  <h2>Ask Genie</h2>
                  <span className="chip chip-live">live pipeline</span>
                </div>
                <p className="muted">Pick a statement, ask a question, and run the multi-agent pipeline.</p>
                <label>Statement
                  <select value={docId} onChange={(e) => setDocId(e.target.value)}>
                    {docs.length === 0 ? (
                      <option value="">No documents — upload one first</option>
                    ) : (
                      docs.map((d) => <option key={d.id} value={d.id}>{`${d.description} — ${d.id.slice(0, 8)}…`}</option>)
                    )}
                  </select>
                </label>
                <label>Question
                  <textarea rows={3} value={question} onChange={(e) => setQuestion(e.target.value)} placeholder="Where am I overspending this month?" />
                </label>
                <div className="row">
                  <button className="primary" onClick={onAsk}>Ask</button>
                  <button className="secondary" onClick={onAskStream}>Ask &amp; stream</button>
                  {streaming && <button className="link" onClick={stopStream}>Stop</button>}
                </div>
                {disclosure && <p className="muted small disclosure">{disclosure || DISCLOSURE}</p>}
              </div>

              <div className="card pipeline">
                <div className="card-head">
                  <h2>Agent pipeline</h2>
                  <span className="muted small">events from the bus</span>
                </div>
                <ol className="events">
                  {events.length === 0 ? (
                    <li className="events-empty">Ask something to watch the agents run.</li>
                  ) : (
                    events.map((ev, i) => (
                      <li key={i} className={ev.klass}>
                        <span className="event-kind">{ev.kind}</span>
                        <span className="event-data">{ev.data}</span>
                      </li>
                    ))
                  )}
                </ol>
              </div>
            </div>

            {report !== null && (
              <div className="card report">
                <div className="card-head">
                  <h2>Report</h2>
                  <span className="chip chip-report">final_report</span>
                </div>
                <pre className="report-body">{report}</pre>
              </div>
            )}
          </section>
        )}

        {signedIn && tab === 'documents' && (
          <section className="view">
            <div className="card">
              <h2>Upload a transactions CSV</h2>
              <p className="muted">Encrypted end-to-end with envelope AES-256-GCM before it reaches Postgres.</p>
              <form className="stack" onSubmit={onUpload}>
                <label>CSV file <input type="file" name="upload-file" accept=".csv,text/csv" required /></label>
                <label>Description <span className="opt">optional</span> <input type="text" name="upload-desc" placeholder="January statement" /></label>
                <label>Classification
                  <select name="upload-class" defaultValue="pii">
                    <option value="pii">pii</option>
                    <option value="internal">internal</option>
                    <option value="public">public</option>
                  </select>
                </label>
                <button type="submit" className="primary">Upload &amp; encrypt</button>
              </form>
            </div>

            <div className="card">
              <h2>Your documents</h2>
              <table className="table">
                <thead><tr><th>ID</th><th>Description</th><th>Classification</th><th>KEK</th></tr></thead>
                <tbody>
                  {docs.length === 0 ? (
                    <tr><td colSpan={4} className="muted">No uploads yet in this session.</td></tr>
                  ) : (
                    docs.map((d) => (
                      <tr key={d.id}>
                        <td><code>{d.id.slice(0, 8)}…</code></td>
                        <td>{d.description}</td>
                        <td><span className="badge">{d.classification}</span></td>
                        <td><code>{d.kek_id || ''}</code></td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </section>
        )}

        {signedIn && tab === 'governance' && (
          <section className="view">
            <div className="card">
              <div className="card-head"><h2>Public disclosures</h2><span className="chip">public</span></div>
              <pre className="json">{gov.disclosures || 'Loading…'}</pre>
            </div>
            <div className="card">
              <div className="card-head"><h2>AI inventory</h2><span className="badge">admin only</span></div>
              <pre className="json">{isAdmin ? gov.inventory || 'Loading…' : 'Sign in as admin to view.'}</pre>
            </div>
            <div className="card">
              <div className="card-head"><h2>AIBOM <span className="muted small">CycloneDX 1.6</span></h2><span className="badge">admin only</span></div>
              <pre className="json">{isAdmin ? gov.aibom || 'Loading…' : 'Sign in as admin to view.'}</pre>
            </div>
            <div className="card">
              <div className="card-head"><h2>Recent incidents</h2><span className="badge">admin only</span></div>
              <pre className="json">{isAdmin ? gov.incidents || 'Loading…' : 'Sign in as admin to view.'}</pre>
            </div>
          </section>
        )}

        {signedIn && tab === 'settings' && (
          <section className="view">
            <div className="card">
              <h2>API base</h2>
              <p className="muted">Point the UI at a different Genie instance.</p>
              <div className="row">
                <input type="text" value={settingsBase} onChange={(e) => setSettingsBase(e.target.value)} placeholder="https://genie.example/v1" />
                <button className="primary" onClick={saveSettings}>Save</button>
              </div>
            </div>
            <div className="card">
              <h2>About</h2>
              <p>
                Genie is a Go multi-agent platform aligned with the{' '}
                <a href="https://rbidocs.rbi.org.in/rdocs/PublicationReport/Pdfs/FREEAIR130820250A24FF2D4578453F824C72ED9F5D5851.PDF" target="_blank" rel="noopener">RBI FREE-AI</a>{' '}
                framework. The console is a Next.js app, statically exported and embedded in the Genie binary.
              </p>
              <p className="muted small health-line">{health}</p>
            </div>
          </section>
        )}
      </main>

      <footer>
        <span className="muted small">
          Genie · MARA + MCP + A2A · OpenTelemetry · RBI FREE-AI aligned ·{' '}
          <a href="https://github.com/c2siorg/genie" target="_blank" rel="noopener">github.com/c2siorg/genie</a>
        </span>
      </footer>
    </>
  );
}
