# Frequently Asked Questions (FAQ)

## General Questions

### What is Genie?

Genie is an open-source financial services platform for e-Rupee commerce. It enables merchants and customers to transact using India's Central Bank Digital Currency (CBDC) with built-in compliance, settlement, and AI-powered decision-making.

**Key Features**:
- e-Rupee payment processing
- Order management workflows
- CBDC ledger integration
- Compliance engine (AML, KYC, velocity monitoring)
- Settlement batching with netting
- Complete audit trail and reconciliation

### Who is Genie for?

- **Merchants**: Accept e-Rupee payments for goods/services
- **Fintechs**: Build payment solutions on top of Genie
- **Banks**: Integrate CBDC settlement
- **Regulators**: Monitor compliance and transaction flows
- **Developers**: Extend with custom payment logic

### What does Genie do?

1. **Process Payments**: Customer → Merchant via e-Rupee (CBDC)
2. **Check Compliance**: AML, KYC, velocity limits, sanctions screening
3. **Settle Funds**: Batch processing with netting optimization
4. **Track Everything**: Immutable audit trail with hash-chain verification
5. **Report**: Regulatory reporting (RBI, SEBI, FIU)

### Is Genie production-ready?

**Phase 1 (Complete)**: Core functionality, accessible to 80%+ of users, passing 83/83 tests

**Not Yet Production-Ready**: 
- MFA not yet implemented
- Advanced fraud detection in development
- HSM integration planned

**Timeline**: Phase 2 (Security) ships July 2026, Phase 3 (Advanced Security) by September 2026

---

## Technical Questions

### What are the system requirements?

**Minimum**:
- Go 1.25.0+
- PostgreSQL 12+
- 512MB RAM
- 2GB disk space

**Recommended**:
- Go 1.25.0+
- PostgreSQL 14+
- 2GB+ RAM
- 10GB+ disk space
- Docker & Docker Compose (for development)

### How do I get started?

See [GETTING_STARTED.md](./GETTING_STARTED.md) for step-by-step instructions.

**Quick Start** (5 minutes):
```bash
git clone https://github.com/c2siorg/genie.git
cd genie
cp .env.example .env
docker-compose up -d
```

### What databases does Genie support?

Currently: **PostgreSQL 12+** (required)

Future support planned for:
- Cloud PostgreSQL (AWS RDS, Google Cloud SQL, Azure Database)
- Other databases via ORM abstraction (planned for Phase 4)

### What LLM does Genie use?

**Default**: Ollama (local, self-hosted)
- Chat model: `llama3.2:1b` (lightweight, CPU-compatible)
- Embedding model: `nomic-embed-text`

**Alternative Models**:
- `qwen3.5:latest` (more powerful, requires more VRAM)
- `granite4.1` (IBM model)
- `llama3.1:8b` (larger version, requires 8GB+ VRAM)

**Other Providers** (planned):
- OpenAI (GPT-4)
- Anthropic (Claude via API)
- Google (PaLM)
- AWS (Bedrock)

### Can I use Genie without Ollama?

Yes! Set `GENIE_LLM=mock` to use a mock LLM that doesn't require local models.

Perfect for:
- Development and testing
- CI/CD pipelines
- Understanding the flow without LLM overhead

### How do I configure payment parameters?

Payment configuration is done via environment variables:

```bash
# JWT secret for signing payment tokens
GENIE_JWT_SECRET=your-secret-here

# CBDC connection
GENIE_CBDC_NODE=http://localhost:9000

# Settlement parameters
GENIE_SETTLEMENT_BATCH_SIZE=100
GENIE_SETTLEMENT_TIMEOUT=300

# Compliance limits
GENIE_VELOCITY_LIMIT_24H=1000000  # ₹10,000 in paise
GENIE_VELOCITY_LIMIT_7D=5000000   # ₹50,000 in paise
```

See [docs/configuration.md](./docs/configuration.md) for all options.

---

## API Questions

### How do I authenticate with the API?

