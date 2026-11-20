package analysis

import (
	"testing"
)

// TestClusterByFailureMode verifies failure mode clustering.
func TestClusterByFailureMode(t *testing.T) {
	traces := []TraceRecord{
		{
			TraceID:         "t1",
			FailureMode:     "FM-SE-001",
			FailureDomain:   "settlement",
			FailureCategory: "operational",
			Severity:        "high",
			Confidence:      85,
		},
		{
			TraceID:         "t2",
			FailureMode:     "FM-SE-001",
			FailureDomain:   "settlement",
			FailureCategory: "operational",
			Severity:        "critical",
			Confidence:      92,
		},
		{
			TraceID:         "t3",
			FailureMode:     "FM-CO-005",
			FailureDomain:   "compliance",
			FailureCategory: "compliance",
			Severity:        "medium",
			Confidence:      75,
		},
	}

	analyzer := NewClusterAnalyzer(traces)
	clusters := analyzer.ClusterByFailureMode()

	if len(clusters) != 2 {
		t.Errorf("Expected 2 clusters, got %d", len(clusters))
	}

	se001Cluster, exists := clusters["FM-SE-001"]
	if !exists {
		t.Errorf("FM-SE-001 cluster not found")
	}

	if se001Cluster.Count != 2 {
		t.Errorf("FM-SE-001: expected count 2, got %d", se001Cluster.Count)
	}

	if len(se001Cluster.Traces) != 2 {
		t.Errorf("FM-SE-001: expected 2 traces, got %d", len(se001Cluster.Traces))
	}

	// Verify metrics
	expectedConfidence := (85 + 92) / 2.0
	if se001Cluster.Metrics.AverageConfidence != expectedConfidence {
		t.Errorf("FM-SE-001: expected avg confidence %.1f, got %.1f", expectedConfidence, se001Cluster.Metrics.AverageConfidence)
	}

	// Verify severity distribution
	if se001Cluster.Metrics.SeverityDistribution["high"] != 1 {
		t.Errorf("Expected 1 'high' severity, got %d", se001Cluster.Metrics.SeverityDistribution["high"])
	}
	if se001Cluster.Metrics.SeverityDistribution["critical"] != 1 {
		t.Errorf("Expected 1 'critical' severity, got %d", se001Cluster.Metrics.SeverityDistribution["critical"])
	}
}

// TestClusterByDomain verifies domain clustering with sub-clustering.
func TestClusterByDomain(t *testing.T) {
	traces := []TraceRecord{
		{
			TraceID:         "t1",
			FailureMode:     "FM-SE-001",
			FailureDomain:   "settlement",
			FailureCategory: "operational",
			Severity:        "high",
			Confidence:      85,
		},
		{
			TraceID:         "t2",
			FailureMode:     "FM-SE-002",
			FailureDomain:   "settlement",
			FailureCategory: "security",
			Severity:        "critical",
			Confidence:      92,
		},
		{
			TraceID:         "t3",
			FailureMode:     "FM-CO-001",
			FailureDomain:   "compliance",
			FailureCategory: "compliance",
			Severity:        "medium",
			Confidence:      75,
		},
	}

	analyzer := NewClusterAnalyzer(traces)
	clusters := analyzer.ClusterByDomain()

	if len(clusters) != 2 {
		t.Errorf("Expected 2 domain clusters, got %d", len(clusters))
	}

	settlementCluster, exists := clusters["settlement"]
	if !exists {
		t.Errorf("settlement cluster not found")
	}

	if settlementCluster.Count != 2 {
		t.Errorf("settlement: expected count 2, got %d", settlementCluster.Count)
	}

	// Verify sub-clustering by category
	if len(settlementCluster.SubClusters) != 2 {
		t.Errorf("settlement: expected 2 sub-clusters (operational, security), got %d", len(settlementCluster.SubClusters))
	}

	operationalSub, exists := settlementCluster.SubClusters["operational"]
	if !exists {
		t.Errorf("settlement->operational sub-cluster not found")
	}

	if operationalSub.Count != 1 {
		t.Errorf("settlement->operational: expected count 1, got %d", operationalSub.Count)
	}
}

