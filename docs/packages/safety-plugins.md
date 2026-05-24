# pkg/safety — Pluggable plugin chain

> **Where:** `pkg/safety/plugin.go` · **Lines of code:** ~180 · **Tests:** 8
> **Inspired by:** Google ADK `safety-plugins`

---

## Overview

The existing `pkg/safety.Detector` interface is single-implementation
per axis (jailbreak, topic, toxicity, bias). The pattern adopted here
lets a host stack arbitrary detectors — including third-party "shields"
like **Model Armor**, **AWS Bedrock Guardrails**, **Lakera Guard** —
behind one uniform plugin interface.

Three things live in this file:

1. **`Plugin`** — `Detector` + a Name and Stage so the chain can be reasoned about.
2. **`Registry`** — a small map for hosts to register/replace plugins by name.
3. **`Chain`** — a `Detector` that fans out across plugins and aggregates verdicts.

Plus **`HTTPShield`** — the template adapter for talking to an external
safety service.

All existing detectors keep working unchanged. The plugin interface
just gives them a name and a stage.

---

## Surface

```go
type Stage string
const (
    StageInbound  Stage = "inbound"   // user → system
    StageInternal Stage = "internal"  // agent → agent
    StageOutbound Stage = "outbound"  // system → user
    StageAny      Stage = "any"
)

type Plugin interface {
    Detector       // existing single-method interface
    Name() string
    Stage() Stage
}

type NamedDetector struct {
    N string
    S Stage
    D Detector
}

type Registry struct { ... }
func NewRegistry() *Registry
func (r *Registry) Register(p Plugin) error
func (r *Registry) Get(name string) (Plugin, bool)
func (r *Registry) List(stage Stage) []Plugin

type ChainMode int
const (
    ModeFirstFlagged ChainMode = iota
    ModeWorstScore
)

type Chain struct {
    Plugins []Plugin
    Mode    ChainMode
}
func (c Chain) Inspect(ctx, text) (Verdict, error)
func (c Chain) PluginNotes(ctx, text) string

type HTTPShield struct {
    N      string
    S      Stage
    Caller func(ctx context.Context, text string) (Verdict, error)
}
```

---

## Why two modes

`ModeFirstFlagged` — short-circuit on the first flagged hit. Used in
hot-path inbound scoring where speed matters more than full attribution.
The verdict's `Reason` carries `"blocked by <plugin>"`.

`ModeWorstScore` — run all plugins, return the worst (highest-score or
flagged-without-score) verdict. Used in outbound audit paths where you
want to see every plugin's verdict in the log even when one already
fired. `PluginNotes()` returns per-plugin attribution as a string.

---

## Stages

Stages let a chain filter plugins:

- **Inbound** — score user input (jailbreak detection, prompt injection)
- **Internal** — score agent-to-agent messages (output-schema validation)
- **Outbound** — score system replies to the user (toxicity, PII leak)
- **Any** — applies regardless of direction

A host wires three chains — one per direction — and reads from the
registry filtered by stage.

---

## Wiring example

```go
reg := safety.NewRegistry()

// Wrap the existing heuristic detector as a Plugin.
reg.Register(safety.NamedDetector{
    N: "jailbreak_heuristic", S: safety.StageInbound,
    D: safety.HeuristicJailbreak{},
})

// Wrap the LLM-as-judge detector.
reg.Register(safety.NamedDetector{
    N: "jailbreak_llm", S: safety.StageInbound,
    D: safety.NewLLMJailbreak(llmProvider, "llama3.2:1b"),
})

// Plug Model Armor via HTTPShield.
reg.Register(safety.HTTPShield{
    N: "model_armor", S: safety.StageInbound,
    Caller: func(ctx context.Context, text string) (safety.Verdict, error) {
        return callModelArmor(ctx, text) // host implementation
    },
})

inboundChain := safety.Chain{
    Plugins: reg.List(safety.StageInbound),
    Mode:    safety.ModeFirstFlagged,
}

// Use it like any Detector.
v, _ := inboundChain.Inspect(ctx, userPrompt)
if v.Flagged {
    // block
}
```

---

## HTTPShield

Template adapter for a third-party shield service. The host implements
the `Caller` function (HTTP POST + JSON parse). The shield is then a
first-class `Plugin`.

This keeps the third-party SDK out of the safety package — only the
host application has the SDK dependency.

---

## What it does NOT do

- **No streaming**. A plugin returns one verdict per text. Streaming
  safety scoring (per token / per chunk) needs a separate interface.
- **No async**. `Inspect` is synchronous; for high latency shields wrap
  the chain in a goroutine and a circuit breaker.
- **No write mutations**. Plugins observe and verdict only — they don't
  mutate the message. To redact, use a separate transform (e.g.
  `governance.PIIBlockPolicy` already does conditional drop).

---

## FREE-AI alignment

- **Rec 19 (Cybersecurity)** — pluggable shields are how a bank's security org owns the safety surface without re-shipping Genie.
- **Rec 26 (AI Toolkit)** — the chain + registry IS the toolkit's safety section.

---

## Anti-patterns

1. **Adding a chatty external shield in `ModeFirstFlagged` ahead of cheap heuristics.** The cheap one short-circuits; the expensive one rarely runs. If you want all attribution, switch to `ModeWorstScore`.
2. **Forgetting to wire stages**. A plugin registered with `StageOutbound` won't fire on inbound; verify the registry's `List(stage)` returns what you expect.
3. **Holding the registry mutex during a network call**. The `Caller` runs outside the registry — keep it that way.

---

## Testing

`pkg/safety/plugin_test.go` covers:

- Registry register + get
- Registry rejects missing name
- Stage filter (inbound + any = 2 results)
- Chain first-flagged short-circuits with attribution in `Reason`
- Chain worst-score runs all and picks highest
- HTTPShield delegates to Caller
- HTTPShield requires Caller (errors without)
- NamedDetector wraps existing detectors transparently

Run:

```bash
go test ./pkg/safety/ -v
```

---

## References

- [Google Model Armor](https://cloud.google.com/blog/products/ai-machine-learning) — Google's safety shield service
- [AWS Bedrock Guardrails](https://aws.amazon.com/bedrock/guardrails/) — AWS analogue
- [Lakera Guard](https://www.lakera.ai/) — third-party prompt injection / PII shield
- [Genie governance composite](../../pkg/governance/policy.go) — the system-level policy chain
