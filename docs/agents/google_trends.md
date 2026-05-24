# agents/google_trends

> **Risk class:** Low · **Capability:** `search_trend_signal` · **In:** `trend_request` · **Out:** `trend_signal`
> **Inspired by:** Google ADK `google-trends-agent`.

---

## Overview

Surfaces consumer-search-interest signals that feed `agents/macro_research`
(e.g. surging "EV loans" → flag for the macro brief) and
`agents/mf_screener` (surging interest in a fund family → flag for the
screener).

Transport-agnostic: takes a `TrendFetcher` implementation (host wires
the real google-trends client, a BigQuery dataset, or a stub) and
returns a normalised signal that downstream agents consume.

---

## Constants

```go
const (
    ID         = "google_trends"
    Capability = "search_trend_signal"
    TypeIn     = "trend_request"
    TypeOut    = "trend_signal"
    NextAgent  = "financial_supervisor"

    surgeMultiple = 1.5 // ≥ → "surging"
    fadeMultiple  = 0.6 // ≤ → "fading"
)
```

---

## Types

### Series (one keyword's interest history)

```go
type Series struct {
    Keyword string
    Geo     string  // "IN", "IN-MH", etc.
    Points  []int   // 0..100 Trends-normalised score, oldest first
}
```

### Request

```go
type Request struct {
    Geo      string
    Keywords []string
    WindowN  int  // size of "latest" window; rest is baseline
}
```

### Signal (per keyword)

```go
type Signal struct {
    Keyword          string
    Direction        string  // "surging" | "fading" | "steady"
    LatestMean       float64
    BaselineMean     float64
    ChangeMultiple   float64
    NoteToDownstream string
}
```

### Response

```go
type Response struct {
    Geo        string
    Signals    []Signal  // sorted by ChangeMultiple desc
    Hints      []string  // fan-out hints for other agents
    Disclaimer string
}
```

### Fetcher (pluggable)

```go
type TrendFetcher interface {
    Fetch(ctx context.Context, geo string, keywords []string) ([]Series, error)
}
```

---

## Classification logic

For each Series of length `N` and window `W`:

```
baseline = mean(Points[:N-W])
latest   = mean(Points[N-W:])
multiple = latest / baseline
```

- `multiple >= 1.5` → `surging`
- `multiple <= 0.6` → `fading`
- otherwise → `steady`

Sort signals by `ChangeMultiple` descending so the most actionable
surges come first.

---

## Downstream hints

| Direction | Hints produced |
|---|---|
| `surging` | `macro_research: flag surge — <keyword>`, `mf_screener: rescore theme — <keyword>` |
| `fading` | `macro_research: flag fade — <keyword>` |
| `steady` | (none) |

---

## Example

### Request

```json
{
  "geo": "IN",
  "keywords": ["ev loan", "metaverse"],
  "window_points": 3
}
```

### Response (with stub fetcher producing 6-point series each)

```json
{
  "geo": "IN",
  "signals": [
    {
      "keyword": "ev loan",
      "direction": "surging",
      "latest_mean": 30.0,
      "baseline_mean": 10.0,
      "change_multiple": 3.0,
      "note_to_downstream": "Search interest in this term has surged versus baseline; investigate macro driver"
    },
    {
      "keyword": "metaverse",
      "direction": "fading",
      "latest_mean": 20.0,
      "baseline_mean": 80.0,
      "change_multiple": 0.25,
      "note_to_downstream": "Search interest has cooled materially versus baseline"
    }
  ],
  "hints": [
    "macro_research: flag surge — ev loan",
    "mf_screener: rescore theme — ev loan",
    "macro_research: flag fade — metaverse"
  ],
  "disclaimer": "Google-Trends-derived interest signal. Not investment advice; correlation with actual market moves is variable."
}
```

---

## FREE-AI alignment

- **Rec 18 (Disclosure)** — Disclaimer disclaims investment advice.

---

## Integration

### Triggered by

- A scheduled "morning trends" job per region.
- An on-demand request: "Show me surging finance keywords this week."

### Hands off to

- `agents/macro_research` — to inflect the weekly macro brief.
- `agents/mf_screener` — to nudge thematic scoring.

### Wraps

- Pluggable `TrendFetcher` (host concern).

---

## Anti-patterns

1. **Using this as a trading signal.** It's a sentiment / interest signal. Correlation with market returns is weak and noisy.
2. **Setting `WindowN` too small.** A 1-point window is noise. Default to ≥3.
3. **Ignoring geo.** "EV loan" trends differently in Delhi vs Mumbai vs Bengaluru. Fetch per `IN-<state>` if you care.

---

## Testing

`agents/google_trends/google_trends_test.go` covers:

- Surging classification (3× baseline)
- Fading classification (0.25× baseline)
- Steady classification (flat)
- Short-series safety (returns steady)
- Analyse produces hints + sorts by multiple
- No-fetcher error path
- HandleMessage dispatch
- Disclaimer presence

Run:

```bash
go test ./agents/google_trends/ -v
```

---

## References

- [Google Trends](https://trends.google.com/trends/?geo=IN) — the canonical data source
- [BigQuery `google_trends.top_terms` dataset](https://console.cloud.google.com/marketplace/product/bigquery-public-data/google-trends) — for batch processing
