// Package examples demonstrates usage of evaluation analysis tools.
//
// This file shows how to:
//   1. Load evaluation results from JSON
//   2. Perform semantic search on traces
//   3. Cluster traces across multiple dimensions
//   4. Export clustering results for Python analysis
//   5. Generate cross-tabulation matrices
//
// Run with:
//
//	go run ./examples/eval_analysis_example.go
//
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/analysis"
)

// ExampleEvalResult represents a single evaluation result from JSON.
type ExampleEvalResult struct {
	TraceID         string `json:"trace_id"`
	FailureMode     string `json:"failure_mode"`
	FailureDomain   string `json:"domain"`
	FailureCategory string `json:"category"`
	ErrorMessage    string `json:"error_message"`
	Severity        string `json:"severity"`
	Agent           string `json:"agent"`
	CustomerSegment string `json:"customer_segment"`
	OrderID         string `json:"order_id"`
	RootCauseGulf   int    `json:"root_cause_gulf"`
	Confidence      int    `json:"confidence"`
	ToolSequence    string `json:"tool_sequence"`
	ComplianceGap   string `json:"compliance_gap"`
}

// loadExampleTraces loads sample evaluation results.
func loadExampleTraces() []analysis.TraceRecord {
	return []analysis.TraceRecord{
		{
			TraceID:         "trace-001",
			FailureMode:     "FM-SE-001",
			FailureDomain:   "settlement",
			FailureCategory: "operational",
			ErrorMessage:    "settlement amount exceeds daily limit for merchant ABC",
			Severity:        "high",
			Agent:           "settlement_executor",
			CustomerSegment: "sme",
			OrderID:         "ord-001",
			RootCauseGulf:   15,
			Confidence:      85,
		},
		{
			TraceID:         "trace-002",
			FailureMode:     "FM-SE-002",
			FailureDomain:   "settlement",
			FailureCategory: "operational",
			ErrorMessage:    "settlement amount over merchant daily limit",
			Severity:        "high",
			Agent:           "settlement_executor",
			CustomerSegment: "retail",
			OrderID:         "ord-002",
			RootCauseGulf:   20,
			Confidence:      82,
		},
		{
			TraceID:         "trace-003",
			FailureMode:     "FM-CO-005",
			FailureDomain:   "compliance",
			FailureCategory: "compliance",
			ErrorMessage:    "aml compliance check failed for high risk entity",
			Severity:        "critical",
			Agent:           "compliance_checker",
			CustomerSegment: "corporate",
			OrderID:         "ord-003",
			RootCauseGulf:   10,
			Confidence:      92,
		},
		{
			TraceID:         "trace-004",
			FailureMode:     "FM-CO-001",
			FailureDomain:   "compliance",
			FailureCategory: "compliance",
			ErrorMessage:    "kyc validation incomplete missing required documents",
			Severity:        "medium",
			Agent:           "kyc_agent",
			CustomerSegment: "retail",
			OrderID:         "ord-004",
			RootCauseGulf:   30,
			Confidence:      75,
		},
		{
			TraceID:         "trace-005",
			FailureMode:     "FM-SE-003",
			FailureDomain:   "settlement",
			FailureCategory: "security",
			ErrorMessage:    "settlement netting calculation overflow numerical error",
			Severity:        "critical",
			Agent:           "netting_engine",
			CustomerSegment: "sme",
			OrderID:         "ord-005",
			RootCauseGulf:   5,
			Confidence:      95,
		},
		{
			TraceID:         "trace-006",
			FailureMode:     "FM-OR-002",
			FailureDomain:   "orchestration",
			FailureCategory: "operational",
			ErrorMessage:    "workflow state machine timeout agent unresponsive",
			Severity:        "high",
			Agent:           "workflow_orchestrator",
			CustomerSegment: "sme",
			OrderID:         "ord-006",
			RootCauseGulf:   25,
			Confidence:      70,
		},
		{
			TraceID:         "trace-007",
			FailureMode:     "FM-AB-001",
			FailureDomain:   "agent_behavior",
			FailureCategory: "security",
			ErrorMessage:    "agent hallucination detected incorrect settlement amount",
			Severity:        "critical",
			Agent:           "settlement_executor",
			CustomerSegment: "corporate",
			OrderID:         "ord-007",
			RootCauseGulf:   50,
			Confidence:      65,
		},
	}
}

