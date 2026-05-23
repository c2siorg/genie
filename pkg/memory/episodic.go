package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

// Episode is one chronological entry in a user's session.
type Episode struct {
	OccurredAt time.Time `json:"occurred_at"`
	Role       string    `json:"role"`
	Content    string    `json:"content"`
}

// EpisodicMemory keeps a rolling buffer of episodes per session. When the
// buffer grows past Threshold the oldest half is consolidated into a
// summary via the configured Summariser.
type EpisodicMemory struct {
	Threshold  int
	Summariser Summariser

	mu       sync.Mutex
	sessions map[string]*session
}

type session struct {
	episodes []Episode
	summary  string
}

// Summariser produces a one-paragraph summary of a list of episodes.
type Summariser interface {
	Summarise(ctx context.Context, episodes []Episode) (string, error)
}

// LLMSummariser uses an llm.Provider to summarise.
type LLMSummariser struct {
	Provider llm.Provider
	Model    string
}

// NewLLMSummariser returns a summariser.
func NewLLMSummariser(p llm.Provider, model string) *LLMSummariser {
	return &LLMSummariser{Provider: p, Model: model}
}

// Summarise asks the LLM for a one-paragraph rollup.
func (s *LLMSummariser) Summarise(ctx context.Context, eps []Episode) (string, error) {
	var sb strings.Builder
	for _, e := range eps {
		fmt.Fprintf(&sb, "[%s] %s: %s\n", e.OccurredAt.Format(time.RFC3339), e.Role, e.Content)
	}
	resp, err := s.Provider.Complete(ctx, llm.CompletionRequest{
		Model: s.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Summarise the conversation below in one short paragraph. Keep facts; drop pleasantries."},
			{Role: llm.RoleUser, Content: sb.String()},
		},
		MaxTokens: 200,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Text), nil
}

// NewEpisodicMemory constructs the buffer with the given threshold and
// summariser. threshold defaults to 20 if non-positive.
func NewEpisodicMemory(threshold int, s Summariser) *EpisodicMemory {
	if threshold <= 0 {
		threshold = 20
	}
	return &EpisodicMemory{Threshold: threshold, Summariser: s, sessions: map[string]*session{}}
}

// Append records an episode under sessionID. May trigger consolidation.
func (m *EpisodicMemory) Append(ctx context.Context, sessionID, role, content string) error {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	if !ok {
		s = &session{}
		m.sessions[sessionID] = s
	}
	s.episodes = append(s.episodes, Episode{OccurredAt: time.Now().UTC(), Role: role, Content: content})
	overflow := len(s.episodes) > m.Threshold
	var oldHalf []Episode
	if overflow {
		half := len(s.episodes) / 2
		oldHalf = append([]Episode(nil), s.episodes[:half]...)
		s.episodes = append([]Episode(nil), s.episodes[half:]...)
	}
	m.mu.Unlock()

	if overflow && m.Summariser != nil {
		summary, err := m.Summariser.Summarise(ctx, oldHalf)
		if err != nil {
			return err
		}
		m.mu.Lock()
		if s.summary == "" {
			s.summary = summary
		} else {
			s.summary = s.summary + "\n" + summary
		}
		m.mu.Unlock()
	}
	return nil
}

// Snapshot returns the current summary + recent episodes for sessionID.
// Useful to seed an LLM call with prior context.
func (m *EpisodicMemory) Snapshot(sessionID string) (summary string, recent []Episode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return "", nil
	}
	out := make([]Episode, len(s.episodes))
	copy(out, s.episodes)
	return s.summary, out
}