**Step 1**: Create a user or login
```bash
curl -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "user@example.com",
    "password": "secure-password",
    "name": "John Doe"
  }'
```

**Step 2**: Use the JWT token
```bash
curl http://localhost:8080/v1/users/me \
  -H "Authorization: Bearer <JWT_TOKEN>"
```

JWT tokens are valid for 24 hours by default.

### What are the API rate limits?

Current limits (Phase 1):
- **Authenticated**: 1000 requests/hour per user
- **Unauthenticated**: 100 requests/hour per IP

See [docs/api.md](./docs/api.md#rate-limiting) for detailed info.

### How do I create an order?

```bash
TOKEN="your-jwt-token"

curl -X POST http://localhost:8080/v1/commerce/order \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "merchant_id": "merchant_001",
    "customer_id": "customer_001",
    "items": [
      {
        "name": "Item 1",
        "quantity": 2,
        "price_paise": 50000
      }
    ]
  }'
```

Returns: Order ID and initial status

### How do I execute a payment?

```bash
TOKEN="your-jwt-token"
ORDER_ID="order_xyz"

curl -X POST http://localhost:8080/v1/commerce/order/$ORDER_ID/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "payment_method": "cbdc",
    "from_account": "customer_001"
  }'
```

The order automatically flows through:
1. Payment initiation
2. Compliance check
3. CBDC settlement
4. Order fulfillment

### What payment methods are supported?

**Phase 1**:
- e-Rupee (CBDC) ✅

**Phase 2 (Planned)**:
- Bank transfers
- Digital wallets

**Phase 3+ (Planned)**:
- Credit/debit cards
- Cross-border payments
- Cryptocurrency

---

## Compliance Questions

### What compliance checks does Genie run?

1. **KYC (Know Your Customer)**: Customer identity verification
2. **AML (Anti-Money Laundering)**: Suspicious activity detection
3. **Velocity Monitoring**: Transaction limit enforcement
4. **Sanctions Screening**: PEP and sanctions list checks
5. **Fraud Detection**: Anomaly and pattern detection (Phase 3)

### Are transactions reversible?

**CBDC Transactions**: Irreversible once confirmed to ledger (by design of CBDC)

**Order-Level Reversals**: Supported via:
- Refunds (within 30 days)
- Chargebacks (dispute process)
- Cancellation (before settlement)

### How do I handle a disputed transaction?

1. **Report**: Open a dispute via `/v1/disputes/report`
2. **Investigation**: Automatic evidence collection
3. **Resolution**: Merchant or system decision
4. **Settlement**: Refund or confirmation

See [docs/disputes.md](./docs/disputes.md) for detailed process.

### Is Genie RBI compliant?

Genie is designed to align with **RBI FREE-AI** framework for AI in finance.

**Compliance Areas**:
- ✅ Deterministic payment processing
- ✅ Audit trail and lineage tracking
- ✅ Explainable compliance decisions
- ✅ Regulatory reporting capability

**Verification**: See [docs/free-ai-mapping.md](./docs/free-ai-mapping.md)

---

## Deployment Questions

### How do I deploy to production?

See [STAGING_DEPLOYMENT_GUIDE.md](./STAGING_DEPLOYMENT_GUIDE.md) for detailed instructions.

**Quick Options**:
1. **Docker Compose** (small scale): `docker-compose -f docker-compose.prod.yaml up -d`
2. **Kubernetes** (large scale): Apply manifests in `/k8s/prod/`
3. **Cloud** (AWS/GCP/Azure): Use provided IaC templates

### How do I scale Genie?

**Horizontal Scaling**:
- Run multiple API instances behind a load balancer
- Use PostgreSQL replication
- Cache API responses with Redis

**Vertical Scaling**:
- Increase CPU/RAM allocations
- Optimize database indexes
- Enable query caching

See [docs/scaling.md](./docs/scaling.md) for detailed guidance.

### What's the backup strategy?

**Recommended**:
- Daily PostgreSQL backups
- WAL archiving for point-in-time recovery
- Cross-region replication
- Test restore procedures monthly

See [docs/operations/backups.md](./docs/operations/backups.md)

### How do I monitor Genie?

Genie includes observability via:
- **Prometheus metrics**: `http://localhost:9464/metrics`
- **OpenTelemetry tracing**: Traces to Jaeger/Tempo
- **Grafana dashboards**: Pre-built dashboards included
- **Application logs**: Structured logging with correlation IDs

See [docs/operations/monitoring.md](./docs/operations/monitoring.md)

---

## Security Questions

### Is Genie secure?

Security score: **8.5/10** (Phase 1)

**Implemented**:
- Input validation & SQL injection prevention
- CBDC ledger immutability
- Audit trail with hash-chain verification
- Role-based access control
- HTTPS only

**Coming in Phase 2**:
- CSRF protection
- HttpOnly cookies
- CSP headers
- Enhanced error handling

See [SECURITY.md](./SECURITY.md) for detailed policy.

### How do I report a security vulnerability?

**DO NOT** open a public issue!

Email: **security@c2si.org**

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if applicable)

