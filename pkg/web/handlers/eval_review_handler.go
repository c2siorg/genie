package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval"
	"github.com/go-chi/chi/v5"
)

// EvalReviewHandler manages HTTP endpoints for trace review and annotation.
//
// Endpoints:
// - GET /v1/eval/traces — List traces with filtering and sampling
// - GET /v1/eval/traces/{trace_id} — Fetch single trace with annotations
// - POST /v1/eval/traces/{trace_id}/feedback — Submit or update annotation
// - GET /v1/eval/clusters — Cluster traces by failure mode
// - GET /v1/eval/search — Semantic or keyword search across traces
//
// Handler bridges HTTP requests to EvalStore and AnnotationStore interfaces.
type EvalReviewHandler struct {
	EvalStore        eval.Store            // Read traces
	AnnotationStore  eval.AnnotationStore  // Read/write annotations
	TraceCache       map[string]*eval.InteractionRecord // in-memory cache for demo
}

// NewEvalReviewHandler constructs a new evaluation review handler.
func NewEvalReviewHandler(
	evalStore eval.Store,
	annotationStore eval.AnnotationStore,
) *EvalReviewHandler {
	return &EvalReviewHandler{
		EvalStore:       evalStore,
		AnnotationStore: annotationStore,
		TraceCache:      make(map[string]*eval.InteractionRecord),
	}
}

// ListTracesRequest represents query parameters for GET /v1/eval/traces.
type ListTracesRequest struct {
	Limit  int    `json:"limit"`  // default 20
	Sample string `json:"sample"` // "random", "failure", "uncertainty"
	Offset int    `json:"offset"` // for pagination
}

// ListTracesResponse wraps a batch of traces ready for review.
type ListTracesResponse struct {
	Traces       []*eval.TraceWithAnnotations `json:"traces"`
	Total        int                          `json:"total"`
	Offset       int                          `json:"offset"`
	Limit        int                          `json:"limit"`
	HasMore      bool                         `json:"has_more"`
}

// ListTraces handles GET /v1/eval/traces?limit=20&sample=random|failure|uncertainty
//
// Query parameters:
//   - limit: number of traces to return (default: 20, max: 100)
//   - sample: sampling strategy:
//     - "random": uniformly random traces
//     - "failure": traces with failed annotations
//     - "uncertainty": traces with low-confidence or deferred annotations
//   - offset: pagination offset (default: 0)
//
// Returns JSON list of traces with their annotations.
func (h *EvalReviewHandler) ListTraces(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	sampleStrategy := r.URL.Query().Get("sample")
	if sampleStrategy == "" {
		sampleStrategy = "random"
	}

	allTraces := h.EvalStore.List()

	// Apply sampling strategy
	filtered := allTraces
	switch sampleStrategy {
	case "failure":
		filtered = h.filterTracesWithFailures(allTraces)
	case "uncertainty":
		filtered = h.filterUncertainTraces(allTraces)
	case "random":
		// Keep all; will paginate below
	default:
		respondError(w, http.StatusBadRequest, "invalid sample strategy")
		return
	}

	total := len(filtered)
	if offset >= total {
		respondJSON(w, http.StatusOK, &ListTracesResponse{
			Traces: []*eval.TraceWithAnnotations{},
			Total:  total,
			Offset: offset,
			Limit:  limit,
		})
		return
	}

	end := offset + limit
	if end > total {
		end = total
	}

	traceBatch := filtered[offset:end]
	ctx := r.Context()

	// Enrich each trace with annotations
	enriched := make([]*eval.TraceWithAnnotations, len(traceBatch))
	for i, trace := range traceBatch {
		enriched[i] = &eval.TraceWithAnnotations{
			TraceID:   trace.ID,
			Scenario:  trace.Scenario,
			Success:   trace.Success,
			Metrics:   trace.Metrics,
			Metadata:  trace.Metadata,
			StartedAt: trace.StartedAt,
			EndedAt:   trace.EndedAt,
		}

		// Fetch annotations for this trace
		if annotations, err := h.AnnotationStore.GetByTraceID(ctx, trace.ID); err == nil && len(annotations) > 0 {
			enriched[i].Annotations = annotations
			enriched[i].CanonicalAnnotation = annotations[0] // Latest is first
		}
	}

	respondJSON(w, http.StatusOK, &ListTracesResponse{
		Traces:  enriched,
		Total:   total,
		Offset:  offset,
		Limit:   limit,
		HasMore: end < total,
	})
}

