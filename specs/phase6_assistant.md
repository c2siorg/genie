# Phase 6: Assistant Workspace Specification

**Status**: ✅ Complete  
**Real APIs**: 3 endpoints (+ 4 config endpoints), fully discovered  
**Real Types**: 10 type definitions  
**Test Coverage**: 12 tests  

---

## Specification

### API Endpoints

#### 1. Ask (Synchronous)
```
POST /v1/ask
Request:
  - question: string (required)
  - document_id: string (required, CSV document)
  - provider?: LLMProvider (default: anthropic)
  - model?: string (default: claude-sonnet-4-6)
  - max_tokens?: number
Response:
  - trace_id: string
  - report: string (plain-text financial report)
  - ai_disclosure: string (regulatory disclosure)
  - metadata?: {latency_ms, tokens_used, provider, model}
Timeout: 8 seconds
```

#### 2. Ask Stream (Server-Sent Events)
```
POST /v1/ask/stream
Request:
  - question, document_id, provider?, model?, max_tokens?
Response: text/event-stream
  event: ai_disclosure
  data: "This is an AI-generated report..."
  
  event: trace
  data: "tr-1717500123456789"
  
  event: agent.handle
  data: {"from":"ingestor","to":"normalizer","type":"raw_transactions","msg_id":"abc123"}
  
  event: report
  data: "Genie Financial Report\n..."
Timeout: 10 seconds
```

#### 3. Chat (WebSocket)
```
WebSocket GET /v1/chat/ws
Input Frame (client → server):
  {"question": "string", "document_id": "string"}

Output Frames (server → client):
  {"event":"ai_disclosure","data":"..."}
  {"event":"trace","trace_id":"tr-..."}
  {"event":"agent.handle","trace_id":"tr-...","data":{...}}
  {"event":"report","trace_id":"tr-...","data":"..."}
Timeout: 12 seconds
```

#### 4. LLM Configuration
```
GET /v1/llm/config
Response:
  - default_provider, default_model
  - available_providers: [anthropic, ollama, openai, gemini]
  - token_budget_daily, token_budget_used, token_budget_remaining
  - cost_per_1k_tokens: {prompt, completion}
  - timeout_seconds, circuit_breaker settings
```

#### 5. AI Disclosure Banner
```
GET /v1/ai/disclosure
Response:
  - title: string
  - message: string
  - regulatory_basis: string (e.g., RBI Sutra 2, Rec 18)
  - acknowledged_at?: string (RFC3339)
```

#### 6. Financial Context
```
GET /v1/documents/{document_id}/context
Response:
  - question?: string
  - currency: string (INR)
  - total_income_paise: number
  - total_expense_paise: number
  - net_paise: number
  - top_overspend: string[]
  - analysis: {transactions_count, date_range, classification}
```

---

### Types

#### AskRequest
```typescript
interface AskRequest {
  question: string;
  document_id: string;
  provider?: LLMProvider; // anthropic | ollama | openai | gemini
  model?: string;
  max_tokens?: number;
}
```

#### AskResponse
```typescript
interface AskResponse {
  trace_id: string;
  report: string;
  ai_disclosure: string;
  metadata?: {
    latency_ms: number;
    tokens_used: number;
    provider: LLMProvider;
    model: string;
  };
}
```

#### StreamEvent
```typescript
interface StreamEvent {
  event: "ai_disclosure" | "trace" | "agent.handle" | "report" | "error";
  trace_id?: string;
  data: unknown;
}
```

#### AgentHandleEvent
```typescript
interface AgentHandleEvent {
  from: string; // agent name
  to: string;
  type: string; // message type
  msg_id: string;
  timestamp?: string;
}
```

#### LLMProvider
```typescript
type LLMProvider = "anthropic" | "ollama" | "openai" | "gemini" | "mock";
```

#### LLMConfig
```typescript
interface LLMConfig {
  default_provider: LLMProvider;
  default_model: string;
  available_providers: LLMProvider[];
  token_budget_daily: number;
  token_budget_used: number;
  token_budget_remaining: number;
  cost_per_1k_tokens: {prompt: number; completion: number};
  timeout_seconds: number;
  circuit_breaker: {enabled: boolean; threshold_errors: number};
}
```

