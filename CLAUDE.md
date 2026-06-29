# Genie Project Guidelines for Claude Code

**Project**: Genie - AI Financial Assistant  
**Module**: github.com/PratikDhanave/multi-agent-reference-architecture-go  
**Status**: Phase 2 Complete (e-Rupee Commerce Integration)  
**License**: MIT  

---

## Project Overview

Genie is an open-source financial services platform built on **Microsoft's Multi-Agent Reference Architecture (MARA)** with **RBI FREE-AI alignment**. It combines deterministic financial logic with 60+ specialist agents covering retail finance, SME lending, KYC/AML, bancassurance, fraud detection, treasury, payments, and cyber security.

### Phase 2: e-Rupee Commerce Integration ✅
- Payment workflow orchestration for RBI digital currency (e-Rupee)
- CBDC ledger integration with settlement finality
- Order management end-to-end (creation → fulfillment)
- Compliance engine (AML, KYC, velocity monitoring)
- Settlement batching with netting calculations
- Order reconciliation and lineage tracking
- **Status**: 5/5 integration tests passing, build successful

---

## Code Organization

```
.
├── cmd/api/                 # Main API entry point
├── pkg/
│   ├── commerce/            # Phase 2: e-Rupee commerce workflows
│   │   ├── handler_stubs.go         # Real payment/settlement bridges
│   │   ├── settlement_flow.go       # Consolidation & netting
│   │   ├── reconciliation.go        # Settlement verification
│   │   ├── integration_test.go      # 5 e2e tests
│   │   └── workflow.go              # Workflow orchestration
│   ├── erupeepayment/       # Payment agent (Phase 1)
│   ├── cbdc/                # CBDC ledger (Phase 1)
│   ├── merchant/            # Merchant onboarding
│   ├── compliance/          # Compliance engine
│   ├── lineage/             # Audit trail tracking
│   └── web/handlers/        # HTTP handlers
├── go.mod                   # Go module definition (1.25.0)
├── Makefile                 # Build automation
├── PROJECT_REFERENCE.md     # Project lineage & context
├── CONTRIBUTORS.md          # Attribution tracking
└── ARDAN_LABS_AUDIT.md      # Code audit (zero external code)
```

---

## Development Workflow

### Build & Test
```bash
# Run all tests
go test ./...

# Run Phase 2 commerce tests specifically
go test ./pkg/commerce -v

# Build the API server
go build ./cmd/api

# Format code
go fmt ./...

# Lint code
golangci-lint run ./...
```

### Key Commands
```bash
# View all available commands
make help

# Start dev environment
make up

# Run tests
make test

# Clean up
make down
```

### Git Workflow
- **Branch naming**: `phase2-*` for features, `fix-*` for bugs
- **Commits**: Use `Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>` trailer
- **Destructive operations blocked**: `git push`, `git reset --hard` require explicit confirmation
- **Read-only git operations allowed**: `git status`, `git log`, `git diff`, `git show`, `git branch`, `git remote`

---

## Code Patterns & Conventions

### Architecture Patterns
1. **Service Stubs Pattern** — Bridge workflow to handlers without HTTP circular calls
   - `RealPaymentAgentStub` — Directly calls payment agent
   - `RealSettlementAgentStub` — Directly calls settlement executor

2. **Settlement Consolidation** — Group orders by merchant with netting
   - `ConsolidateSettlementBatch()` — Applies merchant grouping
   - `ApplyNettingRules()` — Bilateral/multilateral netting placeholder
   - `ConvertToCBDCPositions()` — Format conversion for ledger

3. **Order Reconciliation** — Verify settlement integrity
   - `VerifyOrderSettlement()` — Validates order-to-ledger-to-lineage
   - `IsReconciled()` — Returns true if all checks pass

4. **Workflow Orchestration** — State machine execution
   - States: OrderCreated → PaymentInitiated → PaymentConfirmed → SettlementInitiated → SettlementCompleted → Fulfilled
   - Lineage capture at each transition
   - Compliance check integration

### File Structure
- **Package documentation**: Every package has a header comment explaining purpose and licensing
- **Test files**: `*_test.go` in same package, integration tests in `integration_test.go`
- **Mocks/Stubs**: Live in same package, prefixed with `Mock` or `Stub`
- **Error handling**: All errors propagated with context, no silent failures

### Naming Conventions
- **Interfaces**: `Handler`, `Recorder`, `Executor`, `Agent`
- **Structs**: `PascalCase` (e.g., `ReconciliationCheck`, `SettlementBatch`)
- **Functions**: `PascalCase` for public, `camelCase` for private
- **Constants**: `UPPER_SNAKE_CASE` for config, `camelCase` for enums
- **Test functions**: `Test<FunctionName>_<Scenario>`

---

## Testing Requirements

### Unit Tests
- Every exported function must have a test
- Test both happy path and error cases
- Use table-driven tests for multiple scenarios
- Mock external dependencies

### Integration Tests
- Located in `integration_test.go`
- Test full workflow end-to-end
- Verify state transitions
- Check lineage capture

