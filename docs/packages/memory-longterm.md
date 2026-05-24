# pkg/memory — LongTermMemory tier

> **Where:** `pkg/memory/longterm.go` · **Lines of code:** ~140 · **Tests:** 6
> **Inspired by:** Google ADK `memory-bank`

---

## Overview

Adds a **third memory tier** to `pkg/memory`:

| Tier | Lifespan | Storage shape | Use |
|---|---|---|---|
| Short-term (existing — `EpisodicMemory`) | One session, rolling buffer | conversation Episodes | "what did the user just say?" |
| Semantic (existing — `SemanticMemory`) | Cross-session, per-user vector store | embedded text | "have we seen anything like this before?" |
| **Long-term (new — `LongTermMemory`)** | **Forever (append-only)** | **structured facts** | **"what do we *know* about this user?"** |

Examples of long-term facts:

- "Primary bank: HDFC"
- "Risk appetite: moderate"
- "Monthly net inflow ≈ ₹1.2L"
- "Has 2 dependents"

These are durable, **append-only**, supersede-aware records that survive
across sessions. An offline consolidator job reads short-term +
semantic memory and proposes new facts; the long-term store records
them and marks any older fact for the same key as superseded.

---

## Surface

```go
type Fact struct {
    Key          string    // canonical key (e.g. "primary_bank")
    Value        string    // human-readable value
    Confidence   float64   // 0..1
    Source       string    // free-text provenance
    RecordedAt   time.Time
    SupersededAt *time.Time // nil = current
}

type LongTermMemory struct { ... }

func NewLongTermMemory() *LongTermMemory

func (m *LongTermMemory) Record(userID string, f Fact)
func (m *LongTermMemory) Current(userID, key string) (Fact, bool)
func (m *LongTermMemory) CurrentAll(userID string) []Fact
func (m *LongTermMemory) History(userID, key string) []Fact
func (m *LongTermMemory) SearchValue(userID, query string) []Fact

type Consolidator interface {
    Consolidate(ctx context.Context, userID string) ([]Fact, error)
}
func (m *LongTermMemory) Apply(ctx context.Context, userID string, c Consolidator) (int, error)
```

---

## Design choices

### Append-only

`Record` never overwrites. Adding a new Fact with the same `Key` for a
user marks any previously-current fact for that key as superseded (sets
`SupersededAt = now`). `History(userID, key)` returns the full chain.

Why: audit. "What did we think the user's primary bank was on May 14?"
is a query, not a forensic exercise.

### Per-user partitioning

Facts are partitioned per `userID` so a buggy consolidator can't leak
across users. The map is keyed `userID → []Fact`.

### Consolidator interface

The reference implementation ships *no* consolidator. The policy team
should own the rules; engineering ships the scaffolding. Hosts wire a
consolidator that runs offline (a cron or Kafka consumer) and calls
`Apply(ctx, userID, consolidator)`.

A simple consolidator might read the user's last 60 days of
transactions, infer their primary bank by the account that received the
most credits, and record `Fact{Key:"primary_bank", Value:"HDFC", Confidence:0.85, Source:"60d txn analysis"}`.

---

## Usage example

```go
ltm := memory.NewLongTermMemory()

ltm.Record("alice", memory.Fact{
    Key: "primary_bank", Value: "HDFC", Confidence: 0.85, Source: "60d txn analysis",
})

// Later, after a job swaps to ICICI:
ltm.Record("alice", memory.Fact{
    Key: "primary_bank", Value: "ICICI", Confidence: 0.92, Source: "90d txn analysis",
})

current, _ := ltm.Current("alice", "primary_bank")
// → Fact{Value:"ICICI", ...}

history := ltm.History("alice", "primary_bank")
// → 2 facts, oldest first; the HDFC one has SupersededAt set
```

---

## Thread safety

`LongTermMemory` uses an `sync.RWMutex`. Reads (`Current`, `CurrentAll`,
`History`, `SearchValue`) take a read lock; writes (`Record`, `Apply`)
take a write lock. Safe for concurrent use across goroutines.

---

## What it does NOT do

- **No persistence**. The reference implementation is in-memory.
  Production: implement `LongTermMemory` over Postgres
  (`facts(user_id, key, value, confidence, source, recorded_at,
  superseded_at)` with an index on `(user_id, key)`).
- **No vector search**. Use `SemanticMemory` for fuzzy similarity.
- **No automatic consolidation**. Hosts must run a consolidator job;
  the store is otherwise passive.

---

## Integration with agents

- **Reads** — any agent can pull facts via the environment if you
  extend `agent.Environment` with a `LongTerm()` getter. Today the
  reference shipped is minimal; extend as needed.
- **Writes** — the recommended pattern is offline consolidator only.
  Agents emitting facts mid-pipeline mixes auditability concerns.

---

## FREE-AI alignment

- **Rec 23 (Audit Framework)** — the append-only history is the audit hook.
- **Rec 25 (Disclosures)** — `Source` on every Fact lets the system tell the user "we think your primary bank is HDFC because of 60-day transaction analysis."

---

## Anti-patterns

1. **Using LongTerm for short-term scratch.** That's what `EpisodicMemory` is for.
2. **Overwriting via `Record` with the assumption it replaces.** It supersedes — the old fact is still in history.
3. **Skipping the `Source` field.** Without provenance the consolidator's logic is opaque to audit.
4. **Running a consolidator on every message.** It's an offline job for a reason; running it inline kills hot-path latency and creates contention.

---

## Testing

`pkg/memory/longterm_test.go` covers:

- Record + Current round-trip
- Supersede semantics (history retains both)
- Per-user isolation
- SearchValue substring match
- CurrentAll skips superseded
- Apply with a stub consolidator

Run:

```bash
go test ./pkg/memory/ -v
```

---

## References

- [Google ADK memory-bank](https://github.com/google/adk-samples/tree/main/python/agents/memory-bank) — the pattern source
- [Existing pkg/memory.SemanticMemory](../../pkg/memory/semantic.go) — for fuzzy similarity over user text
- [Existing pkg/memory.EpisodicMemory](../../pkg/memory/episodic.go) — for conversation buffer + LLM summarisation
