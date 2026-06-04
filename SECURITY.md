# Security Policy

## Overview

Genie is a financial services platform handling sensitive data and real transactions. Security is a critical responsibility shared by maintainers and the community.

## Reporting Security Vulnerabilities

**Do NOT open public GitHub issues for security vulnerabilities.**

If you discover a security vulnerability:

1. **Email**: security@c2si.org
2. **Include**:
   - Description of the vulnerability
   - Steps to reproduce (if applicable)
   - Potential impact
   - Suggested fix (if you have one)
   - Your contact information

3. **Response Timeline**:
   - Acknowledgment: Within 24 hours
   - Initial assessment: Within 3-5 business days
   - Status updates: Every 5 business days
   - Public disclosure: Coordinated with reporter

## Responsible Disclosure

We follow [coordinated vulnerability disclosure](https://en.wikipedia.org/wiki/Coordinated_vulnerability_disclosure):

1. **Private Report**: Security team has exclusive information initially
2. **Assessment**: Team confirms and evaluates impact
3. **Fix Development**: Patch is developed and tested
4. **Advance Notice**: Reporters get early notice before public disclosure
5. **Public Release**: CVE and patch released simultaneously

## Security Principles

### Design
- **Defense in Depth**: Multiple security layers
- **Principle of Least Privilege**: Minimal necessary permissions
- **Fail Securely**: Errors don't expose vulnerabilities
- **Secure by Default**: Safe defaults, explicit opt-in for risky features

### Implementation
- **Input Validation**: Reject malformed or malicious input
- **Output Encoding**: Prevent injection attacks
- **Authentication**: Strong, tested auth mechanisms
- **Authorization**: Fine-grained access control
- **Cryptography**: Use standard libraries, avoid custom crypto

### Operations
- **Dependency Management**: Regular updates, vulnerability scanning
- **Logging**: Security events logged, PII excluded
- **Monitoring**: Anomaly detection and alerting
- **Incident Response**: Documented procedures

## Security Features

### Built-in Protections

- **CBDC Ledger**: Immutable hash-chain prevents double-spending
- **Compliance Engine**: AML/KYC velocity monitoring and sanctions screening
- **Payment Integrity**: Cryptographic commitment to payment data
- **Audit Trail**: Complete lineage with hash-chain verification
- **CSRF Protection**: Double-submit cookie pattern (Phase 2)
- **Session Security**: HttpOnly, Secure, SameSite cookies (Phase 2)

### Authentication & Authorization

- **JWT-based**: Stateless, scalable authentication
- **RBAC**: Role-based access control
- **API Keys**: For service-to-service communication
- **MFA**: Planned for Phase 3

### Data Protection

- **Encryption at Rest**: Database encryption (PostgreSQL pgcrypto)
- **Encryption in Transit**: TLS 1.2+ for all network communication
- **Key Management**: Separate encryption keys per environment
- **PII Handling**: Minimal collection, secure deletion procedures

## Security Requirements for Contributors

### Code Review
- All code must be reviewed before merging
- Security implications must be explicitly considered
- Tests required for security-related changes

### Testing
- Include security test cases (e.g., authorization bypass, injection)
- Test error conditions and edge cases
- Verify sensitive data isn't logged

### Dependencies
- Pin exact versions when possible
- Review dependency changes before merging
- Report if dependencies have known vulnerabilities

### Secrets Management
- Never commit secrets (API keys, tokens, passwords)
- Use `.env.example` for templates
- Use environment variables or secret managers in production

### Documentation
- Document security assumptions
- Explain security-related code with comments
- Update threat model when security changes

## Known Security Considerations

### Current Phase (Phase 1)

✅ **Implemented**
- Input validation on all API endpoints
- SQL injection prevention (parameterized queries via pgx)
- XSS prevention (semantic HTML, context-aware output)
- CBDC ledger immutability via hash-chain
- Audit trail with lineage verification
- Compliance engine with velocity limits

⚠️ **Planned (Phase 2)**
- CSRF protection with double-submit cookies
- HttpOnly session management
- Content Security Policy headers
- Additional rate limiting
- Enhanced error handling to prevent information disclosure

🔄 **Future (Phase 3+)**
- Multi-factor authentication
- Hardware security module integration
- Advanced threat detection
- Penetration testing program
- Bug bounty program

### Threat Model

See [architecture documentation](./docs/ai-governance-security.md) for detailed threat model.

## Vulnerability Disclosure Timeline

Example timeline for a critical vulnerability:

| Day | Action |
|-----|--------|
| Day 0 | Report received, acknowledged |
| Day 1-2 | Assessment complete, impact determined |
| Day 3-5 | Fix developed and tested |
| Day 6-7 | Final review and validation |
| Day 8 | CVE assigned, advance notice sent |
| Day 9 | Public disclosure and patch release |

**This timeline may be adjusted based on severity and complexity.**

## Security Advisories

Security advisories for known vulnerabilities are published at:
- GitHub Security Advisories: https://github.com/c2siorg/genie/security/advisories
- Mailing list: security-announce@c2si.org (subscribe)

Subscribe to receive notifications of security updates.

## Compliance Standards

Genie adheres to:

- **RBI FREE-AI**: Indian framework for AI in finance
- **OWASP Top 10**: Web application security
- **PCSE**: Payment Card Security Standards (where applicable)
- **GDPR**: Data protection (for EU data subjects)
- **CCPA**: California privacy rights

## Security Testing

### Before Deployment

- **Code Review**: Security-focused peer review
- **Static Analysis**: golangci-lint, gosec
- **Dependency Audit**: `go mod audit`
- **Manual Testing**: Authorization, injection, state validation

### Ongoing

- **Dependency Updates**: Weekly scans via Dependabot
- **Log Analysis**: Security event monitoring
- **Incident Detection**: Unusual transaction patterns
- **External Audit**: Annual third-party assessment

## Incident Response

### If You Discover a Breach

1. **Immediate Actions**:
   - Stop further exposure if possible
   - Isolate affected systems
   - Preserve evidence

2. **Notification**:
   - Email security@c2si.org immediately
   - Include scope, timeline, and impact
   - Do not disclose publicly yet

3. **Coordination**:
   - Security team investigates
   - Affected parties notified confidentially
   - Public disclosure prepared

## Security Resources

- **OWASP**: https://owasp.org/
- **Go Security**: https://golang.org/security
- **NIST Cybersecurity Framework**: https://www.nist.gov/cyberframework
- **RBI Guidelines**: https://www.rbi.org.in/

## Security Contacts

- **Security Report**: security@c2si.org
- **Governance Questions**: governance@c2si.org
- **General Questions**: info@c2si.org

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | June 1, 2026 | Initial security policy |

---

**Last Updated**: June 1, 2026  
**Version**: 1.0
