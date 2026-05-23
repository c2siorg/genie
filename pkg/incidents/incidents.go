// Package incidents implements the RBI FREE-AI Annexure VI AI Incident
// Reporting form. Records flow into Postgres but the domain is storage-
// agnostic so we can unit-test the recording/grading logic in isolation.
//
// Severity ladder follows the form's three-tier scale (Low / Moderate / High).
// "FailureMode" is the structured root-cause taxonomy the report's body uses
// (bias, hallucination, explainability gap, etc.).
package incidents

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Severity matches the Annexure VI form's "Estimated Impact" field.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityModerate Severity = "moderate"
	SeverityHigh     Severity = "high"
)

// Status matches the Annexure VI form's "Current Status".
type Status string

const (
	StatusOngoing  Status = "ongoing"
	StatusResolved Status = "resolved"
)

// FailureMode is the structured taxonomy the report names in para 4.4.63.
type FailureMode string

const (
	FailureBias                 FailureMode = "bias"
	FailureHallucination        FailureMode = "hallucination"
	FailureExplainability       FailureMode = "explainability_gap"
	FailurePrivacyBreach        FailureMode = "privacy_breach"
	FailureUnintendedAction     FailureMode = "unintended_action"
	FailurePolicyDenied         FailureMode = "policy_denied"
	FailureAgentError           FailureMode = "agent_error"
	FailureUnknown              FailureMode = "unknown"
)

// Incident is the persistent record. Field names mirror the Annexure VI form.
type Incident struct {
	ID                string                 `json:"id"`
	OccurredAt        time.Time              `json:"occurred_at"`
	DetectedAt        time.Time              `json:"detected_at"`
	UseCase           string                 `json:"use_case"`
	Model             string                 `json:"model"`
	ThirdPartyVendor  string                 `json:"third_party_vendor,omitempty"`
	Description       string                 `json:"description"`
	AffectedStakeholders string              `json:"affected_stakeholders"` // internal | external | both
	Severity          Severity               `json:"severity"`
	FailureMode       FailureMode            `json:"failure_mode"`
	RootCause         string                 `json:"root_cause,omitempty"`
	ResponseActions   string                 `json:"response_actions,omitempty"`
	Status            Status                 `json:"status"`
	ActorID           string                 `json:"actor_id,omitempty"` // who reported / detected
	Metadata          map[string]any         `json:"metadata,omitempty"`
}

// Validate returns an error if the incident is missing fields required by
// Annexure VI. Severity defaults to Low if blank.
func (i *Incident) Validate() error {
	if strings.TrimSpace(i.Description) == "" {
		return errors.New("description is required")
	}
	if i.UseCase == "" {
		return errors.New("use_case is required")
	}
	if i.OccurredAt.IsZero() {
		i.OccurredAt = time.Now().UTC()
	}
	if i.DetectedAt.IsZero() {
		i.DetectedAt = time.Now().UTC()
	}
	if i.Severity == "" {
		i.Severity = SeverityLow
	}
	if i.Status == "" {
		i.Status = StatusOngoing
	}
	if i.FailureMode == "" {
		i.FailureMode = FailureUnknown
	}
	if i.ID == "" {
		i.ID = uuid.NewString()
	}
	return nil
}

// Store persists incidents. Implementations live in pkg/storage/postgres and
// in this package's tests.
type Store interface {
	Create(ctx context.Context, i Incident) (Incident, error)
	List(ctx context.Context, limit int) ([]Incident, error)
	CountByModeSince(ctx context.Context, mode FailureMode, since time.Time) (int, error)
}

// InMemoryStore is the test/demo implementation.
type InMemoryStore struct {
	items []Incident
}

// NewInMemoryStore constructs an in-memory store.
func NewInMemoryStore() *InMemoryStore { return &InMemoryStore{} }

func (s *InMemoryStore) Create(_ context.Context, i Incident) (Incident, error) {
	if err := (&i).Validate(); err != nil {
		return Incident{}, err
	}
	s.items = append(s.items, i)
	return i, nil
}

func (s *InMemoryStore) List(_ context.Context, limit int) ([]Incident, error) {
	if limit <= 0 || limit > len(s.items) {
		limit = len(s.items)
	}
	out := make([]Incident, limit)
	copy(out, s.items[len(s.items)-limit:])
	return out, nil
}

func (s *InMemoryStore) CountByModeSince(_ context.Context, mode FailureMode, since time.Time) (int, error) {
	c := 0
	for _, i := range s.items {
		if i.FailureMode == mode && !i.OccurredAt.Before(since) {
			c++
		}
	}
	return c, nil
}
