# Project Reference & Lineage

## Project Identity

**Project Name**: Genie - AI Financial Assistant (Go)  
**Type**: Open-source financial services platform  
**License**: MIT  
**Language**: Go 1.25+  

---

## Repository Information

| Attribute | Value |
|-----------|-------|
| **Repository** | github.com/c2siorg/genie |
| **Organization** | c2siorg (Code2SI) |
| **Organization Type** | Open-source community |
| **Module Name** | github.com/PratikDhanave/multi-agent-reference-architecture-go |
| **Author/Lead** | Pratik Dhanave |
| **Repository URL** | https://github.com/c2siorg/genie |

---

## Project Foundation & Architecture

### Based On
- **Microsoft's Multi-Agent Reference Architecture (MARA)**
  - https://microsoft.github.io/multi-agent-reference-architecture/index.html
  - Industry-standard pattern for multi-agent systems
  - 7 Sutras + 6 Pillars + 26 Recommendations

### Alignment
- **RBI FREE-AI Report** (August 2025)
  - Regulatory framework for AI in Indian financial services
  - 7 Pillars of responsible AI
  - Comprehensive governance and safety requirements

### Standards & Protocols
- **MCP (Model Context Protocol)** - Standardized tool communication
- **A2A (Agent-to-Agent Protocol)** - Inter-agent communication
- **CloudEvents** - Standard event format
- **AsyncAPI** - Async communication patterns
- **OpenInference** - Tracing and observability standard

---

## Project Scope

### Core Capability
AI-powered financial assistant that combines:
- Deterministic financial logic
- Specialist agents (60+)
- GenAI engineering layer
- Governance and safety

### Domain Coverage
- **Retail Finance**: Account management, deposits, lending
- **SME Lending**: Business loans, credit assessment
- **KYC/AML**: Customer onboarding, compliance
- **Bancassurance**: Insurance products, risk assessment
- **Fraud Detection**: Real-time anomaly detection
- **Treasury**: FX, interest rate management
- **Payments**: Settlement, reconciliation
- **Cyber**: Security monitoring

### Technology Stack
- **Language**: Go 1.25+
- **HTTP Framework**: chi/v5 (router & middleware)
- **Database**: PostgreSQL (pgx/v5 driver)
- **Observability**: OpenTelemetry (traces, metrics, logs)
- **Metrics**: Prometheus
- **Encryption**: Go crypto standards
- **WebSockets**: coder/websocket
- **Identity**: WebAuthn, OAuth 2.1, DID/VC
- **LLM Runtime**: Ollama (on-prem by default)

---

## Phase 2: e-Rupee Commerce Integration

### Implementation Scope
This Phase 2 implementation adds:
- **e-Rupee Payment Workflow** - RBI digital currency payments
- **CBDC Ledger Integration** - Distributed ledger for settlement
- **Commerce Order Management** - End-to-end order lifecycle
- **Compliance Engine** - AML/KYC/Velocity checks
- **Merchant Onboarding** - Business account setup
- **Settlement & Batching** - Consolidated settlement with netting
- **Reconciliation** - Order-to-ledger verification
- **Lineage & Audit Trail** - Complete transaction history

### Modules Created
```
pkg/commerce/
  ├── handler_stubs.go        # Payment/Settlement bridges
  ├── settlement_flow.go       # Consolidation & netting
  ├── reconciliation.go        # Settlement verification
  └── integration_test.go      # End-to-end tests (5 scenarios)

cmd/api/
  └── main.go                  # Settlement executor wiring
```

### Key Patterns
- **Service Stubs Pattern**: Bridge workflow to handlers without HTTP cycles
- **Settlement Consolidation**: Group orders by merchant with netting
- **Order Reconciliation**: Validate order → ledger → lineage integrity
- **CBDC Integration**: Direct ledger commit with transaction recording

---

## Code Lineage & Attribution

### Original Code
- **100% Original Implementation** in Phase 2
  - Created by Claude Haiku 4.5
  - Designed for Genie e-Rupee commerce
  - No code copied from external sources

### External Dependencies
All properly declared in `go.mod`:
```
Direct Dependencies:
  ✅ github.com/go-chi/chi/v5 - HTTP routing
  ✅ github.com/google/uuid - UUID generation
  ✅ github.com/jackc/pgx/v5 - PostgreSQL driver
  ✅ github.com/prometheus/client_golang - Metrics
  ✅ go.opentelemetry.io/* - Observability (7 packages)
  ✅ github.com/coder/websocket - WebSocket support
  ✅ github.com/microsoft/agent-governance-toolkit - Governance
  ✅ golang.org/x/crypto - Cryptography
  ✅ gopkg.in/yaml.v3 - YAML parsing

Indirect Dependencies:
  ✅ 50+ transitive dependencies, all in go.sum
```

### No Ardan Labs Code
Complete audit performed:
- ❌ Zero Ardan Labs imports
- ❌ Zero Ardan Labs dependencies
- ❌ Zero Ardan Labs patterns
- ❌ Zero Ardan Labs references

**See**: ARDAN_LABS_AUDIT.md for complete audit report

### Proper Attribution
- **LICENSE**: MIT
- **CONTRIBUTORS.md**: Contribution documentation
- **ARDAN_LABS_AUDIT.md**: Code audit report
- **PROJECT_REFERENCE.md**: This file
- **File Headers**: Package documentation in all new files

---

## Governance & Safety

### RBI FREE-AI Alignment
The implementation follows RBI's AI governance framework:
1. **Responsible AI** - Safety by design
2. **Transparency** - Complete audit trails
3. **Accountability** - Lineage tracking
4. **Explainability** - Decision documentation
5. **Security** - Multi-layer protection
6. **Privacy** - Data protection
7. **Fairness** - Unbiased processing

### Built-in Controls
- **Compliance Engine**: AML, velocity, fraud checks
- **Lineage Recorder**: Full transaction history
- **CBDC Ledger**: Immutable settlement record
- **Reconciliation**: Order-ledger verification
- **Access Control**: JWT + RBAC
- **Observability**: Complete trace coverage

---

## Build & Test Status

✅ **Build**: Successful (Go 1.25+)  
✅ **Tests**: 5/5 integration tests passing  
✅ **Coverage**: e-Rupee workflow end-to-end  
✅ **Audit**: Clean (no Ardan Labs code)  

---

## Documentation References

| Document | Purpose |
|----------|---------|
| README.md | Project overview, 60+ agents, architecture |
| docs/architecture.md | System design and component relationships |
| docs/ai-governance-security.md | Threat model, defense layers, security |
| docs/free-ai-mapping.md | RBI FREE-AI alignment mapping |
| docs/api.md | HTTP API endpoints and usage |
| CONTRIBUTORS.md | Contribution and attribution tracking |
| ARDAN_LABS_AUDIT.md | Complete code audit report |
| PROJECT_REFERENCE.md | This file - project lineage and context |

---

## Related Resources

- **Microsoft MARA**: https://microsoft.github.io/multi-agent-reference-architecture/
- **RBI FREE-AI Report**: https://rbi.org.in/
- **MCP Protocol**: https://modelcontextprotocol.io/
- **e-Rupee (CBDC)**: RBI Digital Currency Initiative

---

## Summary

**Genie** is an open-source, RBI FREE-AI aligned financial assistant built on Microsoft's MARA architecture. Phase 2 adds production-grade e-Rupee commerce integration with complete settlement, reconciliation, and compliance workflows. All code is original with proper attribution and zero external code reuse.

---

**Last Updated**: June 1, 2026  
**Project Status**: Active Development  
**License**: MIT  
**Governance**: RBI FREE-AI Aligned
