# /genie-docs — Project Documentation Guide

Quick reference for all Genie project documentation.

## Usage

```
/genie-docs [topic]
```

## Topics

### Project Information
- **CLAUDE.md** — Development guidelines & workflow
- **PROJECT_REFERENCE.md** — Project identity & lineage
- **CONTRIBUTORS.md** — Attribution & credits
- **ARDAN_LABS_AUDIT.md** — Code audit report (zero external code)
- **README.md** — Project overview & features

### Architecture & Design
- **docs/architecture.md** — System design & components
- **docs/ai-governance-security.md** — Security & threat model
- **docs/free-ai-mapping.md** — RBI compliance mapping
- **docs/api.md** — HTTP endpoint documentation

### Claude Code Setup
- **.claude/settings.json** — Permissions & environment
- **.claude/launch.json** — Dev server configurations
- **.claude/memory/MEMORY.md** — Project memory & context

## Quick Lookups

### I want to...

**Understand the project architecture**
→ Start with: docs/architecture.md

**Set up development**
→ Start with: CLAUDE.md (sections: Development Workflow, Code Organization)

**Check what Phase 2 includes**
→ Start with: PROJECT_REFERENCE.md (section: Phase 2 Implementation)

**Verify code attribution**
→ Start with: ARDAN_LABS_AUDIT.md

**Learn testing patterns**
→ Start with: CLAUDE.md (section: Testing Requirements)

**Understand compliance requirements**
→ Start with: docs/free-ai-mapping.md

**Review API endpoints**
→ Start with: docs/api.md

**Check code patterns**
→ Start with: CLAUDE.md (section: Code Patterns & Conventions)

## File Locations

```
Project Root:
  ├── CLAUDE.md                    # Development guidelines
  ├── PROJECT_REFERENCE.md         # Project lineage
  ├── CONTRIBUTORS.md              # Attribution
  ├── ARDAN_LABS_AUDIT.md          # Code audit
  ├── README.md                    # Overview
  ├── docs/
  │   ├── architecture.md          # System design
  │   ├── ai-governance-security.md # Security
  │   ├── free-ai-mapping.md       # Compliance
  │   └── api.md                   # API reference
  └── .claude/
      ├── settings.json            # Permissions
      ├── launch.json              # Dev servers
      └── memory/
          └── MEMORY.md            # Project memory
```

## Content Overview

### CLAUDE.md (Development Guidelines)
- Project overview & Phase 2 status
- Code organization & structure
- Development workflow (build, test, git)
- Code patterns & conventions
- Testing requirements
- Code attribution rules
- Compliance & governance
- Troubleshooting guide

### PROJECT_REFERENCE.md (Project Context)
- Project identity & repository info
- Foundation (MARA, RBI FREE-AI)
- Project scope (60+ agents, 8 domains)
- Technology stack
- Phase 2 implementation details
- Code lineage & attribution
- Governance & safety controls
- Build & test status

### CONTRIBUTORS.md (Attribution)
- Phase 2 implementation credits
- External dependencies with licenses
- Architecture patterns used
- Original implementation note

### ARDAN_LABS_AUDIT.md (Code Audit)
- Audit methodology (6 dimensions)
- Complete scan results
- Zero Ardan Labs code verification
- Project ownership confirmation

### .claude/memory/MEMORY.md (Project Memory)
- Current status & test results
- Architecture & technical decisions
- Code patterns & naming conventions
- Common issues & fixes
- Development workflow tips
- Standards & compliance info

## Workflow Integration

### For new tasks:
1. Check **CLAUDE.md** for code patterns & conventions
2. Reference **docs/architecture.md** for system design
3. Use **PROJECT_REFERENCE.md** for context

### For problem-solving:
1. Check **CLAUDE.md** troubleshooting section
2. Review **.claude/memory/MEMORY.md** for known issues
3. Look at **docs/ai-governance-security.md** for security context

### For compliance:
1. Review **docs/free-ai-mapping.md** for RBI requirements
2. Check **ARDAN_LABS_AUDIT.md** for code verification
3. See **CLAUDE.md** (Code Attribution section) for rules

## Related Skills

- `/genie-status` — Current project status
- `/genie-test` — Run tests
- `/genie-build` — Build project
- `/genie-lint` — Run linting
