# Sovereign AI Is a Policy, Not a Slide

*Enforcing data residency at the message bus, not inside the LLM call — and why "we use a Mumbai region of OpenAI" isn't what FREE-AI is asking for.*

---

## What "sovereign AI" really means

"Sovereign AI" has become a slide-deck word. Everyone uses it, nobody defines it.

The RBI FREE-AI report doesn't use the term explicitly, but its **Rec 4 (Indigenous AI Models)** combined with **Rec 15 (Data Lifecycle Governance)** asks the substantive question:

> *Where does your customer's PII go, and can you prove it?*

The answers most banks have today:

- "Our LLM provider has an India region." — Doesn't matter if the request itself never gets denied for non-Indian routing.
- "We have a data residency policy." — Where? In the contract, or in the code path?
- "Production traffic stays on-prem." — Define traffic. Define on-prem. Show me.

The honest answer most teams should give is: *"We don't know with confidence."* Because the enforcement point is buried in a wrapper around an SDK call, six layers below the application, and nobody has audited it end-to-end.

This article is about moving the enforcement point so deep that you can answer the regulator's question without flinching.

---

## Where most systems put residency

The default pattern, when teams add residency at all:

```python
def call_llm(prompt, region_hint=None):
    if region_hint == "in":
        provider = ollama_local
    else:
        provider = openai_api
    return provider.complete(prompt)
```

Three things wrong with this:

1. **`region_hint` is application-supplied.** The application has to remember to pass it. Forget once, PII leaks.
2. **The check is inside the LLM call site.** Every new feature that touches an LLM has to re-implement the check. Some will forget.
3. **There's no audit trail.** If `region_hint` was "in" but the application accidentally routed to OpenAI, the regulator sees no record of the denial — because there was no denial. There was a silent miss.

The fix: move the enforcement point upstream of the LLM call. Way upstream.

---

## Where Genie puts residency

In the **governance composite**, which evaluates every message that crosses the bus *before* any agent runs.

```go
type DataResidencyPolicy struct {
    HomeRegion          string  // "in"
    AllowCrossBorderFor []Classification // e.g. only "public"
}

func (p DataResidencyPolicy) Evaluate(ctx context.Context, msg Message) (PolicyResult, error) {
    targetRegion := msg.Metadata["region"]
    if targetRegion == p.HomeRegion || targetRegion == "on-prem" {
        return Allow, nil
    }
    if isCrossBorderAllowed(msg.Classification, p.AllowCrossBorderFor) {
        return Allow, nil
    }
    return Deny, errors.New("PII bound for non-home region")
}
```

And `targetRegion` is populated by the orchestrator from the **declared region of the destination provider**:

```go
type OllamaProvider struct{ ... }
func (p *OllamaProvider) Region() string { return "on-prem" }

type AnthropicProvider struct{ ... }
func (p *AnthropicProvider) Region() string { return "us" }
```

Every provider declares where it lives. Every message carries a classification. The bus checks the matrix before dispatch. PII headed to a non-home region gets denied, an incident is recorded, and the user gets a degraded response from the fallback agent.

Note what just happened: **the application doesn't have to remember anything.** The protection is structural, not behavioural. New feature in six months that calls a new LLM provider? The provider declares its region; the policy reads it; the routing is automatic.

---

## The full enforcement matrix

| Source classification | Target region | Decision |
|---|---|---|
| `public` | `in` / `on-prem` | Allow |
| `public` | `us` / `eu` | Allow (if `AllowCrossBorderForPublic = true`) |
| `internal` | `in` / `on-prem` | Allow |
| `internal` | `us` / `eu` | Deny + incident |
| `pii` | `in` / `on-prem` | Allow |
| `pii` | anything else | Deny + incident |
| `secret` | `on-prem` only | Allow |
| `secret` | anything else | Deny + incident |

That table is the policy. It lives in the board-approved YAML. The risk team owns the matrix; engineering ships the loader.

---

## What this looks like in production

A user uploads their bank statement (`classification = pii`). Asks the chatbot a question.

The orchestrator dispatches the question. The default LLM provider (set by `GENIE_LLM=ollama`) declares `Region() == "on-prem"`. The residency policy evaluates: PII to on-prem → allow. The LLM call happens locally.

Now imagine the host configures `GENIE_LLM=anthropic` for chat (a cost optimization). The provider declares `Region() == "us"`. The residency policy evaluates: PII to US → **deny**. The message drops. An Annexure VI incident records the attempt. The user gets a degraded response from the fallback agent.

The host has to consciously *change the policy* to allow PII to US — which is a YAML edit, with board sign-off, with an audit trail. No silent leak.

---

## The hot-path / cold-path split

This is the deployment shape FREE-AI Rec 4 is asking for, expressed in concrete terms:

- **Hot path** (PAN, account, transaction, holding, balance, KYC): Ollama on-prem. Region = `on-prem`. Residency policy permits.
- **Cold path** (macro research, generic financial education, public news summary): Hosted frontier model (Anthropic / OpenAI / Gemini). Region = `us`. Residency policy permits because `classification = public`.

