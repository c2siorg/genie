// Package advisor - In-memory storage for MVP
// Production version will use PostgreSQL

package advisor

import (
	"fmt"
	"sync"
	"time"
)

// RecommendationStore provides in-memory storage for recommendations.
// Production version should use PostgreSQL with proper transactions.
type RecommendationStore struct {
	mu              sync.RWMutex
	recommendations map[string]*Recommendation
	feedback        map[string][]RecommendationFeedback
}

// NewRecommendationStore creates a new in-memory recommendation store.
func NewRecommendationStore() *RecommendationStore {
	return &RecommendationStore{
		recommendations: make(map[string]*Recommendation),
		feedback:        make(map[string][]RecommendationFeedback),
	}
}

// Save stores a recommendation.
func (s *RecommendationStore) Save(rec *Recommendation) error {
	if rec == nil {
		return fmt.Errorf("recommendation required")
	}
	if rec.RecommendationID == "" {
		return fmt.Errorf("recommendation_id required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.recommendations[rec.RecommendationID] = rec
	return nil
}

// Get retrieves a recommendation by ID.
func (s *RecommendationStore) Get(recommendationID string) (*Recommendation, error) {
	if recommendationID == "" {
		return nil, fmt.Errorf("recommendation_id required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.recommendations[recommendationID]
	if !ok {
		return nil, fmt.Errorf("recommendation not found: %s", recommendationID)
	}

	return rec, nil
}

// GetByUserID retrieves all recommendations for a user.
func (s *RecommendationStore) GetByUserID(userID string, limit int) ([]*Recommendation, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Recommendation
	for _, rec := range s.recommendations {
		if rec.UserID == userID {
			results = append(results, rec)
		}
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// SaveFeedback records user feedback on a recommendation.
func (s *RecommendationStore) SaveFeedback(recommendationID string, feedback RecommendationFeedback) error {
	if recommendationID == "" {
		return fmt.Errorf("recommendation_id required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify recommendation exists
	if _, ok := s.recommendations[recommendationID]; !ok {
		return fmt.Errorf("recommendation not found: %s", recommendationID)
	}

	// Update recommendation status
	s.recommendations[recommendationID].Status = func() RecommendationStatus {
		if feedback.Action == ActionAccept {
			return StatusAccepted
		} else if feedback.Action == ActionReject {
			return StatusRejected
		}
		return StatusDeferred
	}()
	s.recommendations[recommendationID].UpdatedAt = time.Now()

	// Store feedback
	s.feedback[recommendationID] = append(s.feedback[recommendationID], feedback)

	return nil
}

// GetFeedback retrieves feedback for a recommendation.
func (s *RecommendationStore) GetFeedback(recommendationID string) ([]RecommendationFeedback, error) {
	if recommendationID == "" {
		return nil, fmt.Errorf("recommendation_id required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	feedback, ok := s.feedback[recommendationID]
	if !ok {
		return []RecommendationFeedback{}, nil
	}

	return feedback, nil
}

// GetStats calculates statistics for a user.
func (s *RecommendationStore) GetStats(userID string) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total, accepted, rejected int
	var totalSavings int64

	for _, rec := range s.recommendations {
		if rec.UserID == userID {
			total++
			if rec.Status == StatusAccepted {
				accepted++
				totalSavings += rec.EstimatedImpact.MonthlySavingsPaise * 12
			} else if rec.Status == StatusRejected {
				rejected++
			}
		}
	}

	acceptanceRate := 0.0
	if total > 0 {
		acceptanceRate = float64(accepted) / float64(total) * 100
	}

	return map[string]interface{}{
		"total_count":                 total,
		"recommendations_accepted":    accepted,
		"recommendations_rejected":    rejected,
		"acceptance_rate":             acceptanceRate,
		"estimated_total_impact":      totalSavings,
	}
}
