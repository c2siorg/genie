# How to Ship Governance Without Shipping Code

*Building a tiny policy DSL so the risk team can change a guardrail without filing a Jira ticket — and why CEL was the wrong answer.*

---

## The Friday-night problem

It's 8 PM on a Friday. The fraud team has spotted a new mule pattern: small transfers to a fresh beneficiary tier with PAN-mismatched names. They want a rule to deny these at the bus before they reach the recommender.

Two paths:

**Path A.** Engineering writes a new `governance.Policy` Go struct, adds a test, opens a PR, gets review, merges, deploys. Best case: Monday morning. Worst case: next sprint.

**Path B.** The risk team edits a YAML file. The system loads it on the next health check. Live by Friday at 9 PM.

Path B is what **FREE-AI Recommendation 6 (Adaptive Policies)** is asking for. It's also what every CRO I've talked to says they want and can never get.

---

## Why most systems are stuck on Path A

The honest reason: governance lives in code. The composite policy in `pkg/governance` is a Go slice of `Policy` interfaces. Each policy is a struct. Adding a new rule means writing Go.

Engineering doesn't *want* to gatekeep risk rules — they're not the domain experts. But the tool the risk team needs (declarative, safe, reviewable, hot-reloadable) doesn't exist in most codebases. So Friday's mule pattern waits for Monday's sprint.

---

## Why CEL was the wrong answer for me

The obvious move was to pull in **Google's CEL** (Common Expression Language). It's production-grade, well-spec'd, used by Kubernetes admission controllers, and has Go bindings via `cel-go`.

I almost did. Then I looked at the dependency graph.

`github.com/google/cel-go` pulls in the protobuf runtime, the gnostic OpenAPI generator, parts of grpc-go, and a transitive set of indirect deps that adds ~40 modules to the project. For a system whose entire selling point is "minimal dependencies, auditable surface, single Go binary," that's a lot of new attack surface for a feature my risk team will use to write a dozen rules.

The other option: write a small DSL myself. **CEL covers 100% of expression power; I needed maybe 20%.** Equality, comparison, AND/OR/NOT, string contains/startsWith, dotted metadata access. That's it.

I wrote it. It's 330 lines of Go, zero new dependencies, and the grammar fits on one screen:

```
expr     = orExpr
orExpr   = andExpr ("OR" andExpr)*
andExpr  = unary ("AND" unary)*
unary    = "NOT" unary | primary
primary  = "(" expr ")" | cmp
cmp      = term op term
term     = stringLit | numLit | identifier
op       = "==" | "!=" | "<" | ">" | "<=" | ">=" | "contains" | "startsWith"
```

That's the whole language.

---

## What the risk team gets

A YAML file, `config/policies/rules.yaml`:

```yaml
- id: deny_offshore_pii
  when: classification == "pii" AND metadata.region != "in"
  decision: deny
  reason: "PII bound for non-home region"

- id: block_external_partner_without_trace
  when: from startsWith "ext-" AND NOT metadata.trace_id == "present"
  decision: deny
  reason: "External-partner traffic must carry trace_id"

- id: block_sql_injection_attempt
  when: content contains "DROP TABLE"
  decision: deny
  reason: "Likely SQL injection in user content"

- id: block_large_untrusted_transfer
  when: type == "payment_request" AND metadata.is_trusted == "false" AND metadata.amount > 50000
  decision: deny
  reason: "Large transfer to untrusted beneficiary — refer to fraud team"
```

The system loads it at boot, compiles each rule once, and wraps each into a `governance.Policy`. The existing composite picks them up automatically. The risk team's mule rule is live in five minutes.

---

## The boring engineering parts that matter

I'll skip the parser code (recursive descent, ~150 lines), but three design choices deserve explanation:

### 1. Lazy compilation, eager validation

Every rule is parsed once at boot, not per-message. If a rule has a syntax error, the loader fails loudly — the system refuses to start with a broken policy. You don't get the "policy file looked fine, but the typo only fires when message metadata has key X" failure mode.

### 2. Whitelisted field references

The right-hand identifier in a comparison can only resolve to one of:

- `classification`, `type`, `from`, `to`, `role`, `content`
- `metadata.<key>` (for any string-coerced metadata field)

That's it. You can't reach into the goroutine pool, the network, or the file system. The DSL evaluator is a pure function over the Message.

This isn't paranoia — it's threat modeling. A DSL that can express "if classification is X, deny" must not also be able to express "if classification is X, exec curl evil.com." Whitelisted references make the latter impossible by construction.

### 3. Per-rule reason field

Every rule has a `Reason` field that ships into the denial event. When the auditor pulls the incident log and sees a deny, they see *which named rule* fired, not just "denied." Attribution is the whole point.

---

## What the DSL deliberately does NOT do

