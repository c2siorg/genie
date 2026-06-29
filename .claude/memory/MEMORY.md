# Genie Project Memory

Auto-memory for persistent Claude Code context across sessions.

---

## Project Identity

**Name**: Genie - AI Financial Assistant  
**Repository**: github.com/c2siorg/genie  
**Module**: github.com/PratikDhanave/multi-agent-reference-architecture-go  
**Author**: Pratik Dhanave  
**License**: MIT  
**Language**: Go 1.25.0  

---

## Current Status

### Phase 2: e-Rupee Commerce Integration
**Status**: ✅ COMPLETE

| Component | Status | Details |
|-----------|--------|---------|
| Handler stubs | ✅ | Payment & settlement bridges |
| Settlement consolidation | ✅ | Merchant grouping + netting |
| Reconciliation logic | ✅ | Order-to-ledger verification |
| Integration tests | ✅ | 5/5 tests passing (7.87s) |
| Code audit | ✅ | Zero Ardan Labs code |
| Attribution | ✅ | CONTRIBUTORS.md + headers |
| Build | ✅ | Successful (Go 1.25.0) |

### Tests Passing
```
✅ TestE2E_OrderToSettlement_HappyPath
✅ TestE2E_ComplianceBlocks_VelocityExceeded
✅ TestE2E_SettlementBatching_MultipleOrders
✅ TestE2E_AuditTrail_FullLineage
✅ TestE2E_Reconciliation_VerifySettlementIntegrity
✅ 13 additional unit tests
```

---

## Architecture

### Foundation
- **MARA**: Microsoft Multi-Agent Reference Architecture
- **RBI FREE-AI**: Regulatory framework compliance
- **60+ specialist agents**: Across 8 financial domains
- **Decoupled design**: HTTP handlers + internal business logic

### Phase 2 Implementation
```
Payment Workflow:
  OrderCreated → PaymentInitiated → PaymentConfirmed →
  SettlementInitiated → SettlementCompleted → Fulfilled

Key Patterns:
  • Service Stubs: Bridge workflow to handlers
  • Settlement Consolidation: Group + net by merchant
  • Reconciliation: Order-to-ledger-to-lineage verification
  • Lineage Recording: Full audit trail with hash-chain
```

### Core Modules (Phase 2)
- `pkg/commerce/handler_stubs.go` — Real agent bridges (no HTTP cycles)
- `pkg/commerce/settlement_flow.go` — Consolidation + netting
- `pkg/commerce/reconciliation.go` — Settlement verification
- `pkg/commerce/workflow.go` — State machine orchestration
- `pkg/commerce/integration_test.go` — 5 e2e test scenarios

---

## Key Technical Decisions

### 1. Service Stubs Pattern
**Why**: Decouple workflow orchestrator from HTTP handlers to avoid circular calls.

**How**: 
- `RealPaymentAgentStub` calls `erupeepayment.PaymentAgent.Initiate()` directly
- `RealSettlementAgentStub` calls `commercesettlement.SettlementExecutor.ExecuteBatch()` directly
- No HTTP round-trips within workflow execution

**Impact**: Simpler testing, faster execution, cleaner architecture

### 2. Settlement Consolidation
**Why**: Group orders for efficient CBDC ledger commits and enable netting calculations.

**How**:
- Group orders by `MerchantID`
- Apply bilateral/multilateral netting rules
- Convert positions to CBDC ledger format
- One net transaction per merchant per batch

**Impact**: Reduces ledger load, improves settlement efficiency

### 3. Order Reconciliation
**Why**: Verify order settlement integrity across order → CBDC ledger → lineage.

**How**:
- `VerifyOrderSettlement()` checks 4 conditions:
  1. Amount matches CBDC commit
  2. Lineage captures full workflow
  3. No double-spends (CBDC ledger has transaction)
  4. Settlement status is valid

**Impact**: Audit trail integrity, fraud prevention, compliance proof

### 4. Lineage Recording
**Why**: Complete transaction history for regulatory compliance (RBI FREE-AI).

**How**:
- Record at each workflow state transition
- Hash-chain integrity verification
- Events: OrderCreated, PaymentInitiated, ComplianceCheck, etc.
- Queryable via `lineage.Recorder.Verify(ctx)`

**Impact**: RBI compliance, audit readiness, forensics capability

---

## Code Patterns

### Interface-Based Design
All agent interactions use interfaces, not concrete types:
```go
type PaymentAgent interface {
    Initiate(ctx context.Context, req *InitiatePaymentRequest) (*InitiatePaymentResponse, error)
    PollStatus(ctx context.Context, paymentID string) (*PaymentStatus, error)
}
```

### Error Propagation
All errors bubble up with context:
```go
if err != nil {
    return fmt.Errorf("settlement failed: %w", err)
}
```

