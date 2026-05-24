# pkg/agent — SkillRegistry (progressive disclosure)

> **Where:** `pkg/agent/skill.go` · **Lines of code:** ~140 · **Tests:** 6
> **Inspired by:** Google ADK `agent-skills-tutorial`

---

## Overview

The problem: a hierarchical supervisor that fronts 40+ sub-agents and
100+ tools surfaces *all* of them to the LLM in every prompt. Result —
context bloat, slow routing, worse decisions.

The fix: agents declare **Skills**, each with a short description and a
**lazy tool manifest**. The supervisor lists the *skill titles* in the
initial prompt; only when the LLM selects a skill does the toolset for
that skill get materialised and surfaced.

This file adds the data model + a small registry. Existing agents can
opt in by implementing `SkillProvider` — non-implementers keep working
unchanged.

---

## Surface

```go
type Skill struct {
    ID      string  // machine-routable, e.g. "tax_planning_in"
    Title   string  // short human label
    Summary string  // one-sentence description (<= 200 chars)
    Tools   func(ctx context.Context) []SkillTool  // lazy!
}

type SkillTool struct {
    Name        string
    Description string
    Run         func(ctx context.Context, input string) (string, error)
}

type SkillProvider interface {
    Skills() []Skill
}

func SkillsOf(a Agent) []Skill // returns nil if a is not a provider

type SkillRegistry struct { ... }
func NewSkillRegistry() *SkillRegistry
func (r *SkillRegistry) RegisterAgent(a Agent) (int, error)
func (r *SkillRegistry) List() []Skill           // sorted by Title
func (r *SkillRegistry) Manifest() []string      // "id — title — summary" per skill
func (r *SkillRegistry) Invoke(ctx, id) ([]SkillTool, error)
func (r *SkillRegistry) OwnerOf(id) (string, bool)
```

---

## Why "lazy" tools

`Skill.Tools` is `func(ctx) []SkillTool`, not `[]SkillTool`. The supervisor's
initial prompt only shows the manifest — `ID · Title · Summary` per
skill — which is cheap, both in tokens and in agent setup time.

Only when the LLM picks a skill does the supervisor call
`registry.Invoke(ctx, "tax_planning_in")` to materialise that skill's
tools. The other 99 % of the toolbelt never appears in any prompt.

---

## Adoption pattern

A specialist agent that wants to opt in:

```go
type TaxAgent struct{}

// Existing Agent interface methods unchanged.
func (TaxAgent) ID() string             { return "tax-bot" }
func (TaxAgent) Name() string           { return "Tax Bot" }
func (TaxAgent) Capabilities() []string { return []string{"tax"} }
func (TaxAgent) HandleMessage(ctx context.Context, msg Message, env Environment) ([]Message, error) {
    // ... real logic
}

// New — SkillProvider implementation.
func (TaxAgent) Skills() []Skill {
    return []Skill{
        {
            ID: "tax_planning_in", Title: "India tax planning",
            Summary: "Old vs new regime, 80C ceilings, advance tax.",
            Tools: func(_ context.Context) []SkillTool {
                return []SkillTool{
                    {Name: "compare_regimes", Description: "Compute net tax under old vs new"},
                    {Name: "ceiling_check", Description: "Validate Chapter VI-A claim against ceiling"},
                }
            },
        },
        {
            ID: "tax_harvest_in", Title: "STCG/LTCL harvesting",
            Summary: "Identify equity lots to harvest before FY end.",
            // No tools — pure delegation back to the agent's HandleMessage.
        },
    }
}
```

The supervisor:

```go
reg := agent.NewSkillRegistry()
for _, a := range registry.List(ctx) {
    _, _ = reg.RegisterAgent(a)  // non-providers contribute 0 skills
}

manifest := reg.Manifest()
// build prompt with manifest lines

// LLM responds: "I'll use tax_planning_in"
tools, _ := reg.Invoke(ctx, "tax_planning_in")
// build a follow-up prompt with just those tools
```

---

## Collision detection

`RegisterAgent` returns an error if two agents try to register the same
Skill ID. The first owner wins; the second registration errors out
without partial state. Use unique, namespaced IDs (`<domain>_<verb>_<region>`).

---

## What it does NOT do

- **No LLM tool-call dispatch**. The registry just hands you the tools —
  the supervisor chooses how to surface them (function calling, JSON
  schema, prompt template).
- **No persistence**. Registries are in-memory; rebuild at boot.
- **No versioning**. If you change a skill's contract, change the ID.

---

## FREE-AI alignment

- **Rec 23 (AI Inventory)** — the manifest is a *capabilities* inventory at the skill grain (one level above agent IDs).
- **Rec 24 (Audit Framework)** — every Invoke call records which skill the LLM picked → trace shows the routing decision.

---

## Anti-patterns

1. **Putting full tool descriptions in `Summary`.** Defeats the purpose. Summary is for the LLM's *routing* decision; descriptions go on the materialised SkillTool.
2. **Computing tools at registration time.** Use the `Tools func(ctx)` indirection so expensive setup (LLM provider init, network calls) only happens when the skill is invoked.
3. **Skills with no Tools and no HandleMessage path.** Dead routes. Either delegate back to the agent or supply tools.

---

## Testing

`pkg/agent/skill_test.go` covers:

- `SkillsOf` returns skills for providers, nil for non-providers
- Registry registers agents, returns expected manifest
- `Invoke` materialises tools
- Unknown skill errors
- `OwnerOf` lookup
- Non-provider registration is a 0-skill noop

Run:

```bash
go test ./pkg/agent/ -v
```

---

## References

- [Google ADK agent-skills-tutorial](https://github.com/google/adk-samples/tree/main/python/agents/agent-skills-tutorial) — the pattern source
- [Anthropic Model Context Protocol](https://modelcontextprotocol.io/) — for cross-process skill discovery (similar idea, different scope)
