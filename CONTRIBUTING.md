# Contributing to Genie

Thank you for your interest in contributing to Genie! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

1. [Code of Conduct](#code-of-conduct)
2. [Getting Started](#getting-started)
3. [Development Workflow](#development-workflow)
4. [Coding Standards](#coding-standards)
5. [Testing Requirements](#testing-requirements)
6. [Commit Guidelines](#commit-guidelines)
7. [Pull Request Process](#pull-request-process)
8. [Reporting Issues](#reporting-issues)

---

## Code of Conduct

### Our Pledge

We are committed to providing a welcoming and inclusive environment for all contributors. We pledge to:

- Treat all people with respect and dignity
- Welcome contributions from people of all backgrounds and experiences
- Foster a harassment-free community
- Address concerns promptly and fairly

### Expected Behavior

- Use welcoming and inclusive language
- Be respectful of differing opinions and experiences
- Focus on what is best for the community
- Accept constructive criticism gracefully
- Show empathy towards other community members

### Unacceptable Behavior

- Harassment, intimidation, or discrimination
- Hate speech or attacks on personal characteristics
- Unwelcome sexual attention
- Deliberate disruption of discussions
- Publishing others' private information

**Report violations** to: governance@c2si.org

---

## Getting Started

### Prerequisites

- **Go**: Version 1.25.0 or higher
- **Git**: For version control
- **Docker**: For containerized development (optional but recommended)
- **PostgreSQL**: Version 12+ (included in docker-compose)
- **Ollama**: For local LLM testing (optional)

### Setting Up Your Development Environment

1. **Fork the repository** on GitHub
   ```bash
   # Navigate to https://github.com/c2siorg/genie and click "Fork"
   ```

2. **Clone your fork locally**
   ```bash
   git clone https://github.com/YOUR_USERNAME/genie.git
   cd genie
   ```

3. **Add upstream remote**
   ```bash
   git remote add upstream https://github.com/c2siorg/genie.git
   ```

4. **Install dependencies**
   ```bash
   go mod download
   go mod tidy
   ```

5. **Set up environment**
   ```bash
   cp .env.example .env
   # Edit .env with your local settings (use defaults for local development)
   ```

6. **Start development environment**
   ```bash
   docker-compose up -d
   # Wait for services to be ready (30-60 seconds)
   ```

7. **Run tests to verify setup**
   ```bash
   go test ./...
   ```

---

## Development Workflow

### Branch Naming

Follow these conventions for branch names:

- **Features**: `feature/description` (e.g., `feature/cbdc-ledger-integration`)
- **Bugfixes**: `fix/description` (e.g., `fix/payment-reconciliation-race-condition`)
- **Docs**: `docs/description` (e.g., `docs/api-endpoint-reference`)
- **Refactoring**: `refactor/description` (e.g., `refactor/settlement-agent-structure`)
- **Tests**: `test/description` (e.g., `test/e2e-compliance-flows`)
- **Chores**: `chore/description` (e.g., `chore/update-dependencies`)

### Creating a Branch

```bash
# Update main branch
git fetch upstream
git checkout main
git rebase upstream/main

# Create new branch
git checkout -b feature/your-feature-name
```

### Local Development

```bash
# Build the project
go build ./cmd/api

# Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
air

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestName ./pkg/module
```

### Makefile Commands

```bash
make help          # Show all available commands
make build         # Build the binary
make test          # Run all tests
make test-coverage # Generate coverage report
make lint          # Run linter
make fmt           # Format code
make up            # Start Docker Compose stack
make down          # Stop Docker Compose stack
make logs          # View logs
make clean         # Clean build artifacts
```

---

## Coding Standards

### Go Code Style

We follow the official [Go Code Review Comments](https://golang.org/doc/effective_go) and [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).

#### Key Standards

1. **Naming**
   - Packages: short, lowercase, no underscores
   - Functions: `PascalCase` for exported, `camelCase` for private
   - Constants: `UPPER_SNAKE_CASE` for config, `camelCase` for enums
   - Interfaces: End with `-er` (e.g., `Reader`, `Writer`, `Executor`)

2. **File Organization**
   ```
   package_name/
   ├── types.go           # Type definitions
   ├── interface.go       # Interface definitions
   ├── handler.go         # Main implementation
   ├── handler_test.go    # Unit tests
   ├── integration_test.go # Integration tests
   └── README.md          # Package documentation
   ```

3. **Error Handling**
   ```go
   // ✅ Good: Wrap errors with context
   if err != nil {
       return fmt.Errorf("failed to initiate payment: %w", err)
   }

   // ❌ Bad: Ignore errors
   _ = risky.Operation()

   // ❌ Bad: Generic errors
   return errors.New("error")
   ```

4. **Comments**
   - Package-level: Describe package purpose and licensing
   - Function-level: Document public functions
   - Inline: Explain "why", not "what" (code shows what)

   ```go
   // Package payment handles e-Rupee payment processing
   // and CBDC ledger interactions.
   package payment

   // InitiatePayment starts a new payment transaction.
   // Returns payment_id on success or an error if insufficient funds.
   func InitiatePayment(ctx context.Context, req PaymentRequest) (string, error) {
       // Check velocity limits first, before expensive ledger lookup
       if err := checkVelocity(ctx, req.FromAccount); err != nil {
           return "", err
       }
       // ... rest of implementation
   }
   ```

5. **Code Formatting**
   ```bash
   # Automatic formatting (required before commit)
   go fmt ./...

   # Additional linting
   golangci-lint run ./...
   ```

### Architecture Patterns

See [CLAUDE.md](./CLAUDE.md) for detailed architecture patterns used in this project.

---

## Testing Requirements

### Coverage Targets

- **Overall**: ≥ 80% code coverage
- **Critical paths**: ≥ 90% coverage (payment, compliance, settlement)
- **Public APIs**: ≥ 85% coverage

### Test Types

#### Unit Tests
- Test single functions in isolation
- Mock external dependencies
- File: `*_test.go` in same package
- Run: `go test ./pkg/module -v`

#### Integration Tests
- Test module interactions
- Use real databases (via docker-compose)
- File: `integration_test.go` or `*_integration_test.go`
- Run: `go test -tags=integration ./pkg/module -v`

#### E2E Tests
- Test complete workflows (order → payment → settlement)
- File: `pkg/commerce/integration_test.go`
- Run: `go test ./pkg/commerce -v`

### Writing Tests

```go
func TestPaymentInitiation_HappyPath(t *testing.T) {
    // Arrange: Set up test data
    merchant := createTestMerchant(t)
    paymentReq := PaymentRequest{
        FromAccount: merchant.SettlementAccount,
        ToAccount:   "customer_001",
        AmountPaise: 100000, // ₹1000
    }
    
    // Act: Call the function
    paymentID, err := InitiatePayment(context.Background(), paymentReq)
    
    // Assert: Verify results
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if paymentID == "" {
        t.Error("expected non-empty payment_id")
    }
}

func TestPaymentInitiation_InsufficientFunds(t *testing.T) {
    // Arrange
    merchant := createTestMerchantWithBalance(t, 50000) // ₹500
    paymentReq := PaymentRequest{
        FromAccount: merchant.SettlementAccount,
        ToAccount:   "customer_001",
        AmountPaise: 100000, // ₹1000 (exceeds balance)
    }
    
    // Act
    _, err := InitiatePayment(context.Background(), paymentReq)
    
    // Assert
    if err == nil {
        t.Error("expected error for insufficient funds")
    }
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run with coverage
go test -cover ./... | grep total

# Generate HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run only integration tests
go test -tags=integration ./...

# Run tests matching a pattern
go test -run TestPayment ./pkg/payment
```

---

## Commit Guidelines

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

#### Type
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code refactoring (no behavior change)
- `test`: Adding or updating tests
- `docs`: Documentation changes
- `chore`: Maintenance, dependencies, build
- `perf`: Performance improvements

#### Scope
Affected package/component (e.g., `payment`, `compliance`, `commerce`)

#### Subject
- Use imperative mood ("add" not "added" or "adds")
- Don't capitalize first letter
- No period at end
- Limit to 50 characters

#### Body
- Explain WHAT and WHY, not HOW
- Wrap at 72 characters
- Separate from subject with blank line

#### Footer
- Reference issues: `Fixes #123`, `Closes #456`
- Breaking changes: `BREAKING CHANGE: description`

### Example Commits

```
feat(payment): add velocity limit checking

Implement AML velocity limit validation before payment initiation.
Check customer's transaction history over last 24h and compare
against configurable limits per customer risk tier.

Fixes #234
```

```
fix(settlement): prevent double-settlement of orders

Orders in pending state were being included in multiple settlement
batches due to missing status update in settlement flow. Now mark
orders as settled before batch processing to avoid duplication.

Fixes #567
```

### Commit Checklist

Before committing:

- [ ] Code is formatted (`go fmt ./...`)
- [ ] Tests pass locally (`go test ./...`)
- [ ] Code builds successfully (`go build ./cmd/api`)
- [ ] No new linting errors (`golangci-lint run ./...`)
- [ ] Changes are logically grouped (one feature per commit)
- [ ] Commit message follows guidelines above

---

## Pull Request Process

### Before Creating a PR

1. **Sync with upstream**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run full test suite**
   ```bash
   go test ./...
   go test -cover ./...
   ```

3. **Check code quality**
   ```bash
   go fmt ./...
   golangci-lint run ./...
   ```

4. **Run the application**
   ```bash
   # Ensure it builds and basic functionality works
   docker-compose up -d
   go run ./cmd/api
   ```

### Creating the PR

1. **Push your branch**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Open PR on GitHub**
   - Compare your branch against `upstream/main`
   - Use the PR template provided
   - Link related issues
   - Describe what changed and why

3. **PR Description Template**
   ```markdown
   ## Summary
   [Brief description of changes]

   ## Type of Change
   - [ ] Bug fix
   - [ ] New feature
   - [ ] Breaking change
   - [ ] Documentation update

   ## Testing
   [Describe how you tested these changes]

   ## Checklist
   - [ ] Code follows style guidelines
   - [ ] Tests pass locally
   - [ ] New tests added for new functionality
   - [ ] Documentation updated
   - [ ] No new warnings generated

   ## Related Issues
   Closes #123
   ```

### PR Review Process

Maintainers will:
1. Assign reviewers
2. Run automated checks (CI/CD)
3. Review code for quality and style
4. Request changes if needed
5. Approve and merge when ready

**Expected review time**: 2-5 business days (depending on complexity)

### Address Feedback

1. **Read comments carefully**
2. **Make requested changes**
3. **Push to the same branch** (don't force push)
4. **Respond to each comment** (mark as resolved when done)
5. **Re-request review** after changes

---

## Reporting Issues

### Before Creating an Issue

1. **Check existing issues** to avoid duplicates
2. **Search closed issues** (might be documented)
3. **Check documentation** (CLAUDE.md, README.md)

### Issue Types

- **Bug Report**: Unexpected behavior
- **Feature Request**: New functionality
- **Documentation**: Missing or unclear docs
- **Question**: Need help or clarification

### Bug Report Template

```markdown
## Description
Brief description of the bug

## Steps to Reproduce
1. Step 1
2. Step 2
3. Step 3

## Expected Behavior
What should happen

## Actual Behavior
What actually happens

## Environment
- Go Version: 1.25.0
- OS: macOS/Linux/Windows
- Docker Version: (if applicable)

## Logs
```
Relevant error messages or logs
```

## Additional Context
Screenshots, configuration, etc.
```

### Feature Request Template

```markdown
## Description
What would you like to add?

## Motivation
Why do you need this feature?

## Proposed Solution
How should it work?

## Alternatives Considered
Other approaches you've thought of

## Additional Context
Related issues, discussions, examples
```

---

## Licensing

By contributing to Genie, you agree that your contributions will be licensed under the MIT License. See [LICENSE](./LICENSE) for details.

---

## Questions?

- **Documentation**: See [README.md](./README.md) and [CLAUDE.md](./CLAUDE.md)
- **Issues**: Check [GitHub Issues](https://github.com/c2siorg/genie/issues)
- **Discussions**: Use [GitHub Discussions](https://github.com/c2siorg/genie/discussions)
- **Email**: governance@c2si.org

---

## Recognition

Contributors will be recognized in:
- [CONTRIBUTORS.md](./CONTRIBUTORS.md)
- Release notes
- GitHub contributors page

Thank you for helping make Genie better! 🚀

---

**Last Updated**: June 1, 2026  
**Version**: 1.0
