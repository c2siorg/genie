# /genie-test — Run Phase 2 Commerce Tests

Run the e-Rupee commerce integration test suite for Phase 2.

## Usage

```
/genie-test [flags]
```

## Examples

- `/genie-test` — Run all commerce tests with verbose output
- `/genie-test -short` — Run quick tests only
- `/genie-test -run TestE2E` — Run only e2e tests
- `/genie-test -v` — Verbose output with test details

## What it does

Executes: `go test ./pkg/commerce -v`

Runs the Phase 2 integration test suite:
- ✅ TestE2E_OrderToSettlement_HappyPath
- ✅ TestE2E_ComplianceBlocks_VelocityExceeded
- ✅ TestE2E_SettlementBatching_MultipleOrders
- ✅ TestE2E_AuditTrail_FullLineage
- ✅ TestE2E_Reconciliation_VerifySettlementIntegrity
- Plus 13 additional unit tests

## Expected output

```
=== RUN   TestE2E_OrderToSettlement_HappyPath
--- PASS: TestE2E_OrderToSettlement_HappyPath (0.50s)
...
PASS
ok  	github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/commerce	7.87s
```

## Related

- `/genie-build` — Build the API server
- `/genie-lint` — Run code linting
- `/genie-vet` — Run static analysis
