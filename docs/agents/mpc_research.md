# agents/mpc_research

> **Risk class:** Low · **Capability:** `analyse_mpc_event` · **In:** `mpc_summary` · **Out:** `mpc_signal`
> **Inspired by:** Google ADK `fomc-research`, Indianised to RBI MPC mechanics.

---

## Overview

Analyses **RBI Monetary Policy Committee (MPC)** statements. The Indian
analogue of the US FOMC research agent in the ADK samples — adapted for
the MPC calendar (six bi-monthly meetings per FY), the Indian rate stack
(repo, SDF, MSF, CRR, SLR), and RBI's communication style.

The agent does not parse free-text minutes; it takes a structured
`Summary` (which the host can extract from the press release via a thin
adapter) and computes diff-based signals: rate change in bps, stance
shift, hawkishness delta, surprise vs market consensus, projection
revisions.

The most useful output isn't the headline — it's the
**DownstreamHints** array that names the agents which should recompute
on the back of this signal.

---

## Constants

```go
const (
    ID         = "mpc_research"
    Capability = "analyse_mpc_event"
    TypeIn     = "mpc_summary"
    TypeOut    = "mpc_signal"
    NextAgent  = "financial_supervisor"
)
```

---

## Types

### Summary (one MPC outcome)

```go
type Summary struct {
    MeetingDate         string  // YYYY-MM-DD
    RepoRate            float64 // %
    SDFRate             float64
    MSFRate             float64
    CRRPct              float64
    SLRPct              float64
    Stance              string  // "accommodative" | "neutral" | "withdrawal_of_accommodation"
    VoteFor             int     // out of 6
    VoteAgainst         int
    CPIProjectionPctYoY float64
    GDPProjectionPctYoY float64
    ConsensusRepoBps    int     // market expectation
}
```

### Request

```go
type Request struct {
    Current  Summary
    Previous Summary
}
```

### Signal (outbound)

```go
type Signal struct {
    MeetingDate     string
    RepoChangeBps   int
    StanceShift     string   // "easing" | "neutral" | "tightening" | "unchanged"
    HawkishnessΔ    string   // "more_hawkish" | "more_dovish" | "unchanged"
    SurpriseVsMkt   string   // "hawkish_surprise" | "dovish_surprise" | "in_line"
    CPIRevisionBps  int
    GDPRevisionBps  int
    DownstreamHints []string
    Headline        string
    Disclaimer      string
}
```

---

## Classification logic

### Rate change

```
repoBps = (current.RepoRate - previous.RepoRate) * 100
```

### Stance shift

- `accommodative` ← previous wasn't → `easing`
- `withdrawal_of_accommodation` ← previous wasn't → `tightening`
- `neutral` ← previous wasn't → `neutral`
- else → `unchanged`

### Hawkishness delta

- `more_hawkish` if `repoBps > 0` OR `cpiBps > 25`
- `more_dovish` if `repoBps < 0` OR `cpiBps < -25`
- `unchanged` otherwise

### Surprise vs market

- `delta = repoBps - consensus`
- `hawkish_surprise` if delta > 15bps
- `dovish_surprise` if delta < -15bps
- `in_line` otherwise

### Downstream hints

Generated automatically based on what changed:

| When | Hint |
|---|---|
| `repoBps != 0` | `rate_watcher: refresh published rates` |
| `repoBps != 0` | `loan_advisor: reprice floating-rate EMI projections` |
| `repoBps != 0` | `prepayment_advisor: recompute effective APR rankings` |
| `cpiBps > 25` | `macro_research: flag inflation upside risk` |
| `gdpBps < -25` | `macro_research: flag growth downside risk` |

These are *advisory*. A supervisor agent can subscribe to `mpc_signal`
events and use the hints to schedule fan-out.

---

## Example

### Request

```json
{
  "current": {
    "meeting_date": "2026-02-07",
    "repo_rate_pct": 6.75,
    "stance": "withdrawal_of_accommodation",
    "cpi_projection_pct": 4.8,
    "gdp_projection_pct": 6.6,
    "consensus_repo_change_bps": 0
  },
  "previous": {
    "meeting_date": "2025-12-06",
    "repo_rate_pct": 6.50,
    "stance": "neutral",
    "cpi_projection_pct": 4.5,
    "gdp_projection_pct": 6.8
  }
}
```

### Signal

```json
{
  "meeting_date": "2026-02-07",
  "repo_change_bps": 25,
  "stance_shift": "tightening",
  "hawkishness_delta": "more_hawkish",
  "surprise": "hawkish_surprise",
  "cpi_revision_bps": 30,
  "gdp_revision_bps": -20,
  "downstream_hints": [
    "rate_watcher: refresh published rates",
    "loan_advisor: reprice floating-rate EMI projections",
    "prepayment_advisor: recompute effective APR rankings",
    "macro_research: flag inflation upside risk"
  ],
  "headline": "MPC raised repo by 25bps on 2026-02-07; stance withdrawal_of_accommodation.",
  "disclaimer": "Algorithmic summary of MPC statement; refer to the RBI press release for the authoritative text. Not investment advice."
}
```

---

## FREE-AI alignment

- **Rec 18 (Disclosure)** — every Signal carries a disclaimer naming the source and disclaiming investment advice.

---

## Integration

### Triggered by

- A scheduled job on the morning of the next MPC announcement (host adapter scrapes the press release).
- An on-demand request from a research agent: "What did the last MPC do?"

### Hands off to

- `financial_supervisor` — publishes the signal.
- Downstream subscribers (loan/rate/prepayment) — listen for `mpc_signal` and recompute.

---

## Anti-patterns

1. **Treating the headline as investment advice.** It's a structured diff, not a buy/sell call. The Disclaimer is mandatory.
2. **Skipping the consensus field.** Without `ConsensusRepoBps`, the surprise signal is always `in_line` — a hawkish hold can look unchanged.
3. **Re-running on every news article.** This agent expects clean structured input from the official RBI press release, not media reports.

---

## Testing

`agents/mpc_research/mpc_research_test.go` covers:

- 25bps hike with stance change → hawkish surprise
- 25bps cut with in-line consensus → easing
- Hold with no surprise → unchanged headline
- Rate move fans out to ≥3 downstream agents
- CPI upside hint
- HandleMessage dispatch
- Disclaimer presence

Run:

```bash
go test ./agents/mpc_research/ -v
```

---

## References

- [RBI Monetary Policy](https://rbi.org.in/Scripts/AnnualPolicy.aspx) — schedule, statements, minutes
- [RBI Liquidity Adjustment Facility](https://rbi.org.in/) — for the SDF / MSF / repo stack
- [Bloomberg MPC consensus survey](https://www.bloomberg.com/asia) — for `ConsensusRepoBps`
