# Spec-Driven Development Guide for Genie Phase 7+

This guide covers developing new phases using the spec-first approach. For Phases 2-6, see the existing specs. For Phase 7+, follow this workflow.

---

## Quick Start: Phase 7 (Advisor)

### 1. Copy Specification Template (Already Done ✅)

The Phase 7 spec (`phase7_advisor.md`) is ready. It includes:
- API endpoint signatures
- Type definitions
- Validation rules
- Test requirements
- Success criteria

### 2. Add Phase to Configuration

Update `.speckit.yml` (already done ✅):
```yaml
- name: advisor
  title: "Advisor Workspace"
  phase: 7
  status: planned
  file: specs/phase7_advisor.md
```

### 3. Generate Implementation Checklist

In your AI agent (Claude Code, GitHub Copilot):
```
/speckit.constitute
/speckit.plan phase7_advisor
```

This generates:
- [ ] Implement ProfileAnalyzer agent
- [ ] Implement FinancialAnalyst agent
- [ ] Implement RecommendationGenerator agent
- [ ] Add Action type + Action tests
- [ ] Add Risk type + Risk tests
- [ ] Implement POST /v1/advisor/recommendation
- [ ] Implement GET /v1/advisor/recommendation/{id}
- [ ] ... (16 more items)

### 4. Implement in Order

Start with **agents** (behavior), then **types** (safety), then **endpoints** (API):

**Step 1: Backend Agents**
```go
// pkg/advisor/profile_analyzer.go
type ProfileAnalyzer struct { ... }
func (p *ProfileAnalyzer) Analyze(ctx context.Context, user *User) (*UserProfile, error)

// pkg/advisor/analyst.go
type FinancialAnalyst struct { ... }
func (a *FinancialAnalyst) Analyze(profile *UserProfile, history *TransactionHistory) (*Opportunity, error)

// pkg/advisor/generator.go
type RecommendationGenerator struct { ... }
func (g *RecommendationGenerator) Generate(opp *Opportunity) (*Recommendation, error)
```

**Step 2: Frontend Types**
```typescript
// frontend/src/types/advisor.ts
interface Action { ... }
interface Risk { ... }
interface Recommendation { ... }

// frontend/src/hooks/useAdvisor.ts
export function useRecommendation(userId: string) { ... }
export function useRecommendationHistory(userId: string) { ... }
```

**Step 3: API Endpoints**
```go
// pkg/web/handlers/advisor.go
func (h *Handler) GenerateRecommendation(w http.ResponseWriter, r *http.Request)
func (h *Handler) GetRecommendation(w http.ResponseWriter, r *http.Request)
// ... 4 more endpoints
```

### 5. Test Against Spec

After each step, validate:
```bash
# Validate Phase 7 spec
./scripts/validate-specs.sh 7

# Run Phase 7 tests
go test ./pkg/advisor/... -v
npm run build  # Frontend type check

# Check against spec
/speckit.validate phase7_advisor
```

### 6. Success = Spec Passes

When all tests pass and validate against spec:
```
✅ Phase 7: Advisor — All 16 tests passing
✅ API contracts match spec (6 endpoints)
✅ Type definitions match spec (8 types)
✅ Success criteria met
```

---

## Writing Specs for Phase 8+

Use this template when starting a new phase:

### Template: Phase N Specification

```markdown
# Phase N: [Workspace] Specification

**Status**: 🔜 Planned  
**Planned APIs**: X endpoints  
**Planned Types**: Y type definitions  
**Planned Tests**: Z tests  

## Specification

### Overview
One-paragraph summary of what this phase does.

### API Endpoints

#### 1. Endpoint Name
\`\`\`
METHOD /v1/path
Request:
  - field1: type
  - field2?: optional
Response:
  - result: type
\`\`\`

### Types

#### TypeName
\`\`\`typescript
interface TypeName { ... }
\`\`\`

### Test Requirements

**Unit Tests** (N tests):
- [ ] Test case 1
- [ ] Test case 2

**Integration Tests** (M tests):
- [ ] Integration test 1

### Validation Rules

**Business Logic**:
- Rule 1
- Rule 2

**Compliance**:
- RBI mapping 1
- RBI mapping 2

### Success Criteria

✅ All X endpoints operational
✅ All Y types match definitions
✅ All N unit tests pass
✅ All M integration tests pass

### Architecture

Agent pipeline diagram / description

### Related Specifications

- **Phase N-1**: Dependency X
- **Phase N+1**: Dependent on this
```

