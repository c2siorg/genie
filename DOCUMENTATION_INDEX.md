# Genie Documentation Index

**Complete guide to all Genie documentation and resources.**

---

## Quick Start (Start Here!)

1. **New to Genie?** → [README.md](./README.md)
2. **Want to run it locally?** → [GETTING_STARTED.md](./GETTING_STARTED.md)
3. **Have questions?** → [FAQ.md](./FAQ.md)
4. **Want to contribute?** → [CONTRIBUTING.md](./CONTRIBUTING.md)

---

## Documentation Structure

### 📚 Main Documentation Files

| Document | Purpose | Audience |
|----------|---------|----------|
| [README.md](./README.md) | Project overview, features, quick start | Everyone |
| [GETTING_STARTED.md](./GETTING_STARTED.md) | Step-by-step local setup (5-15 min) | Developers |
| [CONTRIBUTING.md](./CONTRIBUTING.md) | How to contribute code and ideas | Contributors |
| [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) | Community standards and behavior | Everyone |
| [SECURITY.md](./SECURITY.md) | Security policy and reporting | Security practitioners |
| [FAQ.md](./FAQ.md) | Answers to common questions | Everyone |
| [ROADMAP.md](./ROADMAP.md) | Future plans and milestones | Product/Planning |
| [CHANGELOG.md](./CHANGELOG.md) | What changed in each release | Users |
| [CLAUDE.md](./CLAUDE.md) | Architecture, patterns, code guidelines | Developers |
| [LICENSE](./LICENSE) | MIT License terms | Legal |

### 🏗️ Architecture & Design

| Document | Topic |
|----------|-------|
| [CLAUDE.md](./CLAUDE.md) | System architecture, design patterns, code organization |
| [WORKFLOW_STATE_TERMINOLOGY.md](./WORKFLOW_STATE_TERMINOLOGY.md) | OrderStatus vs WorkflowStep clarification |
| [docs/architecture.md](./docs/architecture.md) | Detailed system design and component interactions |
| [docs/ai-governance-security.md](./docs/ai-governance-security.md) | RBI FREE-AI compliance and threat model |
| [docs/free-ai-mapping.md](./docs/free-ai-mapping.md) | Mapping to RBI FREE-AI requirements |

### 📖 API Documentation

| Document | Content |
|----------|---------|
| [docs/api.md](./docs/api.md) | Complete API reference with all endpoints |
| [docs/api-payment.md](./docs/api-payment.md) | Payment endpoint details |
| [docs/api-commerce.md](./docs/api-commerce.md) | Order and commerce endpoints |
| [docs/api-merchant.md](./docs/api-merchant.md) | Merchant onboarding endpoints |
| [docs/api-compliance.md](./docs/api-compliance.md) | Compliance check endpoints |
| [docs/api-cbdc.md](./docs/api-cbdc.md) | CBDC ledger endpoints |

### 🧪 Testing & QA

| Document | Purpose |
|----------|---------|
| [PHASE1_TEST_GUIDE.md](./PHASE1_TEST_GUIDE.md) | Manual testing checklist (30-45 min) |
| [PHASE1_TEST_CHECKLIST.md](./PHASE1_TEST_CHECKLIST.md) | Quick QA checklist (20 min) |
| [docs/testing/unit-tests.md](./docs/testing/unit-tests.md) | Unit testing guidelines |
| [docs/testing/integration-tests.md](./docs/testing/integration-tests.md) | Integration testing guide |
| [docs/testing/e2e-tests.md](./docs/testing/e2e-tests.md) | End-to-end testing |

### 🚀 Deployment & Operations

| Document | Topic |
|----------|-------|
| [STAGING_DEPLOYMENT_GUIDE.md](./STAGING_DEPLOYMENT_GUIDE.md) | Complete deployment guide (native, Docker, K8s) |
| [docs/deployment/docker.md](./docs/deployment/docker.md) | Docker deployment |
| [docs/deployment/kubernetes.md](./docs/deployment/kubernetes.md) | Kubernetes deployment |
| [docs/deployment/cloud.md](./docs/deployment/cloud.md) | Cloud providers (AWS, GCP, Azure) |
| [docs/operations/monitoring.md](./docs/operations/monitoring.md) | Monitoring and observability |
| [docs/operations/backups.md](./docs/operations/backups.md) | Backup and recovery procedures |
| [docs/operations/troubleshooting.md](./docs/operations/troubleshooting.md) | Troubleshooting guide |

