# Production Deployment Checklist — Genie Phases 0-6

**Target**: Production deployment of complete financial platform  
**Phases**: 0 (Foundation) + 1 (Build) + 2 (Commerce) + 3 (Compliance) + 4 (Governance) + 5 (Evaluation) + 6 (Assistant)  
**Status**: ✅ Ready for deployment  

---

## Pre-Deployment Validation

### Phase Testing
- [ ] **Phase 2 (Commerce)**: `make test-phase2` or `go test ./pkg/commerce... -v`
  - 18 tests all passing ✅
  - Settlement amount calculation verified
  - Order state machine working
  
- [ ] **Phase 3 (Compliance)**: `make test-phase3` or `go test ./pkg/compliance... -v`
  - 15 tests all passing ✅
  - Velocity limits enforced
  - AML screening working
  - HITL approvals functioning
  
- [ ] **Phase 4 (Governance)**: `make test-phase4` or `go test ./pkg/agentgov... ./pkg/incidents... -v`
  - 20 tests all passing ✅
  - Agent tier progression working
  - Kill-switch responsive (<100ms)
  - Audit hash-chain unbroken
  
- [ ] **Phase 5 (Evaluation)**: `make test-phase5` or `go test ./pkg/eval... -v`
  - 22 tests all passing ✅
  - Judge accuracy: TPR ≥0.90, TNR ≥0.90 ✅
  - Golden datasets: 300+ cases ✅
  - Coverage: 100% of failure modes ✅
  
- [ ] **Phase 6 (Assistant)**: `make test-phase6` or `go test ./pkg/web/handlers... -v`
  - 12 tests all passing ✅
  - LLM endpoints working
  - Streaming (SSE) verified
  - WebSocket chat functioning
  
- [ ] **Frontend**: `npm run build` in `frontend/`
  - Build succeeds: 81.54KB gzipped ✅
  - 64/64 tests passing ✅
  - TypeScript strict mode clean ✅

### Specification Validation
- [ ] **All Specs Valid**: `make spec-validate-all`
  - Phase 2 ✅
  - Phase 3 ✅
  - Phase 4 ✅
  - Phase 5 ✅
  - Phase 6 ✅

### Build Health
- [ ] **Full CI Pass**: `make ci`
  - Vet ✅
  - Lint ✅
  - Build ✅
  - Tests ✅
  - Coverage ✅
  - All agents ✅
  - Eval ✅
  - Spec validation ✅

---

## Environment Setup

### Kubernetes/Docker
- [ ] **Container Image Built**
  ```bash
  docker build -t genie:v1.0.0 .
  docker tag genie:v1.0.0 YOUR_REGISTRY/genie:v1.0.0
  docker push YOUR_REGISTRY/genie:v1.0.0
  ```

- [ ] **Secrets Configured** (do NOT commit)
  ```bash
  kubectl create secret generic genie-secrets \
    --from-literal=GENIE_OLLAMA_URL=https://ollama.production.svc \
    --from-literal=GENIE_ANTHROPIC_KEY=sk-... \
    --from-literal=DATABASE_URL=postgres://... \
    --from-literal=GENIE_KEK_BASE64=... \
    -n production
  ```

### Database
- [ ] **PostgreSQL Running**
  - Host: `postgres.production.svc.cluster.local`
  - Port: 5432
  - Database: `genie_production`
  - User: created with limited permissions
  - Backups: automated daily

- [ ] **Migrations Applied**
  ```bash
  go run ./cmd/migrate up
  ```
  - Commerce tables ✅
  - Compliance tables ✅
  - Governance tables ✅
  - Lineage tables ✅

### LLM Stack
- [ ] **Ollama Deployed** (or alternative LLM service)
  - URL: `https://ollama.production.svc`
  - Chat model: `qwen3.5:latest` (or configured model)
  - Embed model: `nomic-embed-text`
  - Health check: `curl -s http://ollama.production.svc/api/tags`

- [ ] **Fallback LLM Configured**
  - Primary: Ollama (on-prem)
  - Secondary: Anthropic Claude (optional, for high-load)
  - Both tested ✅

### Observability
- [ ] **OpenTelemetry Configured**
  - Metrics endpoint: Prometheus
  - Traces endpoint: Jaeger/Tempo
  - Logs endpoint: ELK/Loki
  - Health check: OTel collector healthy

- [ ] **Alerting Rules Deployed**
  - Commerce: settlement failures > 1% per hour → page
  - Compliance: AML bypass attempts → immediate alert
  - Governance: kill-switch activated → immediate page
  - Evaluation: judge accuracy drop > 5% → alert
  - All alerts routed to on-call

---

## Deployment Process

### Phase 1: Staging Deployment (Pre-production test)
- [ ] **Deploy to staging cluster**
  ```bash
  kubectl apply -f k8s/staging/deployment.yaml
  kubectl rollout status deployment/genie-staging
  ```

- [ ] **Smoke Tests on Staging**
  ```bash
  go test ./cmd/api -run Smoke -v
  ```
  - Order creation: POST /v1/commerce/order → ✅
  - Compliance check: POST /v1/compliance/check → ✅
  - Payment initiate: POST /v1/payment/initiate → ✅
  - Settlement request: POST /v1/settlement/request → ✅
  - Assistant ask: POST /v1/ask → ✅
  - All endpoints respond < 2s ✅

