package mpc_research

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestRateHike(t *testing.T) {
	prev := Summary{MeetingDate: "2025-12-06", RepoRate: 6.50, Stance: "neutral", CPIProjectionPctYoY: 4.5, GDPProjectionPctYoY: 6.8}
	cur := Summary{MeetingDate: "2026-02-07", RepoRate: 6.75, Stance: "withdrawal_of_accommodation",
		CPIProjectionPctYoY: 4.8, GDPProjectionPctYoY: 6.6, ConsensusRepoBps: 0}
	sig := New().Analyse(cur, prev)
	if sig.RepoChangeBps != 25 {
		t.Errorf("expected 25bps hike, got %d", sig.RepoChangeBps)
	}
	if sig.StanceShift != "tightening" {
		t.Errorf("expected tightening shift, got %s", sig.StanceShift)
	}
	if sig.SurpriseVsMkt != "hawkish_surprise" {
		t.Errorf("expected hawkish_surprise, got %s", sig.SurpriseVsMkt)
	}
}

func TestRateCut(t *testing.T) {
	prev := Summary{MeetingDate: "2025-12-06", RepoRate: 6.50, Stance: "neutral", CPIProjectionPctYoY: 4.5, GDPProjectionPctYoY: 6.8}
	cur := Summary{MeetingDate: "2026-02-07", RepoRate: 6.25, Stance: "accommodative",
		CPIProjectionPctYoY: 4.2, GDPProjectionPctYoY: 7.0, ConsensusRepoBps: -25}
	sig := New().Analyse(cur, prev)
	if sig.RepoChangeBps != -25 {
		t.Errorf("expected -25bps, got %d", sig.RepoChangeBps)
	}
	if sig.StanceShift != "easing" {
		t.Errorf("expected easing shift, got %s", sig.StanceShift)
	}
	if sig.SurpriseVsMkt != "in_line" {
		t.Errorf("expected in_line with consensus, got %s", sig.SurpriseVsMkt)
	}
}

func TestHoldNoSurprise(t *testing.T) {
	prev := Summary{MeetingDate: "2025-12-06", RepoRate: 6.50, Stance: "neutral"}
	cur := Summary{MeetingDate: "2026-02-07", RepoRate: 6.50, Stance: "neutral"}
	sig := New().Analyse(cur, prev)
	if sig.RepoChangeBps != 0 {
		t.Errorf("expected zero change, got %d", sig.RepoChangeBps)
	}
	if !strings.Contains(sig.Headline, "kept policy steady") {
		t.Errorf("headline should reflect hold; got %q", sig.Headline)
	}
}

func TestDownstreamHintsOnRateMove(t *testing.T) {
	prev := Summary{RepoRate: 6.50}
	cur := Summary{RepoRate: 6.75}
	sig := New().Analyse(cur, prev)
	hits := 0
	for _, h := range sig.DownstreamHints {
		if strings.Contains(h, "rate_watcher") || strings.Contains(h, "loan_advisor") || strings.Contains(h, "prepayment_advisor") {
			hits++
		}
	}
	if hits < 3 {
		t.Errorf("rate change should fan out to at least 3 downstream agents; got %v", sig.DownstreamHints)
	}
}

func TestInflationUpsideHint(t *testing.T) {
	prev := Summary{CPIProjectionPctYoY: 4.0}
	cur := Summary{CPIProjectionPctYoY: 4.5} // +50bps
	sig := New().Analyse(cur, prev)
	hit := false
	for _, h := range sig.DownstreamHints {
		if strings.Contains(h, "inflation upside") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected inflation upside hint on +50bps CPI revision; got %+v", sig.DownstreamHints)
	}
}

func TestHandleMessageDispatches(t *testing.T) {
	req := Request{Current: Summary{RepoRate: 6.75}, Previous: Summary{RepoRate: 6.50}}
	body, _ := json.Marshal(req)
	msg := agent.NewMessage("up", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected single dispatch of %s, got %+v", TypeOut, out)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	sig := New().Analyse(Summary{}, Summary{})
	if sig.Disclaimer == "" {
		t.Errorf("disclaimer must be present")
	}
}