// TestClusterBySeverity verifies severity-based clustering.
func TestClusterBySeverity(t *testing.T) {
	traces := []TraceRecord{
		{TraceID: "t1", Severity: "critical", Confidence: 90},
		{TraceID: "t2", Severity: "critical", Confidence: 88},
		{TraceID: "t3", Severity: "high", Confidence: 80},
		{TraceID: "t4", Severity: "medium", Confidence: 70},
		{TraceID: "t5", Severity: "low", Confidence: 60},
	}

	analyzer := NewClusterAnalyzer(traces)
	clusters := analyzer.ClusterBySeverity()

	if len(clusters) != 4 {
		t.Errorf("Expected 4 severity clusters, got %d", len(clusters))
	}

	criticalCluster := clusters["critical"]
	if criticalCluster.Count != 2 {
		t.Errorf("critical: expected count 2, got %d", criticalCluster.Count)
	}
	if criticalCluster.Metrics.AverageConfidence != 89.0 {
		t.Errorf("critical: expected avg confidence 89.0, got %.1f", criticalCluster.Metrics.AverageConfidence)
	}
}

// TestClusterByAgent verifies agent-based clustering.
func TestClusterByAgent(t *testing.T) {
	traces := []TraceRecord{
		{TraceID: "t1", Agent: "settlement_executor", Severity: "high"},
		{TraceID: "t2", Agent: "settlement_executor", Severity: "critical"},
		{TraceID: "t3", Agent: "compliance_checker", Severity: "high"},
		{TraceID: "t4", Agent: "unknown", Severity: "medium"},
	}

	analyzer := NewClusterAnalyzer(traces)
	clusters := analyzer.ClusterByAgent()

	if len(clusters) != 3 {
		t.Errorf("Expected 3 agent clusters, got %d", len(clusters))
	}

	settlementCluster := clusters["settlement_executor"]
	if settlementCluster.Count != 2 {
		t.Errorf("settlement_executor: expected count 2, got %d", settlementCluster.Count)
	}

	complianceCluster := clusters["compliance_checker"]
	if complianceCluster.Count != 1 {
		t.Errorf("compliance_checker: expected count 1, got %d", complianceCluster.Count)
	}
}

// TestClusterBySegment verifies customer segment clustering.
func TestClusterBySegment(t *testing.T) {
	traces := []TraceRecord{
		{TraceID: "t1", CustomerSegment: "retail", Severity: "high"},
		{TraceID: "t2", CustomerSegment: "retail", Severity: "medium"},
		{TraceID: "t3", CustomerSegment: "sme", Severity: "critical"},
		{TraceID: "t4", CustomerSegment: "", Severity: "low"},
	}

	analyzer := NewClusterAnalyzer(traces)
	clusters := analyzer.ClusterBySegment()

	if len(clusters) != 3 {
		t.Errorf("Expected 3 segment clusters, got %d", len(clusters))
	}

	retailCluster := clusters["retail"]
	if retailCluster.Count != 2 {
		t.Errorf("retail: expected count 2, got %d", retailCluster.Count)
	}

	unspecCluster := clusters["unspecified"]
	if unspecCluster.Count != 1 {
		t.Errorf("unspecified: expected count 1, got %d", unspecCluster.Count)
	}
}

