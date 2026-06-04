# Changelog

All notable changes to Genie are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Planned for Phase 2
- CSRF protection with double-submit cookie pattern
- HttpOnly cookie-based session management
- Content Security Policy headers
- Enhanced error handling and information disclosure prevention

---

## [1.0.0] - 2026-06-01

### Added
- **Core Payment Processing**
  - e-Rupee payment initiation and confirmation
  - Payment status tracking and history
  - Multiple payment account support
  - Transaction reconciliation with CBDC ledger

- **Commerce Order Management**
  - Create, retrieve, and execute orders
  - Item-level tracking with pricing
  - Order status workflow (pending → paid → fulfilled)
  - Order cancellation and refund support

- **Compliance Engine**
  - KYC (Know Your Customer) verification workflow
  - AML (Anti-Money Laundering) transaction monitoring
  - Velocity limit checking (24h and 7d windows)
  - Sanctions list and PEP screening ready (data source TBD)
  - Configurable compliance rules per risk tier

- **CBDC Ledger Integration**
  - Immutable transaction recording via hash-chain
  - Double-spend prevention with ledger commitment
  - Block-level verification and reconciliation
  - Multi-account support for merchants and customers

- **Settlement & Batching**
  - Daily settlement batch processing
  - Merchant-level order consolidation
  - Bilateral/multilateral netting calculation
  - Settlement status tracking and reporting
  - Reconciliation with ledger and orders

- **Audit Trail & Lineage**
  - Complete workflow event tracking
  - Hash-chain integrity verification
  - Query API for audit trail review
  - Lineage export for regulatory compliance
  - Event-level data capture (payment details, compliance decisions, ledger commits)

- **Merchant Onboarding**
  - Merchant registration and profile management
  - Account setup with CBDC ledger accounts
  - Settlement account configuration
  - Onboarding approval workflow

- **User Management**
  - User registration and login
  - JWT-based authentication
  - Password hashing with bcrypt
  - User profile management
  - Role-based access control (basic)

- **API Endpoints**
  - `POST /v1/users` — Create user account
  - `POST /v1/users/login` — Authenticate and get JWT
  - `GET /v1/users/me` — Get current user profile
  - `POST /v1/payment/initiate` — Start payment
  - `GET /v1/payment/{payment_id}` — Check payment status
  - `POST /v1/commerce/order` — Create order
  - `POST /v1/commerce/order/{order_id}/execute` — Execute order workflow
  - `GET /v1/commerce/order/{order_id}` — Retrieve order
  - `POST /v1/merchant/onboard` — Onboard merchant
  - `GET /v1/merchant/{merchant_id}` — Get merchant info
  - `POST /v1/compliance/check` — Run compliance checks
  - `POST /v1/cbdc/transaction` — Initiate CBDC transaction
  - `GET /v1/cbdc/block/{height}` — Get ledger block
  - `GET /v1/lineage/export/{entity_id}` — Export audit trail

- **Accessibility Features**
  - 50+ ARIA labels on form inputs, buttons, tabs
  - Semantic HTML with proper roles (tablist, tab, alert, region, etc.)
  - Keyboard navigation (Tab, Arrow keys, Home/End, Escape)
  - Form error display with inline feedback (no alert dialogs)
  - Loading skeletons with shimmer animation
  - Focus-visible indicators for keyboard users
  - Screen reader support for tables and forms
  - aria-describedby linking help text to fields
  - Live regions for error/status announcements

- **Automated Verification Script**
  - `pkg/web/handlers/ui/a11y-verify.py` — Accessibility auditor
  - `verify-a11y.sh` — Bash wrapper
  - `a11y-report.json` — Machine-readable metrics
  - 9.2/10 accessibility score achieved

