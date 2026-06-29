# Evaluation Analysis & Semantic Search Guide

This guide covers the semantic search tool, clustering analysis, and EDA (Exploratory Data Analysis) notebook for the Genie evaluation framework.

## Overview

The evaluation analysis pipeline enables:

1. **Semantic Search** — Find traces by semantic similarity (e.g., "settlement amount too high" → top 10 similar traces)
2. **Clustering Analysis** — Group traces by failure mode, domain, agent, severity, customer segment, and semantic similarity
3. **EDA Notebook** — Visualize distributions, correlations, judge agreement, and confidence intervals

## Quick Start

### 1. Generate Evaluation Results

First, run the evaluation harness to generate results:

```bash
cd /Users/genesis/Developer/goworkspace/src/github.com/c2siorg/genie

# Run multi-turn evaluation
go run ./cmd/eval-multiturn -export eval_results.json

# Or run single-turn evaluation
go run ./cmd/eval-singleturn -export eval_results.json
```

This creates `eval_results.json` with structure:
```json
[
  {
    "trace_id": "trace-001",
    "failure_mode": "FM-SE-001",
    "error_message": "settlement amount exceeds merchant daily limit",
    "severity": "high",
    "agent": "settlement_executor",
    "order_id": "ord-123",
    "customer_segment": "sme",
    "domain": "settlement",
    "root_cause_gulf": 15,
    "confidence": 85,
    "tool_sequence": "verify_settlement -> apply_netting",
    "compliance_gap": "velocity_exceeded",
    "settlement_amount": "5000",
    "aml_risk_score": "42"
  },
  ...
]
```

### 2. Semantic Search

Find traces similar to a natural-language query:

```bash
# Find traces related to settlement amount issues
go run ./cmd/eval-semantic-search \
  -query "settlement amount too high" \
  -topk 10 \
  -dataset eval_results.json \
  -output search_results.json \
  -format json

# View results in CSV format
go run ./cmd/eval-semantic-search \
  -query "aml compliance failure" \
  -topk 20 \
  -dataset eval_results.json \
  -output search_results.csv \
  -format csv

# Print results to stdout
go run ./cmd/eval-semantic-search \
  -query "compliance gap velocity" \
  -topk 10 \
  -dataset eval_results.json \
  -output "" \
  -format text
```

**Output (JSON):**
```json
{
  "query": "settlement amount too high",
  "topk": 10,
  "result_count": 8,
  "results": [
    {
      "trace_id": "trace-042",
      "metadata": {
        "trace_id": "trace-042",
        "failure_mode": "FM-SE-002",
        "error_message": "settlement amount exceeds daily limit for merchant ABC",
        "severity": "high",
        "agent": "settlement_executor",
        ...
      },
      "similarity_score": 0.8234,
      "matching_fields": ["error_message", "settlement_amount"],
      "explanation": "Failure mode: FM-SE-002 | Error: settlement amount exceeds daily limit... | Severity: high | High semantic match"
    },
    ...
  ]
}
```

### 3. Clustering Analysis

Group traces across multiple dimensions (Go API):