See [SECURITY.md](./SECURITY.md#reporting-security-vulnerabilities) for full process.

### How is sensitive data protected?

- **Passwords**: bcrypt hashing (never stored plain)
- **API Tokens**: Salted, hashed, with rotation
- **CBDC Keys**: HSM storage (Phase 2+)
- **Audit Trail**: Encrypted at rest
- **PII**: Minimal collection, encrypted, auto-deleted

See [docs/security/data-protection.md](./docs/security/data-protection.md)

---

## Contributing Questions

### How do I contribute to Genie?

1. **Read**: [CONTRIBUTING.md](./CONTRIBUTING.md)
2. **Fork**: The repository on GitHub
3. **Create**: A feature branch
4. **Code**: Following project standards
5. **Test**: Add tests for new functionality
6. **PR**: Submit for review

See [CONTRIBUTING.md](./CONTRIBUTING.md) for detailed workflow.

### What are the coding standards?

- **Language**: Go
- **Style**: [Effective Go](https://golang.org/doc/effective_go)
- **Linting**: golangci-lint
- **Testing**: ≥80% coverage for new code
- **Commits**: Follow conventional format

See [CONTRIBUTING.md#coding-standards](./CONTRIBUTING.md#coding-standards)

### Can I propose a new feature?

Yes! Use the [Feature Request](https://github.com/c2siorg/genie/issues/new?template=feature-request.md) issue template.

**Include**:
- Clear description
- Motivation (why you need it)
- Proposed solution
- Acceptance criteria
- Impact assessment

### What's the review process?

1. **Automated**: CI/CD tests, linting
2. **Code Review**: 1-2 maintainers
3. **Security Review**: If applicable
4. **Approval**: Merge when all checks pass

**Timeline**: 2-5 business days depending on complexity

---

## Community Questions

### Where can I ask questions?

- **Discussions**: [GitHub Discussions](https://github.com/c2siorg/genie/discussions)
- **Issues**: [GitHub Issues](https://github.com/c2siorg/genie/issues)
- **Email**: info@c2si.org

### Is there a community chat?

Currently: GitHub Discussions

Planned (Phase 4): Discord server

### How can I stay updated?

- **GitHub**: Star/watch the repository
- **Releases**: Subscribe to release notifications
- **Email**: Subscribe to mailing list
- **Twitter**: Follow @GenieFintech (coming soon)

---

## More Questions?

- 📖 **Documentation**: [docs/](./docs/) directory
- 🐛 **Report Issues**: [GitHub Issues](https://github.com/c2siorg/genie/issues)
- 💬 **Discuss**: [GitHub Discussions](https://github.com/c2siorg/genie/discussions)
- 📧 **Email**: info@c2si.org

---

**Last Updated**: June 1, 2026  
**Contributing to this FAQ**: See [CONTRIBUTING.md](./CONTRIBUTING.md)