// GetTraceRequest represents the path parameter for GET /v1/eval/traces/{trace_id}.
type GetTraceResponse struct {
	*eval.TraceWithAnnotations
	AvailableFailureModes []eval.FailureModeCode `json:"available_failure_modes"`
	AvailableRubrics      map[string]*eval.Rubric `json:"available_rubrics"`
}

// GetTrace handles GET /v1/eval/traces/{trace_id}
//
// Returns a single trace with:
//   - Full trace metadata
//   - All associated annotations
//   - List of available failure modes for classification
//   - Relevant rubrics for evaluation
func (h *EvalReviewHandler) GetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "trace_id")
	if traceID == "" {
		respondError(w, http.StatusBadRequest, "trace_id required")
		return
	}

	allTraces := h.EvalStore.List()
	var trace *eval.InteractionRecord
	for i := range allTraces {
		if allTraces[i].ID == traceID {
			trace = &allTraces[i]
			break
		}
	}

	if trace == nil {
		respondError(w, http.StatusNotFound, "trace not found")
		return
	}

	ctx := r.Context()

	// Fetch all annotations for this trace
	annotations, _ := h.AnnotationStore.GetByTraceID(ctx, traceID)

	// Build response
	resp := &GetTraceResponse{
		TraceWithAnnotations: &eval.TraceWithAnnotations{
			TraceID:     trace.ID,
			Scenario:    trace.Scenario,
			Success:     trace.Success,
			Metrics:     trace.Metrics,
			Metadata:    trace.Metadata,
			StartedAt:   trace.StartedAt,
			EndedAt:     trace.EndedAt,
			Annotations: annotations,
		},
		AvailableFailureModes: h.listAvailableFailureModes(),
		AvailableRubrics:      eval.AllRubrics,
	}

	if len(annotations) > 0 {
		resp.CanonicalAnnotation = annotations[0]
	}

	respondJSON(w, http.StatusOK, resp)
}

// SubmitFeedbackRequest represents the POST body for /v1/eval/traces/{trace_id}/feedback.
type SubmitFeedbackRequest struct {
	AnnotatorID      string              `json:"annotator_id"`
	Label            eval.AnnotationLabel `json:"label"`
	PrimaryFailure   eval.FailureModeCode    `json:"primary_failure,omitempty"`
	SecondaryFailures []eval.FailureModeCode `json:"secondary_failures,omitempty"`
	Confidence       float64             `json:"confidence"`
	Notes            string              `json:"notes,omitempty"`
	RubricScores     map[string]*eval.RubricScore `json:"rubric_scores,omitempty"`
	Deferred         bool                `json:"deferred"`
	DeferredReason   string              `json:"deferred_reason,omitempty"`
}

// SubmitFeedbackResponse confirms annotation was stored.
type SubmitFeedbackResponse struct {
	ID        string    `json:"id"`
	TraceID   string    `json:"trace_id"`
	Timestamp time.Time `json:"timestamp"`
}

// SubmitFeedback handles POST /v1/eval/traces/{trace_id}/feedback
//
// Creates or updates an annotation for a trace. Multiple evaluators can
// annotate the same trace independently. Returns the annotation ID.
func (h *EvalReviewHandler) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "trace_id")
	if traceID == "" {
		respondError(w, http.StatusBadRequest, "trace_id required")
		return
	}

	var req SubmitFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.AnnotatorID == "" {
		respondError(w, http.StatusBadRequest, "annotator_id required")
		return
	}

	if req.Label != eval.LabelPass && req.Label != eval.LabelFail && req.Label != eval.LabelUncertain {
		respondError(w, http.StatusBadRequest, "invalid label")
		return
	}

	if req.Confidence < 0 || req.Confidence > 1 {
		respondError(w, http.StatusBadRequest, "confidence must be in [0..1]")
		return
	}

	// Build annotation
	annotation := &eval.Annotation{
		AnnotatorID:       req.AnnotatorID,
		TraceID:           traceID,
		Label:             req.Label,
		PrimaryFailure:    req.PrimaryFailure,
		SecondaryFailures: req.SecondaryFailures,
		Confidence:        req.Confidence,
		Notes:             req.Notes,
		RubricScores:      req.RubricScores,
		Deferred:          req.Deferred,
		DeferredReason:    req.DeferredReason,
	}

	ctx := r.Context()
	if err := h.AnnotationStore.Save(ctx, annotation); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save annotation")
		return
	}

	respondJSON(w, http.StatusOK, &SubmitFeedbackResponse{
		ID:        annotation.ID,
		TraceID:   traceID,
		Timestamp: annotation.Timestamp,
	})
}

