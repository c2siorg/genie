// Advisor Workspace Hooks
// Phase 7: API integration for personalized recommendations

import { useCallback, useState } from 'react';
import type {
  Recommendation,
  GenerateRecommendationRequest,
  GenerateRecommendationResponse,
  RecommendationHistoryResponse,
  CalibrationMetrics,
  RecommendationFeedback,
  AdvisorStreamEvent,
} from '../types/advisor';
import { apiFetch, ApiError } from '../lib/api';

// Hook: Generate recommendation
export function useGenerateRecommendation() {
  const [data, setData] = useState<GenerateRecommendationResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  const generate = useCallback(
    async (request: GenerateRecommendationRequest) => {
      setLoading(true);
      setError(null);
      try {
        const response = await apiFetch<GenerateRecommendationResponse>(
          '/v1/advisor/recommendation',
          { method: 'POST', json: request }
        );
        setData(response);
        return response;
      } catch (err) {
        const apiError = err instanceof ApiError ? err : new ApiError(500, 'Failed to generate recommendation');
        setError(apiError);
        throw apiError;
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { data, loading, error, generate };
}

// Hook: Get recommendation detail
export function useRecommendation(recommendationId: string) {
  const [data, setData] = useState<Recommendation | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await apiFetch<Recommendation>(
        `/v1/advisor/recommendation/${recommendationId}`
      );
      setData(response);
      return response;
    } catch (err) {
      const apiError = err instanceof ApiError ? err : new ApiError(500, 'Failed to fetch recommendation');
      setError(apiError);
      throw apiError;
    } finally {
      setLoading(false);
    }
  }, [recommendationId]);

  return { data, loading, error, fetch };
}

// Hook: Submit recommendation feedback
export function useRecommendationFeedback() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  const submit = useCallback(
    async (recommendationId: string, feedback: Omit<RecommendationFeedback, 'recommendation_id'>) => {
      setLoading(true);
      setError(null);
      try {
        const response = await apiFetch<{ recommendation_id: string; status: string }>(
          `/v1/advisor/recommendation/${recommendationId}/feedback`,
          { method: 'POST', json: feedback }
        );
        return response;
      } catch (err) {
        const apiError = err instanceof ApiError ? err : new ApiError(500, 'Failed to submit feedback');
        setError(apiError);
        throw apiError;
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { loading, error, submit };
}

// Hook: Get recommendation history
export function useRecommendationHistory(userId: string) {
  const [data, setData] = useState<RecommendationHistoryResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await apiFetch<RecommendationHistoryResponse>(
        `/v1/advisor/history?user_id=${userId}&limit=20`
      );
      setData(response);
      return response;
    } catch (err) {
      const apiError = err instanceof ApiError ? err : new ApiError(500, 'Failed to fetch history');
      setError(apiError);
      throw apiError;
    } finally {
      setLoading(false);
    }
  }, [userId]);

  return { data, loading, error, fetch };
}

// Hook: Get calibration metrics
export function useCalibrationMetrics(userId: string) {
  const [data, setData] = useState<CalibrationMetrics | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await apiFetch<CalibrationMetrics>(
        `/v1/advisor/calibration?user_id=${userId}`
      );
      setData(response);
      return response;
    } catch (err) {
      const apiError = err instanceof ApiError ? err : new ApiError(500, 'Failed to fetch calibration');
      setError(apiError);
      throw apiError;
    } finally {
      setLoading(false);
    }
  }, [userId]);

  return { data, loading, error, fetch };
}

// Hook: Stream recommendations
export function useRecommendationStream() {
  const [events, setEvents] = useState<AdvisorStreamEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  const stream = useCallback(
    async (userId: string, category?: string) => {
      setLoading(true);
      setError(null);
      setEvents([]);

      try {
        const response = await fetch(
          `/v1/advisor/stream`,
          {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            credentials: 'include',
            body: JSON.stringify({
              user_id: userId,
              category,
            }),
          }
        );

        if (!response.ok) {
          throw new ApiError(response.status, 'Stream failed');
        }

        const reader = response.body?.getReader();
        if (!reader) {
          throw new ApiError(500, 'No reader available');
        }

        const decoder = new TextDecoder();
        let buffer = '';

        // eslint-disable-next-line no-constant-condition
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');

          buffer = lines.pop() || '';

          for (const line of lines) {
            if (line.startsWith('data: ')) {
              try {
                const event = JSON.parse(line.slice(6)) as AdvisorStreamEvent;
                setEvents((prev) => [...prev, event]);
              } catch {
                // Ignore parse errors
              }
            }
          }
        }
      } catch (err) {
        const apiError = err instanceof ApiError ? err : new ApiError(500, 'Stream failed');
        setError(apiError);
        throw apiError;
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { events, loading, error, stream };
}
