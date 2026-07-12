package catalog

import (
	"encoding/json"
	"fmt"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// MpcResearchSpec ports agents/mpc_research. It analyses an RBI Monetary Policy
// Committee (MPC) outcome by diffing the current meeting summary against a prior
// baseline. The diff is pure and deterministic: it derives the repo-rate change in
// basis points (repo_rate_pct delta * 100), CPI and GDP projection revisions in bps,
// a stance shift (easing/tightening/neutral/unchanged) from the change in RBI's
// stated stance, a hawkishness delta (more_hawkish if repo rose or CPI projection
// climbed > 25 bps; more_dovish on the mirror case), and a surprise-vs-market read
// (hawkish_surprise / dovish_surprise / in_line) by comparing the realised repo
// change against the market consensus in bps with a +/-15 bps tolerance band. It
// also emits downstream hints for rate_watcher, loan_advisor, prepayment_advisor and
// macro_research plus a plain headline. Outputs are an algorithmic summary of the MPC
// statement, advisory/informational only per the RBI FREE-AI report — refer to the
// authoritative RBI press release for the official text; not investment advice.
func MpcResearchSpec() afg.Spec {
	return afg.Spec{
		ID:   "mpc_research",
		Risk: "low",
		Handle: func(input string) (string, error) {
			// summary mirrors agents/mpc_research.Summary — one MPC meeting outcome.
			type summary struct {
				MeetingDate         string  `json:"meeting_date"` // YYYY-MM-DD
				RepoRate            float64 `json:"repo_rate_pct"`
				SDFRate             float64 `json:"sdf_rate_pct"`
				MSFRate             float64 `json:"msf_rate_pct"`
				CRRPct              float64 `json:"crr_pct"`
				SLRPct              float64 `json:"slr_pct"`
				Stance              string  `json:"stance"` // accommodative | neutral | withdrawal_of_accommodation
				VoteFor             int     `json:"vote_for"`
				VoteAgainst         int     `json:"vote_against"`
				CPIProjectionPctYoY float64 `json:"cpi_projection_pct"`
				GDPProjectionPctYoY float64 `json:"gdp_projection_pct"`
				ConsensusRepoBps    int     `json:"consensus_repo_change_bps"`
			}
			// request mirrors agents/mpc_research.Request.
			type request struct {
				Current  summary `json:"current"`
				Previous summary `json:"previous"`
			}
			// signal mirrors agents/mpc_research.Signal (wire field names preserved).
			type signal struct {
				MeetingDate      string   `json:"meeting_date"`
				RepoChangeBps    int      `json:"repo_change_bps"`
				StanceShift      string   `json:"stance_shift"`
				HawkishnessDelta string   `json:"hawkishness_delta"`
				SurpriseVsMkt    string   `json:"surprise"`
				CPIRevisionBps   int      `json:"cpi_revision_bps"`
				GDPRevisionBps   int      `json:"gdp_revision_bps"`
				DownstreamHints  []string `json:"downstream_hints"`
				Headline         string   `json:"headline"`
				Disclaimer       string   `json:"disclaimer"`
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}
			cur, prev := req.Current, req.Previous

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

			out := signal{
				MeetingDate:      cur.MeetingDate,
				RepoChangeBps:    repoBps,
				StanceShift:      stanceShift,
				HawkishnessDelta: hawk,
				SurpriseVsMkt:    surprise,
				CPIRevisionBps:   cpiBps,
				GDPRevisionBps:   gdpBps,
				DownstreamHints:  hints,
				Headline:         headline,
				Disclaimer: "Algorithmic summary of MPC statement; refer to the RBI press release for the " +
					"authoritative text. Not investment advice.",
			}
			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
