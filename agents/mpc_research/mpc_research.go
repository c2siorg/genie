// Package mpc_research analyses RBI Monetary Policy Committee (MPC)
// statements and minutes. It is the Indian analogue of the US FOMC
// research agent in the Google ADK samples — adapted for the MPC
// calendar (six bi-monthly meetings per FY), the Indian rate stack
// (repo, reverse-repo, SDF, MSF, CRR, SLR), and RBI's communication style.
//
// What it does:
//  1. Parses a structured MPC summary (rate decision, vote split, stance,
//     forward guidance, growth/inflation projections).
//  2. Diffs the new summary against a baseline (typically the prior meeting).
//  3. Extracts policy signals — direction of change, hawkishness shift,
//     surprise vs market consensus.
//  4. Publishes a `mpc_signal` for downstream agents (loan_advisor,
//     prepayment_advisor, rate_watcher, recommender) to re-evaluate.
//
// No LLM is required — the parsing is structured. An LLM can be layered
// on top for plain-language summarisation via the reporter agent.
package mpc_research

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "mpc_research"
	Capability = "analyse_mpc_event"
	TypeIn     = "mpc_summary"
	TypeOut    = "mpc_signal"
	NextAgent  = "financial_supervisor"
)

// Summary is one MPC meeting outcome.
type Summary struct {
	MeetingDate         string  `json:"meeting_date"` // YYYY-MM-DD
	RepoRate            float64 `json:"repo_rate_pct"`
	SDFRate             float64 `json:"sdf_rate_pct"`
	MSFRate             float64 `json:"msf_rate_pct"`
	CRRPct              float64 `json:"crr_pct"`
	SLRPct              float64 `json:"slr_pct"`
	Stance              string  `json:"stance"`              // "accommodative" | "neutral" | "withdrawal_of_accommodation"
	VoteFor             int     `json:"vote_for"`            // out of 6
	VoteAgainst         int     `json:"vote_against"`
	CPIProjectionPctYoY float64 `json:"cpi_projection_pct"`
	GDPProjectionPctYoY float64 `json:"gdp_projection_pct"`
	ConsensusRepoBps    int     `json:"consensus_repo_change_bps"` // market expectation
}

// Request asks for a diff between this meeting and a prior baseline.
type Request struct {
	Current  Summary `json:"current"`
	Previous Summary `json:"previous"`
}

// Signal is the structured downstream payload.
type Signal struct {
	MeetingDate     string   `json:"meeting_date"`
	RepoChangeBps   int      `json:"repo_change_bps"`
	StanceShift     string   `json:"stance_shift"` // "easing" | "neutral" | "tightening" | "unchanged"
	HawkishnessΔ    string   `json:"hawkishness_delta"` // "more_hawkish" | "more_dovish" | "unchanged"
	SurpriseVsMkt   string   `json:"surprise"`     // "hawkish_surprise" | "dovish_surprise" | "in_line"
	CPIRevisionBps  int      `json:"cpi_revision_bps"`
	GDPRevisionBps  int      `json:"gdp_revision_bps"`
	DownstreamHints []string `json:"downstream_hints"`
	Headline        string   `json:"headline"`
	Disclaimer      string   `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "RBI MPC Research" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskLow }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	sig := a.Analyse(req.Current, req.Previous)
	env.Logf("[mpc_research] %s — repoΔ=%dbps stance=%s surprise=%s",
		sig.MeetingDate, sig.RepoChangeBps, sig.StanceShift, sig.SurpriseVsMkt)
	body, _ := json.Marshal(sig)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Analyse is pure — diffs two summaries and emits a structured signal.
func (a *Agent) Analyse(cur, prev Summary) Signal {
	repoBps := int((cur.RepoRate - prev.RepoRate) * 100)
	cpiBps := int((cur.CPIProjectionPctYoY - prev.CPIProjectionPctYoY) * 100)
	gdpBps := int((cur.GDPProjectionPctYoY - prev.GDPProjectionPctYoY) * 100)

	stanceShift := "unchanged"
	switch {
	case cur.Stance == "accommodative" && prev.Stance != "accommodative":
		stanceShift = "easing"
	case cur.Stance == "withdrawal_of_accommodation" && prev.Stance != "withdrawal_of_accommodation":
		stanceShift = "tightening"
	case cur.Stance == "neutral" && prev.Stance != "neutral":
		stanceShift = "neutral"
	}

	hawk := "unchanged"
	switch {
	case repoBps > 0 || cpiBps > 25:
		hawk = "more_hawkish"
	case repoBps < 0 || cpiBps < -25:
		hawk = "more_dovish"
	}

	surprise := "in_line"
	delta := repoBps - cur.ConsensusRepoBps
	switch {
	case delta > 15:
		surprise = "hawkish_surprise"
	case delta < -15:
		surprise = "dovish_surprise"
	}

	hints := []string{}
	if repoBps != 0 {
		hints = append(hints, "rate_watcher: refresh published rates")
		hints = append(hints, "loan_advisor: reprice floating-rate EMI projections")
		hints = append(hints, "prepayment_advisor: recompute effective APR rankings")
	}
	if cpiBps > 25 {
		hints = append(hints, "macro_research: flag inflation upside risk")
	}
	if gdpBps < -25 {
		hints = append(hints, "macro_research: flag growth downside risk")
	}

	headline := fmt.Sprintf("MPC kept policy steady on %s.", cur.MeetingDate)
	if repoBps > 0 {
		headline = fmt.Sprintf("MPC raised repo by %dbps on %s; stance %s.", repoBps, cur.MeetingDate, cur.Stance)
	} else if repoBps < 0 {
		headline = fmt.Sprintf("MPC cut repo by %dbps on %s; stance %s.", -repoBps, cur.MeetingDate, cur.Stance)
	}

	return Signal{
		MeetingDate:     cur.MeetingDate,
		RepoChangeBps:   repoBps,
		StanceShift:     stanceShift,
		HawkishnessΔ:    hawk,
		SurpriseVsMkt:   surprise,
		CPIRevisionBps:  cpiBps,
		GDPRevisionBps:  gdpBps,
		DownstreamHints: hints,
		Headline:        headline,
		Disclaimer: "Algorithmic summary of MPC statement; refer to the RBI press release for the " +
			"authoritative text. Not investment advice.",
	}
}
