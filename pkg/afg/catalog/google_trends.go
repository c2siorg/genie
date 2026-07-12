package catalog

import (
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// GoogleTrendsSpec ports agents/google_trends.
//
// It surfaces consumer-search-interest signals for a set of keyword series
// (each a Google Trends 0..100 normalised score history, oldest first). For
// each keyword it splits the history into a "latest" window (window_points, or
// 4 by default) and a baseline, computes the mean of each, and classifies the
// change multiple: >= 1.5x baseline = "surging", <= 0.6x = "fading", else
// "steady". Signals are sorted by change multiple (descending), and downstream
// hints are emitted for macro_research / mf_screener on surge/fade. Deterministic
// window-mean arithmetic + threshold classification; outputs are advisory /
// informational per RBI FREE-AI and are not investment advice.
func GoogleTrendsSpec() afg.Spec {
	return afg.Spec{
		ID:   "google_trends",
		Risk: "low",
		Handle: func(input string) (string, error) {
			type series struct {
				Keyword string `json:"keyword"`
				Geo     string `json:"geo"`
				Points  []int  `json:"points"`
			}
			type request struct {
				Geo      string   `json:"geo"`
				WindowN  int      `json:"window_points"`
				Keywords []string `json:"keywords"`
				Series   []series `json:"series"`
			}
			type signal struct {
				Keyword          string  `json:"keyword"`
				Direction        string  `json:"direction"`
				LatestMean       float64 `json:"latest_mean"`
				BaselineMean     float64 `json:"baseline_mean"`
				ChangeMultiple   float64 `json:"change_multiple"`
				NoteToDownstream string  `json:"note_to_downstream"`
			}
			type response struct {
				Geo        string   `json:"geo"`
				Signals    []signal `json:"signals"`
				Hints      []string `json:"downstream_hints"`
				Disclaimer string   `json:"disclaimer"`
			}

			const (
				surgeMultiple = 1.5
				fadeMultiple  = 0.6
			)

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			meanInt := func(xs []int) float64 {
				if len(xs) == 0 {
					return 0
				}
				sum := 0
				for _, x := range xs {
					sum += x
				}
				return float64(sum) / float64(len(xs))
			}

			classify := func(s series, windowN int) signal {
				if windowN <= 0 || len(s.Points) <= windowN {
					return signal{Keyword: s.Keyword, Direction: "steady"}
				}
				split := len(s.Points) - windowN
				baseline := s.Points[:split]
				latest := s.Points[split:]
				baseMean := meanInt(baseline)
				latestMean := meanInt(latest)
				mult := 0.0
				if baseMean > 0 {
					mult = latestMean / baseMean
				}
				dir := "steady"
				switch {
				case mult >= surgeMultiple:
					dir = "surging"
				case mult > 0 && mult <= fadeMultiple:
					dir = "fading"
				}
				note := ""
				switch dir {
				case "surging":
					note = "Search interest in this term has surged versus baseline; investigate macro driver"
				case "fading":
					note = "Search interest has cooled materially versus baseline"
				}
				return signal{
					Keyword:          s.Keyword,
					Direction:        dir,
					LatestMean:       round2(latestMean),
					BaselineMean:     round2(baseMean),
					ChangeMultiple:   round2(mult),
					NoteToDownstream: note,
				}
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			window := req.WindowN
			if window <= 0 {
				window = 4
			}

			signals := []signal{}
			for _, s := range req.Series {
				signals = append(signals, classify(s, window))
			}
			sort.Slice(signals, func(i, j int) bool { return signals[i].ChangeMultiple > signals[j].ChangeMultiple })

			hints := []string{}
			for _, s := range signals {
				switch s.Direction {
				case "surging":
					hints = append(hints, "macro_research: flag surge — "+s.Keyword)
					hints = append(hints, "mf_screener: rescore theme — "+s.Keyword)
				case "fading":
					hints = append(hints, "macro_research: flag fade — "+s.Keyword)
				}
			}

			out := response{
				Geo:     req.Geo,
				Signals: signals,
				Hints:   hints,
				Disclaimer: "Google-Trends-derived interest signal. Not investment advice; correlation " +
					"with actual market moves is variable.",
			}

			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
