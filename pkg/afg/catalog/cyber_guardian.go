package catalog

import (
	"encoding/json"
	"math"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// CyberGuardianSpec ports agents/cyber_guardian.
//
// It detects session- and access-level anomalies the transaction fraud agent
// doesn't see: impossible-travel logins, credential-stuffing volleys,
// unenrolled-device logins, and device-fingerprint churn. It runs a fixed rule
// stack over the user's last K login/session events (oldest first) and adds
// weighted risk: 5+ failed auths (+0.30) or 3+ failed auths (+0.15) for
// credential stuffing; impossible travel between consecutive successful sessions
// via great-circle (haversine) distance over elapsed time above 1000 km/h
// (+0.35); a successful login from a device fingerprint not in the enrolled set
// (+0.20); and 3+ distinct fingerprints in the window (+0.15). The score is
// clamped to [0,1] and labelled low / medium (>=0.30) / high (>=0.60), each with
// a recommended step-up action. Deterministic arithmetic; the score is a
// heuristic advisory signal for step-up authentication per RBI FREE-AI — the
// final block decision should still weigh device-binding and confirmed-fraud
// telemetry.
func CyberGuardianSpec() afg.Spec {
	return afg.Spec{
		ID:   "cyber_guardian",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			type event struct {
				UserID         string  `json:"user_id"`
				Lat            float64 `json:"lat"`
				Lng            float64 `json:"lng"`
				IPAddress      string  `json:"ip"`
				DeviceFP       string  `json:"device_fingerprint"`
				UnixMillis     int64   `json:"unix_millis"`
				SuccessfulAuth bool    `json:"successful_auth"`
			}
			type request struct {
				UserID         string   `json:"user_id"`
				Events         []event  `json:"events"`
				KnownDeviceFPs []string `json:"known_device_fps"`
			}
			type verdict struct {
				UserID          string   `json:"user_id"`
				RiskScore0To1   float64  `json:"risk_score_0_1"`
				Label           string   `json:"label"`
				Flags           []string `json:"flags"`
				RecommendAction string   `json:"recommend_action"`
				Disclaimer      string   `json:"disclaimer"`
			}

			const (
				earthRadiusKM   = 6371.0
				maxPlausibleKMH = 1000.0
			)

			// haversine returns great-circle distance in km between two lat/lng points.
			haversine := func(lat1, lng1, lat2, lng2 float64) float64 {
				radLat1 := lat1 * math.Pi / 180
				radLat2 := lat2 * math.Pi / 180
				dLat := (lat2 - lat1) * math.Pi / 180
				dLng := (lng2 - lng1) * math.Pi / 180
				a := math.Sin(dLat/2)*math.Sin(dLat/2) +
					math.Cos(radLat1)*math.Cos(radLat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
				c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
				return earthRadiusKM * c
			}
			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

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

			out := verdict{
				UserID:          req.UserID,
				RiskScore0To1:   round2(score),
				Label:           label,
				Flags:           flags,
				RecommendAction: action,
				Disclaimer: "Heuristic session-risk score for advisory step-up. Final block decision " +
					"should consider device-binding and recent confirmed-fraud telemetry.",
			}

			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
