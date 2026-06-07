// useCompliance hooks for real-time compliance checks.
// All calls use the real API client (CSRF + HttpOnly cookies).

import { useCallback, useState } from "react";
import { apiFetch, ApiError } from "../lib/api";
import type {
  ComplianceCheck,
  VelocityMetrics,
  FraudHistory,
  AMLRiskScore,
  HITLApprovalRequest,
  HITLDecision,
} from "../types/compliance";

// useComplianceCheck initiates an async payment compliance check.
export function useComplianceCheck() {
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const check = useCallback(
    async (
      paymentId: string,
      fromAccount: string,
      toAccount: string,
      amountPaise: number
    ): Promise<{ check_id: string } | null> => {
      setChecking(true);
      try {
        const result = await apiFetch<{ check_id: string }>(
          "/compliance/check",
          {
            method: "POST",
            json: {
              payment_id: paymentId,
              from_account: fromAccount,
              to_account: toAccount,
              amount_paise: amountPaise,
            },
          }
        );
        setError(null);
        return result;
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Unknown error");
        return null;
      } finally {
        setChecking(false);
      }
    },
    []
  );

  return { check, checking, error };
}

// useComplianceCheckResult retrieves the result of an async compliance check.
export function useComplianceCheckResult(checkId: string | null) {
  const [result, setResult] = useState<ComplianceCheck | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!checkId) return;
    setLoading(true);
    try {
      const r = await apiFetch<ComplianceCheck>(`/compliance/check/${checkId}`);
      setResult(r);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [checkId]);

  return { result, loading, error, fetch };
}

// useVelocityMetrics fetches current velocity limits and usage for an account.
export function useVelocityMetrics(accountId: string | null) {
  const [metrics, setMetrics] = useState<VelocityMetrics | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!accountId) return;
    setLoading(true);
    try {
      const m = await apiFetch<VelocityMetrics>(
        `/compliance/account/${accountId}/velocity`
      );
      setMetrics(m);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [accountId]);

  return { metrics, loading, error, fetch };
}

// useFraudHistory fetches fraud patterns and risk score for an account.
export function useFraudHistory(accountId: string | null) {
  const [history, setHistory] = useState<FraudHistory | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!accountId) return;
    setLoading(true);
    try {
      const h = await apiFetch<FraudHistory>(
        `/compliance/account/${accountId}/fraud-history`
      );
      setHistory(h);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [accountId]);

  return { history, loading, error, fetch };
}

// useAMLRiskScore calculates AML risk for a transaction.
export function useAMLRiskScore() {
  const [scoring, setScoring] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const score = useCallback(
    async (
      userId: string,
      amountPaise: number,
      beneficiaryId: string,
      beneficiaryCountry?: string
    ): Promise<AMLRiskScore | null> => {
      setScoring(true);
      try {
        const result = await apiFetch<AMLRiskScore>("/aml/score", {
          method: "POST",
          json: {
            user_id: userId,
            amount_paise: amountPaise,
            beneficiary_id: beneficiaryId,
            beneficiary_country: beneficiaryCountry,
          },
        });
        setError(null);
        return result;
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Unknown error");
        return null;
      } finally {
        setScoring(false);
      }
    },
    []
  );

  return { score, scoring, error };
}

// useHITLApprovals fetches all pending approvals for the compliance officer.
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

// useHITLApprovalDetail fetches a single approval request.
export function useHITLApprovalDetail(approvalId: string | null) {
  const [approval, setApproval] = useState<HITLApprovalRequest | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!approvalId) return;
    setLoading(true);
    try {
      const a = await apiFetch<HITLApprovalRequest>(
        `/hitl/approvals/${approvalId}`
      );
      setApproval(a);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [approvalId]);

  return { approval, loading, error, fetch };
}

// useHITLDecision submits an approval or denial decision.
export function useHITLDecision() {
  const [deciding, setDeciding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const approve = useCallback(
    async (
      approvalId: string,
      reason: string,
      decidedBy: string
    ): Promise<HITLDecision | null> => {
      setDeciding(true);
      try {
        const result = await apiFetch<HITLDecision>(
          `/hitl/approvals/${approvalId}/approve`,
          {
            method: "POST",
            json: { reason, decided_by: decidedBy },
          }
        );
        setError(null);
        return result;
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Unknown error");
        return null;
      } finally {
        setDeciding(false);
      }
    },
    []
  );

  const deny = useCallback(
    async (
      approvalId: string,
      reason: string,
      decidedBy: string
    ): Promise<HITLDecision | null> => {
      setDeciding(true);
      try {
        const result = await apiFetch<HITLDecision>(
          `/hitl/approvals/${approvalId}/deny`,
          {
            method: "POST",
            json: { reason, decided_by: decidedBy },
          }
        );
        setError(null);
        return result;
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Unknown error");
        return null;
      } finally {
        setDeciding(false);
      }
    },
    []
  );

  return { approve, deny, deciding, error };
}