### 🔒 Security & Compliance

| Document | Topic |
|----------|-------|
| [SECURITY.md](./SECURITY.md) | Security policy and practices |
| [docs/security/data-protection.md](./docs/security/data-protection.md) | Data protection and encryption |
| [docs/security/authentication.md](./docs/security/authentication.md) | Authentication mechanisms |
| [docs/security/authorization.md](./docs/security/authorization.md) | Authorization and access control |
| [docs/compliance/aml-kyc.md](./docs/compliance/aml-kyc.md) | AML/KYC workflow |
| [docs/compliance/rbi-requirements.md](./docs/compliance/rbi-requirements.md) | RBI compliance requirements |

### ⚙️ Configuration & Setup

| Document | Content |
|----------|---------|
| [.env.example](./.env.example) | Environment variables template |
| [docs/configuration.md](./docs/configuration.md) | All configuration options |
| [docker-compose.yaml](./docker-compose.yaml) | Docker Compose setup |
| [Makefile](./Makefile) | Build targets and commands |

### 🎯 Accessibility

| Document | Content |
|----------|---------|
| [A11Y_VERIFICATION.md](./pkg/web/handlers/ui/A11Y_VERIFICATION.md) | Accessibility verification details |
| [PHASE1_DEPLOYMENT_SUMMARY.md](./PHASE1_DEPLOYMENT_SUMMARY.md) | Phase 1 accessibility metrics |

### 📝 GitHub Templates

| File | Purpose |
|------|---------|
| [.github/ISSUE_TEMPLATE/bug-report.md](./.github/ISSUE_TEMPLATE/bug-report.md) | Bug report template |
| [.github/ISSUE_TEMPLATE/feature-request.md](./.github/ISSUE_TEMPLATE/feature-request.md) | Feature request template |
| [.github/pull_request_template.md](./.github/pull_request_template.md) | PR submission template |

---

## Reading Order by Role

### 👤 Users
1. [README.md](./README.md) — What is Genie?
2. [GETTING_STARTED.md](./GETTING_STARTED.md) — How do I run it?
3. [FAQ.md](./FAQ.md) — Answers to common questions
4. [docs/api.md](./docs/api.md) — How do I use the API?

### 👨‍💻 Developers (Contributing Code)
1. [README.md](./README.md) — Project overview
2. [GETTING_STARTED.md](./GETTING_STARTED.md) — Set up locally
3. [CONTRIBUTING.md](./CONTRIBUTING.md) — Contribution workflow
4. [CLAUDE.md](./CLAUDE.md) — Architecture and patterns
5. [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) — Community standards
6. [SECURITY.md](./SECURITY.md) — Security practices (important!)

### 🏗️ DevOps / Infrastructure
1. [STAGING_DEPLOYMENT_GUIDE.md](./STAGING_DEPLOYMENT_GUIDE.md) — Deployment options
2. [docs/deployment/](./docs/deployment/) — Specific platform guides
3. [docs/operations/monitoring.md](./docs/operations/monitoring.md) — Observability
4. [docker-compose.yaml](./docker-compose.yaml) — Local stack setup
5. [SECURITY.md](./SECURITY.md) — Security considerations

### 🔒 Security Practitioners
1. [SECURITY.md](./SECURITY.md) — Security policy
2. [docs/ai-governance-security.md](./docs/ai-governance-security.md) — Threat model
3. [docs/security/](./docs/security/) — Detailed security docs
4. [CLAUDE.md](./CLAUDE.md) — Architecture overview

### 📋 Compliance & Legal
1. [SECURITY.md](./SECURITY.md) — Security policy
2. [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) — Community standards
3. [LICENSE](./LICENSE) — MIT License
4. [docs/compliance/rbi-requirements.md](./docs/compliance/rbi-requirements.md) — RBI compliance
5. [docs/free-ai-mapping.md](./docs/free-ai-mapping.md) — FREE-AI alignment

### 🎨 Product / Managers
1. [README.md](./README.md) — Project overview
2. [ROADMAP.md](./ROADMAP.md) — Future plans
3. [CHANGELOG.md](./CHANGELOG.md) — Release history
4. [FAQ.md](./FAQ.md) — Common questions

