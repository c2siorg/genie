// Assistant types for conversational financial AI.
// Derived from pkg/web/handlers (ask.go, ask_stream.go, chat_ws.go) and pkg/llm packages.

// LLM Providers
export type LLMProvider = "anthropic" | "ollama" | "openai" | "gemini" | "mock";

export interface LLMProviderConfig {
  provider: LLMProvider;
  model: string;
  region: "us" | "in" | "on-prem";
  url?: string; // For Ollama
  apiKey?: string; // For Anthropic, OpenAI, Gemini
}

// Message roles in conversation
export type MessageRole = "system" | "user" | "assistant" | "tool";

// Financial data context passed to assistant
export interface FinancialContext {
  question: string;
  currency: string;
  total_income_paise: number; // In paise (₹0.01 units)
  total_expense_paise: number;
  net_paise: number;
  top_overspend: string[]; // Top spending categories
  analysis: {
    transactions_count: number;
    date_range: string;
    classification: "public" | "internal" | "pii" | "secret";
  };
}

// User request to assistant
export interface AskRequest {
  question: string;
  document_id: string; // CSV document containing financial data
  provider?: LLMProvider; // Override default provider
  model?: string; // Override default model
  max_tokens?: number; // Token budget for response
}

// Synchronous response from /v1/ask
export interface AskResponse {
  trace_id: string; // Unique request identifier
  report: string; // Plain-text financial report
  ai_disclosure: string; // Regulatory disclosure about AI generation
  metadata?: {
    latency_ms: number;
    tokens_used: number;
    provider: LLMProvider;
    model: string;
  };
}

// Streaming event types from /v1/ask/stream
export type StreamEventType =
  | "ai_disclosure"
  | "trace"
  | "agent.handle"
  | "report"
  | "error";

export interface StreamEvent {
  event: StreamEventType;
  trace_id?: string;
  data: unknown; // Type depends on event type
}

// Agent handle event (progress update)
export interface AgentHandleEvent {
  from: string; // Agent name (e.g., "ingestor", "analyzer")
  to: string; // Next agent
  type: string; // Message type (e.g., "raw_transactions", "analysis_result")
  msg_id: string; // Message identifier
  timestamp?: string; // RFC3339
}

// WebSocket message (bidirectional)
export interface WebSocketMessage {
  event: StreamEventType;
  trace_id?: string;
  data: unknown;
}

// WebSocket input frame (user sends this)
export interface WebSocketInputFrame {
  question: string;
  document_id: string;
}

// Conversation message
export interface ConversationMessage {
  id: string;
  role: MessageRole;
  content: string;
  timestamp: string; // RFC3339
  trace_id?: string; // For tracing back to request
  metadata?: Record<string, unknown>;
}

// Conversation state
export interface Conversation {
  id: string;
  user_id: string;
  created_at: string; // RFC3339
  updated_at: string;
  messages: ConversationMessage[];
  financial_context?: FinancialContext;
  current_provider: LLMProvider;
  current_model: string;
}

// LLM Configuration & Limits
export interface LLMConfig {
  default_provider: LLMProvider;
  default_model: string;
  available_providers: LLMProvider[];
  token_budget_daily: number; // Total tokens/day available
  token_budget_used: number; // Tokens used so far
  token_budget_remaining: number;
  cost_per_1k_tokens: {
    prompt: number; // USD
    completion: number;
  };
  timeout_seconds: number;
  circuit_breaker: {
    enabled: boolean;
    threshold_errors: number; // Circuit opens after N errors
  };
}

// Completion request (internal use)
export interface CompletionRequest {
  model: string;
  messages: {
    role: MessageRole;
    content: string;
  }[];
  max_tokens?: number;
  temperature?: number;
  tools?: ToolDefinition[];
}

// Completion response
export interface CompletionResponse {
  text: string;
  tool_calls?: ToolCall[];
  finished_at: string; // RFC3339
  provider: LLMProvider;
  model: string;
  usage?: {
    input_tokens: number;
    output_tokens: number;
    total_tokens: number;
  };
}

// Tool definition (for function calling)
export interface ToolDefinition {
  name: string;
  description: string;
  input_schema: Record<string, unknown>; // JSON Schema
}

// Tool call result
export interface ToolCall {
  id: string;
  name: string;
  input: Record<string, unknown>;
}

// Compliance context
export interface ComplianceMetadata {
  user_id: string;
  user_roles: string[]; // ["user", "advisor", "admin"]
  classification: "public" | "internal" | "pii" | "secret";
  account_id?: string;
  request_timestamp: string; // RFC3339
  ip_address?: string;
  user_agent?: string;
}

// AI Disclosure (regulatory requirement)
export interface AIDisclosure {
  title: string;
  message: string; // Disclosure text (e.g., "This report is AI-generated...")
  regulatory_basis: string; // e.g., "RBI Sutra 2 (People First), Recommendation 18"
  acknowledged_at?: string; // When user acknowledged
}

// Document metadata (for context)
export interface DocumentMetadata {
  id: string;
  user_id: string;
  created_at: string;
  classification: "public" | "internal" | "pii" | "secret";
  description: string;
  encrypted: boolean; // Payload is encrypted at rest
}

// Assistant state for UI
export interface AssistantState {
  conversation: Conversation | null;
  loading: boolean;
  streaming: boolean;
  error: string | null;
  current_trace_id: string | null;
  ai_disclosure_acknowledged: boolean;
  selected_provider: LLMProvider;
  selected_model: string;
  llm_config: LLMConfig | null;
  progress_events: AgentHandleEvent[];
}

// Constitutional AI scoring (for quality)
export interface ConstitutionScore {
  score: number; // 0-10
  rubric: string; // Which sutra violated (if any)
  reasoning: string;
  timestamp: string; // RFC3339
}

// Incident tracking (RBI Annexure VI)
export interface AIIncident {
  id: string;
  occurred_at: string; // RFC3339
  detected_at: string;
  use_case: string; // "financial_advice"
  model: string;
  description: string;
  failure_mode: string; // "hallucination", "bias", etc.
  severity: "low" | "moderate" | "high" | "critical";
  affected_stakeholders: "internal" | "external" | "both";
  status: "ongoing" | "resolved";
  actor_id: string;
}
