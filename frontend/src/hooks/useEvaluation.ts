// useEvaluation hooks for trace review, annotation, and judge calibration.
// All calls use the real API client (CSRF + HttpOnly cookies).

import { useCallback, useState } from "react";
import { apiFetch, ApiError } from "../lib/api";
import type {
  InteractionTrace,
  Annotation,
  EvaluationMetrics,
  TraceWithAnnotations,
  FailureMode,
  CalibrationResult,
  CoverageStatus,
} from "../types/evaluation";

// useTraces fetches paginated list of execution traces with sampling.
export function useTraces(
  limit = 20,
  sample?: "random" | "failure" | "uncertainty"
) {
  const [traces, setTraces] = useState<InteractionTrace[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      params.append("limit", limit.toString());
      if (sample) params.append("sample", sample);

      const t = await apiFetch<InteractionTrace[]>(`/eval/traces?${params.toString()}`);
      setTraces(t);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [limit, sample]);

  return { traces, loading, error, fetch };
}

// useTraceDetail fetches a single trace with its annotations and verdicts.
export function useTraceDetail(traceId: string | null) {
  const [trace, setTrace] = useState<TraceWithAnnotations | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!traceId) return;
    setLoading(true);
    try {
      const t = await apiFetch<TraceWithAnnotations>(`/eval/traces/${traceId}`);
      setTrace(t);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [traceId]);

  return { trace, loading, error, fetch };
}

// useAnnotateTrace submits or updates an annotation for a trace.
export function useAnnotateTrace() {
  const [annotating, setAnnotating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const annotate = useCallback(
    async (
      traceId: string,
      label: "pass" | "fail" | "uncertain",
      primaryFailure?: string,
      notes?: string,
      confidence?: number
    ): Promise<Annotation | null> => {
      setAnnotating(true);
      try {
        const result = await apiFetch<Annotation>(`/eval/traces/${traceId}/feedback`, {
          method: "POST",
          json: {
            label,
            primary_failure: primaryFailure,
            notes,
            confidence,
          },
        });
        setError(null);
        return result;
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Unknown error");
        return null;
      } finally {
        setAnnotating(false);
      }
    },
    []
  );

  return { annotate, annotating, error };
}

// useFailureModes fetches the taxonomy of failure modes.
export function useFailureModes() {
  const [failureModes, setFailureModes] = useState<FailureMode[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const f = await apiFetch<FailureMode[]>("/eval/failure-modes");
      setFailureModes(f);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  return { failureModes, loading, error, fetch };
}

// useEvaluationMetrics fetches dashboard metrics (judge accuracy, pass rates, drift).
export function useEvaluationMetrics() {
  const [metrics, setMetrics] = useState<EvaluationMetrics | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const m = await apiFetch<EvaluationMetrics>("/eval/metrics");
      setMetrics(m);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  return { metrics, loading, error, fetch };
}

// useJudgeCalibration fetches calibration results for all judges.
export function useJudgeCalibration() {
  const [calibration, setCalibration] = useState<CalibrationResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const c = await apiFetch<CalibrationResult[]>("/eval/judges/calibration");
      setCalibration(c);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  return { calibration, loading, error, fetch };
}

// useCoverageStatus fetches golden dataset coverage metrics.
export function useCoverageStatus() {
  const [coverage, setCoverage] = useState<CoverageStatus[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const c = await apiFetch<CoverageStatus[]>("/eval/coverage");
      setCoverage(c);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  return { coverage, loading, error, fetch };
}
