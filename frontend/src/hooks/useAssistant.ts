// useAssistant hooks for conversational AI.
// All calls use the real API client (CSRF + HttpOnly cookies).

import { useCallback, useState, useRef } from "react";
import { apiFetch, ApiError } from "../lib/api";
import type {
  AskRequest,
  AskResponse,
  AgentHandleEvent,
  ConversationMessage,
  WebSocketMessage,
  WebSocketInputFrame,
  LLMConfig,
  FinancialContext,
  AIDisclosure,
} from "../types/assistant";

// useAsk sends a single synchronous question and waits for full response.
export function useAsk() {
  const [response, setResponse] = useState<AskResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const ask = useCallback(
    async (request: AskRequest): Promise<AskResponse | null> => {
      setLoading(true);
      try {
        const result = await apiFetch<AskResponse>("/v1/ask", {
          method: "POST",
          json: request,
        });
        setResponse(result);
        setError(null);
        return result;
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Unknown error");
        return null;
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { response, loading, error, ask };
}

// useAskStream sends a question and streams progress events + final response.
export function useAskStream() {
  const [report, setReport] = useState<string>("");
  const [traceId, setTraceId] = useState<string>("");
  const [aiDisclosure, setAiDisclosure] = useState<string>("");
  const [progressEvents, setProgressEvents] = useState<AgentHandleEvent[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  const askStream = useCallback(
    async (request: AskRequest): Promise<void> => {
      setStreaming(true);
      setReport("");
      setTraceId("");
      setAiDisclosure("");
      setProgressEvents([]);
      setError(null);

      abortControllerRef.current = new AbortController();

      try {
        const response = await fetch("/v1/ask/stream", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify(request),
          signal: abortControllerRef.current.signal,
          credentials: "include", // CSRF + cookies
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const reader = response.body?.getReader();
        if (!reader) {
          throw new Error("No response body");
        }

        const decoder = new TextDecoder();
        let buffer = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines[lines.length - 1]; // Keep incomplete line

          for (let i = 0; i < lines.length - 1; i++) {
            const line = lines[i];

            if (line.startsWith("event:")) {
              const eventType = line.slice(6).trim();
              const dataLine = lines[++i];
              if (dataLine?.startsWith("data:")) {
                const dataStr = dataLine.slice(5).trim();
                handleStreamEvent(eventType, dataStr);
              }
            }
          }
        }

        setStreaming(false);
      } catch (e) {
        if (!(e instanceof Error && e.name === "AbortError")) {
          setError(e instanceof Error ? e.message : "Unknown error");
        }
        setStreaming(false);
      }
    },
    []
  );

  const handleStreamEvent = (eventType: string, dataStr: string) => {
    try {
      const data = JSON.parse(dataStr);

      switch (eventType) {
        case "ai_disclosure":
          setAiDisclosure(data);
          break;
        case "trace":
          setTraceId(data);
          break;
        case "agent.handle":
          setProgressEvents((prev) => [...prev, data as AgentHandleEvent]);
          break;
        case "report":
          setReport(data);
          break;
        case "error":
          setError(data);
          break;
      }
    } catch (e) {
      // Silently skip parse errors in streaming
    }
  };

  const cancel = useCallback(() => {
    abortControllerRef.current?.abort();
    setStreaming(false);
  }, []);

  return {
    report,
    traceId,
    aiDisclosure,
    progressEvents,
    streaming,
    error,
    askStream,
    cancel,
  };
}

// useAssistantChat manages WebSocket bidirectional chat.
export function useAssistantChat() {
  const [messages, setMessages] = useState<ConversationMessage[]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const messageIdRef = useRef(0);

  const connect = useCallback((): void => {
    // Determine protocol (ws or wss based on current location)
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${protocol}//${window.location.host}/v1/chat/ws`;

    try {
      const ws = new WebSocket(url);

      ws.onopen = () => {
        setConnected(true);
        setError(null);
      };

      ws.onmessage = (event: MessageEvent) => {
        try {
          const msg = JSON.parse(event.data) as WebSocketMessage;
          handleWebSocketMessage(msg);
        } catch (e) {
          // Silently skip parse errors
        }
      };

      ws.onerror = () => {
        setError("WebSocket connection error");
        setConnected(false);
      };

      ws.onclose = () => {
        setConnected(false);
      };

      wsRef.current = ws;
    } catch (e) {
      setError(e instanceof Error ? e.message : "Connection failed");
      setConnected(false);
    }
  }, []);

  const handleWebSocketMessage = (msg: WebSocketMessage) => {
    switch (msg.event) {
      case "ai_disclosure":
        // Add disclosure as system message
        setMessages((prev) => [
          ...prev,
          {
            id: `msg-${messageIdRef.current++}`,
            role: "system",
            content: `[AI Disclosure] ${msg.data}`,
            timestamp: new Date().toISOString(),
          },
        ]);
        break;

      case "trace":
        // Silently track trace ID for debugging
        break;

      case "agent.handle":
        // Add progress event as assistant message
        const agent = msg.data as AgentHandleEvent;
        setMessages((prev) => [
          ...prev,
          {
            id: `msg-${messageIdRef.current++}`,
            role: "assistant",
            content: `[Progress] ${agent.from} → ${agent.to}: ${agent.type}`,
            timestamp: new Date().toISOString(),
            trace_id: msg.trace_id,
          },
        ]);
        break;

      case "report":
        // Add final report as assistant message
        setMessages((prev) => [
          ...prev,
          {
            id: `msg-${messageIdRef.current++}`,
            role: "assistant",
            content: msg.data as string,
            timestamp: new Date().toISOString(),
            trace_id: msg.trace_id,
          },
        ]);
        break;

      case "error":
        // Add error as system message
        setMessages((prev) => [
          ...prev,
          {
            id: `msg-${messageIdRef.current++}`,
            role: "system",
            content: `[Error] ${msg.data}`,
            timestamp: new Date().toISOString(),
          },
        ]);
        break;
    }
  };

  const send = useCallback(
    (question: string, documentId: string): void => {
      if (!wsRef.current || !connected) {
        setError("WebSocket not connected");
        return;
      }

      // Add user message to local state
      setMessages((prev) => [
        ...prev,
        {
          id: `msg-${messageIdRef.current++}`,
          role: "user",
          content: question,
          timestamp: new Date().toISOString(),
        },
      ]);

      // Send to server
      const frame: WebSocketInputFrame = {
        question,
        document_id: documentId,
      };
      wsRef.current.send(JSON.stringify(frame));
    },
    [connected]
  );

  const disconnect = useCallback((): void => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setConnected(false);
    setMessages([]);
  }, []);

  return {
    messages,
    connected,
    error,
    connect,
    send,
    disconnect,
  };
}

// useLLMConfig fetches LLM configuration and limits.
export function useLLMConfig() {
  const [config, setConfig] = useState<LLMConfig | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const c = await apiFetch<LLMConfig>("/v1/llm/config");
      setConfig(c);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  return { config, loading, error, fetch };
}

// useAIDisclosure fetches the regulatory AI disclosure banner.
export function useAIDisclosure() {
  const [disclosure, setDisclosure] = useState<AIDisclosure | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const d = await apiFetch<AIDisclosure>("/v1/ai/disclosure");
      setDisclosure(d);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  return { disclosure, loading, error, fetch };
}

// useFinancialContext extracts financial data from document for context display.
export function useFinancialContext(documentId: string | null) {
  const [context, setContext] = useState<FinancialContext | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!documentId) return;
    setLoading(true);
    try {
      const c = await apiFetch<FinancialContext>(
        `/v1/documents/${documentId}/context`
      );
      setContext(c);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [documentId]);

  return { context, loading, error, fetch };
}
