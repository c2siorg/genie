# agents/cyber_guardian

> **Risk class:** Medium · **Capability:** `session_anomaly_detection` · **In:** `session_check` · **Out:** `session_verdict`
> **Inspired by:** Google ADK `cyber-guardian-agent`, complements `agents/fraud`.

---

## Overview

Session- and access-level anomaly detection. Complements `agents/fraud`
(transaction-level) with the "before they spend, did they really log
in?" layer:

- Impossible travel between consecutive logins (Haversine distance / time)
- Credential-stuffing patterns (failed-attempt density)
- Unknown device fingerprints on successful auths
- Device-fingerprint churn within a window

The agent's verdict feeds the step-up authentication system: low → no
friction, medium → soft 2FA on the next sensitive action, high → force
step-up + customer notification on a secondary channel.

---

## Constants

```go
const (
    ID         = "cyber_guardian"
    Capability = "session_anomaly_detection"
    TypeIn     = "session_check"
    TypeOut    = "session_verdict"
    NextAgent  = "financial_supervisor"

    earthRadiusKM   = 6371.0 // Haversine constant
    maxPlausibleKMH = 1000   // anything faster → impossible travel
)
```

---

## Types

### Event (one login / session resume)

```go
type Event struct {
    UserID         string
    Lat            float64
    Lng            float64
    IPAddress      string
    DeviceFP       string  // device fingerprint
    UnixMillis     int64
    SuccessfulAuth bool
}
```

### Request

```go
type Request struct {
    UserID         string
    Events         []Event   // oldest first
    KnownDeviceFPs []string  // user's enrolled devices
}
```

### Verdict (outbound)

```go
type Verdict struct {
    UserID          string
    RiskScore0To1   float64
    Label           string   // "low" | "medium" | "high"
    Flags           []string
    RecommendAction string
    Disclaimer      string
}
```

---

## Detection rules

| Rule | Score | Trigger |
|---|---:|---|
| 5+ failed auth attempts in window | +0.30 | Credential stuffing suspected |
| 3–4 failed auth attempts | +0.15 | Worth a soft challenge |
| Impossible travel between two successful sessions (km/h > 1000) | +0.35 | Account takeover indicator |
| Successful auth from unenrolled device | +0.20 | New device — needs binding |
| ≥3 distinct device fingerprints in window | +0.15 | Churn signal |

### Final labelling

- `score ≥ 0.60` → `high` → force step-up + secondary-channel notify
- `score ≥ 0.30` → `medium` → soft 2FA on next sensitive action
- else → `low` → no friction

The thresholds are deliberately set so a single weak signal lands as
`medium` rather than `low`. Banking security errs cautious.

---

## Impossible travel — Haversine math

For two consecutive successful events at `(lat1, lng1, t1)` and
`(lat2, lng2, t2)`:

```
distKM = haversine(lat1, lng1, lat2, lng2)
hours  = (t2 - t1) / 3_600_000  # millis to hours
kmh    = distKM / hours
if kmh > 1000 → impossible travel
```

1000 km/h is the cruise speed of a commercial jet — anything faster is
physically impossible for a human. The agent skips pairs within 50 km
(same metro) and pairs where `hours <= 0` (clock skew).

---

## Example

### Request

```json
{
  "user_id": "u-1",
  "known_device_fps": ["fp-pixel"],
  "events": [
    {"successful_auth": true, "lat": 19.05, "lng": 72.85, "device_fingerprint": "fp-pixel", "unix_millis": 1700000000000},
    {"successful_auth": true, "lat": 40.71, "lng": -74.00, "device_fingerprint": "fp-pixel", "unix_millis": 1700003600000}
  ]
}
```

Mumbai → NYC in 1 hour. Verdict:

```json
{
  "user_id": "u-1",
  "risk_score_0_1": 0.35,
  "label": "medium",
  "flags": ["Impossible travel detected between consecutive sessions"],
  "recommend_action": "Surface a soft 2FA challenge on the next sensitive action",
  "disclaimer": "Heuristic session-risk score for advisory step-up. Final block decision should consider device-binding and recent confirmed-fraud telemetry."
}
```

---

## FREE-AI alignment

- **Rec 19 (Cybersecurity)** — this agent operationalises session-level cyber posture.
- **Rec 18 (Disclosure)** — Disclaimer notes the heuristic nature and the inputs the final block decision should consider.

---

## Integration

### Triggered by

- The auth gateway after every login event → packs the last K events for that user → publishes `session_check`.
- A scheduled "morning anomaly sweep" job that re-scores all active sessions.

### Hands off to

- The auth gateway — to act on `recommend_action` (raise step-up, force re-login, notify customer).
- `pkg/incidents` — high-label verdicts could auto-record (not done by default; usually paired with confirmed-fraud signal first).

---

## Anti-patterns

1. **Acting on a `medium` verdict like it's `high`.** Medium is a soft challenge, not a hard block. Falsely blocking the customer who's just travelling is costly.
2. **Ignoring `KnownDeviceFPs`.** The unknown-device signal is useless without it.
3. **Skipping the 50km same-metro skip.** A user moving between home and office shouldn't trigger impossible travel.
4. **Using IP geolocation without fallback.** IP geo is famously unreliable (VPNs, mobile carrier NAT). Use device GPS where consented; fall back to IP geo only.

---

## Testing

`agents/cyber_guardian/cyber_guardian_test.go` covers:

- Clean profile → low
- Impossible travel (Mumbai→NYC in 1h) → at least medium
- Credential stuffing (6 failures) → medium or high
- Unknown device flag
- Device churn flag
- HandleMessage dispatch
- Disclaimer presence

Run:

```bash
go test ./agents/cyber_guardian/ -v
```

---

## References

- [Haversine formula](https://en.wikipedia.org/wiki/Haversine_formula) — great-circle distance math
- [RBI Master Direction — Digital Payments Security](https://rbi.org.in/) — for step-up authentication norms
- [FIDO Alliance — device binding](https://fidoalliance.org/) — for known-device enrolment patterns