The router that decides which path applies is **30 lines of code**. The compliance posture is a YAML file. The enforcement is at the bus, not in the LLM call.

This is what sovereign AI looks like when you actually build it.

---

## Why "the LLM provider has an India region" isn't enough

A common claim: "OpenAI / Anthropic / Google have India regions; we use those; we're sovereign."

Two problems:

### 1. Where's the enforcement?

If your application can call the US region *or* the India region depending on a flag, and the flag is application-supplied, you have no enforcement. You have a hope. The same code path that calls India today can call US tomorrow because someone changed an env var. The audit log doesn't see anything wrong, because nothing was denied — the application just made a choice.

In Genie's model, even if you point a "US" provider at a Mumbai data center, the provider has to declare `Region() == "in"` for the policy to allow PII. The provider's *declared* region is what's enforced. If the declared region doesn't match reality, that's a separate (legitimate) audit conversation — but you can't accidentally leak by misconfiguring a flag.

### 2. What about *all the data* before the LLM call?

The LLM call is one moment in a much longer pipeline. Before the LLM:

- The HTTP request body crosses the load balancer.
- The application parses it.
- The application stores it (encrypted, but still in your DB).
- The orchestrator publishes it to the bus.
- Multiple agents handle it.
- Eventually the LLM gets called.

If your residency story only covers the LLM call site, the other six steps are unprotected. Genie's policy fires at the bus, which is *upstream of* the LLM. The bus check is the moment the PII could leave the perimeter. Catch it there, you catch it everywhere.

---

## What gets exposed publicly

The active residency posture is on the `GET /v1/disclosures` endpoint (public, unauthenticated):

```json
{
  "home_region": "in",
  "policy_version": "0.1.0",
  "policy_approved_on": "2025-08-13",
  "principles": [...]
}
```

Anyone — customer, regulator, journalist — can see the policy without logging in. **FREE-AI Rec 25 (Disclosures)** is asking for exactly this.

When the regulator asks "where does your customer's data go?", the answer is "open `/v1/disclosures`, the home region is in the response."

---

## What this earns

- **Rec 4 (Indigenous AI Models)** — Ollama on-prem default, hosted providers gated by residency policy.
- **Rec 15 (Data Lifecycle Governance)** — envelope encryption (separate concern) + residency = the data lifecycle is genuinely governed end-to-end.
- **Rec 18 (Disclosure)** — public endpoint + disclaimer on every AI response.
- **Rec 25 (Disclosures)** — same public endpoint, same disclosure pattern.

Four of the 26 recommendations, addressed by one composite policy + one provider interface.

---

## When you'll want to relax it

Three legitimate cases for relaxing the home-region constraint:

1. **Public-only cold path**. Macro research, financial education — no PII involved. Permit cross-border for `classification = public`. Genie's policy YAML has an explicit toggle: `allow_cross_border_for_public: true`.

2. **Cross-border banking products**. NRI accounts, international remittance. These need explicit per-product policy carve-outs, not a global flag.

3. **Vendor due diligence done**. If you've audited a US provider's controls and your DPO + legal are satisfied, you can add them with their actual region declared as `us` and either flag the classifications allowed or run them only for `internal`-classified data.

What's *not* a legitimate case: "It's faster to call the US API." Performance is not a residency override.

---

## The thesis

Sovereign AI isn't about whether you use an Indian LLM provider. It's about whether your enforcement point can survive an audit.

If your policy is:

- In code, not in a YAML the board owns → it's not adaptive (FREE-AI Rec 6 violation).
- Inside the LLM call site → it's behavioural, not structural; you'll forget.
- Application-supplied → there's no defence in depth.
- Not exposed publicly → you can't honour Rec 25.

Move the enforcement upstream. Declare provider regions in the provider, not in the application. Let the policy at the bus do the work. **Make it structural, not behavioural.**

Then the regulator's question becomes: "Show me the policy file." And the answer is a `cat`.

---

## The repo

Genie is open source under MIT.

- `pkg/governance/sovereignty.go` — the residency policy
- `pkg/llm/*.go` — providers with declared `Region()`
- `pkg/sovereignty/` — the provider registry (allowlist of permitted external providers)
- `GET /v1/disclosures` — the public exposure
- `config/ai-policy.example.yaml` — the board-approved YAML with `home_region` and `allow_cross_border_for_public`

```bash
git clone https://github.com/c2siorg/genie.git
curl localhost:8080/v1/disclosures | jq .
```

---

If you've enforced data residency at a different layer — at the SDK, at the network, at the provider — I'd really like to understand the tradeoffs. The bus is where it landed for us; not the only place it could land.

#SovereignAI #DataResidency #RBI #FREEAI #IndianBanking #ResponsibleAI #DataGovernance #DPDP #FinTechIndia