// ClusterTracesResponse describes trace clusters by failure mode.
type ClusterTracesResponse struct {
	Clusters map[eval.FailureModeCluster][]*eval.FailureModeTrend `json:"clusters"`
	Total    int                                                   `json:"total"`
}

// GetClusters handles GET /v1/eval/clusters?dimension=failure_mode|severity|domain
//
// Groups annotated traces by failure mode cluster and returns trend statistics.
// Useful for identifying systemic issues and prioritizing improvement efforts.
func (h *EvalReviewHandler) GetClusters(w http.ResponseWriter, r *http.Request) {
	dimension := r.URL.Query().Get("dimension")
	if dimension == "" {
		dimension = "failure_mode"
	}

	ctx := r.Context()

	// Fetch all annotations with failures
	failAnnotations, _ := h.AnnotationStore.ListByLabel(ctx, eval.LabelFail)

	// Group by cluster
	clusters := make(map[eval.FailureModeCluster]*eval.FailureModeTrend)

	for _, ann := range failAnnotations {
		if ann.PrimaryFailure == "" {
			continue
		}

		cluster := h.failureModeToCluster(ann.PrimaryFailure)
		if _, ok := clusters[cluster]; !ok {
			clusters[cluster] = &eval.FailureModeTrend{
				Cluster:     cluster,
				FailureMode: ann.PrimaryFailure,
			}
		}

		trend := clusters[cluster]
		trend.Count++
		if ann.Timestamp.After(trend.MostRecentTime) {
			trend.MostRecentTime = ann.Timestamp
			trend.MostRecentTrace = ann.TraceID
		}
	}

	// Calculate percentages
	total := len(failAnnotations)
	for _, trend := range clusters {
		trend.Percentage = float64(trend.Count) / float64(total) * 100
	}

	resp := &ClusterTracesResponse{
		Clusters: make(map[eval.FailureModeCluster][]*eval.FailureModeTrend),
		Total:    total,
	}

	for cluster, trend := range clusters {
		resp.Clusters[cluster] = []*eval.FailureModeTrend{trend}
	}

	respondJSON(w, http.StatusOK, resp)
}

// SearchRequest represents a query for GET /v1/eval/search.
type SearchRequest struct {
	Query      string `json:"query"`
	QueryType  string `json:"query_type"` // "semantic" or "keyword"
	Limit      int    `json:"limit"`
}

// SearchResponse wraps search results.
type SearchResponse struct {
	Results []*eval.TraceWithAnnotations `json:"results"`
	Total   int                          `json:"total"`
	Query   string                       `json:"query"`
}

// Search handles GET /v1/eval/search?query=...&query_type=semantic|keyword
//
// Performs keyword search across trace metadata and annotations.
// Semantic search would require embedding infrastructure (out of scope for MVP).
func (h *EvalReviewHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		respondError(w, http.StatusBadRequest, "query parameter required")
		return
	}

	queryType := r.URL.Query().Get("query_type")
	if queryType == "" {
		queryType = "keyword"
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	allTraces := h.EvalStore.List()
	query = strings.ToLower(query)

	// Simple keyword search across scenario and metadata
	var matched []*eval.InteractionRecord
	for i := range allTraces {
		trace := &allTraces[i]
		if strings.Contains(strings.ToLower(trace.Scenario), query) {
			matched = append(matched, trace)
			continue
		}

		// Search metadata values
		for _, v := range trace.Metadata {
			if vs, ok := v.(string); ok && strings.Contains(strings.ToLower(vs), query) {
				matched = append(matched, trace)
				break
			}
		}
	}

	// Limit results
	if len(matched) > limit {
		matched = matched[:limit]
	}

	ctx := r.Context()
	enriched := make([]*eval.TraceWithAnnotations, len(matched))
	for i, trace := range matched {
		enriched[i] = &eval.TraceWithAnnotations{
			TraceID:   trace.ID,
			Scenario:  trace.Scenario,
			Success:   trace.Success,
			Metrics:   trace.Metrics,
			Metadata:  trace.Metadata,
			StartedAt: trace.StartedAt,
			EndedAt:   trace.EndedAt,
		}

		if annotations, err := h.AnnotationStore.GetByTraceID(ctx, trace.ID); err == nil && len(annotations) > 0 {
			enriched[i].Annotations = annotations
			enriched[i].CanonicalAnnotation = annotations[0]
		}
	}

	respondJSON(w, http.StatusOK, &SearchResponse{
		Results: enriched,
		Total:   len(matched),
		Query:   query,
	})
}

