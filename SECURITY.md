# Security Policy

Genie is an AI financial assistant that handles sensitive data (PII, financial
statements, credentials). We take security reports seriously and appreciate
responsible disclosure.

## Supported versions

| Version | Supported |
| --- | --- |
| 1.0.x | ✅ |
| < 1.0 | ❌ (pre-release; upgrade to 1.0+) |

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, report privately via either:

- **GitHub Security Advisories** — [open a private report](https://github.com/PratikDhanave/genie/security/advisories/new)
  (preferred; keeps the report and fix coordinated in one place).
- **Email** — i.pratikdhanave@gmail.com with the subject line `SECURITY: <short summary>`.

Please include, as far as you can:

- The affected component (e.g. `pkg/auth`, `pkg/crypto`, an agent, or an HTTP endpoint).
- A description of the vulnerability and its impact.
- Steps to reproduce, or a proof-of-concept.
- Any suggested remediation.

## What to expect

- **Acknowledgement** within 3 business days.
- **Initial assessment** (severity + whether we can reproduce) within 10 business days.
- Coordinated disclosure: we will agree on a public-disclosure date with you once a fix
  is available, and credit you in the advisory unless you prefer to remain anonymous.

## Scope

In scope:

- Authentication / authorization bypass (`pkg/auth`, JWT, RBAC, OAuth 2.1, WebAuthn).
- Cryptographic weaknesses in document encryption (`pkg/crypto` envelope, KEK handling).
- Governance-gate bypass (`pkg/governance`) — e.g. exfiltrating `pii`/`secret`-classified data.
- Prompt injection or jailbreaks that defeat the safety/policy chain (`pkg/safety`, `config/ai-policy.example.yaml`).
- Injection, SSRF, or unsafe deserialization in the HTTP edge (`pkg/web`) or loaders (`pkg/loader`).
- Secret leakage in logs, traces, or error responses.

Out of scope:

- Findings that require a compromised host or physical access.
- Denial of service from unrealistic load against a local demo stack.
- Issues in the sibling projects (`geniepython/`, `microsoftagentframeworklearning/`) — report those to their own repositories.
- Vulnerabilities in third-party dependencies without a demonstrated impact on Genie
  (please still tell us, but report upstream too).

## Handling of secrets

Genie never commits secrets. `GENIE_JWT_SECRET`, `GENIE_KEK_BASE64`, and `GENIE_DB_DSN`
are read only from the environment. If you find a committed secret, treat it as a
vulnerability and report it privately — do not open a public issue.
