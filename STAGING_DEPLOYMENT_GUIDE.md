# Genie Phase 1 Staging Deployment Guide

**Target Audience**: DevOps engineers deploying to staging  
**Environment**: Staging (pre-production)  
**Phase**: Phase 1 — UI improvements + e-Rupee backend  
**Duration**: ~30 minutes (first deploy) + 10 minutes (subsequent deploys)

---

## Table of Contents

1. [Pre-Deployment Checklist](#1-pre-deployment-checklist)
2. [Deployment Steps](#2-deployment-steps)
3. [Staging Environment Setup](#3-staging-environment-setup)
4. [Post-Deployment Validation](#4-post-deployment-validation)
5. [Monitoring](#5-monitoring)
6. [Troubleshooting](#6-troubleshooting)

---

## 1. Pre-Deployment Checklist

### 1.1 Code and Build Prerequisites

Before deploying, verify that the code is ready:

```bash
# Navigate to the project directory
cd /path/to/genie

# Ensure you're on the main branch (or approved release branch)
git branch -v

# Pull latest code
git pull origin main

# Verify Go version (1.25.0 required)
go version

# Ensure all dependencies are available
go mod download

# Run build sanity check (no actual build artifacts needed yet)
go build -v ./...
```

**Checklist items:**
- [ ] Code on correct branch (`main` or approved release tag)
- [ ] Go 1.25.0+ installed
- [ ] `go mod download` succeeds without errors
- [ ] All packages compile without errors

### 1.2 Backend Service Health Checks

Verify that backend services are reachable before deploying:

```bash
# Check PostgreSQL connectivity (if not using Docker Compose)
psql -h <db-host> -U genie -d genie -c "SELECT version();"

# Verify Ollama is accessible (optional; fallback to mock is available)
curl -sf http://localhost:11434/api/tags | jq '.models[].name'

# Check DNS resolution for all external services
nslookup postgres.staging.internal
nslookup ollama.staging.internal

# Verify firewall rules allow required ports
# Genie API: 8080, Metrics: 9464, DB: 5432, Ollama: 11434, OTLP: 4317
netstat -tln | grep -E "(8080|9464|5432|11434|4317)"
```

**Checklist items:**
- [ ] PostgreSQL is accessible and healthy
- [ ] Database `genie` exists with user `genie`
- [ ] Ollama is running (optional; mock LLM available as fallback)
- [ ] All required ports are open
- [ ] Network connectivity to all backends verified

### 1.3 Database Readiness

Ensure the database is prepared:

```bash
# Connect to the database
psql -h <db-host> -U genie -d genie

# Inside psql:
-- Check if pgvector extension is installed (required for RAG)
CREATE EXTENSION IF NOT EXISTS vector;

-- Verify schema exists (migrations auto-run at startup)
\dt  -- List all tables

-- Check for existing migrations
SELECT version, success FROM schema_migrations ORDER BY installed_on DESC LIMIT 5;

-- Exit
\q
```

**Database requirements:**
- PostgreSQL 14+ with `pgvector` extension
- User `genie` with full database privileges
- Empty database (migrations auto-run on first API startup)
- Sufficient disk space: minimum 10 GB (trace + metrics storage)

**Checklist items:**
- [ ] PostgreSQL version 14+
- [ ] pgvector extension installed
- [ ] User `genie` exists with password set
- [ ] Database `genie` exists and is empty (or running migrations is acceptable)
- [ ] Disk space: 10 GB minimum available

### 1.4 Environment Variables

Prepare all required environment variables. Create a `.env.staging` file:

```bash
# ── Required Variables ──────────────────────────────────────────────────
# JWT secret for token signing (min 32 chars, can be hex or base64)
GENIE_JWT_SECRET=your-staging-jwt-secret-32-chars-min-change-me

# Key Encryption Key for data at rest (generate with: openssl rand -base64 32)
GENIE_KEK_BASE64=K7m9OQM63z7ZK3xQ2lG0qV9hC5Yf+e0sQ+kXcM3GqQE=

# PostgreSQL connection string
GENIE_DB_DSN=postgres://genie:password@db-staging.internal:5432/genie?sslmode=require

# HTTP server bind address (0.0.0.0:8080 to expose to load balancer)
GENIE_HTTP_ADDR=0.0.0.0:8080

# ── Optional: LLM Configuration ────────────────────────────────────────
# Options: "ollama" (local), "mock" (fallback), or "anthropic" (cloud)
GENIE_LLM=ollama
GENIE_OLLAMA_URL=http://ollama-staging.internal:11434
GENIE_OLLAMA_CHAT=llama3.2:1b
GENIE_OLLAMA_EMBED=nomic-embed-text

# ── Optional: Observability ────────────────────────────────────────────
# OpenTelemetry OTLP endpoint (for traces + metrics)
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector-staging.internal:4317
GENIE_OTEL_INSECURE=true

# Metrics port (exposed to Prometheus scraper)
GENIE_METRICS_ADDR=0.0.0.0:9464

# ── Optional: AI Governance ────────────────────────────────────────────
# Path to AI policy YAML (RBI FREE-AI alignment)
GENIE_AI_POLICY=/config/ai-policy.example.yaml

# ── Optional: Laminar Eval Tracing ────────────────────────────────────
# LMNR_PROJECT_API_KEY=your-laminar-key-here
# LMNR_BASE_URL=http://laminar-staging.internal:8000

# ── Optional: Timeouts (in seconds) ────────────────────────────────────
GENIE_ASK_TIMEOUT=60
GENIE_STREAM_TIMEOUT=90
GENIE_CHATWS_TIMEOUT=120
```

**Checklist items:**
- [ ] `GENIE_JWT_SECRET` set (32+ chars, unique per environment)
- [ ] `GENIE_KEK_BASE64` set (base64-encoded 32-byte key)
- [ ] `GENIE_DB_DSN` points to staging PostgreSQL with `sslmode=require`
- [ ] `GENIE_HTTP_ADDR` set to bind on load balancer-accessible IP
- [ ] All optional vars reviewed and set appropriately

### 1.5 TLS/Certificates

If deploying behind a reverse proxy:

```bash
# Verify TLS certificates exist and are valid
openssl x509 -in /etc/ssl/certs/genie-staging.crt -text -noout

# Check certificate expiry
openssl x509 -in /etc/ssl/certs/genie-staging.crt -noout -enddate

# Verify private key matches certificate
openssl x509 -noout -modulus -in /etc/ssl/certs/genie-staging.crt | openssl md5
openssl rsa -noout -modulus -in /etc/ssl/private/genie-staging.key | openssl md5
```

**Checklist items:**
- [ ] TLS certificates obtained from CA (or self-signed for staging)
- [ ] Certificates not expired (valid for at least 30 days)
- [ ] Private key and certificate match
- [ ] Reverse proxy configured to forward to `genie-api:8080`

---

## 2. Deployment Steps

### 2.1 Build the Binary

Build the API binary locally or in CI/CD pipeline:

```bash
# Navigate to project root
cd /path/to/genie

# Build the binary (native target)
go build -o bin/genie-api ./cmd/api

# Verify binary exists and is executable
file bin/genie-api
ls -lh bin/genie-api

# (Optional) Test binary against mock LLM (no Ollama needed)
GENIE_LLM=mock GENIE_JWT_SECRET=test GENIE_KEK_BASE64=K7m9OQM63z7ZK3xQ2lG0qV9hC5Yf+e0sQ+kXcM3GqQE= \
  GENIE_DB_DSN=postgres://postgres@localhost/test ./bin/genie-api &
sleep 2
curl -sf http://localhost:8080/healthz && echo "Binary OK" || echo "Binary FAILED"
kill $!
```

### 2.2 Build Docker Image (Recommended for Staging)

For container deployments:

```bash
# Build the Docker image
docker build -t genie-api:staging -t genie-api:latest .

# Verify image was created
docker images | grep genie-api

# (Optional) Scan image for vulnerabilities
trivy image genie-api:staging

# Push to registry (if using private registry)
docker tag genie-api:staging registry.staging.internal/genie/genie-api:staging
docker push registry.staging.internal/genie/genie-api:staging
```

### 2.3 Deploy Using Docker Compose (Minimal Stack)

For staging environment with full observability:

```bash
# Create staging-specific docker-compose file
cat > docker-compose.staging.yaml <<'EOF'
version: '3.9'

services:
  postgres:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_USER: genie
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: genie
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U genie -d genie"]
      interval: 5s
      timeout: 3s
      retries: 10
    volumes:
      - postgres-staging-data:/var/lib/postgresql/data

  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.106.0
    command: ["--config=/etc/otel-collector.yaml"]
    volumes:
      - ./deploy/local/otel-collector.yaml:/etc/otel-collector.yaml
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
      - "8889:8889"   # Prometheus metrics
    depends_on:
      - prometheus

  prometheus:
    image: prom/prometheus:v2.53.0
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    volumes:
      - ./deploy/local/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-staging-data:/prometheus
    ports:
      - "9090:9090"

  genie-api:
    image: genie-api:staging
    environment:
      GENIE_HTTP_ADDR: "0.0.0.0:8080"
      GENIE_METRICS_ADDR: "0.0.0.0:9464"
      GENIE_JWT_SECRET: ${GENIE_JWT_SECRET}
      GENIE_KEK_BASE64: ${GENIE_KEK_BASE64}
      GENIE_DB_DSN: "postgres://genie:${DB_PASSWORD}@postgres:5432/genie?sslmode=disable"
      OTEL_EXPORTER_OTLP_ENDPOINT: "otel-collector:4317"
      GENIE_OTEL_INSECURE: "true"
      GENIE_LLM: "${GENIE_LLM:-mock}"
      GENIE_OLLAMA_URL: "http://ollama:11434"
      GENIE_OLLAMA_CHAT: "llama3.2:1b"
    ports:
      - "8080:8080"
      - "9464:9464"
    depends_on:
      postgres:
        condition: service_healthy
      otel-collector:
        condition: service_started
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/healthz"]
      interval: 10s
      timeout: 5s
      retries: 3

  ollama:
    image: ollama/ollama:0.4.0
    ports:
      - "11434:11434"
    volumes:
      - ollama-staging-data:/root/.ollama
    healthcheck:
      test: ["CMD-SHELL", "ollama list >/dev/null 2>&1 || exit 1"]
      interval: 10s
      timeout: 5s
      retries: 12

volumes:
  postgres-staging-data:
  prometheus-staging-data:
  ollama-staging-data:
EOF

# Start the stack
export GENIE_JWT_SECRET="your-staging-jwt-secret-32-chars-min"
export GENIE_KEK_BASE64="K7m9OQM63z7ZK3xQ2lG0qV9hC5Yf+e0sQ+kXcM3GqQE="
export DB_PASSWORD="staging-db-password-change-me"
export GENIE_LLM="mock"  # Use mock LLM for fast startup, switch to ollama later

docker compose -f docker-compose.staging.yaml up -d

# Monitor startup
docker compose -f docker-compose.staging.yaml logs -f genie-api
```

### 2.4 Deploy Using Kubernetes (Optional)

If deploying to Kubernetes:

```bash
# Create Kubernetes namespace
kubectl create namespace genie-staging

# Create secrets
kubectl create secret generic genie-secrets \
  --from-literal=jwt-secret="your-staging-jwt-secret" \
  --from-literal=kek-base64="K7m9OQM63z7ZK3xQ2lG0qV9hC5Yf+e0sQ+kXcM3GqQE=" \
  --from-literal=db-password="staging-password" \
  -n genie-staging

# Create ConfigMap for environment
kubectl create configmap genie-config \
  --from-literal=GENIE_LLM=mock \
  --from-literal=GENIE_DB_DSN="postgres://genie:password@postgres.genie-staging:5432/genie?sslmode=require" \
  --from-literal=OTEL_EXPORTER_OTLP_ENDPOINT="otel-collector.genie-staging:4317" \
  -n genie-staging

# Apply Kubernetes manifests (example: StatefulSet for genie-api)
cat > genie-api-k8s.yaml <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: genie-api
  namespace: genie-staging
spec:
  replicas: 2
  selector:
    matchLabels:
      app: genie-api
  template:
    metadata:
      labels:
        app: genie-api
    spec:
      containers:
      - name: genie-api
        image: registry.staging.internal/genie/genie-api:staging
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9464
          name: metrics
        env:
        - name: GENIE_HTTP_ADDR
          value: "0.0.0.0:8080"
        - name: GENIE_METRICS_ADDR
          value: "0.0.0.0:9464"
        - name: GENIE_JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: genie-secrets
              key: jwt-secret
        - name: GENIE_KEK_BASE64
          valueFrom:
            secretKeyRef:
              name: genie-secrets
              key: kek-base64
        - name: GENIE_DB_DSN
          valueFrom:
            configMapKeyRef:
              name: genie-config
              key: GENIE_DB_DSN
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
---
apiVersion: v1
kind: Service
metadata:
  name: genie-api
  namespace: genie-staging
spec:
  selector:
    app: genie-api
  ports:
  - name: http
    port: 80
    targetPort: 8080
  - name: metrics
    port: 9464
    targetPort: 9464
  type: LoadBalancer
EOF

kubectl apply -f genie-api-k8s.yaml

# Wait for deployment to be ready
kubectl rollout status deployment/genie-api -n genie-staging --timeout=5m

# Verify deployment
kubectl get pods -n genie-staging
kubectl describe svc genie-api -n genie-staging
```

### 2.5 Initialize Database (If First Deploy)

On the first deployment, migrations auto-run, but verify they succeed:

```bash
# View logs for migration status
docker compose -f docker-compose.staging.yaml logs genie-api | grep -i migration

# Or for Kubernetes:
kubectl logs -f deployment/genie-api -n genie-staging | grep -i migration

# Manual fallback: Connect to database and check schema
docker exec -it genie_postgres_1 psql -U genie -d genie -c "\dt"

# Expected tables:
# - users
# - accounts
# - documents
# - incidents
# - mcp_tokens
# (plus others from migrations)
```

---

## 3. Staging Environment Setup

### 3.1 Docker Compose Configuration (Full Stack)

Use the provided `docker-compose.yaml` for a complete local staging environment:

```bash
# Start full stack with all observability
make up

# Wait for readiness (shown by `make up` output)
# Expected output:
# Genie API:      http://localhost:8080/
# Health:         http://localhost:8080/healthz
# Metrics:        http://localhost:9464/metrics
# Prometheus:     http://localhost:9090/
# Grafana:        http://localhost:3000/ (admin/admin)
```

The default stack includes:
- **genie-api** — Main service on port 8080
- **postgres** — Database on port 5432
- **ollama** — Local LLM on port 11434
- **otel-collector** — Telemetry collection on port 4317
- **prometheus** — Metrics backend on port 9090
- **grafana** — Visualization on port 3000

### 3.2 Environment Variables Template

Copy and customize for your staging environment:

```bash
# Create staging environment file
cp .env.example .env.staging

# Edit with staging values
cat .env.staging
```

**Critical variables for staging:**

| Variable | Staging Value | Notes |
|----------|---------------|-------|
| `GENIE_JWT_SECRET` | `staging-xyz-32-chars` | Unique to staging, min 32 chars |
| `GENIE_KEK_BASE64` | `K7m9OQM63z7Z...` | Generate: `openssl rand -base64 32` |
| `GENIE_DB_DSN` | `postgres://genie:pass@postgres:5432/genie` | Use `sslmode=disable` for Docker, `require` for remote |
| `GENIE_LLM` | `mock` or `ollama` | Start with mock for faster iteration |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://otel-collector:4317` | For tracing (optional in early staging) |

### 3.3 Database Migration Steps

Migrations auto-run on API startup, but you can pre-migrate if needed:

```bash
# Option 1: Let API auto-migrate (recommended)
# Migrations run on first API startup; check logs for success.

# Option 2: Manual migration (if needed)
# Ensure genie-api service is NOT running, then:
docker run --rm \
  -e GENIE_DB_DSN="postgres://genie:password@postgres:5432/genie" \
  genie-api:staging \
  /genie-api  # Just connecting to run migrations

# Option 3: Direct SQL (staging only, not recommended)
docker exec genie-postgres psql -U genie -d genie -f /migrations/001_initial.sql
```

**Verify migrations:**

```bash
# Connect to database
docker exec -it genie-postgres psql -U genie -d genie

# Inside psql:
\dt  -- List tables (should include: users, accounts, documents, incidents, mcp_tokens)
SELECT version FROM schema_migrations ORDER BY installed_on DESC LIMIT 5;
\q
```

### 3.4 Test Data Setup

(Optional) Load minimal test data for staging:

```bash
# Create test user via API
curl -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "test-staging@genie.local",
    "name": "Staging Tester",
    "password": "test-password-123"
  }' | jq .

# Create test merchant (e-Rupee Commerce)
curl -X POST http://localhost:8080/v1/merchants \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Test Merchant",
    "category": "retail",
    "settlement_account": "123456789",
    "email": "merchant@staging.local"
  }' | jq .

# Create test account
curl -X POST http://localhost:8080/v1/accounts \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Staging Checking",
    "currency": "INR"
  }' | jq .
```

---

## 4. Post-Deployment Validation

### 4.1 Health Check Endpoints

```bash
# Liveness check (is API running?)
curl -v http://localhost:8080/healthz
# Expected: 200 OK

# Readiness check (is API ready to serve requests?)
curl -v http://localhost:8080/readyz
# Expected: 200 OK (after DB + LLM probe succeeds)

# Metrics endpoint (Prometheus scrape target)
curl -s http://localhost:9464/metrics | head -30
# Expected: prometheus metrics (genie_* prefix)

# Governance health (agents operational?)
curl -s http://localhost:8080/v1/inventory | jq '.agents | length'
# Expected: 35+ agents registered
```

### 4.2 UI Validation in Browser

Open http://localhost:8080 in a browser and verify:

#### Frontend Loads
- [ ] Page loads without console errors
- [ ] Genie logo visible in header
- [ ] Navigation menu accessible
- [ ] No 404 errors in Network tab

#### ARIA Labels (Accessibility)
Right-click → Inspect Element, then check:

```html
<!-- Input fields should have accessible labels -->
<input aria-label="Email address" type="email" />
<button aria-label="Sign up">Sign Up</button>

<!-- Headings should be semantic -->
<h1>Genie Financial Assistant</h1>
<h2>Dashboard</h2>

<!-- Images should have alt text -->
<img src="..." alt="Merchant settlement chart" />
```

**Check in DevTools:**
1. Open DevTools (F12)
2. Press Ctrl+Shift+U (or Cmd+Shift+U on Mac) to open Lighthouse
3. Run Accessibility audit
4. Verify no Critical issues

#### Keyboard Navigation
- [ ] Tab through all form fields (email, password, buttons)
- [ ] Submit button responds to Enter key
- [ ] Menu items accessible via keyboard
- [ ] Focus visible at all times (blue outline)

### 4.3 Backend e2e Tests

Run the commerce workflow tests to validate backend functionality:

```bash
# Run Phase 1 integration tests (e-Rupee Commerce)
go test -v ./pkg/commerce/... -run TestE2E

# Expected output:
# TestE2E_OrderToSettlement_HappyPath ... PASS
# TestE2E_ComplianceBlocks_VelocityExceeded ... PASS
# TestE2E_SettlementBatching_MultipleOrders ... PASS
# TestE2E_AuditTrail_FullLineage ... PASS
# TestE2E_Reconciliation_VerifySettlementIntegrity ... PASS
```

Run payment handler tests:

```bash
go test -v ./pkg/web/handlers -run TestPayment

# Expected output:
# TestPaymentHandler_CreatePayment ... PASS
# TestPaymentHandler_GetPaymentStatus ... PASS
# TestPaymentHandler_GetTransactionHistory ... PASS
```

Run commerce handler tests:

```bash
go test -v ./pkg/web/handlers -run TestCommerce

# Expected output:
# TestCreateOrder ... PASS
# TestCreateOrderWithInvalidData ... PASS
# TestGetOrder ... PASS
# TestExecuteWorkflow ... PASS
```

### 4.4 API Endpoint Testing

Test key e-Rupee Commerce endpoints:

#### 1. User Signup & Authentication

```bash
# Signup
TOKEN_RESPONSE=$(curl -s -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "test@staging.local",
    "name": "Test User",
    "password": "Test@1234"
  }')

TOKEN=$(echo $TOKEN_RESPONSE | jq -r '.token')
echo "Token: $TOKEN"

# Verify signup
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/users/me | jq .
```

**Expected Response:**
```json
{
  "id": "user_xxx",
  "email": "test@staging.local",
  "name": "Test User",
  "created_at": "2026-06-01T00:00:00Z"
}
```

#### 2. Payment Flow (e-Rupee)

```bash
# Create payment account
ACCT=$(curl -s -X POST http://localhost:8080/v1/accounts \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "e-Rupee Wallet",
    "currency": "INR"
  }')

ACCOUNT_ID=$(echo $ACCT | jq -r '.id')
echo "Account ID: $ACCOUNT_ID"

# Initiate payment (correct endpoint: /v1/payment/initiate)
PAYMENT=$(curl -s -X POST http://localhost:8080/v1/payment/initiate \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
    \"from_account\": \"$ACCOUNT_ID\",
    \"to_merchant\": \"merchant_xyz\",
    \"amount_paise\": 100000,
    \"type\": \"retail\"
  }")

PAYMENT_ID=$(echo $PAYMENT | jq -r '.id')
echo "Payment ID: $PAYMENT_ID"

# Check payment status (correct endpoint: /v1/payment/{payment_id})
curl -s -X GET "http://localhost:8080/v1/payment/$PAYMENT_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Expected Response:**
```json
{
  "id": "payment_xxx",
  "status": "pending|confirmed|failed",
  "amount_paise": 100000,
  "from_account": "account_xxx",
  "to_merchant": "merchant_xyz",
  "created_at": "2026-06-01T00:00:00Z"
}
```

#### 3. Commerce Order Workflow

```bash
# Create order (correct endpoint: /v1/commerce/order)
ORDER=$(curl -s -X POST http://localhost:8080/v1/commerce/order \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "merchant_id": "merchant_001",
    "customer_id": "customer_001",
    "items": [
      {
        "sku": "ITEM-001",
        "quantity": 2,
        "unit_price_paise": 50000
      }
    ]
  }')

ORDER_ID=$(echo $ORDER | jq -r '.id')
echo "Order ID: $ORDER_ID"

# Execute workflow (Order → Payment → Settlement → Fulfillment)
# Correct endpoint: /v1/commerce/order/{order_id}/execute
curl -s -X POST "http://localhost:8080/v1/commerce/order/$ORDER_ID/execute" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Get order status (correct endpoint: /v1/commerce/order/{order_id})
curl -s -X GET "http://localhost:8080/v1/commerce/order/$ORDER_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Expected Status Flow:**
```
pending → paid → fulfilled
(internal audit trail tracks: order_created → payment_initiated → payment_confirmed → settlement_initiated → settlement_completed → fulfilled)
```

#### 4. Merchant Onboarding (e-Rupee)

```bash
# Onboard merchant (correct endpoint: /v1/merchant/onboard)
MERCHANT=$(curl -s -X POST http://localhost:8080/v1/merchant/onboard \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Test Merchant",
    "category": "retail",
    "settlement_account": "0000000000000001",
    "email": "merchant@test.local"
  }')

MERCHANT_ID=$(echo $MERCHANT | jq -r '.id')
echo "Merchant ID: $MERCHANT_ID"

# Get merchant details (correct endpoint: /v1/merchant/{merchant_id})
curl -s -X GET "http://localhost:8080/v1/merchant/$MERCHANT_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### 5. CBDC Settlement

```bash
# Initiate CBDC transaction
TXNS=$(curl -s -X POST "http://localhost:8080/v1/cbdc/transaction" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "from_account": "account_merchant_001",
    "to_account": "account_customer_001",
    "amount_paise": 100000
  }')

TXN_ID=$(echo $TXNS | jq -r '.id')
echo "Transaction ID: $TXN_ID"

# Get CBDC block (ledger commitment)
curl -s -X GET "http://localhost:8080/v1/cbdc/block/1" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Get account limits (velocity/compliance checks)
curl -s -X GET "http://localhost:8080/v1/cbdc/limits/account_merchant_001" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Check CBDC ledger health
curl -s -X GET "http://localhost:8080/v1/cbdc/health" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### 6. Payment Compliance Check

```bash
# Check velocity compliance
curl -s -X POST "http://localhost:8080/v1/compliance/check" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "customer_id": "customer_001",
    "transaction_amount_paise": 500000,
    "transaction_type": "payment"
  }' | jq .

# Expected response:
# { "compliant": true, "reason": "within velocity limits" }
# or
# { "compliant": false, "reason": "velocity exceeded", "limit": 1000000 }
```

### 4.5 Smoke Test (All-in-One)

Run the built-in smoke test (no token management needed):

```bash
# From project root:
make smoke

# Expected output:
# → http://localhost:8080/healthz
#   ok
# → http://localhost:8080/readyz
#   ok
# → POST /v1/users (smoke-1234567890@genie.local)
#   ok (token len=...)
# → GET /v1/users/me
#   ok
# → POST /v1/accounts
#   ok
# → POST /v1/documents (tiny CSV)
#   ok (doc=...)
# → POST /v1/ask
#   ok
# → GET /v1/disclosures
#   ok
#
# smoke: PASS
```

---

## 5. Monitoring

### 5.1 What to Watch in Logs

After deployment, monitor for these key indicators:

#### API Startup Logs

```bash
# Docker Compose logs
docker compose logs -f genie-api

# Kubernetes logs
kubectl logs -f deployment/genie-api -n genie-staging
```

**Look for:**

```
listening addr=:8080                           # API started
database connected                              # DB migration success
agents registered: 35                           # Governance bundle loaded
opa engine initialized                          # Policy engine ready
opentelemetry exporter: otlp                   # Telemetry configured
```

**Watch for errors:**

```
ERROR missing required env                      # Missing GENIE_JWT_SECRET or GENIE_KEK_BASE64
ERROR database ping failed                      # DB connectivity issue
ERROR opa engine init                           # Policy engine failed (degraded, API still runs)
ERROR ui.embed                                  # UI not embedded (API still works, frontend unavailable)
WARN llm provider unavailable                   # Ollama down, falling back to mock
```

#### Request Logs

```bash
# Sample request logs (sparse by default, use DEBUG if needed)
docker compose logs genie-api | grep -E "POST|GET|DELETE|request"
```

**Expected patterns:**

```
POST /v1/users                                  # User signup
GET  /v1/users/me                               # Auth test
POST /v1/accounts                               # Account creation
POST /v1/orders                                 # Order creation
POST /v1/payments                               # Payment processing
```

### 5.2 Performance Baselines

Establish performance baselines for comparison:

```bash
# Load test: 10 concurrent users, 30 seconds
ab -n 300 -c 10 -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/users/me

# Expected baseline (mock LLM):
# Requests per second: 50+
# Mean response time: 20ms
# 99th percentile: 100ms
# Error rate: <1%
```

**Metric endpoints:**

```bash
# Prometheus metrics
curl -s http://localhost:9464/metrics | grep -E "genie_|http_"

# Grafana dashboards (if enabled)
# http://localhost:3000/d/genie-api-dashboard
```

**Key metrics to monitor:**

| Metric | Baseline | Alert Threshold |
|--------|----------|-----------------|
| `genie_request_duration_ms` | 50ms (p95) | >500ms |
| `genie_db_connection_pool_size` | 10 | 0 (pool exhausted) |
| `genie_agent_execution_duration_ms` | 100ms (p95) | >5000ms |
| `genie_payment_success_rate` | 99%+ | <95% |
| `genie_order_settlement_latency_ms` | 500ms (p95) | >5000ms |

### 5.3 Common Issues and Fixes

#### Issue: `readyz` returns 500 (not ready)

```bash
# Check database connectivity
curl -i http://localhost:8080/readyz

# Troubleshoot:
docker compose exec postgres pg_isready -U genie -d genie
docker compose logs postgres | grep -i error

# If DB is down:
docker compose restart postgres
```

#### Issue: LLM errors (Ollama not ready)

```bash
# Check Ollama status
curl -s http://localhost:11434/api/tags

# If Ollama is down:
docker compose restart ollama
# Wait for ollama-pull service to complete (auto-pulls models)
sleep 60

# Verify model loaded
curl -s http://localhost:11434/api/tags | jq '.models[].name'
```

#### Issue: Payment tests fail with "account not found"

```bash
# Ensure database migrations ran
docker compose logs genie-api | grep -i migration

# If migrations stuck:
docker compose down -v
docker compose up --build
```

#### Issue: High memory usage on genie-api

```bash
# Check memory limits
docker stats genie-api

# If exceeding limits:
# 1. Check for memory leaks: kubectl top pod deployment/genie-api -n genie-staging
# 2. Review trace buffer size (OTEL settings)
# 3. Restart container: docker compose restart genie-api
```

#### Issue: 503 Service Unavailable / Rate Limit Hit

```bash
# Check if rate limiter is engaged
docker compose logs genie-api | grep "rate limit"

# Default: 60-request burst, 1 request/sec sustained
# Wait 1 minute for burst reset, or:
docker compose restart genie-api
```

---

## 6. Troubleshooting

### 6.1 Deployment Failures

**Build fails: "module not found"**

```bash
# Ensure Go 1.25.0+
go version

# Tidy dependencies
go mod tidy
go mod download

# Try build again
go build -v ./cmd/api
```

**Build fails: "database error"**

```bash
# Verify database is running
docker compose ps postgres

# Check database logs
docker compose logs postgres

# Ensure pgvector extension is installed
docker compose exec postgres psql -U genie -d genie -c "CREATE EXTENSION IF NOT EXISTS vector;"
```

### 6.2 Runtime Issues

**API crashes with "panic: runtime error"**

```bash
# Check recent logs
docker compose logs genie-api | tail -50

# Common causes:
# - Missing environment variable: add to .env file
# - Database connection timeout: restart postgres
# - Nil pointer in agent code: check agent logs

# Restart with verbose logging
GENIE_LOG_LEVEL=debug docker compose up genie-api
```

**Requests hang (no response after 10 seconds)**

```bash
# Check API health
curl -i http://localhost:8080/healthz

# Monitor active connections
docker compose exec postgres \
  psql -U genie -d genie -c "SELECT count(*) FROM pg_stat_activity;"

# If >8 connections: pool may be exhausted
# Restart API: docker compose restart genie-api
```

### 6.3 Data Issues

**Cannot find user created in previous test**

```bash
# Check database state
docker compose exec postgres psql -U genie -d genie -c "SELECT id, email FROM users LIMIT 5;"

# If empty, database was reset (docker compose down -v resets volumes)
# Recreate user:
curl -X POST http://localhost:8080/v1/users ...
```

**Order stuck in "pending" status**

```bash
# Check workflow logs
docker compose logs genie-api | grep order_id

# Manually check order in database
docker compose exec postgres psql -U genie -d genie \
  -c "SELECT id, status, created_at FROM orders WHERE id = 'order_xxx';"

# If workflow stuck: restart API and retry
docker compose restart genie-api
curl -X POST http://localhost:8080/v1/orders/$ORDER_ID/execute ...
```

### 6.4 Observability Issues

**Prometheus metrics not updating**

```bash
# Check if metrics endpoint is live
curl -s http://localhost:9464/metrics | head -5

# If empty: API may not be exporting metrics
# Restart and check logs:
docker compose restart genie-api
docker compose logs genie-api | grep -i prometheus

# If OTEL endpoint unreachable:
docker compose logs otel-collector
docker compose restart otel-collector
```

**Traces not appearing in Grafana Tempo**

```bash
# Verify OTEL collector is receiving spans
docker compose logs otel-collector | grep -i span

# Check API is exporting traces
docker compose logs genie-api | grep -i otel

# Restart collector
docker compose restart otel-collector
```

### 6.5 Cleanup & Recovery

**Full reset (start from scratch)**

```bash
# Stop and remove all containers + volumes
docker compose down -v

# Rebuild image
docker build -t genie-api:staging .

# Start fresh
docker compose up --build
```

**Partial reset (keep data)**

```bash
# Restart just the API (keep DB, Ollama)
docker compose restart genie-api

# Or rebuild without volume cleanup
docker compose up --build genie-api
```

**Inspect database state**

```bash
# Connect to database CLI
docker compose exec postgres psql -U genie -d genie

# Common queries:
\dt                              -- List tables
SELECT count(*) FROM users;      -- Count users
SELECT * FROM incidents LIMIT 5; -- View recent incidents
\q                               -- Exit
```

---

## Appendix: Quick Reference Commands

### Deployment

| Command | Purpose |
|---------|---------|
| `make build` | Build genie-api binary |
| `docker build -t genie-api:staging .` | Build Docker image |
| `docker compose up --build` | Start full stack |
| `make smoke` | Run smoke tests |

### Monitoring

| Command | Purpose |
|---------|---------|
| `docker compose logs -f genie-api` | Stream API logs |
| `curl http://localhost:8080/healthz` | Check API health |
| `curl http://localhost:9464/metrics` | View Prometheus metrics |
| `docker compose ps` | Show container status |

### Testing

| Command | Purpose |
|---------|---------|
| `go test ./pkg/commerce/... -v` | Run commerce tests |
| `go test ./pkg/web/handlers/... -v` | Run handler tests |
| `curl -X GET http://localhost:8080/v1/users/me -H "Authorization: Bearer $TOKEN"` | Test authentication |

### Cleanup

| Command | Purpose |
|---------|---------|
| `docker compose down` | Stop containers |
| `docker compose down -v` | Stop + remove volumes |
| `docker system prune -a` | Remove unused Docker artifacts |

---

## Summary Checklist

Before declaring staging "ready":

- [ ] Pre-deployment checks all passed
- [ ] Binary/image builds successfully
- [ ] Docker Compose stack starts without errors
- [ ] `/healthz` and `/readyz` return 200
- [ ] UI loads in browser without console errors
- [ ] ARIA labels present on form fields
- [ ] Tab navigation works end-to-end
- [ ] Smoke test passes (`make smoke`)
- [ ] Commerce e2e tests pass (`go test ./pkg/commerce`)
- [ ] User signup → auth → account creation → order creation flow works
- [ ] Payment endpoint accepts requests and returns valid responses
- [ ] Order workflow executes without errors
- [ ] Prometheus metrics are being collected
- [ ] No critical errors in logs
- [ ] Performance baseline established

**Estimated total deployment time:** 30 minutes (first) → 10 minutes (subsequent)

---

**Last Updated:** June 1, 2026  
**Phase:** Phase 1 (UI + e-Rupee Backend)  
**Status:** Ready for Staging Deployment
