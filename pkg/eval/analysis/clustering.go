// Package analysis provides exploratory data analysis (EDA) and statistical tools
// for evaluation results.
//
// Clustering analysis groups evaluation traces by:
//   - Failure mode (FM-DOMAIN-NNN taxonomy)
//   - Failure domain (settlement, compliance, orchestration, etc.)
//   - Failure category (operational, security, compliance, availability)
//   - Agent type
//   - Customer segment
//   - Severity level
//   - Semantic similarity (using token-based clustering)
//
// Usage:
//
//	analyzer := NewClusterAnalyzer(traces)
//	modeClust := analyzer.ClusterByFailureMode()
//	domainClust := analyzer.ClusterByDomain()
//	semanticClust := analyzer.ClusterBySemanticity(0.7)
//
// Each cluster contains metadata for visualization and cross-tabulation analysis.
package analysis

import (
	"fmt"
	"sort"
	"strings"
)

// ClusterAnalyzer performs multi-dimensional clustering on evaluation traces.
type ClusterAnalyzer struct {
	traces []TraceRecord
}

// TraceRecord represents a single evaluation trace for clustering.
type TraceRecord struct {
	TraceID         string
	FailureMode     string // FM-DOMAIN-NNN
	FailureDomain   string // settlement, compliance, orchestration, etc.
	FailureCategory string // operational, security, compliance, availability
	ErrorMessage    string
	Severity        string
	Agent           string
	CustomerSegment string
	OrderID         string
	RootCauseGulf   int
	Confidence      int
	ToolSequence    string
	ComplianceGap   string
	SettlementAmount string
	AMLRiskScore    string
}

// Cluster represents a group of traces sharing a common attribute.
type Cluster struct {
	Key        string         `json:"key"`
	Label      string         `json:"label"`
	Count      int            `json:"count"`
	Traces     []string       `json:"trace_ids"`
	Metrics    ClusterMetrics `json:"metrics"`
	SubClusters map[string]*Cluster `json:"sub_clusters,omitempty"`
}

// ClusterMetrics aggregates statistics about a cluster.
type ClusterMetrics struct {
	AverageSeverity    float64 `json:"average_severity"`
	AverageConfidence  float64 `json:"average_confidence"`
	AverageRootCauseGulf float64 `json:"average_root_cause_gulf"`
	SeverityDistribution map[string]int `json:"severity_distribution"`
	AgentDistribution   map[string]int `json:"agent_distribution"`
	SegmentDistribution map[string]int `json:"segment_distribution"`
	DomainDistribution  map[string]int `json:"domain_distribution"`
}

// NewClusterAnalyzer creates a new analyzer for the given traces.
func NewClusterAnalyzer(traces []TraceRecord) *ClusterAnalyzer {
	return &ClusterAnalyzer{
		traces: traces,
	}
}

// ClusterByFailureMode groups traces by their failure mode ID.
func (ca *ClusterAnalyzer) ClusterByFailureMode() map[string]*Cluster {
	clusters := make(map[string]*Cluster)

	for _, trace := range ca.traces {
		key := trace.FailureMode
		if key == "" {
			key = "unknown"
		}

		if _, exists := clusters[key]; !exists {
			clusters[key] = &Cluster{
				Key:        key,
				Label:      key,
				Traces:     []string{},
				Metrics:    ClusterMetrics{},
				SubClusters: make(map[string]*Cluster),
			}
		}

		clusters[key].Traces = append(clusters[key].Traces, trace.TraceID)
	}

	// Compute metrics for each cluster
	for key, cluster := range clusters {
		cluster.Count = len(cluster.Traces)
		ca.computeMetrics(cluster, key)
	}

	return clusters
}

// ClusterByDomain groups traces by their failure domain.
func (ca *ClusterAnalyzer) ClusterByDomain() map[string]*Cluster {
	clusters := make(map[string]*Cluster)

	for _, trace := range ca.traces {
		key := trace.FailureDomain
		if key == "" {
			key = "unknown"
		}

		if _, exists := clusters[key]; !exists {
			clusters[key] = &Cluster{
				Key:        key,
				Label:      key,
				Traces:     []string{},
				Metrics:    ClusterMetrics{},
				SubClusters: make(map[string]*Cluster),
			}
		}

		clusters[key].Traces = append(clusters[key].Traces, trace.TraceID)
	}

	// Compute metrics and create sub-clusters by category
	for key, cluster := range clusters {
		cluster.Count = len(cluster.Traces)
		ca.computeMetrics(cluster, key)

		// Create sub-clusters by failure category
		subClusters := ca.clusterByCategory(cluster.Traces)
		cluster.SubClusters = subClusters
	}

	return clusters
}

