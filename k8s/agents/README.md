# Decoupled agent Kubernetes manifests

Each agent runs as its own Deployment + Service in the `genie-staging` namespace,
reachable by the backend via the `GENIE_AGENT_<ID>_MODE=http` toggle.

## What's here

Committed examples for the three **stateless advisor agents** (real binaries +
Dockerfiles, smoke-tested):

- `genie-profile-analyzer.yaml`
- `genie-financial-analyst.yaml`
- `genie-recommendation-generator.yaml`

Each bundles: Deployment (2 replicas, `/health` probes, 64Mi/50m requests,
`terminationGracePeriodSeconds: 35`, label `genie-agent: "true"`), Service
(ClusterIP :8080), PodDisruptionBudget (`minAvailable: 1`), HorizontalPodAutoscaler
(2–5 replicas @ 70% CPU), and a NetworkPolicy restricting ingress to `app: genie-api`.

## Generating the rest

All 34 agents share one template. Generate any agent's manifest with:

```bash
scripts/gen-agent-k8s.sh <agent-dir-name> > k8s/agents/genie-<name>.yaml
# e.g.
scripts/gen-agent-k8s.sh analyzer        > k8s/agents/genie-analyzer.yaml
scripts/gen-agent-k8s.sh aa_fetcher      > k8s/agents/genie-aa-fetcher.yaml
```

`<agent-dir-name>` is the `agents/cmd/<name>` directory; underscores become
hyphens in K8s resource names.

### Agents that need extra env

After generating, append env to the Deployment for:

- **portfolio_advisor** — `GENIE_DB_DSN` (Postgres; deploy behind PgBouncer)
- **educator** — `GENIE_OLLAMA_URL` / `GENIE_OLLAMA_EMBED` (only if enabling RAG)

## Prerequisites (one-time)

```bash
# Shared agent auth token
kubectl create secret generic genie-agent-secret \
  --from-literal=token=$(openssl rand -hex 32) -n genie-staging
```

The `genie-api` egress NetworkPolicy (in `k8s/staging/deployment.yaml`) already
permits backend → `genie-agent: "true"` pods on port 8080. **Without that rule,
all backend→agent calls are silently dropped.**

## Deploy + cut over one agent

```bash
kubectl apply -f k8s/agents/genie-profile-analyzer.yaml
kubectl exec deploy/genie-api -n genie-staging -- \
  curl -sf -H "X-Agent-Token: $TOKEN" http://genie-profile-analyzer:8080/health
# flip the toggle (rollback = set back to memory)
kubectl set env deploy/genie-api -n genie-staging \
  GENIE_AGENT_PROFILE_ANALYZER_MODE=http \
  GENIE_AGENT_PROFILE_ANALYZER_URL=http://genie-profile-analyzer:8080
```