// === Helper methods ===

// filterTracesWithFailures returns traces that have failure annotations.
func (h *EvalReviewHandler) filterTracesWithFailures(traces []eval.InteractionRecord) []eval.InteractionRecord {
	ctx := context.Background()
	failAnnotations, _ := h.AnnotationStore.ListByLabel(ctx, eval.LabelFail)

	failTraceIDs := make(map[string]bool)
	for _, ann := range failAnnotations {
		failTraceIDs[ann.TraceID] = true
	}

	var result []eval.InteractionRecord
	for i := range traces {
		if failTraceIDs[traces[i].ID] {
			result = append(result, traces[i])
		}
	}
	return result
}

// filterUncertainTraces returns traces with low-confidence or deferred annotations.
func (h *EvalReviewHandler) filterUncertainTraces(traces []eval.InteractionRecord) []eval.InteractionRecord {
	ctx := context.Background()
	allAnnotations, _ := h.AnnotationStore.List(ctx, 0, 10000)

	uncertainTraceIDs := make(map[string]bool)
	for _, ann := range allAnnotations {
		if ann.Confidence < 0.7 || ann.Deferred {
			uncertainTraceIDs[ann.TraceID] = true
		}
	}

	var result []eval.InteractionRecord
	for i := range traces {
		if uncertainTraceIDs[traces[i].ID] {
			result = append(result, traces[i])
		}
	}
	return result
}

// listAvailableFailureModes returns all defined failure mode codes.
func (h *EvalReviewHandler) listAvailableFailureModes() []eval.FailureModeCode {
	return []eval.FailureModeCode{
		eval.FailureModeDoubleSpend,
		eval.FailureModeSettlementAmountWrong,
		eval.FailureModeInconsistentLedger,
		eval.FailureModeReconciliationFail,
		eval.FailureModeAMLBypass,
		eval.FailureModeVelocityExceeded,
		eval.FailureModeKYCMissing,
		eval.FailureModeOrderStateBroken,
		eval.FailureModePaymentMismatch,
		eval.FailureModeLineageIncomplete,
		eval.FailureModeMerchantUnboarded,
		eval.FailureModeMerchantLimitExceeded,
		eval.FailureModeHallucination,
		eval.FailureModeLogicError,
		eval.FailureModeLLMConfusion,
		eval.FailureModeTimeout,
		eval.FailureModeServiceUnavailable,
		eval.FailureModeDatabaseError,
	}
}

// failureModeToCluster maps a failure mode code to its cluster.
func (h *EvalReviewHandler) failureModeToCluster(fm eval.FailureModeCode) eval.FailureModeCluster {
	switch fm {
	case eval.FailureModeDoubleSpend, eval.FailureModeSettlementAmountWrong, eval.FailureModeInconsistentLedger, eval.FailureModeReconciliationFail:
		return eval.ClusterSettlement
	case eval.FailureModeAMLBypass, eval.FailureModeVelocityExceeded, eval.FailureModeKYCMissing:
		return eval.ClusterCompliance
	case eval.FailureModeOrderStateBroken, eval.FailureModePaymentMismatch, eval.FailureModeLineageIncomplete:
		return eval.ClusterCommerce
	case eval.FailureModeMerchantUnboarded, eval.FailureModeMerchantLimitExceeded:
		return eval.ClusterMerchant
	case eval.FailureModeHallucination, eval.FailureModeLogicError, eval.FailureModeLLMConfusion:
		return eval.ClusterLLM
	case eval.FailureModeTimeout, eval.FailureModeServiceUnavailable, eval.FailureModeDatabaseError:
		return eval.ClusterInfra
	default:
		return eval.ClusterCommerce
	}
}

// Mount registers all eval review routes on the given chi router.
func (h *EvalReviewHandler) Mount(r chi.Router) {
	r.Route("/traces", func(r chi.Router) {
		r.Get("/", h.ListTraces)
		r.Get("/{trace_id}", h.GetTrace)
		r.Post("/{trace_id}/feedback", h.SubmitFeedback)
	})
	r.Get("/clusters", h.GetClusters)
	r.Get("/search", h.Search)
}