func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func main() {
	fmt.Println(repeatString("=", 70))
	fmt.Println("GENIE EVALUATION ANALYSIS EXAMPLE")
	fmt.Println(repeatString("=", 70))

	// Load example traces
	traces := loadExampleTraces()
	fmt.Printf("\nLoaded %d traces\n\n", len(traces))

	// Create analyzer
	analyzer := analysis.NewClusterAnalyzer(traces)

	// Example 1: Cluster by Failure Mode
	fmt.Println(repeatString("=", 70))
	fmt.Println("EXAMPLE 1: Cluster by Failure Mode")
	fmt.Println(repeatString("=", 70))
	modeClust := analyzer.ClusterByFailureMode()
	for mode, cluster := range modeClust {
		fmt.Printf("\n%s:\n", mode)
		fmt.Printf("  Count: %d\n", cluster.Count)
		fmt.Printf("  Trace IDs: %v\n", cluster.Traces)
		fmt.Printf("  Avg Confidence: %.1f%%\n", cluster.Metrics.AverageConfidence)
		fmt.Printf("  Avg Root Cause Gulf: %.1f%%\n", cluster.Metrics.AverageRootCauseGulf)
		fmt.Printf("  Severity Distribution: %v\n", cluster.Metrics.SeverityDistribution)
	}

	// Example 2: Cluster by Domain (with sub-clustering)
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("EXAMPLE 2: Cluster by Domain (with Category Sub-clusters)")
	fmt.Println(repeatString("=", 70))
	domainClust := analyzer.ClusterByDomain()
	for domain, cluster := range domainClust {
		fmt.Printf("\nDomain: %s\n", domain)
		fmt.Printf("  Total Count: %d\n", cluster.Count)
		fmt.Printf("  Avg Confidence: %.1f%%\n", cluster.Metrics.AverageConfidence)

		if len(cluster.SubClusters) > 0 {
			fmt.Println("  Sub-clusters by Category:")
			for cat, subcluster := range cluster.SubClusters {
				fmt.Printf("    %s: %d traces\n", cat, subcluster.Count)
			}
		}
	}

	// Example 3: Cluster by Severity
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("EXAMPLE 3: Cluster by Severity")
	fmt.Println(repeatString("=", 70))
	severityClust := analyzer.ClusterBySeverity()
	for severity, cluster := range severityClust {
		fmt.Printf("\n%s:\n", severity)
		fmt.Printf("  Count: %d\n", cluster.Count)
		fmt.Printf("  Avg Confidence: %.1f%%\n", cluster.Metrics.AverageConfidence)
		fmt.Printf("  Agent Distribution: %v\n", cluster.Metrics.AgentDistribution)
	}

	// Example 4: Cluster by Agent
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("EXAMPLE 4: Cluster by Agent")
	fmt.Println(repeatString("=", 70))
	agentClust := analyzer.ClusterByAgent()
	for agent, cluster := range agentClust {
		fmt.Printf("\n%s:\n", agent)
		fmt.Printf("  Count: %d\n", cluster.Count)
		fmt.Printf("  Severity Distribution: %v\n", cluster.Metrics.SeverityDistribution)
		fmt.Printf("  Domain Distribution: %v\n", cluster.Metrics.DomainDistribution)
	}

	// Example 5: Semantic Clustering
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("EXAMPLE 5: Semantic Clustering (Jaccard Similarity >= 0.5)")
	fmt.Println(repeatString("=", 70))
	semanticClust := analyzer.ClusterBySemanticity(0.5)
	for clustKey, cluster := range semanticClust {
		fmt.Printf("\n%s:\n", clustKey)
		fmt.Printf("  Count: %d\n", cluster.Count)
		fmt.Printf("  Trace IDs: %v\n", cluster.Traces)
		fmt.Printf("  Avg Confidence: %.1f%%\n", cluster.Metrics.AverageConfidence)
	}

	// Example 6: Cross-Tabulation (Domain vs Severity)
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("EXAMPLE 6: Cross-Tabulation (Domain vs Severity)")
	fmt.Println(repeatString("=", 70))
	ct := analyzer.CrossTab("domain", "severity")
	fmt.Println("\nDomain x Severity Contingency Table:")
	fmt.Printf("%15s |", "")
	for _, ylabel := range ct.YLabels {
		fmt.Printf(" %10s |", ylabel)
	}
	fmt.Println()
	fmt.Println("------- | ----------- | ----------- | ---------- | ----------- |")

	for _, xlabel := range ct.XLabels {
		fmt.Printf("%15s |", xlabel)
		for _, ylabel := range ct.YLabels {
			count := ct.Counts[xlabel][ylabel]
			fmt.Printf(" %10d |", count)
		}
		fmt.Println()
	}

	// Example 7: Cross-Tabulation (Agent vs Domain)
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("EXAMPLE 7: Cross-Tabulation (Agent vs Domain)")
	fmt.Println(repeatString("=", 70))
	ct2 := analyzer.CrossTab("agent", "domain")
	fmt.Println("\nAgent x Domain Contingency Table:")
	fmt.Printf("%30s |", "")
	for _, ylabel := range ct2.YLabels {
		fmt.Printf(" %12s |", ylabel)
	}
	fmt.Println()
	fmt.Println("------ | ------------- | ------------- | ------------- | ------------- |")

	for _, xlabel := range ct2.XLabels {
		fmt.Printf("%30s |", xlabel)
		for _, ylabel := range ct2.YLabels {
			count := ct2.Counts[xlabel][ylabel]
			fmt.Printf(" %12d |", count)
		}
		fmt.Println()
	}

	// Example 8: Export clustering results to JSON
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("EXAMPLE 8: Export Clustering Results to JSON")
	fmt.Println(repeatString("=", 70))

	// Create a summary report
	summary := map[string]interface{}{
		"total_traces": len(traces),
		"clusters": map[string]interface{}{
			"by_failure_mode": len(modeClust),
			"by_domain":       len(domainClust),
			"by_severity":     len(severityClust),
			"by_agent":        len(agentClust),
			"semantic":        len(semanticClust),
		},
		"failure_modes":  modeClust,
		"domains":        domainClust,
		"severities":     severityClust,
		"agents":         agentClust,
		"cross_tabs": map[string]interface{}{
			"domain_severity": ct,
			"agent_domain":    ct2,
		},
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	// Write to file
	outputFile := "clustering_analysis.json"
	err = os.WriteFile(outputFile, jsonData, 0644)
	if err != nil {
		log.Fatalf("Failed to write JSON file: %v", err)
	}

	fmt.Printf("Clustering analysis exported to %s\n", outputFile)

	// Example 9: Summary Statistics
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("EXAMPLE 9: Summary Statistics")
	fmt.Println(repeatString("=", 70))

	// Compute global statistics
	var totalConfidence, totalGulf float64
	var criticalCount, highCount, mediumCount, lowCount int

	for _, trace := range traces {
		totalConfidence += float64(trace.Confidence)
		totalGulf += float64(trace.RootCauseGulf)

		switch trace.Severity {
		case "critical":
			criticalCount++
		case "high":
			highCount++
		case "medium":
			mediumCount++
		case "low":
			lowCount++
		}
	}

	avgConfidence := totalConfidence / float64(len(traces))
	avgGulf := totalGulf / float64(len(traces))

	fmt.Printf("\nGlobal Statistics:\n")
	fmt.Printf("  Total Traces: %d\n", len(traces))
	fmt.Printf("  Avg Confidence: %.1f%%\n", avgConfidence)
	fmt.Printf("  Avg Root Cause Gulf: %.1f%%\n", avgGulf)
	fmt.Printf("\nSeverity Distribution:\n")
	fmt.Printf("  Critical: %d (%.1f%%)\n", criticalCount, float64(criticalCount)/float64(len(traces))*100)
	fmt.Printf("  High: %d (%.1f%%)\n", highCount, float64(highCount)/float64(len(traces))*100)
	fmt.Printf("  Medium: %d (%.1f%%)\n", mediumCount, float64(mediumCount)/float64(len(traces))*100)
	fmt.Printf("  Low: %d (%.1f%%)\n", lowCount, float64(lowCount)/float64(len(traces))*100)

	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("Example completed successfully!")
	fmt.Println(repeatString("=", 70))
}