// TestSemanticClustering verifies token-based semantic similarity clustering.
func TestSemanticClustering(t *testing.T) {
	traces := []TraceRecord{
		{
			TraceID:      "t1",
			ErrorMessage: "settlement amount exceeds daily limit",
		},
		{
			TraceID:      "t2",
			ErrorMessage: "settlement amount over daily limit",
		},
		{
			TraceID:      "t3",
			ErrorMessage: "aml compliance check failed",
		},
		{
			TraceID:      "t4",
			ErrorMessage: "unknown error",
		},
	}

	analyzer := NewClusterAnalyzer(traces)
	clusters := analyzer.ClusterBySemanticity(0.5) // 50% threshold

	// t1 and t2 should cluster together (high token overlap)
	// t3 should be separate (no token overlap)
	// t4 should be separate (no token overlap)

	if len(clusters) < 2 {
		t.Errorf("Expected at least 2 semantic clusters, got %d", len(clusters))
	}

	// Verify that t1 and t2 are in same cluster
	found := false
	for _, cluster := range clusters {
		if len(cluster.Traces) >= 2 {
			// Check if both t1 and t2 are in this cluster
			hasT1 := false
			hasT2 := false
			for _, id := range cluster.Traces {
				if id == "t1" {
					hasT1 = true
				}
				if id == "t2" {
					hasT2 = true
				}
			}
			if hasT1 && hasT2 {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("t1 and t2 should be in same semantic cluster")
	}
}

// TestCrossTabulation verifies 2D contingency table generation.
func TestCrossTabulation(t *testing.T) {
	traces := []TraceRecord{
		{FailureDomain: "settlement", Severity: "critical"},
		{FailureDomain: "settlement", Severity: "critical"},
		{FailureDomain: "settlement", Severity: "high"},
		{FailureDomain: "compliance", Severity: "high"},
		{FailureDomain: "compliance", Severity: "medium"},
	}

	analyzer := NewClusterAnalyzer(traces)
	ct := analyzer.CrossTab("domain", "severity")

	if ct.XAxis != "domain" || ct.YAxis != "severity" {
		t.Errorf("Cross-tab axes mismatch")
	}

	// Check counts
	if ct.Counts["settlement"]["critical"] != 2 {
		t.Errorf("settlement x critical: expected 2, got %d", ct.Counts["settlement"]["critical"])
	}

	if ct.Counts["settlement"]["high"] != 1 {
		t.Errorf("settlement x high: expected 1, got %d", ct.Counts["settlement"]["high"])
	}

	if ct.Counts["compliance"]["high"] != 1 {
		t.Errorf("compliance x high: expected 1, got %d", ct.Counts["compliance"]["high"])
	}

	if ct.Counts["compliance"]["medium"] != 1 {
		t.Errorf("compliance x medium: expected 1, got %d", ct.Counts["compliance"]["medium"])
	}
}

// TestMetricsComputation verifies metric aggregation.
func TestMetricsComputation(t *testing.T) {
	traces := []TraceRecord{
		{
			TraceID:         "t1",
			FailureMode:     "FM-TEST-001",
			Severity:        "critical",
			Confidence:      95,
			RootCauseGulf:   10,
			Agent:           "agent_a",
			CustomerSegment: "retail",
			FailureDomain:   "settlement",
		},
		{
			TraceID:         "t2",
			FailureMode:     "FM-TEST-001",
			Severity:        "high",
			Confidence:      85,
			RootCauseGulf:   30,
			Agent:           "agent_b",
			CustomerSegment: "sme",
			FailureDomain:   "settlement",
		},
		{
			TraceID:         "t3",
			FailureMode:     "FM-TEST-001",
			Severity:        "medium",
			Confidence:      75,
			RootCauseGulf:   50,
			Agent:           "agent_a",
			CustomerSegment: "retail",
			FailureDomain:   "compliance",
		},
	}

	analyzer := NewClusterAnalyzer(traces)
	clusters := analyzer.ClusterByFailureMode()

	cluster := clusters["FM-TEST-001"]

	// Verify count
	if cluster.Count != 3 {
		t.Errorf("Expected count 3, got %d", cluster.Count)
	}

	// Verify average confidence
	expectedConfidence := (95 + 85 + 75) / 3.0
	if cluster.Metrics.AverageConfidence != expectedConfidence {
		t.Errorf("Expected avg confidence %.1f, got %.1f", expectedConfidence, cluster.Metrics.AverageConfidence)
	}

	// Verify average root cause gulf
	expectedGulf := (10 + 30 + 50) / 3.0
	if cluster.Metrics.AverageRootCauseGulf != expectedGulf {
		t.Errorf("Expected avg gulf %.1f, got %.1f", expectedGulf, cluster.Metrics.AverageRootCauseGulf)
	}

	// Verify severity distribution
	if cluster.Metrics.SeverityDistribution["critical"] != 1 {
		t.Errorf("Expected 1 critical, got %d", cluster.Metrics.SeverityDistribution["critical"])
	}
	if cluster.Metrics.SeverityDistribution["high"] != 1 {
		t.Errorf("Expected 1 high, got %d", cluster.Metrics.SeverityDistribution["high"])
	}
	if cluster.Metrics.SeverityDistribution["medium"] != 1 {
		t.Errorf("Expected 1 medium, got %d", cluster.Metrics.SeverityDistribution["medium"])
	}

	// Verify agent distribution
	if cluster.Metrics.AgentDistribution["agent_a"] != 2 {
		t.Errorf("Expected 2 agent_a, got %d", cluster.Metrics.AgentDistribution["agent_a"])
	}
	if cluster.Metrics.AgentDistribution["agent_b"] != 1 {
		t.Errorf("Expected 1 agent_b, got %d", cluster.Metrics.AgentDistribution["agent_b"])
	}

	// Verify segment distribution
	if cluster.Metrics.SegmentDistribution["retail"] != 2 {
		t.Errorf("Expected 2 retail, got %d", cluster.Metrics.SegmentDistribution["retail"])
	}
	if cluster.Metrics.SegmentDistribution["sme"] != 1 {
		t.Errorf("Expected 1 sme, got %d", cluster.Metrics.SegmentDistribution["sme"])
	}

	// Verify domain distribution
	if cluster.Metrics.DomainDistribution["settlement"] != 2 {
		t.Errorf("Expected 2 settlement, got %d", cluster.Metrics.DomainDistribution["settlement"])
	}
	if cluster.Metrics.DomainDistribution["compliance"] != 1 {
		t.Errorf("Expected 1 compliance, got %d", cluster.Metrics.DomainDistribution["compliance"])
	}
}

// TestEmptyAnalyzer verifies behavior with empty trace set.
func TestEmptyAnalyzer(t *testing.T) {
	analyzer := NewClusterAnalyzer([]TraceRecord{})

	clusters := analyzer.ClusterByFailureMode()
	if len(clusters) != 0 {
		t.Errorf("Expected 0 clusters for empty analyzer, got %d", len(clusters))
	}

	clusters = analyzer.ClusterBySeverity()
	if len(clusters) != 0 {
		t.Errorf("Expected 0 clusters for empty analyzer, got %d", len(clusters))
	}
}

// TestJaccardSimilarity verifies token set similarity computation.
func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		tokens1  []string
		tokens2  []string
		expected float64
	}{
		{
			tokens1:  []string{"settlement", "amount", "exceeds"},
			tokens2:  []string{"settlement", "amount", "exceeds"},
			expected: 1.0, // Identical
		},
		{
			tokens1:  []string{"settlement", "amount"},
			tokens2:  []string{"settlement", "amount", "exceeds"},
			expected: 2.0 / 3.0, // 2 intersection / 3 union
		},
		{
			tokens1:  []string{"aml"},
			tokens2:  []string{"compliance"},
			expected: 0.0, // No overlap
		},
		{
			tokens1:  []string{},
			tokens2:  []string{},
			expected: 0.0, // Both empty
		},
	}

	for _, test := range tests {
		result := jaccardSimilarity(test.tokens1, test.tokens2)
		if result != test.expected {
			t.Errorf("Jaccard(%v, %v): expected %.3f, got %.3f", test.tokens1, test.tokens2, test.expected, result)
		}
	}
}
