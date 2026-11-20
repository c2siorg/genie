# Claude Code Skills - Implementation Guide

Add these skills to `.claude/skills/` to enhance your workflow.

---

## 🧪 Testing & Quality Skills

### /genie-vet — Go Static Analysis
```markdown
# /genie-vet — Run Go Vet Analysis

Run built-in Go static analysis checks.

## Usage
/genie-vet [package]

## Examples
- /genie-vet — Run across all packages
- /genie-vet ./pkg/commerce — Analyze specific package
- /genie-vet ./... — Verbose analysis

## What it does
Executes: go vet ./...

Checks for:
- Suspicious constructs
- Type errors
- Unreachable code
- Unused variables
- Common mistakes

## Expected issues found
Typical findings help prevent bugs early.
```

### /genie-coverage — Test Coverage Report
```markdown
# /genie-coverage — Generate Test Coverage

Generate and view test coverage report.

## Usage
/genie-coverage [threshold]

## Examples
- /genie-coverage — Show coverage summary
- /genie-coverage 80 — Check 80% threshold
- /genie-coverage -html — Generate HTML report

## What it does
Executes: go test -cover ./...

Shows:
- Overall coverage percentage
- Per-package coverage
- Uncovered code areas
- Coverage by file

## Coverage targets
- Phase 2 commerce: >90% target
- Overall project: >80% target
```

### /genie-race — Race Condition Detector
```markdown
# /genie-race — Detect Race Conditions

Run tests with race detector enabled.

## Usage
/genie-race [package]

## Examples
- /genie-race — Test all with race detector
- /genie-race ./pkg/commerce — Test specific package
- /genie-race -short — Quick race check

## What it does
Executes: go test -race ./...

Detects:
- Concurrent access to shared memory
- Race conditions
- Data races
- Synchronization issues

## Expected output
Reports any race conditions found.
Good for concurrent order processing tests.
```

---

## 🔍 Code Analysis Skills

### /genie-godoc — Generate Documentation
```markdown
# /genie-godoc — Generate Go Documentation

Generate godoc documentation for the project.

## Usage
/genie-godoc [package]

## Examples
- /genie-godoc — Generate all docs
- /genie-godoc ./pkg/commerce — Package docs
- /genie-godoc -serve — Start local server

## What it does
Executes: go doc ./...

Generates:
- Function signatures
- Package documentation
- Type definitions
- Usage examples

## View locally
godoc -http=:6060
Then visit http://localhost:6060/pkg/github.com/PratikDhanave/...
```

### /genie-imports — Organize Imports
```markdown
# /genie-imports — Fix Import Organization

Organize and clean up import statements.

## Usage
/genie-imports [package]

## Examples
- /genie-imports ./... — Fix all imports
- /genie-imports ./pkg/commerce — Fix package imports

## What it does
Executes: goimports -w ./...

Does:
- Removes unused imports
- Adds missing imports
- Organizes import order
- Groups imports logically

## Installation
go install golang.org/x/tools/cmd/goimports@latest
```

### /genie-deadcode — Find Dead Code
```markdown
# /genie-deadcode — Identify Unused Code

Find and list dead code in the project.

## Usage
/genie-deadcode [package]

## Examples
- /genie-deadcode ./... — Find all dead code
- /genie-deadcode ./pkg/commerce — Check package

## What it does
Executes: deadcode ./...

Identifies:
- Unused functions
- Unused variables
- Unused types
- Dead imports

## Installation
go install github.com/dominikh/go-tools/cmd/deadcode@latest
```

---

## 🔨 Build & Deployment Skills

### /genie-install — Install Genie Locally
```markdown
# /genie-install — Install Genie Binary

Build and install Genie in your GOPATH.

## Usage
/genie-install [version]

## Examples
- /genie-install — Install latest
- /genie-install v0.1.0 — Install specific version

## What it does
Executes: go install ./cmd/api@latest

Installs:
- API binary to GOPATH/bin
- All dependencies
- Ready to run

## After install
genie-api  # Run the installed binary
```

### /genie-docker — Build Docker Image
```markdown
# /genie-docker — Build Docker Container

Build Genie Docker image.

## Usage
/genie-docker [tag]

## Examples
- /genie-docker — Build with default tag
- /genie-docker genie:latest — Custom tag
- /genie-docker -push — Build and push to registry

## What it does
Builds Docker image with:
- Go 1.25.0 runtime
- Genie API binary
- Minimal dependencies
- Production-ready

## Expected output
Successfully tagged: genie:latest
docker run -p 8080:8080 genie:latest
```

