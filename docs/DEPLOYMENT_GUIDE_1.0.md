# Genie 1.0 Deployment Guide

**Version**: 1.0.0  
**Date**: June 4, 2026  
**Target**: Production deployment for RBI e-Rupee, CBDC, and financial services

---

## Table of Contents

1. [Pre-Deployment Checklist](#pre-deployment-checklist)
2. [Environment Variables](#environment-variables)
3. [Infrastructure Setup](#infrastructure-setup)
4. [Database Configuration](#database-configuration)
5. [Secrets Management](#secrets-management)
6. [Health Checks & Monitoring](#health-checks--monitoring)
7. [Migration Path](#migration-path)
8. [Troubleshooting](#troubleshooting)
9. [Rollback Procedure](#rollback-procedure)

---

## Pre-Deployment Checklist

### Security

- [ ] **TLS Certificate**: Valid certificate for production domain (expires >90 days)
- [ ] **JWT Secret**: 256-bit random secret (hex/base64), shared between instances
- [ ] **CSRF Secret**: 256-bit random secret, unique per environment
- [ ] **Database Password**: Strong password (min 16 chars, alphanumeric + symbols)
- [ ] **Encryption Key**: 32-byte DEK (Data Encryption Key) for at-rest encryption
- [ ] **Firewall Rules**: Database accessible only from application servers
- [ ] **IP Whitelisting**: Allow traffic only from known sources (RBI gateways, etc.)

### Compliance

- [ ] **RBI FREE-AI**: Review docs/free-ai-mapping.md for all 7 Sutras
- [ ] **Audit Trail**: Verify lineage recording in database
- [ ] **Incident Response Plan**: Document escalation & communication flow
- [ ] **Security.txt**: Create /.well-known/security.txt with security contacts
- [ ] **Privacy Policy**: Aligned with RBI guidelines

### Infrastructure

- [ ] **Kubernetes Cluster** (recommended):
  - Min 3 control nodes, 3 worker nodes
  - Load balancer (AWS ALB, GCP Cloud LB, Nginx, etc.)
  - Persistent volumes for database
  - Network policies for pod-to-pod communication

  OR **Docker Compose** (for small deployments):
  - Single-node setup acceptable for <1M txn/day

- [ ] **Database Server** (PostgreSQL 12+):
  - Primary + 1 hot standby (HA recommended)
  - Automated backups (daily, 30-day retention)
  - Point-in-time recovery enabled

- [ ] **Monitoring & Logging**:
  - OpenTelemetry collector for traces
  - Prometheus for metrics
  - Grafana dashboards
  - ELK stack or CloudWatch for logs

### Testing

- [ ] **Smoke Test**: Deploy to staging, run 100 test transactions
- [ ] **Load Test**: Simulate peak load (QPS = 10x average)
- [ ] **Security Scan**: Run OWASP ZAP, verify no critical findings
- [ ] **Backup Test**: Restore database from backup, verify data integrity

---

## Environment Variables

### Required (must be set before startup)

```bash
# API server
GENIE_HTTP_ADDR=":8080"                    # Listen address
GENIE_ENV="production"                     # production | staging | dev

# Authentication
GENIE_JWT_SECRET="<256-bit-hex>"          # JWT signing key (hex or base64)
GENIE_JWT_ISSUER="genie.rbi.io"           # JWT issuer claim
GENIE_JWT_AUDIENCE="genie-api"             # JWT audience claim
GENIE_JWT_TTL="3600"                       # Token TTL in seconds (default: 1 hour)

# Security
GENIE_CSRF_SERVER_SECRET="<256-bit-hex>"  # CSRF HMAC key
GENIE_CSRF_TOKEN_TTL="900"                 # CSRF token lifetime (default: 15 min)

# Database
GENIE_DB_DSN="postgres://user:pass@host:5432/genie?sslmode=require"
GENIE_DB_POOL_SIZE="20"                    # Connection pool size
GENIE_DB_POOL_TIMEOUT="30"                 # Timeout in seconds

# Encryption at rest
GENIE_KEK_BASE64="<base64-32-bytes>"       # Key Encryption Key for documents

# OpenTelemetry (optional, defaults to stdout exporter)
OTEL_EXPORTER_OTLP_ENDPOINT="https://otel.example.com:4317"
GENIE_OTEL_INSECURE="false"                # true = skip TLS on OTLP gRPC
OTEL_SAMPLER_ARG="0.1"                     # Trace sampling ratio (10%)

# LLM Stack (for agent execution)
CLAUDE_API_KEY="sk-ant-..."                # Anthropic Claude API key
GENIE_MODEL="claude-opus-4-5"              # Model version (default: opus)
```

### Optional

```bash
# Logging
GENIE_LOG_LEVEL="info"                     # info | warn | error | debug
GENIE_LOG_FORMAT="json"                    # json | text
GENIE_LOG_FILE="/var/log/genie/app.log"   # File path (omit for stdout)

# Feature flags
GENIE_ENABLE_CSRF="true"                   # CSRF protection (always true)
GENIE_ENABLE_RATE_LIMIT="true"             # Rate limiting per IP
GENIE_RATE_LIMIT_QPS="100"                 # Requests per second per IP
GENIE_CORS_ALLOWED_ORIGINS="https://app.example.com"  # Comma-separated

# Session management
GENIE_SESSION_COOKIE_TTL="86400"           # 24 hours
GENIE_SESSION_REFRESH_THRESHOLD="0.5"      # Refresh at 50% TTL

# Timeouts
GENIE_READ_TIMEOUT="10"                    # seconds
GENIE_WRITE_TIMEOUT="30"                   # seconds
GENIE_SHUTDOWN_TIMEOUT="30"                # graceful shutdown

# Compliance
GENIE_RBI_SUTRA_MODE="strict"              # strict | lenient
GENIE_INCIDENT_REPORT_URL="https://rbi-incident.example.com"
```

### Secret Rotation (v1.1+)

```bash
# For key rotation without downtime
GENIE_JWT_SECRET_OLD="<previous-key>"      # Old key for validation during rotation
GENIE_CSRF_SERVER_SECRET_OLD="<previous>"
```

---

## Infrastructure Setup

### Docker Compose (for non-production / learning)

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: genie
      POSTGRES_PASSWORD: <strong-password>
      POSTGRES_DB: genie
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./init-db.sql:/docker-entrypoint-initdb.d/01-init.sql
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U genie"]
      interval: 10s
      timeout: 5s
      retries: 5

  genie-api:
    build:
      context: .
      dockerfile: Dockerfile
    environment:
      GENIE_HTTP_ADDR: ":8080"
      GENIE_ENV: "staging"
      GENIE_DB_DSN: "postgres://genie:<password>@postgres:5432/genie?sslmode=disable"
      GENIE_JWT_SECRET: ${GENIE_JWT_SECRET}
      GENIE_CSRF_SERVER_SECRET: ${GENIE_CSRF_SERVER_SECRET}
      GENIE_KEK_BASE64: ${GENIE_KEK_BASE64}
      CLAUDE_API_KEY: ${CLAUDE_API_KEY}
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/healthz"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
```

### Kubernetes (production)

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: genie

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: genie-config
  namespace: genie
data:
  GENIE_HTTP_ADDR: ":8080"
  GENIE_ENV: "production"
  GENIE_LOG_LEVEL: "info"
  GENIE_LOG_FORMAT: "json"

---
apiVersion: v1
kind: Secret
metadata:
  name: genie-secrets
  namespace: genie
type: Opaque
stringData:
  GENIE_JWT_SECRET: "<base64-256-bit-key>"
  GENIE_CSRF_SERVER_SECRET: "<base64-256-bit-key>"
  GENIE_KEK_BASE64: "<base64-32-bytes>"
  GENIE_DB_DSN: "postgres://genie:password@postgres-svc:5432/genie?sslmode=require"
  CLAUDE_API_KEY: "sk-ant-..."

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: genie-api
  namespace: genie
spec:
  replicas: 3
  selector:
    matchLabels:
      app: genie-api
  template:
    metadata:
      labels:
        app: genie-api
    spec:
      containers:
      - name: genie
        image: genie:1.0.0
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
          name: http
        envFrom:
        - configMapRef:
            name: genie-config
        - secretRef:
            name: genie-secrets
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - ALL

---
apiVersion: v1
kind: Service
metadata:
  name: genie-api-svc
  namespace: genie
spec:
  type: LoadBalancer
  selector:
    app: genie-api
  ports:
  - name: http
    port: 80
    targetPort: 8080

---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: genie-api-netpol
  namespace: genie
spec:
  podSelector:
    matchLabels:
      app: genie-api
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 5432  # PostgreSQL
  - to:
    - podSelector: {}
    ports:
    - protocol: TCP
      port: 53    # DNS
```

---

## Database Configuration

### Initial Setup

```bash
# 1. Create database and user
psql -U postgres -c "CREATE USER genie WITH PASSWORD '<password>';"
psql -U postgres -c "CREATE DATABASE genie OWNER genie;"

# 2. Grant permissions
psql -U postgres -d genie -c "
  GRANT ALL PRIVILEGES ON SCHEMA public TO genie;
  GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO genie;
  GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO genie;
"

# 3. Run migrations (included in genie binary)
# Migrations auto-run on startup; no manual step needed

# 4. Verify
psql -U genie -d genie -c "SELECT version();"
```

### Connection String (DSN)

**Standard**:
```
postgres://user:password@host:5432/dbname?sslmode=require
```

**With Connection Pooling** (via application):
```
postgres://user:password@host:5432/dbname?sslmode=require&pool_max_conns=20&pool_min_conns=5
```

**With High Availability** (multiple replicas):
```
postgres://user:password@primary:5432,replica:5432/dbname?sslmode=require&target_session_attrs=read-write
```

### Backup & Recovery

```bash
# Full backup (daily, off-peak hours)
pg_dump -Fc -U genie -d genie > /backups/genie-$(date +%Y%m%d).dump

# Restore from backup
pg_restore -d genie /backups/genie-20260604.dump

# Point-in-time recovery (from WAL archives)
# Configure in postgresql.conf:
#   wal_level = replica
#   archive_mode = on
#   archive_command = 'cp %p /backups/wal_archive/%f'
# Then use pg_basebackup + WAL replay
```

---

## Secrets Management

### Option 1: Environment Variables (for small deployments)

**Recommended for**: Single-node, non-critical deployments

```bash
# Set in .env file (version-controlled separately, never in git)
# Load with: source .env

export GENIE_JWT_SECRET="abc123..."
export GENIE_CSRF_SERVER_SECRET="def456..."
export GENIE_KEK_BASE64="ghi789..."
```

### Option 2: HashiCorp Vault (for production)

**Recommended for**: Multi-node, regulated environments

```bash
# 1. Configure Vault
vault kv put secret/genie/prod \
  jwt_secret="<value>" \
  csrf_secret="<value>" \
  kek_base64="<value>"

# 2. In application startup (pkg/bootstrap/vault.go):
client, _ := api.NewClient(&api.Config{
  Address: "https://vault.example.com:8200",
})
secret, _ := client.Logical().Read("secret/data/genie/prod")
```

### Option 3: Cloud Provider Secrets

**AWS Secrets Manager**:
```bash
aws secretsmanager create-secret \
  --name genie/prod/jwt-secret \
  --secret-string "abc123..."

# In application:
client := secretsmanager.NewFromConfig(cfg)
result, _ := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
  SecretId: aws.String("genie/prod/jwt-secret"),
})
```

**Google Secret Manager**:
```bash
gcloud secrets create genie-jwt-secret \
  --replication-policy="automatic" \
  --data-file=<(echo "abc123...")

# In application:
client := secretmanager.NewClient(ctx)
result, _ := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
  Name: "projects/PROJECT_ID/secrets/genie-jwt-secret/versions/latest",
})
```

### Key Rotation

**Without downtime** (v1.1+):

```bash
# 1. Add new key as GENIE_JWT_SECRET_NEW
# 2. Keep old key as GENIE_JWT_SECRET (for validation)
# 3. All new tokens signed with new key
# 4. Old tokens validated against both (old expires naturally)
# 5. Once all old tokens expired, remove GENIE_JWT_SECRET_OLD

# Recommended frequency: quarterly
```

---

## Health Checks & Monitoring

### Liveness Probe

**Endpoint**: `GET /healthz`

**Response** (healthy):
```json
{
  "status": "ok",
  "timestamp": "2026-06-04T15:30:45Z"
}
```

**Response** (unhealthy):
```json
{
  "status": "error",
  "message": "database connection lost"
}
```

**Interpretation**: If this fails, restart the container.

### Readiness Probe

**Endpoint**: `GET /readyz`

**Response** (ready):
```json
{
  "status": "ready",
  "checks": {
    "database": "ok",
    "llm_stack": "ok",
    "migrations": "ok"
  }
}
```

**Response** (not ready):
```json
{
  "status": "not_ready",
  "checks": {
    "database": "error: connection timeout",
    "llm_stack": "ok"
  }
}
```

**Interpretation**: Don't send traffic to this instance until ready. It's safe to wait.

### Metrics (Prometheus)

**Scrape endpoint**: `GET /metrics` (port 8081, separate from main HTTP)

**Key metrics**:
```
# Requests
http_requests_total{method="POST",path="/v1/orders",status="200"} 1234
http_request_duration_seconds{method="POST",quantile="0.99"} 0.125

# Errors
http_requests_total{method="POST",path="/v1/orders",status="500"} 12
genie_csrf_validation_failures_total 5
genie_session_creation_failures_total 2

# Database
genie_db_connections_active 15
genie_db_query_duration_seconds{query="select_user",quantile="0.95"} 0.050

# LLM
genie_llm_calls_total{agent="analyzer",status="success"} 456
genie_llm_latency_seconds{agent="analyzer",quantile="0.99"} 2.5
```

### Alerting Rules (Prometheus AlertManager)

```yaml
groups:
- name: genie-alerts
  interval: 30s
  rules:

  - alert: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
    for: 5m
    annotations:
      summary: "High error rate (>5%) for 5 minutes"

  - alert: CSRFValidationFailures
    expr: rate(genie_csrf_validation_failures_total[5m]) > 1
    for: 1m
    annotations:
      summary: "CSRF validation failures detected (potential attack)"

  - alert: DatabaseConnectionPoolExhausted
    expr: genie_db_connections_active >= 19
    for: 2m
    annotations:
      summary: "Database connection pool near capacity"

  - alert: LLMStackUnhealthy
    expr: genie_llm_health{status="error"} > 0
    for: 5m
    annotations:
      summary: "LLM stack offline"
```

### Log Aggregation

**ELK Stack** (Elasticsearch, Logstash, Kibana):

```json
{
  "timestamp": "2026-06-04T15:30:45Z",
  "level": "error",
  "service": "genie-api",
  "message": "payment processing failed",
  "trace_id": "abc123xyz",
  "user_id": "user-123",
  "error": "insufficient balance",
  "request_path": "/v1/payment/initiate",
  "request_method": "POST"
}
```

**Query in Kibana**:
```
level:error AND service:genie-api AND timestamp:[now-1h TO now]
```

---

## Migration Path

### From Phase 1 to Phase 2 (Zero-downtime)

**Step 1**: Deploy with both auth schemes enabled

```go
// pkg/web/handlers/routes.go
r.Use(mid.SessionAndBearerMiddleware(sm, issuer))  // Accept both
```

**Step 2**: Keep old JWT endpoint available

```go
// /v1/users/login — returns both JWT + session cookie
```

**Step 3**: Migrate clients gradually

```javascript
// Old clients: Authorization: Bearer <jwt>
// New clients: Send credentials: 'include' (uses cookie)
// Dual support: Try both (cookie first, fall back to bearer)
```

**Step 4**: Monitor compatibility

```bash
# Check both auth methods in use
curl -s http://localhost:8080/metrics | grep "http_auth_method"
# Result: http_auth_method{method="bearer"} 2000
#         http_auth_method{method="session_cookie"} 1500
```

**Step 5**: Once <5% bearer usage, deprecate old method

```go
// v1.1: Remove bearer fallback
// Clients must migrate to session cookies
```

---

## Troubleshooting

### Application won't start

**Error**: `failed to connect to database`

**Solution**:
1. Verify DSN in GENIE_DB_DSN
2. Check PostgreSQL is running: `psql -U genie -d genie -c "SELECT 1"`
3. Verify network (firewall, security groups): `telnet host 5432`
4. Check credentials: `psql -U genie -d genie -W` (should prompt for password)

**Error**: `JWT secret is required`

**Solution**:
1. Set GENIE_JWT_SECRET: `export GENIE_JWT_SECRET="..."`
2. Secret must be >=32 bytes (hex: >=64 chars)
3. Verify: `echo ${GENIE_JWT_SECRET} | wc -c` (should be 65+ for 32-byte hex)

**Error**: `failed to load LLM config`

**Solution**:
1. Check CLAUDE_API_KEY is set and valid
2. Verify internet connectivity: `curl https://api.anthropic.com/v1/health`
3. Check API key format: `echo ${CLAUDE_API_KEY} | head -c 10}` (should start with "sk-ant-")

### High latency

**Symptoms**: Response times >500ms, TTFB slow

**Debug steps**:
```bash
# 1. Check database latency
psql -U genie -d genie -c "SELECT current_timestamp; SELECT current_timestamp;" \
  | time psql -U genie -d genie

# 2. Profile CPU
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 3. Check connection pool exhaustion
curl -s http://localhost:8081/metrics | grep db_connections_active
```

**Solutions**:
- Increase GENIE_DB_POOL_SIZE
- Add database read replicas
- Cache frequently-accessed data (Redis)

### CSRF token validation failing

**Error**: `403 Forbidden: csrf token required`

**Cause**: Missing or expired token

**Fix**:
1. Client must call POST /v1/users/login first
2. Extract X-CSRF-Token from response header
3. Include in all subsequent POST/PUT/DELETE requests
4. If token expires (>15 min), login again

**Debug**:
```bash
# Test CSRF flow
curl -i -X POST http://localhost:8080/v1/users/login \
  -d '{"email":"test@example.com","password":"password"}' \
  | grep -A 5 "X-CSRF-Token"
```

### Session cookie not persisting

**Symptoms**: Logged in, but next request returns 401

**Cause**: credentials: 'include' not set in fetch

**Fix** (JavaScript):
```javascript
// BEFORE (incorrect)
fetch('/v1/orders', {
  headers: { 'Authorization': 'Bearer ' + token }
})

// AFTER (correct)
fetch('/v1/orders', {
  credentials: 'include'  // REQUIRED for cookies
})
```

### Database backups failing

**Error**: `pg_dump: command not found` or `permission denied`

**Solution**:
```bash
# 1. Install PostgreSQL client tools
apt-get install postgresql-client

# 2. Set correct permissions
chmod 755 /backups/

# 3. Run as postgres user
sudo -u postgres pg_dump -d genie > /backups/genie.dump
```

---

## Rollback Procedure

### Scenario: Critical bug discovered post-deployment

**Step 1**: Stop new version (5 min)

```bash
# Kubernetes
kubectl rollout undo deployment/genie-api -n genie

# Docker Compose
docker-compose down genie-api
docker-compose up -d genie-api:old-tag
```

**Step 2**: Verify health (2 min)

```bash
curl -f http://localhost:8080/healthz
curl -f http://localhost:8080/readyz
```

**Step 3**: Monitor errors (5 min)

```bash
# Check logs for errors
kubectl logs -n genie -l app=genie-api --tail=100 | grep ERROR
```

**Step 4**: Assess data impact

```bash
# Query database for inconsistent state
psql -U genie -d genie -c "
  SELECT COUNT(*) FROM orders WHERE status IS NULL;
  SELECT COUNT(*) FROM transactions WHERE settled_at IS NULL AND created_at < now() - interval '30 min';
"
```

**Step 5**: Notify stakeholders

- Incident response team
- RBI compliance officer
- Customer support (if user-facing)

**Step 6**: Root cause analysis

- Review application logs (full trace)
- Check database state
- Review code changes
- Document in post-mortem

### Scenario: Database corruption

**Step 1**: Stop application

```bash
docker-compose down genie-api
```

**Step 2**: Restore latest backup

```bash
# List backups
ls -lh /backups/genie-*.dump

# Restore
pg_restore -c -d genie /backups/genie-20260604.dump
```

**Step 3**: Verify data integrity

```bash
psql -U genie -d genie -c "
  SELECT COUNT(*) as total_orders FROM orders;
  SELECT SUM(amount_paise) as total_transactions FROM transactions;
  -- Compare against pre-corruption metrics
"
```

**Step 4**: Resume application

```bash
docker-compose up -d genie-api
```

**Step 5**: Data loss assessment

If <1 hour of data lost:
- Acceptable (per RBI guidelines for e-Rupee <Sutra 7>)
- Notify affected users (if any)

If >1 hour of data lost:
- Critical incident
- Escalate to RBI immediately
- Suspend payment processing until resolved

---

## Post-Deployment Verification

```bash
#!/bin/bash

# 1. Health checks
echo "=== Health Checks ==="
curl -f http://localhost:8080/healthz || exit 1
curl -f http://localhost:8080/readyz || exit 1

# 2. Auth flow
echo "=== Auth Flow ==="
RESPONSE=$(curl -s -X POST http://localhost:8080/v1/users/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"password"}')
TOKEN=$(echo $RESPONSE | jq -r '.token')
CSRF=$(curl -s -I http://localhost:8080/v1/users/me \
  -H "Authorization: Bearer $TOKEN" | grep X-CSRF-Token | cut -d' ' -f2)

# 3. CSRF protection
echo "=== CSRF Protection ==="
curl -s -X POST http://localhost:8080/v1/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{}' \
  | grep -q "csrf token required" && echo "PASS" || echo "FAIL"

# 4. Security headers
echo "=== Security Headers ==="
curl -I http://localhost:8080/v1/orders \
  | grep -q "Content-Security-Policy" && echo "PASS" || echo "FAIL"

# 5. Database connectivity
echo "=== Database Connectivity ==="
psql -U genie -d genie -c "SELECT 1" && echo "PASS" || echo "FAIL"

# 6. Metrics
echo "=== Metrics Available ==="
curl -f http://localhost:8081/metrics | grep -q "http_requests_total" && echo "PASS" || echo "FAIL"

echo "=== All Checks Complete ==="
```

---

## Support

For deployment issues, contact:
- **Architecture**: Pratik Dhanave (i.pratikdhanave@gmail.com)
- **Operations**: ops@genie.io
- **RBI Liaison**: compliance@genie.io

---

**Document Version**: 1.0  
**Last Updated**: June 4, 2026  
**Author**: Claude Haiku 4.5  
**License**: MIT
