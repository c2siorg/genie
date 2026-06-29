# Documentation Themes & Organization

**Complete thematic organization of all Genie documentation.**

---

## Theme Categories

### 🚀 Getting Started
Documents for new users and developers getting started with Genie.

| Document | Purpose | Audience | Time |
|----------|---------|----------|------|
| [README.md](./README.md) | Project overview & quick start | Everyone | 5 min |
| [GETTING_STARTED.md](./GETTING_STARTED.md) | Detailed setup guide | Developers | 15 min |
| [FAQ.md](./FAQ.md) | Common questions answered | Everyone | 10 min |
| [DOCUMENTATION_INDEX.md](./DOCUMENTATION_INDEX.md) | Navigation guide | Everyone | 5 min |

**Theme Tag**: `#getting-started` `#onboarding` `#quick-start`

---

### 💻 Development & Contributing
Documents for developers contributing code and improvements.

| Document | Purpose | Audience | Complexity |
|----------|---------|----------|-----------|
| [CONTRIBUTING.md](./CONTRIBUTING.md) | Contribution workflow & standards | Contributors | Intermediate |
| [CLAUDE.md](./CLAUDE.md) | Architecture & code patterns | Developers | Advanced |
| [WORKFLOW_STATE_TERMINOLOGY.md](./WORKFLOW_STATE_TERMINOLOGY.md) | State machine reference | Developers | Intermediate |
| [.github/ISSUE_TEMPLATE/bug-report.md](./.github/ISSUE_TEMPLATE/bug-report.md) | Bug reporting | Contributors | Beginner |
| [.github/ISSUE_TEMPLATE/feature-request.md](./.github/ISSUE_TEMPLATE/feature-request.md) | Feature requests | Contributors | Beginner |
| [.github/pull_request_template.md](./.github/pull_request_template.md) | PR submission | Contributors | Beginner |

**Theme Tag**: `#development` `#contributing` `#code-quality` `#collaboration`

---

### 📖 API & Integration
Documents for using Genie's API endpoints and integrating with other systems.