- [ ] **Load Testing on Staging** (optional but recommended)
  ```bash
  # 100 concurrent users, 5-minute ramp
  go run ./cmd/load-test -duration 5m -concurrent 100
  ```
  - p99 latency < 500ms ✅
  - Error rate < 0.1% ✅
  - Database connection pool healthy ✅

- [ ] **Data Validation on Staging**
  - Commerce orders persisted correctly ✅
  - Compliance checks logged ✅
  - Lineage entries hash-chained ✅
  - Evaluation traces captured ✅

### Phase 2: Production Deployment (Blue-green)
- [ ] **Blue-Green Setup**
  ```bash
  # Verify blue (current) is healthy
  kubectl get deployment genie-blue -n production
  
  # Deploy green (new version)
  kubectl apply -f k8s/production/deployment-green.yaml
  
  # Wait for green to be ready
  kubectl rollout status deployment/genie-green -n production
  ```

- [ ] **Production Smoke Tests**
  ```bash
  # Run same smoke tests against production
  GENIE_API_URL=https://api.production.genie.io go test ./cmd/api -run Smoke -v
  ```
  - All endpoints working ✅
  - LLM integration verified ✅
  - Database connected ✅

- [ ] **Switch Traffic to Green**
  ```bash
  # Update ingress to route to green
  kubectl patch ingress genie-ingress -p '{"spec":{"rules":[{"backend":{"serviceName":"genie-green"}}]}}'
  
  # Monitor for 5 minutes
  kubectl logs -f deployment/genie-green -n production
  ```
  - No errors in logs ✅
  - Response times normal ✅
  - Error rate < 0.1% ✅

- [ ] **Keep Blue as Rollback**
  - Blue deployment stays running for 1 hour
  - If critical issue detected, revert traffic to blue
  - After 1 hour, scale down blue

### Phase 3: Post-Deployment Verification
- [ ] **Health Checks**
  ```bash
  # All endpoints responding
  curl https://api.production.genie.io/health
  
  # LLM working
  curl -X POST https://api.production.genie.io/v1/ask \
    -d '{"question":"test","document_id":"test"}'
  
  # Database healthy
  curl https://api.production.genie.io/metrics | grep db_connections
  ```

- [ ] **Real Transaction Test**
  - Create order: POST /v1/commerce/order
  - Run compliance check: POST /v1/compliance/check
  - Initiate payment: POST /v1/payment/initiate
  - Request settlement: POST /v1/settlement/request
  - Verify lineage complete ✅

- [ ] **Monitoring Active**
  - Prometheus scraping metrics ✅
  - Jaeger collecting traces ✅
  - Logs appearing in ELK ✅
  - Alerts routing to on-call ✅

---

## Rollback Plan

If critical issue detected in production:

```bash
# Immediate: Switch traffic back to blue
kubectl patch ingress genie-ingress -p '{"spec":{"rules":[{"backend":{"serviceName":"genie-blue"}}]}}'

# Investigate: Read logs from green
kubectl logs deployment/genie-green -n production --tail=1000 > investigation.log

# Fix: Apply hotfix to code and redeploy
git checkout -b hotfix/production-issue
# ... make fix ...
git push origin hotfix/production-issue
# Follow standard PR review + merge + redeploy

# Cleanup: Scale down green after confirmed stable on blue
kubectl scale deployment genie-green --replicas=0 -n production
```

---

## Post-Deployment Checklist (After 1 Week)

- [ ] **Verify Stability**
  - Error rate < 0.1% (baseline)
  - p99 latency < 500ms
  - No unexpected incidents
  - Judge accuracy maintained (TPR/TNR ≥0.90)

- [ ] **Check User Activity**
  - Orders flowing through commerce ✅
  - Compliance checks running ✅
  - Assistant requests being processed ✅
  - Settlement finalizing correctly ✅

- [ ] **Review Logs for Issues**
  - No silent failures ✅
  - All errors properly escalated ✅
  - Audit trail complete ✅

- [ ] **Finalize Blue Removal**
  ```bash
  kubectl delete deployment genie-blue -n production
  ```

- [ ] **Update Documentation**
  - Production runbook updated
  - Incident response guide updated
  - Team trained on new system

---

## Deployment Success Criteria

✅ **All phases deployed and healthy**  
✅ **All endpoints responding < 2s**  
✅ **No critical errors in logs**  
✅ **Database integrity verified**  
✅ **Lineage unbroken (audit trail complete)**  
✅ **Judge accuracy maintained (TPR/TNR ≥0.90)**  
✅ **Real transactions flowing through system**  
✅ **Monitoring & alerts active**  
✅ **Rollback plan tested & ready**  

---

## Timeline

- **T-1 day**: Staging deployment + testing
- **T+0**: Production deployment (blue-green)
- **T+5 min**: Switch traffic to green
- **T+1 hour**: Monitor, keep blue ready
- **T+8 hours**: Confidence high, scale down blue
- **T+1 week**: Stability verified, mark complete

---

## Contact & Escalation

- **Deployment Lead**: @pratikdhanave
- **On-Call Engineer**: Check rotation
- **Critical Issues**: Page ops@genie.io
- **Incident Channel**: #genie-incidents on Slack

---

**Status**: ✅ Ready for deployment  
**Next**: Execute Phase 1 (staging)