- **Documentation**
  - [README.md](./README.md) — Project overview
  - [CONTRIBUTING.md](./CONTRIBUTING.md) — Contribution guidelines
  - [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) — Community standards
  - [GETTING_STARTED.md](./GETTING_STARTED.md) — Setup instructions
  - [SECURITY.md](./SECURITY.md) — Security policy
  - [ROADMAP.md](./ROADMAP.md) — Development roadmap
  - [FAQ.md](./FAQ.md) — Frequently asked questions
  - [CLAUDE.md](./CLAUDE.md) — Architecture and patterns
  - [docs/](./docs/) — API reference, deployment guides, configuration
  - [WORKFLOW_STATE_TERMINOLOGY.md](./WORKFLOW_STATE_TERMINOLOGY.md) — State machine documentation
  - [PHASE1_TEST_GUIDE.md](./PHASE1_TEST_GUIDE.md) — Manual testing checklist
  - [STAGING_DEPLOYMENT_GUIDE.md](./STAGING_DEPLOYMENT_GUIDE.md) — Deployment instructions

- **Testing**
  - 83/83 backend tests passing
  - 5/5 E2E integration tests:
    - Happy path (order → settlement → fulfillment)
    - Compliance blocking (velocity exceeded)
    - Settlement batching (multiple orders consolidated)
    - Audit trail verification (full lineage captured)
    - Reconciliation verification (amounts match, no double-spends)
  - 30+ unit tests for payment, commerce, CBDC, compliance
  - Integration tests for complete workflows
  - Test coverage reports with golangci-lint

- **Observability**
  - Prometheus metrics on port 9464
  - OpenTelemetry tracing support
  - Structured logging with correlation IDs
  - Health check endpoints (/healthz, /readyz)

- **Deployment**
  - Docker support with Dockerfile
  - Docker Compose for full stack (API, PostgreSQL, Ollama, Prometheus, Grafana, OTel)
  - Kubernetes manifests for production
  - Binary builds for macOS, Linux, Windows

### Changed
- N/A (first release)

### Deprecated
- N/A (no deprecated features yet)

### Removed
- N/A (no removed features)

### Fixed
- N/A (no bug fixes in first release)

### Security
- Input validation on all API endpoints
- SQL injection prevention via parameterized queries
- XSS prevention with semantic HTML
- CBDC ledger immutability via hash-chain
- Rate limiting on API endpoints
- JWT-based authentication with secure token handling

---

## Version History Reference

| Version | Release Date | Phase | Status |
|---------|--------------|-------|--------|
| 1.0.0 | June 1, 2026 | Phase 1 | ✅ Released |
| 1.1.0 | July 15, 2026 | Phase 2 | 📋 Planned |
| 2.0.0 | Sept 15, 2026 | Phase 3 | 📋 Planned |
| 2.1.0 | Nov 30, 2026 | Phase 4 | 📋 Planned |
| 3.0.0 | Feb 28, 2027 | Phase 5 | 📋 Planned |
| 3.1.0 | Apr 30, 2027 | Phase 6 | 📋 Planned |

---

## Upcoming Changes

### Phase 2 (Security) - v1.1.0
- CSRF protection implementation
- HttpOnly cookie-based sessions
- CSP headers and security headers
- MFA foundation (TOTP, SMS)

### Phase 3 (Advanced Security) - v2.0.0
- Full MFA implementation
- Fraud detection engine
- Advanced compliance checks
- Real-time monitoring

### Phase 4 (AI & Automation) - v2.1.0
- Settlement optimization
- Dispute resolution automation
- Intelligent customer service
- Predictive compliance

### Phase 5 (Scale & Internationalization) - v3.0.0
- Multi-region deployment
- Cross-border payments
- Multi-currency support
- Localization

### Phase 6 (Analytics & Reporting) - v3.1.0
- Real-time analytics dashboard
- Regulatory reporting automation
- Business intelligence tools
- Risk analytics

---

## How to Report Issues

- **Bugs**: [Bug Report](https://github.com/c2siorg/genie/issues/new?template=bug-report.md)
- **Features**: [Feature Request](https://github.com/c2siorg/genie/issues/new?template=feature-request.md)
- **Security**: Email security@c2si.org

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on how to contribute to Genie.

---

## License

Genie is released under the [MIT License](./LICENSE).

---

## Acknowledgments

See [CONTRIBUTORS.md](./CONTRIBUTORS.md) for a list of all contributors who have helped shape Genie.

---

**Last Updated**: June 1, 2026
