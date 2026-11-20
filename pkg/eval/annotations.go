// Package eval provides evaluation infrastructure for Genie.
// annotations.go implements manual trace annotation for evaluation review workflows.
//
// Annotations allow evaluators to mark traces as pass/fail, classify failure modes,
// assess confidence, and attach human-readable notes. The Annotation struct captures
// all review metadata for downstream analysis (drift detection, rubric calibration).
package eval

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FailureModeCode represents a string code for a classified failure category.
// This is used in Annotation for quick reference; FailureMode struct in failure_modes.go
// provides richer metadata (severity, domain, description, etc.).
type FailureModeCode string

// FailureModeCluster groups related failure modes for trending analysis.
type FailureModeCluster string

// Standard failure mode codes for settlement, compliance, commerce, and infrastructure domains.
const (
	// Settlement failure modes
	FailureModeDoubleSpend         FailureModeCode = "double_spend"
	FailureModeSettlementAmountWrong FailureModeCode = "settlement_amount_wrong"
	FailureModeInconsistentLedger  FailureModeCode = "inconsistent_ledger"
	FailureModeReconciliationFail  FailureModeCode = "reconciliation_fail"

	// Compliance failure modes
	FailureModeAMLBypass           FailureModeCode = "aml_bypass"
	FailureModeVelocityExceeded    FailureModeCode = "velocity_exceeded"
	FailureModeKYCMissing          FailureModeCode = "kyc_missing"

	// Commerce failure modes
	FailureModeOrderStateBroken    FailureModeCode = "order_state_broken"
	FailureModePaymentMismatch     FailureModeCode = "payment_mismatch"
	FailureModeLineageIncomplete   FailureModeCode = "lineage_incomplete"

	// Merchant failure modes
	FailureModeMerchantUnboarded   FailureModeCode = "merchant_unboarded"
	FailureModeMerchantLimitExceeded FailureModeCode = "merchant_limit_exceeded"

	// Hallucination/Logic failure modes
	FailureModeHallucination       FailureModeCode = "hallucination"
	FailureModeLogicError          FailureModeCode = "logic_error"
	FailureModeLLMConfusion        FailureModeCode = "llm_confusion"

	// Infrastructure failure modes
	FailureModeTimeout             FailureModeCode = "timeout"
	FailureModeServiceUnavailable  FailureModeCode = "service_unavailable"
	FailureModeDatabaseError       FailureModeCode = "database_error"

	// Cluster names for multi-dimensional analysis
	ClusterSettlement FailureModeCluster = "settlement"
	ClusterCompliance FailureModeCluster = "compliance"
	ClusterCommerce   FailureModeCluster = "commerce"
	ClusterMerchant   FailureModeCluster = "merchant"
	ClusterLLM        FailureModeCluster = "llm_behavior"
	ClusterInfra      FailureModeCluster = "infrastructure"
)