This is where engineers get tempted to add things. Resist.

- **No regex.** Use the existing `governance.PIIBlockPolicy` (Go-side, well-tested) or write a CEL adapter if you really need it.
- **No date / time arithmetic.** "Within last 5 minutes" needs a Go-side policy with a clock dependency.
- **No nested struct walks beyond `metadata.<key>`.** No `metadata.user.roles[0]`.
- **No mutations.** Decision is allow/deny only. The DSL never modifies the message.
- **No external lookups.** No SQL, no HTTP, no Redis. A rule executes in microseconds against in-memory message data.

If a rule needs any of these, write a Go-side `governance.Policy`. The DSL is the 80 % case; the long tail belongs in code.

---

## How it composes with what was already there

The existing Genie composite stack:

```go
composite := governance.NewComposite(
    governance.MaxContentLengthPolicy{Max: 262144},
    governance.RequiredMetadataPolicy{...},
    governance.RBACPolicy{...},
    governance.ClassificationPolicy{...},
    governance.DataResidencyPolicy{HomeRegion: "in"},
    governance.PIIBlockPolicy{},
    governance.PromptInjectionPolicy{},
    // ... etc
)
```

Adding the DSL:

```go
raw, _ := os.ReadFile("config/policies/rules.yaml")
var rules []dsl.Rule
yaml.Unmarshal(raw, &rules)
compiled, _ := dsl.Compile(rules)

composite := governance.NewComposite(append(coreStack, dsl.AsPolicies(compiled)...)...)
```

That's the whole integration. Existing policies still fire. The DSL rules fire after them. The composite still denies on the first failure. The audit log gets the rule ID. The Annexure VI form auto-generates on deny.

---

## What this unlocks

A few patterns the risk team can now own:

**Concentration limits.** "Deny any single message > ₹10L from a single payer in the last 5 minutes." (Well — the time window needs a Go-side helper, but the threshold lives in YAML.)

**Vendor allowlists.** "Allow messages from `from = 'partner-a'`, `from = 'partner-b'`; deny all other `from startsWith 'partner-'`."

**Classification ceilings per agent.** "Deny `to = 'public_chat'` if `classification` is pii."

**Geographic restrictions.** "Deny messages where `metadata.geo` is in {CU, IR, KP, RU, SY}."

Each of these used to require a code change. Now each is a YAML edit, reviewed by the risk team, deployed by configuration management.

---

## The audit story

When the regulator asks "show me your policy on PII residency," the answer is:

> "Open `config/policies/rules.yaml`. Search for `deny_offshore_pii`. The `when` clause expresses the rule in DSL. The board approved this YAML version on 2025-08-13. The hash of the file is logged at every boot, so we can prove which version was active on which date. Every denial event references the rule ID."

That's much harder than "let me get back to you on which policy module that lives in" — because the regulator can verify it themselves with `cat`.

---

## When you'll outgrow this

Three signs the DSL has reached its limit:

1. You're writing rules with three nested ANDs / ORs to express what would be one SQL JOIN. Use a Go-side policy or move to CEL.
2. You need cross-message state ("more than 5 messages from X in the last minute"). The DSL is per-message; cross-message lives in Go.
3. You want time-window math. Add a `time_since_<event>` helper in Go; expose it as a metadata field; then the DSL rule reads naturally.

When that day comes, the DSL's design choice pays off: swap the evaluator for `cel-go` without changing the YAML or the policy interface. The host code doesn't change. The risk team's rule files don't change.

---

## The thesis

The fastest way to ship FREE-AI Rec 6 isn't to install a heavyweight policy engine. It's to write the 20 % of an engine your risk team actually needs, and to leave the 80 % they don't need on the shelf where it can't surprise you.

**Code is debt. The smallest amount that solves the problem is the right amount.**

For us, that was 330 lines and zero new dependencies. Your DSL might need 600 lines and one optional dependency. That's still smaller than CEL plus protobuf plus gnostic plus the rest of the kitchen.

---

## The repo

Genie is open source under MIT.

- `pkg/policy/dsl/dsl.go` — the parser + evaluator
- `pkg/policy/dsl/dsl_test.go` — 11 unit tests covering every operator
- `docs/packages/policy-dsl.md` — the deep-dive doc
- `config/policies/` — where the YAML lives in production

```bash
git clone https://github.com/c2siorg/genie.git
go test ./pkg/policy/dsl/ -v
```

---

If you've solved this problem differently — embedded Lua, Starlark, OPA / Rego, a homegrown rule engine — I'd genuinely like to compare notes on what shapes survived contact with a real risk team.

#ResponsibleAI #PolicyAsCode #RBI #FREEAI #FinTechIndia #GovernanceAsCode #BankingCompliance #SoftwareArchitecture