---

## Key Documents by Topic

### Getting Started
- [GETTING_STARTED.md](./GETTING_STARTED.md) — Local setup (5-15 min)
- [README.md](./README.md) — Quick overview
- [FAQ.md](./FAQ.md) — Common questions

### Contributing Code
- [CONTRIBUTING.md](./CONTRIBUTING.md) — Full contribution guide
- [CLAUDE.md](./CLAUDE.md) — Code patterns and architecture
- [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) — Community standards

### API Usage
- [docs/api.md](./docs/api.md) — Complete API reference
- [docs/api-*.md](./docs/) — Endpoint-specific guides
- [FAQ.md](./FAQ.md#api-questions) — API FAQs

### Deployment
- [STAGING_DEPLOYMENT_GUIDE.md](./STAGING_DEPLOYMENT_GUIDE.md) — Complete guide
- [docs/deployment/](./docs/deployment/) — Platform-specific guides
- [docker-compose.yaml](./docker-compose.yaml) — Local setup

### Security
- [SECURITY.md](./SECURITY.md) — Security policy
- [docs/security/](./docs/security/) — Detailed security docs
- [docs/ai-governance-security.md](./docs/ai-governance-security.md) — Threat model

### Testing
- [PHASE1_TEST_GUIDE.md](./PHASE1_TEST_GUIDE.md) — Manual testing (30-45 min)
- [PHASE1_TEST_CHECKLIST.md](./PHASE1_TEST_CHECKLIST.md) — Quick QA (20 min)
- [docs/testing/](./docs/testing/) — Testing guides

### Accessibility
- [PHASE1_DEPLOYMENT_SUMMARY.md](./PHASE1_DEPLOYMENT_SUMMARY.md) — A11y metrics
- [A11Y_VERIFICATION.md](./pkg/web/handlers/ui/A11Y_VERIFICATION.md) — Verification details
- [PHASE1_TEST_GUIDE.md](./PHASE1_TEST_GUIDE.md#accessibility-verification-script) — A11y testing

---

## Finding Information

### I want to...
| Goal | Document |
|------|----------|
| **Get started quickly** | [GETTING_STARTED.md](./GETTING_STARTED.md) |
| **Understand the architecture** | [CLAUDE.md](./CLAUDE.md) & [docs/architecture.md](./docs/architecture.md) |
| **Use the API** | [docs/api.md](./docs/api.md) |
| **Deploy to production** | [STAGING_DEPLOYMENT_GUIDE.md](./STAGING_DEPLOYMENT_GUIDE.md) |
| **Monitor and operate** | [docs/operations/monitoring.md](./docs/operations/monitoring.md) |
| **Contribute code** | [CONTRIBUTING.md](./CONTRIBUTING.md) |
| **Report a security issue** | [SECURITY.md](./SECURITY.md) |
| **Understand compliance** | [docs/free-ai-mapping.md](./docs/free-ai-mapping.md) |
| **Test the system** | [PHASE1_TEST_GUIDE.md](./PHASE1_TEST_GUIDE.md) |
| **Check accessibility** | [A11Y_VERIFICATION.md](./pkg/web/handlers/ui/A11Y_VERIFICATION.md) |

---

## Document Statistics

- **Total Documents**: 40+
- **Total Pages**: ~150 equivalent
- **Coverage Areas**: Getting started, API, deployment, security, compliance, testing, operations
- **Target Audiences**: Users, developers, DevOps, security, compliance

---

## Maintenance

All documentation is maintained in the repository alongside code:
- Documentation changes require the same review process as code
- Updates tracked in [CHANGELOG.md](./CHANGELOG.md)
- Outdated docs marked with deprecation notice
- Community contributions welcome via PRs

---

## Questions & Support

- 📖 **Not finding what you need?** Check [FAQ.md](./FAQ.md)
- 🐛 **Found an error in docs?** Open an issue
- 💬 **Have a question?** Ask in [GitHub Discussions](https://github.com/c2siorg/genie/discussions)
- 📧 **Contact us**: info@c2si.org

---

**Last Updated**: June 1, 2026  
**Total Documentation**: Comprehensive suite covering all aspects of Genie
