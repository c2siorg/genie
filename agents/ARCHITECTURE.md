# Agent Architecture — Decoupled, Deployable, Universal

**Status**: Phase 7 implementation  
**Goal**: Agents run anywhere (frontend, backend, cloud platforms, standalone services)  
**Pattern**: Inspired by [Google Cloud Agent Platform](https://console.cloud.google.com/projectselector2/agent-platform)

---

## 📦 Core Principles

1. **Stateless**: Agents have zero internal state. All data flows through context/input.
2. **Decoupled**: Agents don't know about HTTP, database, or framework-specific code.
3. **Composable**: Agents combine into pipelines via standard interfaces.
4. **Deployable**: Same agent code runs as HTTP service, embedded in backend, or in frontend WebWorker.
5. **Observable**: All execution carries trace_id for audit/debugging.

---

## 🏗️ Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    Agent Abstraction Layer                       │
│  agents/core/agent.go → Agent interface (Execute, Health, Name)  │
└─────────────────────────────────────────────────────────────────┘
                                    ▲
                    ┌───────────────┼───────────────┐
                    │               │               │
            ┌───────────────┐  ┌────────────┐  ┌──────────────┐
            │ProfileAnalyzer│  │FinancialAn │  │Recommendation│
            │    Agent      │  │ alyst Agent│  │ GeneratorAgent│
            │               │  │            │  │               │
            │agents/agents/ │  │agents/agents │ │agents/agents/ │
            │profile-...    │  │financial-... │ │recommendation │
            └───────────────┘  └────────────┘  └──────────────┘
                    ▲               ▲               ▲
                    │               │               │
         ┌──────────┴───────────────┼───────────────┴──────────┐
         │                          │                          │
         │    Transport Layer       │    Client Layer          │
         │                          │                          │
    ┌─────────────────┐      ┌─────────────────┐   ┌────────────────────┐
    │  HTTP Transport │      │  HTTP Client    │   │  Memory Client     │
    │ agents/transpo- │      │ agents/clients/ │   │ agents/clients/    │
    │ rts/http.go     │      │ http_client.go  │   │ memory_client.go   │
    │                 │      │                 │   │                    │
    │ • /health       │      │ Call remote     │   │ Call local agent   │
    │ • /execute      │      │ agents via HTTP │   │ (no network)       │
    │ • /info         │      │                 │   │                    │
    └─────────────────┘      └─────────────────┘   └────────────────────┘
         │                          │                          │
         │                          │                          │
    ┌────┴──────────┐      ┌────────┴───────┐      ┌───────────┴────┐
    │                │      │                │      │                │
  DEPLOYMENT OPTIONS  │      │                │      │                │
    │                │      │                │      │                │
┌─────────────────┐  │  ┌──────────────┐   │   ┌──────────────┐
│ Standalone      │  │  │ Backend API  │   │   │ Frontend     │
│ Service         │  │  │ (Embedded)   │   │   │ (WebWorker)  │
│                 │  │  │              │   │   │              │
│ $ agent serve   │  │  │ backend/main │   │   │ worker.js    │
│   :8080         │  │  │   calls      │   │   │ (via HTTP)   │
└─────────────────┘  │  │ MemoryClient │   │   └──────────────┘
                     │  │              │   │
                     │  └──────────────┘   │
                     └─────────────────────┘
```

---

## 🎯 The Three Deployment Models

### 1. **Embedded (MemoryClient)**
Agents run in same process as backend. Zero network latency.
```go
// Backend code
registry := clients.NewAgentRegistry()
analyzer := profile_analyzer.NewAgent()
registry.Register(analyzer)

// Call directly
client := clients.NewMemoryClient(analyzer)
profile, err := client.Execute(ctx, input)
```

### 2. **HTTP Service**
Standalone agent deployed as separate service. Network-isolated, scalable.
```bash
# Deploy ProfileAnalyzer as standalone service
$ go run ./agents/cmd/profile-analyzer-server
# Listens on :8080

# Backend calls it
client := clients.NewHTTPClient("http://profile-analyzer:8080", "profile-analyzer")
profile, err := client.Execute(ctx, input)
```

### 3. **Cloud Platform**
Deploy to Google Cloud Agent Platform, AWS Bedrock, Azure, etc. with minimal changes.
```go
// Agent code has ZERO cloud-specific dependencies
// Just implements core.Agent interface
// Cloud platform handles: scaling, monitoring, multi-tenancy, etc.
```

---

## 📂 Directory Structure

```
agents/
├── core/                          # ✅ PHASE 1: Agent abstraction
│   ├── agent.go                   # Core Agent interface
│   ├── context.go                 # Execution context (trace_id, user_id)
│   ├── errors.go                  # Agent error types
│   └── advisor_types.go           # Advisor domain types (shared)
│
├── agents/                        # ✅ PHASE 2: Agent implementations
│   ├── profile-analyzer/
│   │   ├── agent.go               # ProfileAnalyzer implementation
│   │   └── agent_test.go          # Unit tests
│   ├── financial-analyst/
│   │   ├── agent.go               # FinancialAnalyst implementation
│   │   └── agent_test.go
│   └── recommendation-generator/
│       ├── agent.go               # RecommendationGenerator implementation
│       └── agent_test.go
│
├── transports/                    # ✅ PHASE 3: HTTP/gRPC servers
│   ├── http.go                    # HTTPServer - expose agents as HTTP
│   ├── grpc.go                    # (Future) gRPC server
│   └── websocket.go               # (Future) WebSocket for streaming
│
├── clients/                       # ✅ PHASE 3: Agent clients
│   ├── memory_client.go           # Call agents directly (embedded)
│   ├── http_client.go             # Call agents via HTTP (remote)
│   └── grpc_client.go             # (Future) Call agents via gRPC
│
├── storage/                       # PHASE 4: Persistence abstraction
│   ├── store.go                   # Store interface
│   ├── memory.go                  # In-memory store
│   └── postgres.go                # PostgreSQL store
│
├── cmd/                           # PHASE 4: Standalone servers
│   ├── profile-analyzer-server/   # Standalone ProfileAnalyzer service
│   ├── analyst-server/            # Standalone FinancialAnalyst service
│   └── generator-server/          # Standalone RecommendationGenerator service
│
└── ARCHITECTURE.md                # This file
```

---

## 🔄 Execution Flow

### Example: RecommendationGenerator Pipeline

```
Frontend (user clicks "Get Recommendation")
    │
    ├─→ HTTP: POST /v1/advisor/recommendation
    │        (to backend)
    │
Backend API Handler
    │
    ├─→ Creates ExecutionContext (user_id, trace_id)
    │
    ├─→ Orchestrates Pipeline:
    │   1. ProfileAnalyzer.Execute(ctx, {"user_id": "u123"})
    │      └─→ Returns UserProfile
    │
    │   2. FinancialAnalyst.Execute(ctx, userProfile)
    │      └─→ Returns []SpendingOpportunity
    │
    │   3. RecommendationGenerator.Execute(ctx, opportunity)
    │      └─→ Returns Recommendation
    │
    └─→ Stores in database
    └─→ Returns to Frontend
```

### Example: Calling Remote Agent

```go
// Backend wants to call ProfileAnalyzer from separate service
ctx := core.WithContext(context.Background(), core.ExecutionContext{
    UserID:  "user-123",
    TraceID: "tr-abc-def",
})

// Option 1: Remote call (HTTP)
client := clients.NewHTTPClient("http://profile-analyzer:8080", "profile-analyzer")
profile, err := client.Execute(ctx, &ProfileAnalyzerInput{
    UserID:    "user-123",
    KYCStatus: "verified_standard",
})

// Option 2: Embedded call (Memory)
agent := profile_analyzer.NewAgent()
client := clients.NewMemoryClient(agent)
profile, err := client.Execute(ctx, &ProfileAnalyzerInput{...})

// Both options have IDENTICAL interface!
```

---

## 🚀 Implementation Roadmap

### Phase 1: Core Abstraction ✅
- [ ] Agent interface + ExecutionContext
- [ ] Error types
- [ ] Type definitions (shared across agents)

### Phase 2: Agent Implementations ✅
- [ ] ProfileAnalyzer agent (decoupled from HTTP)
- [ ] FinancialAnalyst agent
- [ ] RecommendationGenerator agent
- [ ] Unit tests for each

### Phase 3: Transport Layer ✅
- [ ] HTTP server (expose agents)
- [ ] HTTPClient (call remote agents)
- [ ] MemoryClient (call embedded agents)
- [ ] Agent registry (manage pipelines)

### Phase 4: Standalone Services
- [ ] ProfileAnalyzer service (cmd/profile-analyzer-server/)
- [ ] FinancialAnalyst service (cmd/analyst-server/)
- [ ] RecommendationGenerator service (cmd/generator-server/)
- [ ] Kubernetes manifests

### Phase 5: Storage Abstraction
- [ ] Store interface (database-agnostic)
- [ ] In-memory store (MVP)
- [ ] PostgreSQL store (production)
- [ ] Migration path

### Phase 6: Advanced Features
- [ ] gRPC transport (faster than HTTP)
- [ ] WebSocket streaming (real-time UI updates)
- [ ] Rate limiting + auth middleware
- [ ] Observability (traces, metrics, logs)

---

## 💡 Key Design Decisions

### 1. **Agents are stateless functions**
```go
// Agent.Execute(ctx, input) → (output, error)
// No agent.state, agent.db, agent.cache
// All data flows: input → agent → output
```

### 2. **Context carries request metadata**
```go
// Instead of: agent.userID, agent.traceID (couples agents to HTTP)
// Use: ctx = core.WithContext(ctx, ExecutionContext{UserID, TraceID})
```

### 3. **Clients implement Agent interface**
```go
// HTTPClient, MemoryClient both implement core.Agent
// Pipeline doesn't know if agent is local or remote
type Pipeline struct {
    Agents []core.Agent  // Could be mix of local + remote!
}
```

### 4. **Agents don't know about storage**
```go
// Agent returns Recommendation (or any output type)
// Backend handles: validation, storage, audit logging
// Agent is pure: input → logic → output
```

---

## 🧪 Testing Strategy

### Unit Tests (in each agent)
```go
func TestProfileAnalyzer_Execute(t *testing.T) {
    agent := profile_analyzer.NewAgent()
    ctx := context.Background()
    
    profile, err := agent.Execute(ctx, &ProfileAnalyzerInput{
        UserID:    "u123",
        KYCStatus: "verified_standard",
    })
    
    // Assert profile properties
}
```

### Integration Tests (Pipeline)
```go
func TestRecommendationPipeline(t *testing.T) {
    registry := clients.NewAgentRegistry()
    registry.Register(profile_analyzer.NewAgent())
    registry.Register(analyst.NewAgent())
    registry.Register(generator.NewAgent())
    
    pipeline := registry.CreatePipeline("full", "profile-analyzer", "analyst", "generator")
    
    result, err := pipeline.Execute(ctx, input)
    // Assert full pipeline result
}
```

### E2E Tests (with HTTP transport)
```go
func TestHTTPServer_Execute(t *testing.T) {
    agent := profile_analyzer.NewAgent()
    server := transports.NewHTTPServer(agent)
    
    // Start server on random port
    // Call via HTTPClient
    // Assert result
}
```

---

## 📊 Benefits by Deployment Model

| Aspect | Embedded | HTTP Service | Cloud Platform |
|--------|----------|--------------|----------------|
| **Latency** | <1ms | 10-50ms | 50-200ms |
| **Scaling** | Vertical | Horizontal | Auto-scaling |
| **Dev Env** | Single binary | Docker Compose | Cloud provider |
| **Complexity** | Low | Medium | High |
| **Best For** | Development | Microservices | Enterprise |

---

## 🔐 Security Considerations

1. **Authentication**: HTTPClient should use TLS + bearer tokens
2. **Authorization**: ExecutionContext should carry user permissions
3. **Audit**: All Execute calls logged with trace_id
4. **Rate Limiting**: HTTPServer should enforce rate limits
5. **Input Validation**: Every agent validates its input

---

## 📈 Future Enhancements

- **Agent versioning**: Multiple versions coexist, clients pin versions
- **Agent composition**: Complex agents built from simpler ones
- **Feedback loops**: Agents learn from feedback
- **A/B testing**: Route requests to different agent versions
- **Multi-language**: Same interface in Python, Node, Rust
- **Observability**: OpenTelemetry instrumentation
- **Rate limiting**: Token buckets per user, org, endpoint

---

## 🎓 Learning Resources

- [Agent abstraction layer](./core/agent.go) — Start here
- [HTTPServer implementation](./transports/http.go) — See transport layer
- [ProfileAnalyzer example](./agents/profile-analyzer/agent.go) — See agent pattern
- [Memory client](./clients/memory_client.go) — Understand embedding
- [HTTP client](./clients/http_client.go) — Understand remote calls

---

**Questions?** File an issue or reach out to the team.  
**Ready to deploy?** See [DEPLOYMENT.md](./DEPLOYMENT.md) for production setup.
