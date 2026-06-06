// provenance.tsx — the signature "provenance-first" pattern (plan.md §6).
//
// Any AI/agent output in the UI can open this drawer for an entity (an order,
// settlement, decision...). It pulls the real lineage hash-chain from the
// backend and shows: who (agent), what (action/decision + reason), and whether
// the audit chain is intact — the core of RBI FREE-AI traceability.
//
// Backend contract (pkg/web/handlers/lineage.go + router.go — both are POST):
//   POST /v1/lineage/query?resource_id=<id>  -> { total, entries: [...] }
//   POST /v1/lineage/verify?resource_id=<id> -> { valid, total_entries, ... }

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { apiFetch } from "../lib/api";

export interface LineageEntry {
  id: string;
  timestamp: string;
  user_id: string;
  resource_id: string;
  resource_type: string;
  action: string;
  decision: string;
  reason_code: string;
  agent_id?: string;
  trace_id?: string;
  hash?: string;
  prev_hash?: string;
}

export interface LineageQueryResponse {
  total: number;
  entries: LineageEntry[];
}

export interface LineageVerifyResponse {
  valid: boolean;
  total_entries: number;
  broken_at?: string;
  error?: string;
}

interface ProvenanceState {
  open: (resourceId: string) => void;
  close: () => void;
  resourceId: string | null;
}

const Ctx = createContext<ProvenanceState | null>(null);

export function ProvenanceProvider({ children }: { children: ReactNode }) {
  const [resourceId, setResourceId] = useState<string | null>(null);
  const open = useCallback((id: string) => setResourceId(id), []);
  const close = useCallback(() => setResourceId(null), []);
  return (
    <Ctx.Provider value={{ open, close, resourceId }}>
      {children}
      <ProvenanceDrawer resourceId={resourceId} onClose={close} />
    </Ctx.Provider>
  );
}

export function useProvenance(): ProvenanceState {
  const v = useContext(Ctx);
  if (!v) throw new Error("useProvenance must be used within a ProvenanceProvider");
  return v;
}

type LoadState =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "loaded"; entries: LineageEntry[]; verify: LineageVerifyResponse };

export function ProvenanceDrawer({
  resourceId,
  onClose,
}: {
  resourceId: string | null;
  onClose: () => void;
}) {
  const [state, setState] = useState<LoadState>({ kind: "idle" });

  useEffect(() => {
    if (!resourceId) {
      setState({ kind: "idle" });
      return;
    }
    let cancelled = false;
    setState({ kind: "loading" });
    const qs = `?resource_id=${encodeURIComponent(resourceId)}`;
    Promise.all([
      apiFetch<LineageQueryResponse>(`/lineage/query${qs}`, { method: "POST" }),
      apiFetch<LineageVerifyResponse>(`/lineage/verify${qs}`, { method: "POST" }),
    ])
      .then(([q, v]) => {
        if (!cancelled) setState({ kind: "loaded", entries: q.entries, verify: v });
      })
      .catch((e: unknown) => {
        if (!cancelled)
          setState({ kind: "error", message: e instanceof Error ? e.message : "failed" });
      });
    return () => {
      cancelled = true;
    };
  }, [resourceId]);

  if (!resourceId) return null;

  return (
    <aside className="provenance" role="dialog" aria-label="Provenance" aria-modal="false">
      <header className="provenance__header">
        <strong>Provenance</strong>
        <code className="provenance__rid">{resourceId}</code>
        <button onClick={onClose} aria-label="Close provenance">
          ✕
        </button>
      </header>

      {state.kind === "loading" && <p className="provenance__status">Loading lineage…</p>}
      {state.kind === "error" && (
        <p className="provenance__status provenance__status--error" role="alert">
          Could not load provenance: {state.message}
        </p>
      )}

      {state.kind === "loaded" && (
        <>
          <div
            className={`provenance__integrity provenance__integrity--${
              state.verify.valid ? "ok" : "broken"
            }`}
            data-valid={state.verify.valid}
          >
            {state.verify.valid
              ? `✓ Hash chain intact (${state.verify.total_entries} entries)`
              : `✗ Hash chain BROKEN${
                  state.verify.broken_at ? ` at ${state.verify.broken_at}` : ""
                }`}
          </div>
          <ol className="provenance__chain">
            {state.entries.map((e) => (
              <li key={e.id} className="provenance__entry">
                <div className="provenance__line">
                  <span className="provenance__action">{e.action}</span>
                  <span className="provenance__decision">{e.decision}</span>
                </div>
                <div className="provenance__meta">
                  {e.agent_id && <span>agent: {e.agent_id}</span>}
                  <span>{e.reason_code}</span>
                  <time>{e.timestamp}</time>
                </div>
              </li>
            ))}
            {state.entries.length === 0 && (
              <li className="provenance__status">No lineage entries for this entity.</li>
            )}
          </ol>
        </>
      )}
    </aside>
  );
}
