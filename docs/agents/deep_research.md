# agents/deep_research

> **Risk class:** Medium · **Capability:** `deep_research` · **In:** `research_request` · **Out:** `research_brief`
> **Inspired by:** Google ADK `deep-search`, adapted for the Indian regulatory corpus.

---

## Overview

Multi-turn ReAct research agent for the Indian banking corpus — RBI
circulars, Sahamati AA specs, FATF / FIU-IND guidance, macro releases.

Where `agents/macro_research` is one-shot (single LLM call against a
prompt), `deep_research` *iterates*: search → synthesise → re-search →
cite. Uses `pkg/reasoning.ReAct` under the hood with a fixed toolbelt
of corpus-specific fetchers.

Critically, ships an **offline deterministic fallback** so the agent
works in the sandbox, in CI, and as a degraded-mode handler when the
LLM provider is unavailable.

---

## Constants

```go
const (
    ID         = "deep_research"
    Capability = "deep_research"
    TypeIn     = "research_request"
    TypeOut    = "research_brief"
    NextAgent  = "financial_supervisor"
)
```

---

## Constructor

```go
func New(provider llm.Provider, model string, resolver Resolver) *Agent
```

- `provider`: pass `nil` to force offline mode (deterministic, no network).
- `model`: ignored when `provider == nil`.
- `resolver`: pass `nil` to use `StubResolver{}`; in production wire a real fetcher.

---

## Types

### Request

```go
type Request struct {
    Question string
    Sources  []string // optional corpus filter; default = all
    MaxSteps int      // ReAct loop bound; default = 4
}
```

### Brief (outbound)

```go
type Brief struct {
    Question   string
    Summary    string
    Citations  []Citation
    Steps      int
    Mode       string // "react" | "offline"
    Disclaimer string
}

type Citation struct {
    Title string
    URL   string
    Quote string
}
```

### Resolver (pluggable)

```go
type Resolver interface {
    Resolve(ctx context.Context, corpus, query string) (snippet string, source Citation, err error)
}
```

Default corpora handled by `StubResolver`: `rbi`, `sahamati`, `fiu_ind`.
A host can register any corpus name — `world_bank`, `imf`, internal
SharePoint — by implementing `Resolver`.

---

## Two execution modes

### Mode "react" — when an LLM is wired

1. Build tool list — one `reasoning.Tool` per corpus, name `search_<corpus>`.
2. System prompt: "You are a precise financial researcher. Use the available tools to gather facts before answering. Cite the source corpus for every claim."
3. Run `reasoning.ReAct(ctx, provider, model, system, question, tools, maxSteps)`.
4. Scrape citations from the tool-observation steps in the trace.
5. Return Brief with mode `"react"`.

### Mode "offline" — when provider is nil OR ReAct errors

1. Resolve each requested corpus once via the Resolver.
2. Stitch the snippets into a summary block.
3. Return Brief with mode `"offline"` and the matching disclaimer.

Both paths return the same `Brief` shape — callers don't need to branch.
Mode is informational; useful for telemetry ("what % of briefs ran
offline?").

---

## Example

### Request

```json
{
  "question": "What's the FIU-IND timeline for filing a Suspicious Transaction Report?",
  "sources": ["fiu_ind"],
  "max_steps": 0
}
```

### Brief (offline mode)

```json
{
  "question": "What's the FIU-IND timeline for filing a Suspicious Transaction Report?",
  "summary": "[fiu_ind] FIU-IND requires STR within 7 days, CTR for cash ≥₹10L, CCR within 7 days.",
  "citations": [
    {"title": "FIU-IND reporting", "url": "https://fiuindia.gov.in/"}
  ],
  "steps": 1,
  "mode": "offline",
  "disclaimer": "Offline deterministic synthesis (no LLM). Suitable for sandbox and CI; for production research enable an LLM provider."
}
```

---

## FREE-AI alignment

- **Rec 18 (Disclosure)** — Disclaimer cites the synthesis mode and warns about source verification.
- **Rec 25 (Disclosures)** — citations are mandatory; the agent will not return an uncited claim.
- **Rec 24 (Audit Framework)** — every step of the ReAct trace is in the OTel span tree.

---

## Integration

### Triggered by

- A back-office UI asking "what's the regulatory position on X?"
- An auditor agent that needs to ground a critique in a regulatory citation.
- An LLM-as-judge step inside `agents/auditor` that wants to refute a claim.

### Hands off to

- `financial_supervisor` for the final brief dispatch.
- `agents/reporter` for plain-language framing.

### Wraps

- `pkg/reasoning.ReAct` — the loop.
- `pkg/llm.Provider` — the LLM (optional).

---

## Anti-patterns

1. **Calling `New(nil, "", nil)` and assuming it's a real research engine.** It's a deterministic stub — fine for CI, not for customer-facing answers.
2. **Skipping the citation field.** The Disclaimer alone isn't enough; the citation is the audit hook.
3. **Setting `MaxSteps` very high.** Each step is an LLM round-trip; budget accordingly. The default 4 is empirically a good tradeoff.
4. **Passing a Resolver that hits a live web search without rate-limiting.** Wrap it in a circuit breaker.

---

## Testing

`agents/deep_research/deep_research_test.go` covers:

- Offline mode with default corpora (returns 3 citations)
- Offline mode with filtered corpora (returns N citations)
- HandleMessage dispatch
- Custom Resolver invocation
- Disclaimer presence
- RiskLevel = Medium

The ReAct path itself is exercised by `pkg/reasoning`'s own tests with a
mock provider.

Run:

```bash
go test ./agents/deep_research/ -v
```

---

## References

- [ReAct paper (Yao et al. 2022)](https://arxiv.org/abs/2210.03629)
- [RBI Master Directions index](https://rbi.org.in/Scripts/BS_ViewMasDirections.aspx)
- [Sahamati AA specs](https://sahamati.org.in/)
- [FIU-IND reporting guide](https://fiuindia.gov.in/)