```go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/c2siorg/genie/pkg/eval/analysis"
)

func main() {
	// Load traces
	traces := []analysis.TraceRecord{
		{
			TraceID:         "trace-001",
			FailureMode:     "FM-SE-001",
			FailureDomain:   "settlement",
			FailureCategory: "operational",
			Severity:        "critical",
			Agent:           "settlement_executor",
			// ... other fields
		},
		// ... more traces
	}

	// Create analyzer
	analyzer := analysis.NewClusterAnalyzer(traces)

	// Cluster by failure mode
	modeClust := analyzer.ClusterByFailureMode()
	for mode, cluster := range modeClust {
		fmt.Printf("%s: %d traces\n", mode, cluster.Count)
		fmt.Printf("  Avg Confidence: %.1f%%\n", cluster.Metrics.AverageConfidence)
		fmt.Printf("  Severity Distribution: %v\n", cluster.Metrics.SeverityDistribution)
	}

	// Cluster by domain with sub-clustering
	domainClust := analyzer.ClusterByDomain()
	for domain, cluster := range domainClust {
		fmt.Printf("%s: %d traces\n", domain, cluster.Count)
		for cat, subcluster := range cluster.SubClusters {
			fmt.Printf("  %s: %d traces\n", cat, subcluster.Count)
		}
	}

	// Cross-tabulation: Domain vs Severity
	ct := analyzer.CrossTab("domain", "severity")
	fmt.Printf("Domain vs Severity:\n")
	for _, x := range ct.XLabels {
		for _, y := range ct.YLabels {
			count := ct.Counts[x][y]
			fmt.Printf("  %s x %s: %d\n", x, y, count)
		}
	}

	// Semantic clustering (70% token overlap)
	semanticClust := analyzer.ClusterBySemanticity(0.7)
	for clustID, cluster := range semanticClust {
		fmt.Printf("%s: %d traces (avg confidence: %.1f%%)\n",
			clustID, cluster.Count, cluster.Metrics.AverageConfidence)
	}

	// Export to JSON
	data, _ := json.MarshalIndent(modeClust, "", "  ")
	fmt.Println(string(data))
}
```

**Cluster Structure:**
```json
{
  "FM-SE-001": {
    "key": "FM-SE-001",
    "label": "FM-SE-001",
    "count": 12,
    "trace_ids": ["trace-001", "trace-042", ...],
    "metrics": {
      "average_severity": 3.5,
      "average_confidence": 82.3,
      "average_root_cause_gulf": 18.5,
      "severity_distribution": {
        "critical": 4,
        "high": 6,
        "medium": 2
      },
      "agent_distribution": {
        "settlement_executor": 8,
        "reconciliation_agent": 4
      },
      ...
    },
    "sub_clusters": {}
  },
  ...
}
```

### 4. EDA Jupyter Notebook

Comprehensive exploratory analysis with visualizations:

```bash
# Install dependencies
pip install pandas numpy matplotlib seaborn scipy

# Launch Jupyter
jupyter notebook notebooks/eval_eda.ipynb
```

The notebook includes:

1. **Data Loading** — Load `eval_results.json` into pandas DataFrame
2. **Failure Mode Distribution** — Top 15 failure modes (bar + pie chart)
3. **Domain & Category Analysis** — Distribution across system domains
4. **Severity & Confidence Analysis** — Distributions, scatter plots, confidence intervals
5. **Agent & Segment Analysis** — Which agents fail most? Customer segment patterns
6. **Cross-Tabulation Heatmaps** — Domain vs Severity, Category vs Agent
7. **Confidence Intervals** — 95% CI trends for severity and domain
8. **Judge Agreement Analysis** — Evaluator consistency by failure mode
9. **Correlation Analysis** — Relationships between confidence, gulf, severity
10. **Summary Report** — Quantitative findings and top issues

**Key Outputs:**
- `eval_eda_summary.json` — Summary statistics for reporting
- Heatmaps showing failure concentrations
- Confidence interval plots for metric credibility
- Judge agreement heatmap (confidence std by failure mode)

## Data Flow

```
Evaluation Harness (Go)
  ↓
eval_results.json
  ├─→ Semantic Search Tool (Go)
  │   └─→ search_results.json / .csv / .txt
  │
  ├─→ Clustering Analysis (Go)
  │   └─→ JSON clusters + metrics
  │
  └─→ EDA Notebook (Python/Jupyter)
      ├─→ Distributions
      ├─→ Heatmaps
      ├─→ Confidence Intervals
      └─→ eval_eda_summary.json
```

## Semantic Search Algorithm

The semantic search tool combines multiple similarity metrics:

### Metrics:
1. **Error Message Similarity (40% weight)**
   - Normalized Levenshtein distance
   - Measures character-level differences
   - Best for: "settlement amount exceeds limit" vs "settlement amount over limit"

2. **Failure Mode/Domain Similarity (30% weight)**
   - Jaccard similarity on tokenized text
   - Measures token-level overlap
   - Best for: "settlement" vs "settlement_executor"

3. **Severity/Compliance Similarity (20% weight)**
   - Token overlap on operational impact
   - Best for: "critical compliance" vs "critical_aml"