### Key Elements

1. **API Contracts First** — Define endpoints before implementation
2. **Types Upfront** — Type definitions = spec validation tool
3. **Tests as Spec** — Test cases = success criteria
4. **Validation Rules** — Edge cases + constraints
5. **RBI Mapping** — Link to compliance framework

---

## Spec Validation in CI/CD

### Local Validation
```bash
# Validate current phase
./scripts/validate-specs.sh 7

# Strict mode (fail on warnings)
STRICT=true ./scripts/validate-specs.sh 7

# Validate all phases
./scripts/validate-specs.sh all
```

### CI/CD Integration
```yaml
# In your CI config (GitHub Actions, etc.)
- name: Validate Specs
  run: ./scripts/validate-specs.sh all --strict
```

### What Gets Validated

1. ✅ Spec file exists
2. ✅ Endpoints documented
3. ✅ Types defined in code
4. ✅ Tests passing
5. ✅ Cross-phase dependencies work
6. ✅ Lineage/audit integrated

---

## Common Patterns

### Pattern 1: Data-Driven Endpoint

**Example**: Commerce order creation

```markdown
### Endpoint
POST /v1/commerce/order

### Type
\`\`\`typescript
interface Order {
  order_id: string;
  items: OrderItem[];
  total_paise: number;  // CALCULATED from items
}
\`\`\`

### Validation Rule
- total_paise = SUM(item.quantity × item.unit_price_paise)
- Must match exactly (±0 tolerance)

### Test
- Create order with 2 items → total correct
- Create order with 0 items → reject
```

### Pattern 2: Async Decision Endpoint

**Example**: Compliance check

```markdown
### Endpoints
POST /v1/compliance/check → returns check_id
GET /v1/compliance/check/{check_id} → returns decision

### Type
\`\`\`typescript
interface ComplianceCheck {
  decision: "ALLOW" | "REVIEW" | "BLOCK";
  aml_result: "pass" | "review" | "block";
}
\`\`\`

### Validation Rule
- Decision depends on multiple signals (AML + velocity + fraud)
- BLOCK takes precedence (if any signal blocks, decision = BLOCK)

### Test
- OFAC match → decision = BLOCK
- AML flag + low velocity → decision = REVIEW
- Clear screening + normal velocity → decision = ALLOW
```

### Pattern 3: Streaming Endpoint

**Example**: Assistant ask/stream

```markdown
### Endpoint
POST /v1/ask/stream → Server-Sent Events

### Events (in order)
1. ai_disclosure (required first)
2. trace (trace ID)
3. agent.handle* (progress updates, repeating)
4. report (final result, required last)

### Validation Rule
- Events must be ordered
- ai_disclosure always first
- report always last
- Client can disconnect after report

### Test
- Verify event order
- Verify event data schema
- Test client disconnect mid-stream
```

---

## Spec Evolution

As implementation reveals edge cases:

1. **Document the edge case** in spec
2. **Add test case** to spec
3. **Update validation rule** in spec
4. **Re-validate** against spec
5. **Commit** spec update with code change

Example commit:
```
feat: Add advisor recommendations endpoint

Update Phase 7 spec:
- Add edge case: User profile missing → use defaults
- Add test: No profile, request succeeds with generic recommendation
- Update validation rule: Profile optional, fallback to conservative profile

Co-Authored-By: Claude ...
```

---

## Spec Ownership

**Specification Owner**: Person/team responsible for phase implementation
- Keeps spec updated as implementation evolves
- Runs `./scripts/validate-specs.sh` before every PR
- Documents all edge cases discovered
- Responds to cross-phase dependency requests

**Spec Reviewer**: Before merge
- Checks: Spec matches implementation
- Checks: All success criteria met
- Checks: Tests passing
- Approves merge

---

## Resources

- **Spec-Kit**: https://github.com/github/spec-kit
- **Genie Specs**: `/specs` directory
- **Configuration**: `.speckit.yml`
- **Validation**: `./scripts/validate-specs.sh`

---

## Status: Phase 7 Ready 🚀

Phase 7 (Advisor) spec is complete and ready for implementation. Use this guide to start spec-first development.

Next: Pick a team member, start implementation checklist, validate against spec.
