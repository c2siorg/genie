# Genie — Honest Test Coverage Map

**Generated**: 2026-06-06 (Phase 0 baseline)
**Command**: `go test -coverprofile=cover.out ./... && go tool cover -func=cover.out`

> This file is the **honest** coverage baseline. Earlier in the project, documents claimed "95%+ / 100% coverage / 2,060 tests." That was not true — much of it was non-compiling test theater (tests that asserted on data they had just created, and files that did not compile). Those files have been removed or fixed. The numbers below are the real, measured baseline after the build was made green and the theater purged.

## Total

| Metric | Value |
|--------|-------|
| **Total statement coverage** | **60.0%** |
| Packages passing (`go test ./...`) | 134 |
| Build (`go build ./...`) | ✅ green |
| Test suite (`go test ./...`) | ✅ green (0 failures) |

## Distribution (158 measured packages)

| Band | Count |
|------|-------|
| ≥ 80% | 42 |
| 50–79% | 72 |
| 1–49% | 16 |
| 0% (no/!minimal tests — mostly `cmd/`, `examples/`, scaffolds) | 28 |

## Critical / core packages (the ones that matter most)

| Package | Coverage | Notes |
|---------|----------|-------|
| `pkg/storage/postgres` | **0.9%** | 🔴 critical gap — DB layer barely tested |
| `agents/settlement` | 43.3% | scaffold-heavy |
| `pkg/web/handlers` | 47.2% | large surface; raise with handler tests per slice |
| `pkg/lineage` | 51.2% | audit/provenance — should be higher |
| `pkg/web/mid` | 70.4% | CSRF/cookies sub-pkgs are ~100% |
| `pkg/auth` | 72.6% | sub-pkgs (oauth/webauthn/elevation) 73–88% |
| `pkg/commerce` | 78.4% | settlement workflow |
| `pkg/merchant` | 85.9% | |
| `pkg/commercesettlement` | 86.5% | |
| `pkg/erupeepayment` | 87.2% | |
| `pkg/cbdc` | 94.6% | ledger — strong |
| `pkg/compliance` | 98.1% | strong |

## Targets (criticality-tiered — see `plan.md` §6.1)

We deliberately do **not** chase "100% line coverage" — that incentivizes test theater. Targets by tier:

| Tier | Code | Target |
|------|------|--------|
| Critical | money math, settlement, compliance decisions, CBDC ledger, auth/CSRF | 90–95% with real branch/error/boundary assertions |
| Core | handlers, agents, lineage, payment flow | 80–90% |
| Glue | wiring, config, DTOs, `cmd/` | best-effort smoke; no theater to inflate |

**Highest-priority gaps to close (per-slice, in plan phases):**
1. `pkg/storage/postgres` 0.9% → 70%+ (Phase 2 — Commerce slice).
2. `pkg/web/handlers` 47% → 80%+ (per-slice handler tests, Phases 2–6).
3. `pkg/lineage` 51% → 85%+ (provenance is FREE-AI-critical).

## How to regenerate

```bash
go test -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1        # total
go tool cover -html=cover.out -o cover.html    # per-line HTML
```

## Phase 0 cleanup performed to reach this honest baseline

- Fixed the broken build (`agents/kyc` — defined 4 missing sub-agent interfaces; corrected a package-name-collision return type).
- Added missing agent-`ID` constants (`merchant`, `settlement`, `kyc`) + a real `agents/kyc` test (registry conventions now pass).
- Removed pure-theater test files (0 real package calls, asserting on self-created maps): `erupeepayment/payment_flow_test.go`, `compliance/compliance_test.go`, `eval/failure_mode_coverage_test.go`, and 2 broken integration files.
- Fixed a real latent bug: `isValidGST`/`isValidPAN` upper-cased input, silently accepting malformed lowercase GSTIN/PAN.
- Updated stale UI-contract tests to assert the **current secure** behavior (HttpOnly cookie + CSRF) instead of the retired insecure localStorage-token model — without reintroducing the security regression.
- Relaxed an environment-dependent perf assertion to a generous hang-catching bound (precise SLOs belong in benchmarks).
- Quarantined `tests/integration/payment_workflow_test.go` (written against a fictional `erupeepayment.PaymentRequest`) for a proper rewrite in the Commerce slice (Phase 2).
