# agents/auto_insurance

> **Risk class:** Medium · **Capability:** `service_motor_policy` · **In:** `motor_request` · **Out:** `motor_response`
> **Inspired by:** Google ADK `auto-insurance-agent`, tuned for IRDAI motor products.

---

## Overview

Handles motor-insurance touchpoints inside a banking app. Bancassurance
is a major fee-income line for Indian banks; customers expect **FNOL**
(First Notice of Loss), **roadside assistance**, and **renewal quote**
flows to live alongside their account.

The agent is the rules-and-routing layer:

- **FNOL** — compute total-loss flag (≥75 % of IDV) and route to cashless garages.
- **Roadside** — dispatch to a partner; pick the nearest network garage.
- **Renewal quote** — apply NCB-ladder mechanics and indicative-premium math.

---

## Constants

```go
const (
    ID         = "auto_insurance"
    Capability = "service_motor_policy"
    TypeIn     = "motor_request"
    TypeOut    = "motor_response"
    NextAgent  = "financial_supervisor"

    totalLossThresholdPct = 0.75 // repair > 75% IDV → total loss
)
```

---

## Constructor

```go
func New(garages map[string][]string) *Agent
```

`garages` is a city→[network garages] map injected by the host (in
production, a live API call to the insurer's PPN). Passing `nil` is
acceptable for tests.

---

## Types

### Request

```go
type Request struct {
    Kind                 string  // "fnol" | "roadside" | "renewal_quote"
    PolicyNumber         string
    VehicleRegNumber     string
    IDVRupees            float64 // Insured Declared Value
    EstRepairCostRupees  float64
    IncidentType         string  // accident | theft | flood | fire | third-party
    LocationLat          float64
    LocationLng          float64
    HoursToExpiry        int     // for renewal
    NCBPct               float64 // 0..50 in steps
    ClaimedThisYear      bool
    ZeroDepAddOn         bool
}
```

### Response

```go
type Response struct {
    Kind            string
    Action          string
    NetworkGarages  []string
    TotalLoss       bool
    SettlementHint  float64
    NewNCBPct       float64
    RenewalPremium  float64
    NextSteps       []string
    Disclaimer      string
}
```

---

## Business rules

### FNOL

- Compute `TotalLoss = (IDVRupees > 0 && EstRepairCostRupees ≥ IDVRupees × 0.75)`.
- If total loss: settlement = IDV; next steps include "total-loss process initiated".
- Always: include nearest-city network garages.

### Roadside

- Dispatch partner; include towing-up-to-50km hint and the network garage list.

### Renewal quote

- **NCB ratchet**:
  - Any claim this year → NCB resets to 0.
  - Clean year → NCB walks the ladder: 0 → 20 → 25 → 35 → 45 → 50 → 50.
- **Indicative premium**:
  - base = IDV × 3 %
  - 70 % of base is the Own-Damage (OD) component; NCB applies to OD only.
  - 30 % is the Third-Party (TP) component; NCB does not apply.
  - `premium = (OD × (1 − NCB)) + TP`
  - If `ZeroDepAddOn = true`: premium × 1.10.
- **Urgency hint**: if `HoursToExpiry < 72`, surface "Driving uninsured is a Motor Vehicles Act offence."

---

## Geocoding stub

`cityFromLatLng(lat, lng)` is a placeholder with hard-coded boxes for
Delhi, Mumbai, Bengaluru. Production wires a reverse-geocoder
(MapMyIndia, Google Maps API, OpenStreetMap Nominatim).

---

## Example

### Request (FNOL)

```json
{
  "kind": "fnol",
  "policy_number": "MOTOR-12345",
  "idv_rupees": 500000,
  "est_repair_cost_rupees": 400000,
  "lat": 19.05,
  "lng": 72.85
}
```

### Response

```json
{
  "kind": "fnol",
  "action": "register_claim_total_loss",
  "network_garages": ["Garage A", "Garage B"],
  "total_loss_flag": true,
  "settlement_hint_rupees": 500000,
  "next_steps": [
    "Claim registered with insurer; reference number issued via SMS.",
    "Upload photos of damage and FIR (if applicable) within 48 hours.",
    "Estimated repair ≥ 75 % of IDV — total-loss process initiated; settlement at IDV."
  ],
  "disclaimer": "Indicative service action per IRDAI motor product. Final settlement and roadside dispatch subject to policy terms, insurer confirmation, and partner availability."
}
```

---

## FREE-AI alignment

- **Rec 18 (Disclosure)** — Disclaimer cites IRDAI dependency.
- **Rec 25 (Disclosures)** — total-loss math is in the open (in this doc + the code).

---

## Integration

### Triggered by

- A "report claim" button in the bank's app.
- A "renew now" flow before expiry.

### Hands off to

- `financial_supervisor` for downstream notifications.
- Insurer integration (host concern) for settlement.
- Roadside partner dispatch API (host concern).

---

## Anti-patterns

1. **Skipping the total-loss flag.** Once repair ≥ 75 % of IDV, the claim is processed differently. Mis-routing it costs the customer.
2. **Applying NCB to the TP component.** TP is statutory; NCB only flows on OD.
3. **Pricing without `ZeroDepAddOn` opt-in.** Zero-dep is a 10 % premium addition; quoting without it under-quotes.
4. **Trusting the stub geocoder in production.** Wire a real one.

---

## Testing

`agents/auto_insurance/auto_insurance_test.go` covers:

- FNOL partial-loss vs total-loss
- Network garage routing by city
- Roadside dispatch action
- Renewal NCB clean-year bump
- Renewal NCB claim reset
- Urgency hint near expiry
- Unknown kind handling
- HandleMessage dispatch
- Disclaimer presence

Run:

```bash
go test ./agents/auto_insurance/ -v
```

---

## References

- [IRDAI Motor Insurance Regulations](https://www.irdai.gov.in/) — IDV, NCB, OD/TP split
- [Motor Vehicles Act 1988](https://parivahan.gov.in/) — the legal requirement for active insurance
- [IRDAI Preferred Provider Network circular](https://www.irdai.gov.in/) — for cashless garages