### Testing Strategy
- Unit tests: Each function tested independently
- Integration tests: Full workflow end-to-end in `integration_test.go`
- Mocks: In same package, prefixed with `Mock` or `Stub`
- Table-driven tests: Multiple scenarios per test function

### Naming Conventions
- Public: `PascalCase` (types, functions)
- Private: `camelCase`
- Constants: `UPPER_SNAKE_CASE` (config) or `camelCase` (enums)
- Tests: `Test<FunctionName>_<Scenario>`

---

## Common Issues & Fixes

### Issue: `erupeepayment.TypeRetail undefined`
**Solution**: Use `erupeepayment.TypePersonal` (correct constant)

### Issue: `transaction.CommitTransaction signature mismatch`
**Solution**: Use correct signature:
```go
ledger.CommitTransaction(paymentID, fromAccount, toAccount, amountPaise)
```

### Issue: `lineage.Verify()` not found
**Solution**: Use correct method:
```go
result := lineageRec.Verify(ctx)
if result.Valid {
    // Success
}
```

### Issue: Tests timeout
**Solution**: Increase timeout, check for blocking operations

### Issue: Imports failing
**Solution**: Run `go mod tidy` to clean up dependencies

---

## Development Workflow

### Quick Start
```bash
# 1. Run all tests
go test ./...

# 2. Run Phase 2 tests specifically
go test ./pkg/commerce -v

# 3. Build the API
go build ./cmd/api

# 4. Format code
go fmt ./...

# 5. Lint
golangci-lint run ./...
```

### Git Workflow
- Branch: `phase2-*` for features
- Commits: Include `Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>`
- Read-only git: status, log, diff, show, branch, remote (allowed)
- Destructive: push, reset --hard (blocked)

### Code Review Checklist
- ✅ All tests pass
- ✅ No linting errors
- ✅ Code formatted
- ✅ File has license header
- ✅ External deps in go.mod
- ✅ Lineage integration (if workflow)
- ✅ Error handling (no silent failures)

---

## Documentation

| File | Purpose |
|------|---------|
| **CLAUDE.md** | Project guidelines & development workflow |
| **PROJECT_REFERENCE.md** | Project identity, lineage, architecture |
| **ARDAN_LABS_AUDIT.md** | Code audit (zero external code) |
| **CONTRIBUTORS.md** | Attribution tracking |
| **README.md** | Project overview & features |
| **docs/architecture.md** | System design & components |
| **docs/ai-governance-security.md** | Threat model & security |
| **docs/free-ai-mapping.md** | RBI compliance mapping |
| **docs/api.md** | HTTP endpoint documentation |

---

## Dependencies

### Direct
- `chi/v5` — HTTP routing
- `pgx/v5` — PostgreSQL driver
- `OpenTelemetry` — Observability (7 packages)
- `Prometheus` — Metrics
- `Microsoft governance toolkit` — Agent governance
- `Others in go.mod` — All properly declared

### Indirect
- 50+ transitive dependencies (all in go.sum)

### Important Notes
- ✅ Zero Ardan Labs code (verified in ARDAN_LABS_AUDIT.md)
- ✅ All external code properly attributed
- ✅ Licenses checked and compatible

---

## Standards

### Compliance
- **RBI FREE-AI** — Regulatory framework
- **MARA** — Architectural reference
- **MCP** — Agent communication protocol
- **CloudEvents** — Event standard
- **AsyncAPI** — Async patterns

### Built-In Controls
- ✅ Compliance engine (AML, KYC, velocity)
- ✅ Lineage recorder (audit trail)
- ✅ Hash-chain ledger (finality)
- ✅ Reconciliation (double-spend prevention)
- ✅ Access control (JWT + RBAC)

---

## Resources

- **Repository**: https://github.com/c2siorg/genie
- **MARA Framework**: https://microsoft.github.io/multi-agent-reference-architecture/
- **RBI FREE-AI**: https://rbi.org.in/
- **Go Module**: github.com/PratikDhanave/multi-agent-reference-architecture-go

---

## Claude Code Setup

### Permissions Allowed
```
✅ go test, build, run, list, mod, fmt, vet
✅ golangci-lint run
✅ git status, log, diff, show, branch, remote
✅ gh pr/issue/run view/list, gh api
✅ grep -r, rg, find
✅ opa eval, opa test
```

### Environment Variables
```
GO_VERSION=1.25.0
GOPROXY=https://proxy.golang.org
GOSUMDB=sum.golang.org
```

### Configuration Files
- `.claude/settings.json` — Permissions + env
- `.claude/settings.local.json` — Extended dev permissions
- `.claude/launch.json` — Dev server configs
- `.claude/memory/MEMORY.md` — This file

---

**Last Updated**: June 1, 2026  
**Project Status**: Phase 2 Complete ✅  
**Next Phase**: Phase 3 (TBD)  
**License**: MIT