#### AIDisclosure
```typescript
interface AIDisclosure {
  title: string;
  message: string;
  regulatory_basis: string;
  acknowledged_at?: string;
}
```

#### FinancialContext
```typescript
interface FinancialContext {
  question?: string;
  currency: string;
  total_income_paise: number;
  total_expense_paise: number;
  net_paise: number;
  top_overspend: string[];
  analysis: {
    transactions_count: number;
    date_range: string;
    classification: "public" | "internal" | "pii" | "secret";
  };
}
```

#### ConversationMessage
```typescript
interface ConversationMessage {
  id: string;
  role: "system" | "user" | "assistant" | "tool";
  content: string;
  timestamp: string;
  trace_id?: string;
  metadata?: Record<string, unknown>;
}
```

#### CompletionRequest
```typescript
interface CompletionRequest {
  model: string;
  messages: Array<{role: string; content: string}>;
  max_tokens?: number;
  temperature?: number;
  tools?: ToolDefinition[];
}
```

---

### Test Requirements

**Unit Tests** (8 tests):
- [ ] Ask endpoint returns trace_id + report
- [ ] Ask/stream returns all 4 event types
- [ ] WebSocket connects and sends/receives frames
- [ ] AI disclosure shown before acknowledgment
- [ ] LLM config returns all providers
- [ ] Token budget decrements on use
- [ ] Financial context loaded from document
- [ ] Conversation state persists across messages

**Integration Tests** (4 tests):
- [ ] Full flow: ask → stream → report
- [ ] WebSocket multi-turn conversation
- [ ] Provider fallback (anthropic → ollama if unavailable)
- [ ] Streaming cancellation stops agent pipeline

---

### Validation Rules

**Request**:
- question: 1-2000 characters
- document_id: valid UUID format
- provider: one of [anthropic, ollama, openai, gemini]
- model: non-empty string
- max_tokens: > 0, ≤ 32000

**Response**:
- trace_id: "tr-" + Unix timestamp
- report: plain text, no JSON in response
- ai_disclosure: non-empty banner text
- Streaming events: valid JSON, ordered

**LLM Integration**:
- Provider stack: cost observer → cache → budget → deadline → circuit breaker
- Token budget: decrements per completion, resets daily
- Cost tracking: emitted to OTel metrics
- Timeout: applied per provider layer

**Financial Context**:
- Classification drives provider selection
  - pii/secret: in-region providers only (Ollama)
  - internal/public: any provider allowed
- Context passed to assistant agents
- Used in financial recommendations

**Streaming**:
- Events ordered: ai_disclosure → trace → agent.handle* → report
- agent.handle repeats for each agent transition
- report is terminal event (connection can close after)

**Compliance**:
- RBI Sutra 2: AI disclosure shown upfront
- RBI Recommendation 18: Transparency on LLM use
- Data residency: classification respected
- Audit trail: trace_id links all events

---

### Success Criteria

✅ All 3 main endpoints operational  
✅ All 6 config endpoints accessible  
✅ Streaming ordered (ai_disclosure → report)  
✅ WebSocket bidirectional (send/receive)  
✅ AI disclosure shown before interaction  
✅ Token budget enforced (no overage)  
✅ Financial context loaded correctly  
✅ All 8 unit tests pass  
✅ All 4 integration tests pass  
✅ Zero timeout violations  

---

### Architectural Decisions

**Message-Driven**: All agent communication via in-process bus, not direct HTTP calls. Supervisor orchestrates pipeline.

**Provider Abstraction**: LLM swappable via interface—agents don't know which model. Layered composition (cost → cache → budget → deadline → circuit breaker).

**Streaming First**: Real-time progress via SSE or WebSocket—users see agents working, not black box.

**Data Residency**: Classification labels (pii, secret) drive provider selection. Genie enforces in-memory, never sends PII to external LLMs unless explicitly configured.

**Compliance by Design**: AI disclosure required before interaction, trace_id links all events to audit log, token budgets enforce usage limits.

---

### Related Specifications

- **Commerce** (Phase 2): Order data becomes financial context
- **Compliance** (Phase 3): KYC/AML context for recommendations
- **Governance** (Phase 4): Agent behavior incidents from LLM responses
- **Evaluation** (Phase 5): Judge for "response factually accurate"