// Annotation represents a manual evaluation review of a single trace.
//
// Evaluators use Annotation to mark pass/fail, classify failure root causes,
// assign confidence scores, and attach notes. Annotations enable:
// - Calibration of automated judges
// - Drift detection via trend analysis
// - Root cause analysis across failure modes
// - Evaluator consensus measurements
type Annotation struct {
	// ID is a unique identifier for this annotation (prefix: "ann-").
	ID string `json:"id"`

	// AnnotatorID identifies the human evaluator (user ID or email).
	AnnotatorID string `json:"annotator_id"`

	// TraceID links to the original execution trace.
	TraceID string `json:"trace_id"`

	// Label is the pass/fail judgment: "pass", "fail", or "uncertain".
	Label AnnotationLabel `json:"label"`

	// PrimaryFailure is the root cause classification (only set if Label=="fail").
	// Examples: "double_spend", "settlement_amount_wrong", "aml_bypass", "hallucination".
	PrimaryFailure FailureModeCode `json:"primary_failure,omitempty"`

	// SecondaryFailures captures contributing factors (e.g., ["timeout", "network_retry"]).
	SecondaryFailures []FailureModeCode `json:"secondary_failures,omitempty"`

	// Confidence is the evaluator's self-reported confidence [0..1].
	// 0 = guessing, 0.5 = uncertain, 1.0 = completely certain.
	Confidence float64 `json:"confidence"`

	// Notes is free-form annotation explaining the decision.
	// Useful for edge cases, ambiguous traces, or calibration discussions.
	Notes string `json:"notes,omitempty"`

	// RubricScores captures evaluation against specific rubrics (optional).
	// Maps rubric ID (e.g., "RB-SE-001") to the assigned score/level.
	RubricScores map[string]*RubricScore `json:"rubric_scores,omitempty"`

	// Timestamp is when the annotation was created.
	Timestamp time.Time `json:"timestamp"`

	// UpdatedAt tracks the last modification time.
	UpdatedAt time.Time `json:"updated_at"`

	// Deferred indicates this trace is too ambiguous and should be reviewed later.
	// When set, PrimaryFailure may be empty; Confidence would be low.
	Deferred bool `json:"deferred"`

	// DeferredReason explains why this trace was deferred (if Deferred==true).
	DeferredReason string `json:"deferred_reason,omitempty"`
}

// AnnotationLabel represents a pass/fail/uncertain judgment.
type AnnotationLabel string

const (
	LabelPass      AnnotationLabel = "pass"
	LabelFail      AnnotationLabel = "fail"
	LabelUncertain AnnotationLabel = "uncertain"
)

// AnnotationStore persists and retrieves annotations.
type AnnotationStore interface {
	// Save persists an annotation.
	Save(ctx context.Context, ann *Annotation) error

	// Get retrieves a single annotation by ID.
	Get(ctx context.Context, id string) (*Annotation, error)

	// GetByTraceID retrieves all annotations for a trace.
	GetByTraceID(ctx context.Context, traceID string) ([]*Annotation, error)

	// GetByAnnotatorID retrieves all annotations from a specific evaluator.
	GetByAnnotatorID(ctx context.Context, annotatorID string) ([]*Annotation, error)

	// ListByLabel retrieves all annotations with a specific label.
	ListByLabel(ctx context.Context, label AnnotationLabel) ([]*Annotation, error)

	// ListByFailureMode retrieves all annotations with a specific primary failure code.
	ListByFailureMode(ctx context.Context, fm FailureModeCode) ([]*Annotation, error)

	// List retrieves all annotations (with optional pagination).
	List(ctx context.Context, offset, limit int) ([]*Annotation, error)

	// Delete removes an annotation.
	Delete(ctx context.Context, id string) error
}

// InMemoryAnnotationStore is a thread-safe in-memory implementation.
type InMemoryAnnotationStore struct {
	mu          sync.RWMutex
	annotations map[string]*Annotation
	nextID      int
}

// NewInMemoryAnnotationStore constructs a new in-memory annotation store.
func NewInMemoryAnnotationStore() *InMemoryAnnotationStore {
	return &InMemoryAnnotationStore{
		annotations: make(map[string]*Annotation),
	}
}

// Save persists an annotation. If ID is empty, generates a new one.
func (s *InMemoryAnnotationStore) Save(ctx context.Context, ann *Annotation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ann.ID == "" {
		s.nextID++
		ann.ID = fmt.Sprintf("ann-%d", s.nextID)
	}
	if ann.Timestamp.IsZero() {
		ann.Timestamp = time.Now()
	}
	ann.UpdatedAt = time.Now()

	s.annotations[ann.ID] = ann
	return nil
}

// Get retrieves a single annotation by ID.
func (s *InMemoryAnnotationStore) Get(ctx context.Context, id string) (*Annotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ann, ok := s.annotations[id]
	if !ok {
		return nil, fmt.Errorf("annotation not found: %s", id)
	}
	return ann, nil
}