### /genie-release — Create Release Build
```markdown
# /genie-release — Create Release Build

Build optimized release binary.

## Usage
/genie-release [version]

## Examples
- /genie-release 0.2.0 — Build release 0.2.0
- /genie-release -sign — Sign binary

## What it does
Builds with optimizations:
- -ldflags for version info
- Stripped debug symbols
- Small binary size
- Performance optimized

## Output
bin/genie-api-0.2.0-linux-amd64
```

---

## 📊 Debugging & Monitoring Skills

### /genie-profile — CPU Profiling
```markdown
# /genie-profile — Profile CPU Usage

Generate CPU profile for performance analysis.

## Usage
/genie-profile [duration]

## Examples
- /genie-profile — 30s profile
- /genie-profile 60 — 60s profile
- /genie-profile -analyze — Analyze profile

## What it does
Executes: go test -cpuprofile=cpu.prof ./pkg/commerce

Generates:
- CPU profile data
- Hotspot analysis
- Function timing
- Performance bottlenecks

## Analyze
go tool pprof cpu.prof
> top    # Show top functions
> list   # Detailed view
```

### /genie-trace — Execution Trace
```markdown
# /genie-trace — Generate Execution Trace

Trace program execution for analysis.

## Usage
/genie-trace [package]

## Examples
- /genie-trace ./pkg/commerce — Trace tests
- /genie-trace -duration 5s — 5s trace

## What it does
Generates execution trace showing:
- Goroutine execution
- Network blocking
- Synchronization events
- Memory allocation

## Analyze
go tool trace trace.out
# Opens browser with visualization
```

---

## 🔄 Development Workflow Skills

### /genie-watch — Auto-Run Tests on Changes
```markdown
# /genie-watch — Watch Files & Auto-Run Tests

Automatically run tests when files change.

## Usage
/genie-watch [pattern]

## Examples
- /genie-watch — Watch all .go files
- /genie-watch pkg/commerce — Watch package
- /genie-watch -short — Run short tests

## What it does
Monitors for file changes:
- Detects .go file changes
- Auto-runs tests
- Reports results
- Continuous feedback

## Installation
go install github.com/cosmtrek/air@latest

Or use custom script:
while inotifywait -r pkg/; do go test ./...; done
```

### /genie-fmt — Format & Fix Code
```markdown
# /genie-fmt — Format & Auto-Fix Code

Format code and apply common fixes.

## Usage
/genie-fmt [package]

## Examples
- /genie-fmt ./... — Format all
- /genie-fmt ./pkg/commerce — Format package
- /genie-fmt -fix — Apply fixes

## What it does
Executes: gofmt -w ./...

Then:
- goimports (organize imports)
- golangci-lint --fix (apply fixes)
- go fmt (standard formatting)

## Result
Consistent, clean code
```

### /genie-diff — Show Changes
```markdown
# /genie-diff — Show Uncommitted Changes

View all uncommitted changes in code.

## Usage
/genie-diff [file-pattern]

## Examples
- /genie-diff — Show all changes
- /genie-diff pkg/commerce — Package changes
- /genie-diff --stat — Summary only

## What it does
Executes: git diff

Shows:
- Modified files
- Added lines
- Removed lines
- Diff stats

## Git workflow
Review before commit!
```

---

## 📚 Documentation Skills

### /genie-readme — View README
```markdown
# /genie-readme — View Project README

Display the main project README.

## Usage
/genie-readme

## What it shows
- Project overview
- 60+ specialist agents
- 8 financial domains
- Architecture overview
- Getting started
- Contributing guidelines

## Related
- /genie-docs — Documentation guide
- /genie-status — Project status
```

### /genie-api — API Documentation
```markdown
# /genie-api — View API Documentation

Display HTTP endpoint documentation.

## Usage
/genie-api [endpoint]

## Examples
- /genie-api — Show all endpoints
- /genie-api /v1/commerce — Specific endpoint
- /genie-api -search payment — Search endpoints

## Shows
- Method (GET, POST, etc)
- URL path
- Request body format
- Response format
- Error codes
- Examples
```

### /genie-architecture — View Architecture Diagram
```markdown
# /genie-architecture — Show Architecture Diagram

Display system architecture and components.

## Usage
/genie-architecture

## Shows
- Component relationships
- Data flow
- Module organization
- External systems
- Integration points

## Detail levels
- /genie-architecture -detail — Detailed view
- /genie-architecture -simple — Simplified view
- /genie-architecture -component OrderManagement — Component details
```

---

## 🚀 Phase 2 Specific Skills

### /genie-commerce-flow — Show Commerce Workflow
```markdown
# /genie-commerce-flow — View e-Rupee Commerce Workflow

Display the Phase 2 order-to-settlement workflow.

## Usage
/genie-commerce-flow

## Shows
- Order creation flow
- Payment processing
- Compliance checks
- Settlement execution
- Reconciliation
- State transitions
- Lineage capture points

## States
OrderCreated → PaymentInitiated → PaymentConfirmed →
SettlementInitiated → SettlementCompleted → Fulfilled
```