4. **Context Similarity (10% weight)**
   - Agent and tool sequence matching
   - Best for: "settlement_executor" vs "settlement_agent"

### Formula:
```
score = 0.4 × error_similarity 
      + 0.3 × failure_similarity 
      + 0.2 × severity_similarity 
      + 0.1 × context_similarity
```

All similarity functions return [0, 1] where 1 = perfect match.

### Example:
```
Query: "settlement amount too high"

Trace A: "settlement amount exceeds daily limit"
  - Error sim: 0.92 (high char overlap)
  - Failure sim: 0.85 (token overlap: settlement)
  - Severity sim: 0.70 (both mention limits)
  - Score: 0.40×0.92 + 0.30×0.85 + 0.20×0.70 + 0.10×0.0 = 0.816

Trace B: "AML compliance check failed"
  - Error sim: 0.15 (low char overlap)
  - Failure sim: 0.10 (no token overlap)
  - Severity sim: 0.20 (different domain)
  - Score: 0.40×0.15 + 0.30×0.10 + 0.20×0.20 + 0.10×0.0 = 0.140
```

## Clustering Algorithms

### Failure Mode Clustering
Groups traces by their FM-DOMAIN-NNN identifier.

### Domain Clustering
Groups by settlement, compliance, orchestration, etc. with category sub-clustering.

### Semantic Clustering
Token-based Jaccard similarity with configurable threshold:
- **0.9+**: Nearly identical error messages
- **0.7-0.9**: Strong semantic similarity
- **0.5-0.7**: Moderate overlap
- **<0.5**: Weak relationship

## Metrics Explained

### Average Severity
Numeric scale: critical=4, high=3, medium=2, low=1
- 3.5 = mostly high with some critical
- 2.0 = mostly medium

### Average Confidence
0-100% scale. Measures evaluator agreement:
- 90%+ = strong consensus
- 70-90% = moderate confidence
- <70% = uncertain classification

### Root Cause Gulf
0-100% metric. Measures distance between symptom and root cause:
- 0% = exact root cause identified
- 50% = symptom and root cause equally unclear
- 100% = only symptom visible, root cause unknown

## Integration Examples

### Example 1: Find all settlement amount issues
```bash
go run ./cmd/eval-semantic-search \
  -query "settlement amount exceeds limit" \
  -topk 50 \
  -dataset eval_results.json \
  -output settlement_issues.json
```

### Example 2: Analyze semantic clusters
```go
analyzer := analysis.NewClusterAnalyzer(traces)
semanticClust := analyzer.ClusterBySemanticity(0.7)
// Export clusters for Python visualization
```

### Example 3: Multi-dimensional analysis
```python
# In Jupyter notebook
ct = df.groupby(['domain', 'severity']).size().unstack(fill_value=0)
sns.heatmap(ct, annot=True, fmt='d', cmap='YlOrRd')
```

## Troubleshooting

### Issue: Semantic search returns no results
**Solution**: Lower the similarity threshold or broaden the query
```bash
# Try shorter, more general query
go run ./cmd/eval-semantic-search \
  -query "settlement compliance" \
  -topk 20
```

### Issue: Clustering produces too many micro-clusters
**Solution**: Use higher similarity threshold or filter small clusters
```go
// Only keep clusters with 5+ traces
for _, cluster := range semanticClust {
    if cluster.Count >= 5 {
        // Process cluster
    }
}
```

### Issue: Jupyter notebook kernel crashes
**Solution**: Reduce dataset size or increase memory
```python
# Load only critical/high severity
df_filtered = df[df['severity'].isin(['critical', 'high'])]
```

## Performance Notes

- **Semantic Search**: O(n × m) where n=traces, m=query tokens. ~10ms for 1000 traces
- **Clustering**: O(n log n) per dimension. ~100ms for 1000 traces
- **Notebook EDA**: Depends on pandas operations. ~1-5s for full notebook with 1000 traces

## See Also

- [Failure Mode Taxonomy](FAILURE_MODES.md)
- [Evaluation Framework](EVAL_FRAMEWORK.md)
- [Multi-Turn Evaluation Guide](MULTITURN_EVAL_GUIDE.md)