// ClusterByCategory groups traces by failure category.
func (ca *ClusterAnalyzer) ClusterByCategory() map[string]*Cluster {
	clusters := make(map[string]*Cluster)

	for _, trace := range ca.traces {
		key := trace.FailureCategory
		if key == "" {
			key = "unknown"
		}

		if _, exists := clusters[key]; !exists {
			clusters[key] = &Cluster{
				Key:        key,
				Label:      key,
				Traces:     []string{},
				Metrics:    ClusterMetrics{},
				SubClusters: make(map[string]*Cluster),
			}
		}

		clusters[key].Traces = append(clusters[key].Traces, trace.TraceID)
	}

	// Compute metrics
	for key, cluster := range clusters {
		cluster.Count = len(cluster.Traces)
		ca.computeMetrics(cluster, key)
	}

	return clusters
}

// ClusterBySeverity groups traces by severity level.
func (ca *ClusterAnalyzer) ClusterBySeverity() map[string]*Cluster {
	clusters := make(map[string]*Cluster)

	for _, trace := range ca.traces {
		key := trace.Severity
		if key == "" {
			key = "unknown"
		}

		if _, exists := clusters[key]; !exists {
			clusters[key] = &Cluster{
				Key:        key,
				Label:      key,
				Traces:     []string{},
				Metrics:    ClusterMetrics{},
				SubClusters: make(map[string]*Cluster),
			}
		}

		clusters[key].Traces = append(clusters[key].Traces, trace.TraceID)
	}

	// Compute metrics
	for key, cluster := range clusters {
		cluster.Count = len(cluster.Traces)
		ca.computeMetrics(cluster, key)
	}

	return clusters
}

// ClusterByAgent groups traces by the agent that failed.
func (ca *ClusterAnalyzer) ClusterByAgent() map[string]*Cluster {
	clusters := make(map[string]*Cluster)

	for _, trace := range ca.traces {
		key := trace.Agent
		if key == "" {
			key = "unknown"
		}

		if _, exists := clusters[key]; !exists {
			clusters[key] = &Cluster{
				Key:        key,
				Label:      key,
				Traces:     []string{},
				Metrics:    ClusterMetrics{},
				SubClusters: make(map[string]*Cluster),
			}
		}

		clusters[key].Traces = append(clusters[key].Traces, trace.TraceID)
	}

	// Compute metrics
	for key, cluster := range clusters {
		cluster.Count = len(cluster.Traces)
		ca.computeMetrics(cluster, key)
	}

	return clusters
}

// ClusterBySegment groups traces by customer segment.
func (ca *ClusterAnalyzer) ClusterBySegment() map[string]*Cluster {
	clusters := make(map[string]*Cluster)

	for _, trace := range ca.traces {
		key := trace.CustomerSegment
		if key == "" {
			key = "unspecified"
		}

		if _, exists := clusters[key]; !exists {
			clusters[key] = &Cluster{
				Key:        key,
				Label:      key,
				Traces:     []string{},
				Metrics:    ClusterMetrics{},
				SubClusters: make(map[string]*Cluster),
			}
		}

		clusters[key].Traces = append(clusters[key].Traces, trace.TraceID)
	}

	// Compute metrics
	for key, cluster := range clusters {
		cluster.Count = len(cluster.Traces)
		ca.computeMetrics(cluster, key)
	}

	return clusters
}

// ClusterBySemanticity performs simple token-based semantic clustering.
// Threshold controls similarity (0.7 = 70% token overlap).
func (ca *ClusterAnalyzer) ClusterBySemanticity(threshold float64) map[string]*Cluster {
	clusters := make(map[string]*Cluster)
	assigned := make(map[string]bool)
	clusterID := 0

	for _, trace := range ca.traces {
		if assigned[trace.TraceID] {
			continue
		}

		// Start a new cluster with this trace
		clusterKey := fmt.Sprintf("semantic_%d", clusterID)
		clusters[clusterKey] = &Cluster{
			Key:        clusterKey,
			Label:      fmt.Sprintf("Semantic Cluster %d", clusterID),
			Traces:     []string{trace.TraceID},
			Metrics:    ClusterMetrics{},
			SubClusters: make(map[string]*Cluster),
		}
		assigned[trace.TraceID] = true

		// Try to add similar traces to this cluster
		seedTokens := tokenizeMessage(trace.ErrorMessage)
		for _, other := range ca.traces {
			if assigned[other.TraceID] {
				continue
			}

			otherTokens := tokenizeMessage(other.ErrorMessage)
			sim := jaccardSimilarity(seedTokens, otherTokens)

			if sim >= threshold {
				clusters[clusterKey].Traces = append(clusters[clusterKey].Traces, other.TraceID)
				assigned[other.TraceID] = true
			}
		}

		clusterID++
	}

	// Compute metrics
	for _, cluster := range clusters {
		cluster.Count = len(cluster.Traces)
		ca.computeMetrics(cluster, "")
	}

	return clusters
}