### /genie-settlement — Settlement Information
```markdown
# /genie-settlement — Show Settlement Process

Display settlement batching and netting details.

## Usage
/genie-settlement [merchant-id]

## Shows
- Settlement batching logic
- Netting calculations
- CBDC position conversion
- Reconciliation rules
- Example scenarios

## For specific merchant
/genie-settlement MERCHANT-001
# Shows recent settlements for merchant
```

### /genie-lineage — Show Lineage Tracking
```markdown
# /genie-lineage — View Audit Lineage

Display transaction lineage and audit trail.

## Usage
/genie-lineage [order-id]

## Shows
- Complete transaction history
- State transitions
- Timestamps
- Lineage events
- Hash-chain integrity
- Compliance markers

## Example
/genie-lineage ORDER-12345
# Shows full audit trail for order
```

---

## 🔐 Compliance & Governance Skills

### /genie-compliance — Check Compliance Status
```markdown
# /genie-compliance — Verify RBI FREE-AI Alignment

Check compliance with RBI FREE-AI framework.

## Usage
/genie-compliance [aspect]

## Examples
- /genie-compliance — Full compliance report
- /genie-compliance transparency — Check transparency
- /genie-compliance governance — Check governance

## Checks
- RBI FREE-AI alignment
- Governance compliance
- Security controls
- Audit trail completeness
- Regulatory requirements
```

### /genie-audit — Run Security Audit
```markdown
# /genie-audit — Run Security Audit

Scan for security issues and vulnerabilities.

## Usage
/genie-audit

## What it does
- gosec security scanner
- Dependency vulnerability check
- Secret detection
- Access control review
- Compliance verification

## Installation
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

---

## 📈 Metrics & Reporting Skills

### /genie-metrics — Show Project Metrics
```markdown
# /genie-metrics — Display Project Metrics

Show project statistics and metrics.

## Usage
/genie-metrics [type]

## Examples
- /genie-metrics — All metrics
- /genie-metrics code — Code metrics
- /genie-metrics test — Test metrics

## Shows
- Lines of code
- Test coverage
- Cyclomatic complexity
- Comment ratio
- Package breakdown
- Test counts
```

### /genie-report — Generate Report
```markdown
# /genie-report — Generate Project Report

Generate comprehensive project report.

## Usage
/genie-report [format]

## Examples
- /genie-report — Text report
- /genie-report html — HTML report
- /genie-report pdf — PDF report

## Includes
- Status overview
- Test results
- Coverage metrics
- Build status
- Documentation completeness
- Code quality scores
```

---

## How to Add a Skill

### Step 1: Create the Markdown File
```bash
# Create in .claude/skills/
touch .claude/skills/genie-SKILLNAME.md
```

### Step 2: Add the Skill Definition
Use the format above with:
- `/skill-name` in the title
- Usage section
- Examples section
- What it does
- Installation (if needed)
- Expected output
- Related skills

### Step 3: Test the Skill
```bash
# Type the skill in Claude Code
/genie-SKILLNAME

# Verify it works
```

### Step 4: Update Skills Guide
Add entry to `.claude/skills/SKILLS_GUIDE.md`

---

## Most Useful to Add First

**Quick wins** (high value, minimal setup):
1. `/genie-vet` — Catch issues early
2. `/genie-coverage` — Track test quality
3. `/genie-race` — Find concurrency bugs
4. `/genie-fmt` — Consistent code style
5. `/genie-imports` — Clean imports

**Development workflow** (enhance productivity):
1. `/genie-watch` — Auto-test on changes
2. `/genie-diff` — Review before commit
3. `/genie-metrics` — Track project health
4. `/genie-report` — Status snapshots

**Phase 2 specific** (commerce domain):
1. `/genie-commerce-flow` — Understand workflow
2. `/genie-settlement` — Settlement details
3. `/genie-lineage` — Audit trails
4. `/genie-compliance` — Governance check

**Performance & debugging** (when needed):
1. `/genie-profile` — Performance analysis
2. `/genie-trace` — Execution visualization
3. `/genie-deadcode` — Code cleanup

---

## Templates Available

All skills use the same template structure:
- **Markdown format** (.md files)
- **Simple layout** (title, usage, examples, details)
- **Consistent style** with other genie-* skills
- **Built on existing tools** (go, git, etc.)

Copy from existing skills and customize!

---

## Already Included

These are already configured:
- ✅ `/genie-test` — Commerce tests
- ✅ `/genie-build` — Build API
- ✅ `/genie-lint` — Linting
- ✅ `/genie-status` — Status
- ✅ `/genie-docs` — Documentation

Total available: **40+ potential skills**  
Currently active: **5 skills**  
Recommended to add: **5-10 more**
