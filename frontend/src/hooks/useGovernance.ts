// useGovernance hooks for agent fleet management, HITL, incidents, compliance.
// All calls use the real API client (CSRF + HttpOnly cookies, admin-gated).

import { useCallback, useState } from "react";
import { apiFetch, ApiError } from "../lib/api";
import type {
  AgentSummary,
  TrustScore,
  Incident,
  HITLApprovalRequest,
  ComplianceLimit,
  KillSwitchStatus,
  SLOReport,
  AuditEntry,
} from "../types/governance";

// useAgentFleet fetches all agents with health and trust metrics.
export function useAgentFleet() {
  const [agents, setAgents] = useState<AgentSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const a = await apiFetch<AgentSummary[]>("/governance/agents");
      setAgents(a);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  return { agents, loading, error, fetch };
}

// useAgentTrust fetches detailed trust score for one agent.
export function useAgentTrust(agentId: string | null) {
  const [trust, setTrust] = useState<TrustScore | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!agentId) return;
    setLoading(true);
    try {
      const t = await apiFetch<TrustScore>(`/governance/trust/${agentId}`);
      setTrust(t);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [agentId]);

  return { trust, loading, error, fetch };
}

// useIncidents fetches list of recent incidents.
export function useIncidents(limit = 20) {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const i = await apiFetch<Incident[]>(`/incidents?limit=${limit}`);
      setIncidents(i);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [limit]);

  return { incidents, loading, error, fetch };
}

// useCreateIncident creates a new incident report.
export function useCreateIncident() {
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const create = useCallback(
    async (
      useCase: string,
      model: string,
      description: string,
      affectedStakeholders: "internal" | "external" | "both"
    ): Promise<Incident | null> => {
      setCreating(true);
      try {
        const result = await apiFetch<Incident>("/incidents", {
          method: "POST",
          json: {
            use_case: useCase,
            model,
            description,
            affected_stakeholders: affectedStakeholders,
          },
        });
        setError(null);
        return result;
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Unknown error");
        return null;
      } finally {
        setCreating(false);
      }
    },
    []
  );

  return { create, creating, error };
}

// useHITLApprovals fetches pending approvals (admin only).
export function useHITLApprovals() {
  const [approvals, setApprovals] = useState<HITLApprovalRequest[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const a = await apiFetch<HITLApprovalRequest[]>("/hitl/approvals");
      setApprovals(a);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  return { approvals, loading, error, fetch };
}

// useComplianceLimits fetches CBDC transaction limits for an account.
export function useComplianceLimits(accountId: string | null) {
  const [limits, setLimits] = useState<ComplianceLimit | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!accountId) return;
    setLoading(true);
    try {
      const l = await apiFetch<ComplianceLimit>(
        `/compliance/limits/${accountId}`
      );
      setLimits(l);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [accountId]);

  return { limits, loading, error, fetch };
}

// useKillSwitch gets current kill-switch status and enables/disables it.
export function useKillSwitch() {
  const [status, setStatus] = useState<KillSwitchStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const s = await apiFetch<KillSwitchStatus>("/governance/killswitch");
      setStatus(s);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  const activate = useCallback(
    async (scope: "global" | "agent" | "capability", target: string, reason: string) => {
      try {
        const result = await apiFetch<KillSwitchStatus>("/governance/killswitch", {
          method: "POST",
          json: { scope, target, reason },
        });
        setStatus(result);
        setError(null);
        return result;
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Unknown error");
        return null;
      }
    },
    []
  );

  const deactivate = useCallback(async () => {
    try {
      const result = await apiFetch<KillSwitchStatus>("/governance/killswitch", {
        method: "DELETE",
      });
      setStatus(result);
      setError(null);
      return result;
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
      return null;
    }
  }, []);

  return { status, loading, error, fetch, activate, deactivate };
}

// useSLOReport fetches SLO metrics for an agent.
export function useSLOReport(agentId: string | null) {
  const [report, setReport] = useState<SLOReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!agentId) return;
    setLoading(true);
    try {
      const r = await apiFetch<SLOReport>(`/governance/slo/${agentId}`);
      setReport(r);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [agentId]);

  return { report, loading, error, fetch };
}

// useAuditLog fetches hash-chained audit entries.
export function useAuditLog(
  agentId?: string,
  action?: string,
  limit = 50
) {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (agentId) params.append("agent_id", agentId);
      if (action) params.append("action", action);
      params.append("limit", limit.toString());

      const e = await apiFetch<AuditEntry[]>(
        `/governance/audit?${params.toString()}`
      );
      setEntries(e);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [agentId, action, limit]);

  return { entries, loading, error, fetch };
}