| Document | Purpose | Audience | Detail Level |
|----------|---------|----------|--------------|
| [docs/api.md](./docs/api.md) | Complete API reference | Developers | Comprehensive |
| [docs/api-payment.md](./docs/api-payment.md) | Payment endpoints | Developers | Detailed |
| [docs/api-commerce.md](./docs/api-commerce.md) | Commerce endpoints | Developers | Detailed |
| [docs/api-merchant.md](./docs/api-merchant.md) | Merchant endpoints | Developers | Detailed |
| [docs/api-compliance.md](./docs/api-compliance.md) | Compliance endpoints | Developers | Detailed |
| [docs/api-cbdc.md](./docs/api-cbdc.md) | CBDC endpoints | Developers | Detailed |
| [FAQ.md#api-questions](./FAQ.md#api-questions) | API FAQs | Developers | Quick answers |

**Theme Tag**: `#api` `#integration` `#endpoints` `#rest`

---

### 🚀 Deployment & Operations
Documents for deploying and operating Genie in production.

| Document | Purpose | Audience | Environment |
|----------|---------|----------|-------------|
| [STAGING_DEPLOYMENT_GUIDE.md](./STAGING_DEPLOYMENT_GUIDE.md) | Complete deployment guide | DevOps/Ops | All |
| [docs/deployment/docker.md](./docs/deployment/docker.md) | Docker deployment | DevOps | Containers |
| [docs/deployment/kubernetes.md](./docs/deployment/kubernetes.md) | Kubernetes deployment | DevOps | K8s |
| [docs/deployment/cloud.md](./docs/deployment/cloud.md) | Cloud providers | DevOps | AWS/GCP/Azure |
| [docs/operations/monitoring.md](./docs/operations/monitoring.md) | Monitoring & observability | Ops/SRE | Production |
| [docs/operations/backups.md](./docs/operations/backups.md) | Backup & recovery | Ops/DBA | Data protection |
| [docker-compose.yaml](./docker-compose.yaml) | Local dev stack | Developers | Development |

**Theme Tag**: `#deployment` `#devops` `#operations` `#production` `#infrastructure`

---

### 🔒 Security & Compliance
Documents covering security policies, compliance requirements, and data protection.

| Document | Purpose | Audience | Focus |
|----------|---------|----------|-------|
| [SECURITY.md](./SECURITY.md) | Security policy & vulnerability reporting | Security/Compliance | Policy |
| [docs/security/data-protection.md](./docs/security/data-protection.md) | Data protection & encryption | Security/Devs | Implementation |
| [docs/security/authentication.md](./docs/security/authentication.md) | Authentication mechanisms | Developers | Technical |
| [docs/security/authorization.md](./docs/security/authorization.md) | Authorization & access control | Developers | Technical |
| [docs/compliance/aml-kyc.md](./docs/compliance/aml-kyc.md) | AML/KYC workflow | Compliance/Devs | Regulatory |
| [docs/compliance/rbi-requirements.md](./docs/compliance/rbi-requirements.md) | RBI compliance | Compliance/Legal | Regulatory |
| [docs/free-ai-mapping.md](./docs/free-ai-mapping.md) | RBI FREE-AI alignment | Compliance/Legal | Regulatory |
| [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) | Community standards | Community | Governance |

**Theme Tag**: `#security` `#compliance` `#data-protection` `#regulatory` `#aml-kyc`

---

### 📋 Testing & Quality Assurance
Documents for testing Genie and ensuring quality.

| Document | Purpose | Audience | Coverage |
|----------|---------|----------|----------|
| [PHASE1_TEST_GUIDE.md](./PHASE1_TEST_GUIDE.md) | Manual testing checklist | QA/Testers | End-to-end |
| [PHASE1_TEST_CHECKLIST.md](./PHASE1_TEST_CHECKLIST.md) | Quick QA checklist | QA/Testers | Quick pass |
| [docs/testing/unit-tests.md](./docs/testing/unit-tests.md) | Unit testing guide | Developers | Code-level |
| [docs/testing/integration-tests.md](./docs/testing/integration-tests.md) | Integration testing | Developers | Module-level |
| [docs/testing/e2e-tests.md](./docs/testing/e2e-tests.md) | End-to-end testing | QA/Developers | System-level |

**Theme Tag**: `#testing` `#quality-assurance` `#qa` `#coverage`

---

### 📊 Architecture & Design
Documents explaining system design, architecture, and technical decisions.

| Document | Purpose | Audience | Detail |
|----------|---------|----------|--------|
| [CLAUDE.md](./CLAUDE.md) | Architecture & patterns | Architects/Devs | Comprehensive |
| [docs/architecture.md](./docs/architecture.md) | System design | Architects | Detailed |
| [docs/ai-governance-security.md](./docs/ai-governance-security.md) | Threat model & governance | Security/Architects | Security-focused |
| [WORKFLOW_STATE_TERMINOLOGY.md](./WORKFLOW_STATE_TERMINOLOGY.md) | State machine design | Developers | Reference |

**Theme Tag**: `#architecture` `#design` `#system-design` `#patterns`

---

### 🗺️ Roadmap & Planning
Documents for understanding Genie's future direction and planning.

| Document | Purpose | Audience | Horizon |
|----------|---------|----------|---------|
| [ROADMAP.md](./ROADMAP.md) | 6-phase development plan | Everyone | 2026-2027 |
| [CHANGELOG.md](./CHANGELOG.md) | Version history & releases | Users/Devs | Current + Future |
| [FAQ.md#general-questions](./FAQ.md#general-questions) | General context | Everyone | Now |

**Theme Tag**: `#roadmap` `#planning` `#releases` `#vision`

---

### ⚙️ Configuration & Setup
Documents for configuring Genie for different environments and use cases.

| Document | Purpose | Audience | Scope |
|----------|---------|----------|-------|
| [.env.example](./.env.example) | Environment variables | Operators | Template |
| [docs/configuration.md](./docs/configuration.md) | All config options | Operators | Reference |
| [docker-compose.yaml](./docker-compose.yaml) | Dev stack setup | Developers | Local |
| [Makefile](./Makefile) | Build & test targets | Developers | Development |

**Theme Tag**: `#configuration` `#environment` `#setup`

---

### ♿ Accessibility
Documents for web accessibility features and verification.

| Document | Purpose | Audience | Standard |
|----------|---------|----------|----------|
| [PHASE1_DEPLOYMENT_SUMMARY.md](./PHASE1_DEPLOYMENT_SUMMARY.md) | A11y metrics | Accessibility/QA | WCAG |
| [A11Y_VERIFICATION.md](./pkg/web/handlers/ui/A11Y_VERIFICATION.md) | A11y verification | Developers | WCAG 2.1 |

**Theme Tag**: `#accessibility` `#a11y` `#wcag` `#inclusive-design`

---

### 👥 Community & Governance
Documents for community participation and project governance.

| Document | Purpose | Audience | Type |
|----------|---------|----------|------|
| [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) | Community standards | Everyone | Governance |
| [CONTRIBUTING.md](./CONTRIBUTING.md) | Contribution guidelines | Contributors | Governance |
| [SECURITY.md](./SECURITY.md) | Security policy | Everyone | Governance |
| [GITHUB_TOPICS.md](./GITHUB_TOPICS.md) | Repository discovery | Community | Marketing |

**Theme Tag**: `#community` `#governance` `#code-of-conduct` `#collaboration`

---

## Theme-Based Navigation

### By User Role

**🎯 Product Managers**
- [README.md](./README.md) — Overview
- [ROADMAP.md](./ROADMAP.md) — Planning
- [CHANGELOG.md](./CHANGELOG.md) — Releases
- [FAQ.md](./FAQ.md) — User questions

**👨‍💻 Backend Developers**
- [GETTING_STARTED.md](./GETTING_STARTED.md) — Setup
- [CLAUDE.md](./CLAUDE.md) — Architecture
- [CONTRIBUTING.md](./CONTRIBUTING.md) — Workflow
- [docs/api.md](./docs/api.md) — Endpoints
- [docs/testing/](./docs/testing/) — Testing

**🎨 Frontend Developers**
- [GETTING_STARTED.md](./GETTING_STARTED.md) — Setup
- [A11Y_VERIFICATION.md](./pkg/web/handlers/ui/A11Y_VERIFICATION.md) — Accessibility
- [PHASE1_TEST_GUIDE.md](./PHASE1_TEST_GUIDE.md#accessibility-verification-script) — A11y Testing

**🏗️ DevOps/Infrastructure**
- [STAGING_DEPLOYMENT_GUIDE.md](./STAGING_DEPLOYMENT_GUIDE.md) — Deployment
- [docs/deployment/](./docs/deployment/) — Platform guides
- [docs/operations/](./docs/operations/) — Operations
- [docker-compose.yaml](./docker-compose.yaml) — Local stack

**🔒 Security & Compliance**
- [SECURITY.md](./SECURITY.md) — Policy
- [docs/security/](./docs/security/) — Security details
- [docs/compliance/](./docs/compliance/) — Compliance
- [docs/free-ai-mapping.md](./docs/free-ai-mapping.md) — RBI alignment

**🧪 QA & Testing**
- [PHASE1_TEST_GUIDE.md](./PHASE1_TEST_GUIDE.md) — Manual testing
- [PHASE1_TEST_CHECKLIST.md](./PHASE1_TEST_CHECKLIST.md) — Quick checklist
- [docs/testing/](./docs/testing/) — Testing guides

---

## By Topic

### Fintech Topics
- [README.md](./README.md) — What is Genie?
- [docs/api-payment.md](./docs/api-payment.md) — Payment API
- [docs/compliance/](./docs/compliance/) — Compliance
- [ROADMAP.md](./ROADMAP.md) — Future features

### CBDC & Digital Currency
- [README.md](./README.md) — CBDC overview
- [docs/api-cbdc.md](./docs/api-cbdc.md) — CBDC API
- [docs/compliance/rbi-requirements.md](./docs/compliance/rbi-requirements.md) — RBI requirements

### Settlement & Banking
- [docs/api-merchant.md](./docs/api-merchant.md) — Merchant API
- [docs/api-commerce.md](./docs/api-commerce.md) — Commerce API
- [STAGING_DEPLOYMENT_GUIDE.md](./STAGING_DEPLOYMENT_GUIDE.md) — Deployment

### Compliance & Regulation
- [SECURITY.md](./SECURITY.md) — Security policy
- [docs/compliance/](./docs/compliance/) — Compliance docs
- [docs/free-ai-mapping.md](./docs/free-ai-mapping.md) — RBI FREE-AI alignment

---

## Statistics

- **Total Documents**: 40+
- **Themes**: 10 major categories
- **Pages**: ~150 equivalent
- **Audience Groups**: 8 distinct user roles
- **Coverage Areas**: Getting started, API, deployment, security, compliance, testing, architecture

---

**Last Updated**: June 1, 2026  
**Total Theme Tags**: 40+
