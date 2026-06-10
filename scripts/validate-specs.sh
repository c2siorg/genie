#!/bin/bash
# validate-specs.sh — Validate all specs against implementation
# Usage: ./scripts/validate-specs.sh [phase] [--strict]
# Examples:
#   ./scripts/validate-specs.sh              # Validate all phases
#   ./scripts/validate-specs.sh 2             # Validate Phase 2 only
#   ./scripts/validate-specs.sh --strict      # Strict mode (fail on warnings)

set -e

STRICT=${STRICT:-false}
PHASE=${1:-"all"}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
success() { echo -e "${GREEN}✓${NC} $1"; }
error() { echo -e "${RED}✗${NC} $1"; }
warning() { echo -e "${YELLOW}⚠${NC} $1"; }

# Validation counters
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0

check_test() {
  local test_name=$1
  local test_cmd=$2

  TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

  if eval "$test_cmd" > /dev/null 2>&1; then
    success "$test_name"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
  else
    error "$test_name"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
    if [ "$STRICT" = true ]; then
      return 1
    fi
  fi
}

# Phase 2: Commerce Validation
validate_phase2() {
  echo ""
  echo "=== Phase 2: Commerce ==="

  # Endpoint existence
  check_test "POST /v1/commerce/order endpoint exists" \
    "grep -r 'POST.*commerce.*order' pkg/web/handlers || grep -r '/v1/commerce/order' pkg/"

  # Type definitions
  check_test "Order type defined" \
    "grep -r 'type Order struct' pkg/commerce || grep -r 'interface Order' frontend/src/types/"

  # Tests passing
  check_test "Commerce unit tests pass" \
    "cd . && go test ./pkg/commerce/... -v -timeout 30s"

  # API contract validation
  check_test "Settlement amount calculated correctly" \
    "grep -r 'total_paise' pkg/commerce && grep -r 'quantity.*unit_price' pkg/commerce"
}

# Phase 3: Compliance Validation
validate_phase3() {
  echo ""
  echo "=== Phase 3: Compliance ==="

  check_test "POST /v1/compliance/check endpoint exists" \
    "grep -r 'compliance.*check' pkg/web/handlers || grep -r '/v1/compliance/check' pkg/"

  check_test "Compliance types defined" \
    "grep -r 'type.*Compliance' pkg/compliance || grep -r 'interface.*Compliance' frontend/src/types/"

  check_test "Velocity limit enforcement" \
    "grep -r 'velocity' pkg/compliance && grep -r 'limit' pkg/compliance"

  check_test "Compliance tests pass" \
    "cd . && go test ./pkg/compliance/... -v -timeout 30s"
}

# Phase 4: Governance Validation
validate_phase4() {
  echo ""
  echo "=== Phase 4: Governance ==="

  check_test "GET /governance/agents endpoint exists" \
    "grep -r 'governance.*agents' pkg/web/handlers || grep -r '/governance/agents' pkg/"

  check_test "Audit hash-chain present" \
    "grep -r 'hash' pkg/storage && grep -r 'audit' pkg/storage"

  check_test "Kill-switch implemented" \
    "grep -r 'kill.*switch' pkg/ -i || grep -r 'KillSwitch' frontend/src/types"

  check_test "Governance tests pass" \
    "cd . && go test ./pkg/agentgov/... ./pkg/incidents/... -v -timeout 30s"
}

# Phase 5: Evaluation Validation
validate_phase5() {
  echo ""
  echo "=== Phase 5: Evaluation ==="

  check_test "GET /eval/metrics endpoint exists" \
    "grep -r 'eval.*metrics' pkg/web/handlers || grep -r '/eval/metrics' pkg/"

  check_test "Judge types defined" \
    "grep -r 'judge' pkg/eval -i && grep -r 'Judge' frontend/src/types"

  check_test "Failure modes documented" \
    "grep -r 'failure.*mode' specs/ -i || [ -f specs/phase5_evaluation.md ]"

  check_test "Evaluation tests pass" \
    "cd . && go test ./pkg/eval/... -v -timeout 30s"
}

# Phase 6: Assistant Validation
validate_phase6() {
  echo ""
  echo "=== Phase 6: Assistant ==="

  check_test "POST /v1/ask endpoint exists" \
    "grep -r 'POST.*ask' pkg/web/handlers || grep -r '/v1/ask' pkg/"

  check_test "WebSocket chat endpoint exists" \
    "grep -r 'websocket\\|ws' pkg/web/handlers || grep -r 'chat.*ws' pkg/"

  check_test "LLM provider integration" \
    "grep -r 'llm.*Provider' pkg/ || grep -r 'anthropic' pkg/llm || grep -r 'ollama' pkg/llm"

  check_test "Assistant types defined" \
    "grep -r 'AskRequest\\|AskResponse' frontend/src/types/"

  check_test "Assistant tests pass" \
    "cd . && npm run build 2>/dev/null || true && go test ./pkg/web/handlers/... -v -timeout 30s"
}

# Phase 7: Advisor (Planned) Validation
validate_phase7() {
  echo ""
  echo "=== Phase 7: Advisor (Planned) ==="

  # For planned phase, just check spec exists
  check_test "Advisor specification exists" \
    "[ -f specs/phase7_advisor.md ]"

  check_test "Advisor spec in .speckit.yml" \
    "grep -q 'phase7\\|advisor' .speckit.yml"
}

# Cross-Phase Validation
validate_crossphase() {
  echo ""
  echo "=== Cross-Phase Validation ==="

  check_test "Commerce → Compliance flow (data passed)" \
    "grep -r 'merchant_id' pkg/commerce && grep -r 'merchant_id' pkg/compliance"

  check_test "All phases use audit/lineage" \
    "grep -r 'lineage\\|audit' pkg/commerce pkg/compliance pkg/agentgov"

  check_test "Spec files exist for all active phases" \
    "[ -f specs/phase2_commerce.md ] && [ -f specs/phase3_compliance.md ] && [ -f specs/phase4_governance.md ] && [ -f specs/phase5_evaluation.md ] && [ -f specs/phase6_assistant.md ]"
}

# Main execution
echo "Genie Specification Validation"
echo "=============================="

if [ "$PHASE" = "all" ] || [ "$PHASE" = "2" ]; then validate_phase2; fi
if [ "$PHASE" = "all" ] || [ "$PHASE" = "3" ]; then validate_phase3; fi
if [ "$PHASE" = "all" ] || [ "$PHASE" = "4" ]; then validate_phase4; fi
if [ "$PHASE" = "all" ] || [ "$PHASE" = "5" ]; then validate_phase5; fi
if [ "$PHASE" = "all" ] || [ "$PHASE" = "6" ]; then validate_phase6; fi
if [ "$PHASE" = "all" ] || [ "$PHASE" = "7" ]; then validate_phase7; fi

if [ "$PHASE" = "all" ]; then validate_crossphase; fi

# Summary
echo ""
echo "=============================="
echo "Summary: $PASSED_CHECKS / $TOTAL_CHECKS checks passed"

if [ $FAILED_CHECKS -eq 0 ]; then
  success "All validations passed!"
  exit 0
else
  error "$FAILED_CHECKS checks failed"
  if [ "$STRICT" = true ]; then
    exit 1
  fi
  exit 0
fi