### Current Test Coverage
```
pkg/commerce:
  ✅ TestE2E_OrderToSettlement_HappyPath
  ✅ TestE2E_ComplianceBlocks_VelocityExceeded
  ✅ TestE2E_SettlementBatching_MultipleOrders
  ✅ TestE2E_AuditTrail_FullLineage
  ✅ TestE2E_Reconciliation_VerifySettlementIntegrity
  + 13 additional unit tests (all passing)
```

### Running Tests
```bash
# Run with verbose output
go test ./pkg/commerce -v

# Run with coverage
go test -cover ./...

# Run specific test
go test -run TestE2E_OrderToSettlement_HappyPath ./pkg/commerce
```

---

## Code Attribution & Licensing

### Important Rules
- ✅ **All Phase 2 code is 100% original** — Created specifically for Genie
- ✅ **Proper attribution** — Every file has license headers
- ✅ **Zero Ardan Labs code** — Comprehensive audit in `ARDAN_LABS_AUDIT.md`
- ✅ **External dependencies declared** — All in `go.mod` with proper licenses

### When Adding New Code
1. Add file header with package documentation
2. Include author/license comment (MIT)
3. Declare any new external dependencies in go.mod
4. Update CONTRIBUTORS.md if significant contribution
5. Reference PROJECT_REFERENCE.md for context

### External Dependencies
```
Direct:
  ✅ chi/v5           — HTTP routing
  ✅ pgx/v5           — PostgreSQL driver
  ✅ OpenTelemetry    — Observability (7 packages)
  ✅ Prometheus       — Metrics
  ✅ Microsoft toolkit — Governance
  ✅ Others in go.mod

Indirect:
  ✅ 50+ transitive (all in go.sum, managed by Go)
```

---

## Standards & Frameworks

### Architecture Standards
- **MARA** — Microsoft Multi-Agent Reference Architecture
- **RBI FREE-AI** — Indian regulatory framework for AI in finance
- **MCP** — Model Context Protocol for agent communication
- **CloudEvents** — Standard event format
- **AsyncAPI** — Async communication patterns

### Compliance & Governance
- **AML/KYC** — Integrated compliance checks
- **Velocity Monitoring** — Transaction limit enforcement
- **Lineage Recording** — Complete audit trails
- **Hash-Chain Integrity** — CBDC ledger finality
- **No Double-Spends** — Reconciliation verification

---

## Common Development Tasks

### Adding a New Handler
```go
// 1. Define types in types.go
type MyRequest struct { ... }

// 2. Implement handler in handler.go
func (h *Handler) MyHandler(w http.ResponseWriter, r *http.Request) {
    // Implementation
}

// 3. Register in router
r.Post("/v1/my-endpoint", h.MyHandler)

// 4. Add tests in handler_test.go
func TestMyHandler(t *testing.T) { ... }
```

### Adding a Test
```go
func TestMyFunction_Scenario(t *testing.T) {
    // Arrange
    input := setupInput()
    
    // Act
    result, err := MyFunction(input)
    
    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !expectedResult(result) {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

### Checking Lineage
```go
// Query lineage for an order
lineageResult := lineageRec.Verify(ctx)
if !lineageResult.Valid {
    t.Fatalf("lineage broken at %s: %v", lineageResult.BrokenAt, lineageResult.Error)
}
```

---

## Troubleshooting

### Build Issues
- **Module not found**: Run `go mod tidy`
- **Import errors**: Check package names in `go.mod`
- **Compilation errors**: Ensure Go 1.25.0+

### Test Failures
- **Timeout**: Increase test timeout in test flags
- **Mocks failing**: Check if interfaces match actual implementations
- **Lineage tests**: Ensure `lineage.Recorder` is properly initialized

### Common Errors
| Error | Solution |
|-------|----------|
| `erupeepayment.TypeRetail undefined` | Use `erupeepayment.TypePersonal` |
| `transaction.CommitTransaction signature mismatch` | Use: `ledger.CommitTransaction(paymentID, fromAccount, toAccount, amountPaise)` |
| `lineage.Verify() not found` | Use correct method: `lineageRec.Verify(ctx)` returns `LineageIntegrityResult` |

---

## Useful Resources

- **Repository**: https://github.com/c2siorg/genie
- **Module**: github.com/PratikDhanave/multi-agent-reference-architecture-go
- **Documentation**: `docs/` directory
  - `architecture.md` — System design
  - `ai-governance-security.md` — Security & threat model
  - `free-ai-mapping.md` — RBI compliance mapping
  - `api.md` — HTTP endpoints
- **MARA Framework**: https://microsoft.github.io/multi-agent-reference-architecture/
- **RBI FREE-AI**: https://rbi.org.in/

---

## Quick Reference

### Permissions Allowed in Claude Code
```
✅ go test, build, run, list, mod, fmt, vet
✅ golangci-lint run
✅ git status, log, diff, show, branch, remote
✅ gh pr/issue/run view/list
✅ grep -r, rg, find
✅ opa eval, opa test
```

### Permissions Denied
```
❌ git push, git reset --hard
❌ rm -rf (destructive operations)
```

### Environment
```
GO_VERSION=1.25.0
GOPROXY=https://proxy.golang.org
GOSUMDB=sum.golang.org
```

---

**Last Updated**: June 1, 2026  
**Phase**: 2 (e-Rupee Commerce) ✅ Complete  
**License**: MIT  
**Governance**: RBI FREE-AI Aligned