// clusterByCategory is a helper that clusters traces by category.
func (ca *ClusterAnalyzer) clusterByCategory(traceIDs []string) map[string]*Cluster {
	clusters := make(map[string]*Cluster)
	traceMap := make(map[string]TraceRecord)

	for _, trace := range ca.traces {
		traceMap[trace.TraceID] = trace
	}

	for _, id := range traceIDs {
		trace := traceMap[id]
		key := trace.FailureCategory
		if key == "" {
			key = "unknown"
		}

		if _, exists := clusters[key]; !exists {
			clusters[key] = &Cluster{
				Key:        key,
				Label:      key,
				Traces:     []string{},
				Metrics:    ClusterMetrics{},
				SubClusters: make(map[string]*Cluster),
			}
		}

		clusters[key].Traces = append(clusters[key].Traces, id)
	}

	// Compute metrics
	for _, cluster := range clusters {
		cluster.Count = len(cluster.Traces)
		ca.computeMetrics(cluster, "")
	}

	return clusters
}

// computeMetrics aggregates statistics for a cluster.
func (ca *ClusterAnalyzer) computeMetrics(cluster *Cluster, filterKey string) {
	traceMap := make(map[string]TraceRecord)
	for _, trace := range ca.traces {
		traceMap[trace.TraceID] = trace
	}

	metrics := ClusterMetrics{
		SeverityDistribution: make(map[string]int),
		AgentDistribution:    make(map[string]int),
		SegmentDistribution:  make(map[string]int),
		DomainDistribution:   make(map[string]int),
	}

	var totalSeverity float64
	var totalConfidence float64
	var totalRootCauseGulf float64
	severityMap := map[string]float64{"critical": 4, "high": 3, "medium": 2, "low": 1}

	for _, id := range cluster.Traces {
		trace := traceMap[id]

		// Accumulate severity
		if sev, ok := severityMap[trace.Severity]; ok {
			totalSeverity += sev
		}
		metrics.SeverityDistribution[trace.Severity]++

		// Accumulate confidence
		totalConfidence += float64(trace.Confidence)

		// Accumulate root cause gulf
		totalRootCauseGulf += float64(trace.RootCauseGulf)

		// Distributions
		metrics.AgentDistribution[trace.Agent]++
		metrics.SegmentDistribution[trace.CustomerSegment]++
		metrics.DomainDistribution[trace.FailureDomain]++
	}

	count := float64(len(cluster.Traces))
	if count > 0 {
		metrics.AverageSeverity = totalSeverity / count
		metrics.AverageConfidence = totalConfidence / count
		metrics.AverageRootCauseGulf = totalRootCauseGulf / count
	}

	cluster.Metrics = metrics
}

// tokenizeMessage splits text into tokens.
func tokenizeMessage(text string) []string {
	lower := strings.ToLower(text)
	tokens := strings.FieldsFunc(lower, func(r rune) bool {
		return !isAlphaNumeric(r)
	})
	return tokens
}

// isAlphaNumeric checks if a rune is alphanumeric.
func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '_'
}

// jaccardSimilarity computes Jaccard similarity of token sets.
func jaccardSimilarity(tokens1, tokens2 []string) float64 {
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)

	for _, t := range tokens1 {
		set1[t] = true
	}
	for _, t := range tokens2 {
		set2[t] = true
	}

	intersection := 0
	union := len(set1)

	for token := range set2 {
		if set1[token] {
			intersection++
		} else {
			union++
		}
	}

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// CrossTabulation represents a 2D frequency table.
type CrossTabulation struct {
	XAxis   string                    `json:"x_axis"`
	YAxis   string                    `json:"y_axis"`
	Counts  map[string]map[string]int `json:"counts"`
	XLabels []string                  `json:"x_labels"`
	YLabels []string                  `json:"y_labels"`
}

// CrossTab generates a cross-tabulation between two dimensions.
// Dimensions: "failure_mode", "domain", "category", "severity", "agent", "segment"
func (ca *ClusterAnalyzer) CrossTab(xDim, yDim string) *CrossTabulation {
	ct := &CrossTabulation{
		XAxis:  xDim,
		YAxis:  yDim,
		Counts: make(map[string]map[string]int),
	}

	xLabels := make(map[string]bool)
	yLabels := make(map[string]bool)

	for _, trace := range ca.traces {
		xKey := ca.getDimensionValue(trace, xDim)
		yKey := ca.getDimensionValue(trace, yDim)

		if _, exists := ct.Counts[xKey]; !exists {
			ct.Counts[xKey] = make(map[string]int)
		}

		ct.Counts[xKey][yKey]++
		xLabels[xKey] = true
		yLabels[yKey] = true
	}

	// Sort labels for consistent output
	for label := range xLabels {
		ct.XLabels = append(ct.XLabels, label)
	}
	sort.Strings(ct.XLabels)

	for label := range yLabels {
		ct.YLabels = append(ct.YLabels, label)
	}
	sort.Strings(ct.YLabels)

	return ct
}

// getDimensionValue returns the value of a trace for a given dimension.
func (ca *ClusterAnalyzer) getDimensionValue(trace TraceRecord, dim string) string {
	switch dim {
	case "failure_mode":
		return trace.FailureMode
	case "domain":
		return trace.FailureDomain
	case "category":
		return trace.FailureCategory
	case "severity":
		return trace.Severity
	case "agent":
		return trace.Agent
	case "segment":
		return trace.CustomerSegment
	default:
		return "unknown"
	}
}