// GetByTraceID retrieves all annotations for a specific trace.
func (s *InMemoryAnnotationStore) GetByTraceID(ctx context.Context, traceID string) ([]*Annotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Annotation
	for _, ann := range s.annotations {
		if ann.TraceID == traceID {
			result = append(result, ann)
		}
	}
	return result, nil
}

// GetByAnnotatorID retrieves all annotations from a specific evaluator.
func (s *InMemoryAnnotationStore) GetByAnnotatorID(ctx context.Context, annotatorID string) ([]*Annotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Annotation
	for _, ann := range s.annotations {
		if ann.AnnotatorID == annotatorID {
			result = append(result, ann)
		}
	}
	return result, nil
}

// ListByLabel retrieves all annotations with a specific label.
func (s *InMemoryAnnotationStore) ListByLabel(ctx context.Context, label AnnotationLabel) ([]*Annotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Annotation
	for _, ann := range s.annotations {
		if ann.Label == label {
			result = append(result, ann)
		}
	}
	return result, nil
}

// ListByFailureMode retrieves all annotations with a specific primary failure code.
func (s *InMemoryAnnotationStore) ListByFailureMode(ctx context.Context, fm FailureModeCode) ([]*Annotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Annotation
	for _, ann := range s.annotations {
		if ann.PrimaryFailure == fm {
			result = append(result, ann)
		}
	}
	return result, nil
}

// List retrieves all annotations with pagination.
func (s *InMemoryAnnotationStore) List(ctx context.Context, offset, limit int) ([]*Annotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Annotation
	for _, ann := range s.annotations {
		result = append(result, ann)
	}

	// Simple pagination
	if offset >= len(result) {
		return []*Annotation{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}

	return result[offset:end], nil
}

// Delete removes an annotation.
func (s *InMemoryAnnotationStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.annotations, id)
	return nil
}

// TraceWithAnnotations pairs a trace with its annotations for review.
type TraceWithAnnotations struct {
	TraceID     string          `json:"trace_id"`
	Scenario    string          `json:"scenario"`
	Success     bool            `json:"success"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
	StartedAt   time.Time       `json:"started_at"`
	EndedAt     time.Time       `json:"ended_at"`
	Annotations []*Annotation   `json:"annotations,omitempty"`
	CanonicalAnnotation *Annotation `json:"canonical_annotation,omitempty"` // consensus or latest
}

// SimilarTracesResult describes traces similar to a given trace for clustering analysis.
type SimilarTracesResult struct {
	TraceID      string  `json:"trace_id"`
	SimilarityScore float64 `json:"similarity_score"` // [0..1]
	FailureMode  FailureMode `json:"failure_mode,omitempty"`
	Distance     float64 `json:"distance"` // for clustering algorithms
}

// FailureModeTrend tracks failure mode prevalence over time.
type FailureModeTrend struct {
	FailureMode     FailureModeCode `json:"failure_mode"`
	Cluster         FailureModeCluster `json:"cluster"`
	Count           int         `json:"count"`
	Percentage      float64     `json:"percentage"`
	MostRecentTrace string      `json:"most_recent_trace"`
	MostRecentTime  time.Time   `json:"most_recent_time"`
	RubricsAffected []string    `json:"rubrics_affected"`
}

// AnnotationStats aggregates annotation coverage metrics.
type AnnotationStats struct {
	TotalTraces          int
	AnnotatedTraces      int
	UnannotatedTraces    int
	PassCount            int
	FailCount            int
	UncertainCount       int
	DeferredCount        int
	AvgConfidence        float64
	FailureModeClusters  map[FailureModeCluster]int
	TopFailureModes      []*FailureModeTrend
	AnnotatorCount       int
	AnnotatorAgreement   float64 // inter-rater agreement
}

// AnnotationQuery filters annotations for list operations.
type AnnotationQuery struct {
	TraceID       string
	AnnotatorID   string
	Label         AnnotationLabel
	PrimaryFailure FailureModeCode
	Cluster       FailureModeCluster
	StartTime     time.Time
	EndTime       time.Time
	MinConfidence float64
	OnlyDeferred  bool
	Offset        int
	Limit         int
}
