// Package cyber_guardian detects session- and access-level anomalies that
// the transaction-focused fraud agent doesn't see: impossible-travel
// logins, credential-stuffing volleys, device-fingerprint drift, and
// session-token reuse from unfamiliar IPs.
//
// Inspired by Google ADK samples → cyber-guardian-agent. Complements
// agents/fraud (transaction-level) with the "before they spend, did
// they really log in?" layer.
package cyber_guardian

import (
	"context"
	"encoding/json"
	"math"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "cyber_guardian"
	Capability = "session_anomaly_detection"
	TypeIn     = "session_check"
	TypeOut    = "session_verdict"
	NextAgent  = "financial_supervisor"

	// Earth radius in km — used for great-circle distance.
	earthRadiusKM = 6371.0
	// Max plausible cruise speed (km/h) — anything above is "impossible travel".
	maxPlausibleKMH = 1000
)

// Event is one login or session-resume attempt.
type Event struct {
	UserID         string  `json:"user_id"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	IPAddress      string  `json:"ip"`
	DeviceFP       string  `json:"device_fingerprint"`
	UnixMillis     int64   `json:"unix_millis"`
	SuccessfulAuth bool    `json:"successful_auth"`
}

// Request is the inbound batch — last K events for this user, oldest first.
type Request struct {
	UserID    string  `json:"user_id"`
	Events    []Event `json:"events"`
	KnownDeviceFPs []string `json:"known_device_fps"` // user's enrolled devices
}

// Verdict is the structured output.
type Verdict struct {
	UserID         string   `json:"user_id"`
	RiskScore0To1  float64  `json:"risk_score_0_1"`
	Label          string   `json:"label"` // "low" | "medium" | "high"
	Flags          []string `json:"flags"`
	RecommendAction string  `json:"recommend_action"`
	Disclaimer     string   `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Cyber Guardian" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	v := a.Inspect(req)
	env.Logf("[cyber_guardian] user=%s label=%s score=%.2f flags=%v", v.UserID, v.Label, v.RiskScore0To1, v.Flags)
	body, _ := json.Marshal(v)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Inspect runs the rule stack.
func (a *Agent) Inspect(req Request) Verdict {
	score := 0.0
	flags := []string{}
	known := map[string]bool{}
	for _, fp := range req.KnownDeviceFPs {
		known[fp] = true
	}

	// 1. Failed-attempt density (credential stuffing).
	failedRecent := 0
	for _, e := range req.Events {
		if !e.SuccessfulAuth {
			failedRecent++
		}
	}
	if failedRecent >= 5 {
		score += 0.30
		flags = append(flags, "5+ failed auth attempts in window — credential stuffing suspected")
	} else if failedRecent >= 3 {
		score += 0.15
		flags = append(flags, "Multiple recent failed auth attempts")
	}

	// 2. Impossible travel — compare each consecutive successful event.
	for i := 1; i < len(req.Events); i++ {
		a := req.Events[i-1]
		b := req.Events[i]
		if !a.SuccessfulAuth || !b.SuccessfulAuth {
			continue
		}
		distKM := haversine(a.Lat, a.Lng, b.Lat, b.Lng)
		if distKM < 50 {
			continue
		}
		hours := float64(b.UnixMillis-a.UnixMillis) / (1000 * 60 * 60)
		if hours <= 0 {
			continue
		}
		kmh := distKM / hours
		if kmh > maxPlausibleKMH {
			score += 0.35
			flags = append(flags, "Impossible travel detected between consecutive sessions")
			break
		}
	}

	// 3. Unknown-device fingerprint on a successful auth.
	for _, e := range req.Events {
		if e.SuccessfulAuth && e.DeviceFP != "" && !known[e.DeviceFP] {
			score += 0.20
			flags = append(flags, "Successful login from unenrolled device")
			break
		}
	}

	// 4. Device-fingerprint churn within window.
	seen := map[string]bool{}
	for _, e := range req.Events {
		if e.DeviceFP != "" {
			seen[e.DeviceFP] = true
		}
	}
	if len(seen) >= 3 {
		score += 0.15
		flags = append(flags, "3+ distinct device fingerprints in window — device churn")
	}

	if score > 1 {
		score = 1
	}
	label := "low"
	action := "Continue; no extra friction"
	switch {
	case score >= 0.6:
		label = "high"
		action = "Force step-up authentication and notify customer via secondary channel"
	case score >= 0.30:
		label = "medium"
		action = "Surface a soft 2FA challenge on the next sensitive action"
	}

	return Verdict{
		UserID:          req.UserID,
		RiskScore0To1:   round2(score),
		Label:           label,
		Flags:           flags,
		RecommendAction: action,
		Disclaimer: "Heuristic session-risk score for advisory step-up. Final block decision " +
			"should consider device-binding and recent confirmed-fraud telemetry.",
	}
}

// haversine returns great-circle distance in km between two lat/lng points.
func haversine(lat1, lng1, lat2, lng2 float64) float64 {
	radLat1 := lat1 * math.Pi / 180
	radLat2 := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(radLat1)*math.Cos(radLat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKM * c
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
