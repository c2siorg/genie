// Package main implements a semantic search tool for finding execution traces
// by semantic similarity.
//
// The tool uses string-based similarity metrics (cosine distance over token
// embeddings, Levenshtein distance for error messages) to find the top N traces
// most similar to a query.
//
// Usage:
//
//	go run ./cmd/eval-semantic-search -query "settlement amount too high" -topk 10 -dataset traces.json
//
// Flags:
//
//	-query string     Query string (e.g., "settlement amount too high")
//	-topk int         Number of results to return (default: 10)
//	-dataset string   Path to evaluation results JSON file (default: eval_results.json)
//	-output string    Output file path (default: semantic_search_results.json)
//	-format string    Output format: json, csv, text (default: json)
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
)

// TraceMetadata captures searchable trace properties.
type TraceMetadata struct {
	TraceID           string `json:"trace_id"`
	FailureMode       string `json:"failure_mode"`
	ErrorMessage      string `json:"error_message"`
	Severity          string `json:"severity"`
	Agent             string `json:"agent"`
	OrderID           string `json:"order_id"`
	CustomerSegment   string `json:"customer_segment"`
	Domain            string `json:"domain"`
	RootCauseGulf     int    `json:"root_cause_gulf"`
	Confidence        int    `json:"confidence"`
	ToolSequence      string `json:"tool_sequence"`
	ComplianceGap     string `json:"compliance_gap"`
	SettlementAmount  string `json:"settlement_amount"`
	AMLRiskScore      string `json:"aml_risk_score"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	TraceID         string  `json:"trace_id"`
	Metadata        TraceMetadata `json:"metadata"`
	SimilarityScore float64 `json:"similarity_score"`
	MatchingFields  []string `json:"matching_fields"`
	Explanation     string  `json:"explanation"`
}

// SearchResults holds the full search output.
type SearchResults struct {
	Query       string         `json:"query"`
	TopK        int            `json:"topk"`
	ResultCount int            `json:"result_count"`
	Results     []SearchResult `json:"results"`
}

// EvalResult mirrors the evaluation result structure.
type EvalResult struct {
	TraceID         string        `json:"trace_id"`
	FailureMode     string        `json:"failure_mode"`
	ErrorMessage    string        `json:"error_message"`
	Severity        string        `json:"severity"`
	Agent           string        `json:"agent"`
	OrderID         string        `json:"order_id"`
	CustomerSegment string        `json:"customer_segment"`
	Domain          string        `json:"domain"`
	RootCauseGulf   int           `json:"root_cause_gulf"`
	Confidence      int           `json:"confidence"`
	ToolSequence    string        `json:"tool_sequence"`
	ComplianceGap   string        `json:"compliance_gap"`
	SettlementAmount string       `json:"settlement_amount"`
	AMLRiskScore    string        `json:"aml_risk_score"`
	RawTrace        json.RawMessage `json:"raw_trace,omitempty"`
}

// tokenize splits text into words and applies basic normalization.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	// Remove punctuation and split on whitespace
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return !isAlphaNumeric(r)
	})
	return words
}

// isAlphaNumeric checks if a rune is alphanumeric.
func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '_'
}

// tokenSetSimilarity computes Jaccard similarity of tokenized strings.
// Returns a score in [0, 1] where 1 = identical token sets.
func tokenSetSimilarity(text1, text2 string) float64 {
	tokens1 := tokenize(text1)
	tokens2 := tokenize(text2)

	// Create sets using maps
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)

	for _, t := range tokens1 {
		set1[t] = true
	}
	for _, t := range tokens2 {
		set2[t] = true
	}

	// Compute intersection and union
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

// levenshteinDistance computes the edit distance between two strings.
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	prev := make([]int, len(s2)+1)
	curr := make([]int, len(s2)+1)

	for i := range prev {
		prev[i] = i
	}

	for i := 0; i < len(s1); i++ {
		curr[0] = i + 1
		for j := 0; j < len(s2); j++ {
			cost := 0
			if s1[i] != s2[j] {
				cost = 1
			}
			curr[j+1] = min(curr[j]+1, min(prev[j+1]+1, prev[j]+cost))
		}
		prev, curr = curr, prev
	}

	return prev[len(s2)]
}

// normalizedLevenshteinSimilarity returns similarity in [0, 1].
func normalizedLevenshteinSimilarity(s1, s2 string) float64 {
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}
	if maxLen == 0 {
		return 1.0
	}
	distance := levenshteinDistance(s1, s2)
	return 1.0 - float64(distance)/float64(maxLen)
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// computeSimilarity combines multiple similarity metrics into a single score.
// Weights:
// - Error message similarity: 0.4 (most important for debugging)
// - Failure mode/domain: 0.3 (structural information)
// - Severity/compliance gap: 0.2 (operational impact)
// - Other fields: 0.1 (contextual)
func computeSimilarity(query string, trace TraceMetadata) float64 {
	const (
		errorWeight       = 0.4
		failureWeight     = 0.3
		severityWeight    = 0.2
		contextWeight     = 0.1
	)

	// Error message similarity (highest weight)
	errorSim := normalizedLevenshteinSimilarity(query, trace.ErrorMessage)

	// Failure mode and domain token similarity
	failureSim := tokenSetSimilarity(query, trace.FailureMode)
	domainSim := tokenSetSimilarity(query, trace.Domain)
	failureScore := (failureSim + domainSim) / 2

	// Severity and compliance gap (secondary signals)
	severitySim := tokenSetSimilarity(query, trace.Severity)
	complianceSim := tokenSetSimilarity(query, trace.ComplianceGap)
	severityScore := (severitySim + complianceSim) / 2

	// Context fields (lowest weight)
	agentSim := tokenSetSimilarity(query, trace.Agent)
	toolSeqSim := tokenSetSimilarity(query, trace.ToolSequence)
	contextScore := (agentSim + toolSeqSim) / 2

	// Weighted combination
	score := (errorWeight * errorSim) +
		(failureWeight * failureScore) +
		(severityWeight * severityScore) +
		(contextWeight * contextScore)

	return score
}

// identifyMatchingFields returns which fields contributed to the match.
func identifyMatchingFields(query string, trace TraceMetadata) []string {
	var fields []string
	threshold := 0.3

	if normalizedLevenshteinSimilarity(query, trace.ErrorMessage) > threshold {
		fields = append(fields, "error_message")
	}
	if tokenSetSimilarity(query, trace.FailureMode) > threshold {
		fields = append(fields, "failure_mode")
	}
	if tokenSetSimilarity(query, trace.Domain) > threshold {
		fields = append(fields, "domain")
	}
	if tokenSetSimilarity(query, trace.Severity) > threshold {
		fields = append(fields, "severity")
	}
	if tokenSetSimilarity(query, trace.ComplianceGap) > threshold {
		fields = append(fields, "compliance_gap")
	}
	if tokenSetSimilarity(query, trace.Agent) > threshold {
		fields = append(fields, "agent")
	}
	if tokenSetSimilarity(query, trace.ToolSequence) > threshold {
		fields = append(fields, "tool_sequence")
	}

	return fields
}

// generateExplanation creates a human-readable explanation of why a trace matched.
func generateExplanation(query string, trace TraceMetadata, score float64) string {
	var parts []string

	if trace.FailureMode != "" {
		parts = append(parts, fmt.Sprintf("Failure mode: %s", trace.FailureMode))
	}
	if trace.ErrorMessage != "" {
		parts = append(parts, fmt.Sprintf("Error: %s", truncate(trace.ErrorMessage, 60)))
	}
	if trace.Severity != "" {
		parts = append(parts, fmt.Sprintf("Severity: %s", trace.Severity))
	}
	if score > 0.7 {
		parts = append(parts, "High semantic match")
	} else if score > 0.5 {
		parts = append(parts, "Moderate semantic match")
	}

	return strings.Join(parts, " | ")
}

// truncate shortens a string to a maximum length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// search finds the top K most similar traces to the query.
func search(query string, traces []TraceMetadata, topK int) []SearchResult {
	type scoredItem struct {
		result SearchResult
		score  float64
	}

	var scoredItems []scoredItem

	for _, trace := range traces {
		score := computeSimilarity(query, trace)
		if score > 0 {
			result := SearchResult{
				TraceID:         trace.TraceID,
				Metadata:        trace,
				SimilarityScore: math.Round(score*10000) / 10000, // 4 decimal places
				MatchingFields:  identifyMatchingFields(query, trace),
				Explanation:     generateExplanation(query, trace, score),
			}
			scoredItems = append(scoredItems, scoredItem{result: result, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scoredItems, func(i, j int) bool {
		return scoredItems[i].score > scoredItems[j].score
	})

	// Extract top K results
	if len(scoredItems) > topK {
		scoredItems = scoredItems[:topK]
	}

	results := make([]SearchResult, len(scoredItems))
	for i, s := range scoredItems {
		results[i] = s.result
	}

	return results
}

// loadTraces reads evaluation results from a JSON file and extracts trace metadata.
func loadTraces(filename string) ([]TraceMetadata, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var results []EvalResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("unmarshal JSON: %w", err)
	}

	traces := make([]TraceMetadata, len(results))
	for i, r := range results {
		traces[i] = TraceMetadata{
			TraceID:           r.TraceID,
			FailureMode:       r.FailureMode,
			ErrorMessage:      r.ErrorMessage,
			Severity:          r.Severity,
			Agent:             r.Agent,
			OrderID:           r.OrderID,
			CustomerSegment:   r.CustomerSegment,
			Domain:            r.Domain,
			RootCauseGulf:     r.RootCauseGulf,
			Confidence:        r.Confidence,
			ToolSequence:      r.ToolSequence,
			ComplianceGap:     r.ComplianceGap,
			SettlementAmount:  r.SettlementAmount,
			AMLRiskScore:      r.AMLRiskScore,
		}
	}

	return traces, nil
}

// outputJSON writes results as JSON.
func outputJSON(results *SearchResults, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

// outputCSV writes results as CSV.
func outputCSV(results *SearchResults, w io.Writer) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header
	header := []string{
		"trace_id",
		"failure_mode",
		"severity",
		"similarity_score",
		"agent",
		"order_id",
		"customer_segment",
		"domain",
		"matching_fields",
		"explanation",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Rows
	for _, result := range results.Results {
		row := []string{
			result.TraceID,
			result.Metadata.FailureMode,
			result.Metadata.Severity,
			fmt.Sprintf("%.4f", result.SimilarityScore),
			result.Metadata.Agent,
			result.Metadata.OrderID,
			result.Metadata.CustomerSegment,
			result.Metadata.Domain,
			strings.Join(result.MatchingFields, ";"),
			result.Explanation,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// outputText writes results as human-readable text.
func outputText(results *SearchResults, w io.Writer) error {
	fmt.Fprintf(w, "Semantic Search Results\n")
	fmt.Fprintf(w, "=======================\n")
	fmt.Fprintf(w, "Query: %q\n", results.Query)
	fmt.Fprintf(w, "Results: %d / %d\n\n", results.ResultCount, results.TopK)

	for i, result := range results.Results {
		fmt.Fprintf(w, "[%d] Trace: %s (Score: %.4f)\n", i+1, result.TraceID, result.SimilarityScore)
		fmt.Fprintf(w, "    Failure: %s\n", result.Metadata.FailureMode)
		fmt.Fprintf(w, "    Severity: %s | Domain: %s\n", result.Metadata.Severity, result.Metadata.Domain)
		fmt.Fprintf(w, "    Agent: %s | Order: %s\n", result.Metadata.Agent, result.Metadata.OrderID)
		fmt.Fprintf(w, "    Error: %s\n", truncate(result.Metadata.ErrorMessage, 80))
		fmt.Fprintf(w, "    Matching: %s\n", strings.Join(result.MatchingFields, ", "))
		fmt.Fprintf(w, "    Explanation: %s\n\n", result.Explanation)
	}

	return nil
}

func main() {
	query := flag.String("query", "", "Query string (e.g., 'settlement amount too high')")
	topk := flag.Int("topk", 10, "Number of results to return")
	dataset := flag.String("dataset", "eval_results.json", "Path to evaluation results JSON file")
	output := flag.String("output", "semantic_search_results.json", "Output file path")
	format := flag.String("format", "json", "Output format: json, csv, text")

	flag.Parse()

	if *query == "" {
		fmt.Fprintln(os.Stderr, "Error: -query flag is required")
		flag.Usage()
		os.Exit(1)
	}

	// Load traces
	traces, err := loadTraces(*dataset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading traces: %v\n", err)
		os.Exit(1)
	}

	if len(traces) == 0 {
		fmt.Fprintf(os.Stderr, "No traces found in %s\n", *dataset)
		os.Exit(1)
	}

	// Perform search
	results := search(*query, traces, *topk)

	// Create output structure
	searchResults := &SearchResults{
		Query:       *query,
		TopK:        *topk,
		ResultCount: len(results),
		Results:     results,
	}

	// Write output
	var w io.Writer
	var closeFunc func() error

	if *output == "" || *output == "-" {
		w = os.Stdout
		closeFunc = func() error { return nil }
	} else {
		f, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		w = f
		closeFunc = f.Close
	}
	defer closeFunc()

	var outputErr error
	switch *format {
	case "json":
		outputErr = outputJSON(searchResults, w)
	case "csv":
		outputErr = outputCSV(searchResults, w)
	case "text":
		outputErr = outputText(searchResults, w)
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s\n", *format)
		os.Exit(1)
	}

	if outputErr != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", outputErr)
		os.Exit(1)
	}

	if *output != "" && *output != "-" {
		fmt.Printf("Results written to %s\n", *output)
	}
}
