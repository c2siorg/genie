// Package analysis provides exploratory data analysis (EDA) and clustering tools
// for evaluation traces.
//
// # Clustering Analysis
//
// ClusterAnalyzer groups evaluation traces across multiple dimensions:
//   - Failure mode (FM-DOMAIN-NNN)
//   - Failure domain (settlement, compliance, orchestration, merchant/customer, agent behavior, lineage)
//   - Failure category (operational, security, compliance, availability)
//   - Agent type
//   - Customer segment
//   - Severity level (critical, high, medium, low)
//   - Semantic similarity (token-based clustering)
//
// Each cluster aggregates metrics:
//   - Count of traces in cluster
//   - Average severity score
//   - Average confidence
//   - Average root cause gulf
//   - Distributions across secondary dimensions
//
// # Usage
//
//	// Create analyzer
//	analyzer := analysis.NewClusterAnalyzer(traces)
//
//	// Cluster by failure mode
//	modeClust := analyzer.ClusterByFailureMode()
//	for mode, cluster := range modeClust {
//		fmt.Printf("%s: %d traces, avg confidence: %.1f%%\n",
//			mode, cluster.Count, cluster.Metrics.AverageConfidence)
//	}
//
//	// Cluster by domain with semantic sub-clustering
//	domainClust := analyzer.ClusterByDomain()
//	for domain, cluster := range domainClust {
//		for subcategory, subcluster := range cluster.SubClusters {
//			fmt.Printf("  %s -> %s: %d traces\n", domain, subcategory, subcluster.Count)
//		}
//	}
//
//	// Perform cross-tabulation analysis
//	ct := analyzer.CrossTab("severity", "domain")
//	// Use ct.Counts for contingency table analysis
//
//	// Semantic clustering with 70% similarity threshold
//	semanticClust := analyzer.ClusterBySemanticity(0.7)
//
// # Output for Python Analysis
//
// Clustering results are serialized to JSON for downstream Python analysis:
//   - Each cluster includes trace IDs for subset analysis
//   - Metrics enable statistical comparison across groups
//   - Cross-tabulation supports 2D contingency analysis
//
// # Data Flow
//
// Evaluation Traces (Go JSON)
//   → ClusterAnalyzer
//   → JSON Clusters + Metrics
//   → Python pandas/seaborn
//   → Visualizations & Reports
//
// # Metrics Computed
//
// For each cluster:
//   - SeverityDistribution: histogram of severity levels
//   - AgentDistribution: histogram of agents involved
//   - SegmentDistribution: histogram of customer segments
//   - DomainDistribution: histogram of failure domains
//   - AverageSeverity: mean severity (scaled: critical=4, high=3, medium=2, low=1)
//   - AverageConfidence: mean confidence % (0-100)
//   - AverageRootCauseGulf: mean gulf % (0-100, where 0=exact root cause identified)
//
// # Integration with Semantic Search
//
// Semantic clustering uses Jaccard similarity on tokenized error messages:
//   1. Tokenize error messages (split on non-alphanumeric)
//   2. Compute token set overlap
//   3. Group traces above similarity threshold into semantic clusters
//
// This enables "similar error discovery" without embeddings.
//
package analysis
