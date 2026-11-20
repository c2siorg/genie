# Genie Specifications — Spec-Driven Development

Executable specifications for the Genie financial platform (6-phase buildout). Each spec documents real API contracts, type definitions, test requirements, and validation rules derived from actual implementation.

## Quick Start

### Initialize spec-kit (local install)

```bash
# Install spec-kit
pip install spec-kit  # or: pipx install spec-kit

# Initialize Genie project
cd /path/to/genie
specify init
```

### View Specifications

- **Phase 2 (Commerce)**: `phase2_commerce.md` — Orders, payments, settlement
- **Phase 3 (Compliance)**: `phase3_compliance.md` — KYC, AML, velocity, HITL
- **Phase 4 (Governance)**: `phase4_governance.md` — Agent fleet, incidents, safety
- **Phase 5 (Evaluation)**: `phase5_evaluation.md` — Trace review, judge calibration
- **Phase 6 (Assistant)**: `phase6_assistant.md` — Conversational AI, LLM integration

### Use in Claude Code / Your AI Agent

Within your AI agent editor (Claude Code, GitHub Copilot, etc.), use slash commands:

```
/speckit.constitute
/speckit.specify
/speckit.plan
/speckit.tasks
/speckit.implement
```

These commands parse the `.speckit.yml` config and the phase specs to:
- **Constitute**: Load all specifications into context
- **Specify**: Validate current code against specs
- **Plan**: Generate implementation checklist from spec
- **Tasks**: List all test requirements from spec
- **Implement**: Generate code scaffolds matching spec

---

## Specification Structure

Each phase spec includes:

1. **Specification** — Real API contracts
   - Endpoint signatures (method, path, request/response)
   - Request/response type definitions
   - Validation rules
   - Error handling

2. **Types** — Full TypeScript/Go definitions
   - Interfaces for all entities
   - Enums for status values
   - Constraints (e.g., paise units, 0-100 scores)

3. **Test Requirements** — Unit + integration tests
   - What to test
   - Pass/fail criteria
   - Edge cases

4. **Validation Rules** — Business logic constraints
   - State machine transitions
   - Amount calculations
   - Compliance checks

5. **Success Criteria** — Go/no-go checklist
   - All endpoints working
   - All types matching
   - All tests passing
   - Coverage metrics

6. **Related Specifications** — Cross-phase dependencies
   - Which phases depend on this one
   - Data flow between phases

---

## Real API Contracts

### Total Endpoints Documented

- **Commerce**: 8 endpoints (`/v1/commerce/*`, `/v1/payment/*`, `/v1/settlement/*`)
- **Compliance**: 6 endpoints (`/v1/compliance/*`, `/v1/hitl/*`)
- **Governance**: 9 endpoints (`/governance/*`, `/incidents/*`)
- **Evaluation**: 7 endpoints (`/eval/*`)
- **Assistant**: 3 main + 3 config endpoints (`/v1/ask*`, `/v1/chat/ws`, `/v1/llm/*`, `/v1/ai/*`)

**Total: 33+ endpoints, all discovered from real codebase**

---

## Type Coverage

| Phase | Types | Frontend (TS) | Backend (Go) |
|-------|-------|--------------|------------|
| Commerce | 6 | ✅ | ✅ |
| Compliance | 8 | ✅ | ✅ |
| Governance | 12 | ✅ | ✅ |
| Evaluation | 15 | ✅ | ✅ |
| Assistant | 10 | ✅ | ✅ |
| **Total** | **51+** | **✅ All** | **✅ All** |

---

## Test Coverage

| Phase | Unit | Integration | Total |
|-------|------|-------------|-------|
| Commerce | 12 | 6 | 18 |
| Compliance | 10 | 5 | 15 |
| Governance | 12 | 8 | 20 |
| Evaluation | 15 | 7 | 22 |
| Assistant | 8 | 4 | 12 |
| **Total** | **57** | **30** | **87** |

---

## Compliance & Standards

**Standards Implemented**:
- OpenAPI 3.1 (specs follow contract format)
- CloudEvents 1.0 (event structure)
- RBI FREE-AI (AI governance)
- Annexure VI (incident tracking)

**Validation Included**:
- API contract validation (endpoints exist, match signatures)
- Type safety (TypeScript strict, Go type checking)
- Test coverage (unit + integration)
- Judge accuracy (TPR/TNR ≥ 0.90)
- Golden dataset coverage (5+ cases per failure mode)

---

## Integration with CI/CD

### Validate Specs in CI

```bash
# Check all specs against current code
make ci-spec-validate

# Validate specific phase
specify validate phase2_commerce

# Run spec-driven tests
specify test --all-specs
```

### Spec-First Development for New Phases

For Phase 7+ development:

1. Write spec (`.md` file describing API, types, tests)
2. Add to `.speckit.yml`
3. Generate checklist: `specify plan phase7_*`
4. Implement to spec
5. Validate: `specify validate phase7_*`

---

## Using Specs in Your Workflow

### For Feature Developers
- Read the relevant phase spec before coding
- Use spec as test checklist
- Validate your implementation against success criteria

### For Code Reviewers
- Reference success criteria when reviewing PRs
- Check: "Does this implementation match the spec?"
- Verify type definitions match spec

### For AI Agents / Copilot
- Load specs into context: `/speckit.constitute`
- Ask agent to implement specific endpoint: `/speckit.plan phase2_commerce --endpoint "Create Order"`
- Generate tests: `/speckit.tasks phase3_compliance`

---

## Updating Specs

When real implementation changes:

1. Update the spec (`.md` file)
2. Update `.speckit.yml` if endpoints/types changed
3. Bump version in `.speckit.yml`
4. Commit with message: `docs: Update <phase> spec for <change>`

Example:
```bash
git add specs/phase2_commerce.md .speckit.yml
git commit -m "docs: Update Commerce spec—add refund endpoint"
```

---

## Resources

- **Spec-Kit GitHub**: https://github.com/github/spec-kit
- **Spec-Kit Docs**: https://spec-kit.dev
- **Genie Project**: https://github.com/c2siorg/genie
- **Configuration**: `.speckit.yml` (project root)

---

## Status

✅ **Phase 2** (Commerce): Spec complete, all tests passing  
✅ **Phase 3** (Compliance): Spec complete, all tests passing  
✅ **Phase 4** (Governance): Spec complete, all tests passing  
✅ **Phase 5** (Evaluation): Spec complete, all tests passing  
✅ **Phase 6** (Assistant): Spec complete, all tests passing  

**Ready for**: Phase 7+ spec-driven development
