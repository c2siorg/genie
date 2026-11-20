package catalog

import (
	"encoding/json"
	"net"
	"net/url"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// PhishingClassifierSpec ports agents/phishing_classifier. It scores a URL /
// SMS / email body / UPI VPA for the likelihood of being a financial phishing
// attempt using a deterministic, auditable rule stack (no LLM, no network).
func PhishingClassifierSpec() afg.Spec {
	return afg.Spec{
		ID:   "phishing_classifier",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			// Wire payload — any field empty is skipped.
			type request struct {
				URL  string `json:"url,omitempty"`
				Text string `json:"text,omitempty"`
				VPA  string `json:"upi_vpa,omitempty"`
			}
			// Wire output.
			type verdict struct {
				Score      float64  `json:"score_0_1"`
				Label      string   `json:"label"` // "safe" | "suspicious" | "phishing"
				Reasons    []string `json:"reasons"`
				Disclaimer string   `json:"disclaimer"`
			}

			// Trusted Indian banking domains (illustrative).
			trustedDomains := map[string]bool{
				"sbi.co.in": true, "hdfcbank.com": true, "icicibank.com": true,
				"axisbank.com": true, "kotak.com": true, "rbi.org.in": true,
				"npci.org.in": true, "uidai.gov.in": true,
			}
			urgencyTokens := []string{
				"urgent", "immediately", "expires today", "account blocked",
				"verify now", "act now", "final notice", "last chance",
			}
			credentialTokens := []string{
				"share otp", "share password", "share pin", "share cvv",
				"upi pin", "verify your mpin", "share aadhaar otp",
			}
			lotteryTokens := []string{
				"you have won", "lottery", "lucky draw", "free reward", "claim prize",
			}
			kycTokens := []string{
				"kyc update", "kyc expiring", "kyc deactivated", "re-kyc",
			}

			anyContains := func(s string, needles []string) bool {
				for _, n := range needles {
					if strings.Contains(s, n) {
						return true
					}
				}
				return false
			}
			endsWithDomain := func(host, domain string) bool {
				return host == domain || strings.HasSuffix(host, "."+domain)
			}
			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			scoreURL := func(raw string) (float64, []string) {
				reasons := []string{}
				score := 0.0
				u, err := url.Parse(strings.TrimSpace(raw))
				if err != nil || u.Host == "" {
					return 0.0, nil
				}
				host := strings.ToLower(u.Hostname())

				// 1. IP literal in URL — practically diagnostic of phishing in retail finance.
				if ip := net.ParseIP(host); ip != nil {
					score += 0.7
					reasons = append(reasons, "URL uses a raw IP address instead of a domain")
				}
				// 2. Punycode.
				if strings.Contains(host, "xn--") {
					score += 0.3
					reasons = append(reasons, "URL contains punycode — likely homograph impersonation")
				}
				// 3. Untrusted look-alike: contains a trusted brand string but is not on a trusted domain.
				for trusted := range trustedDomains {
					brand := strings.Split(trusted, ".")[0]
					if strings.Contains(host, brand) && !endsWithDomain(host, trusted) {
						score += 0.5
						reasons = append(reasons, "Domain mimics "+trusted+" without being it")
						break
					}
				}
				// 4. Long subdomain chain.
				if strings.Count(host, ".") > 3 {
					score += 0.2
					reasons = append(reasons, "Unusually deep subdomain chain")
				}
				// 5. @ in URL (credential injection).
				if strings.Contains(raw, "@") {
					score += 0.3
					reasons = append(reasons, "URL contains an @ — credential confusion attack")
				}
				return score, reasons
			}

			scoreText := func(text string) (float64, []string) {
				lower := strings.ToLower(text)
				score := 0.0
				reasons := []string{}
				if anyContains(lower, credentialTokens) {
					score += 0.5
					reasons = append(reasons, "Asks for OTP / PIN / password — banks never ask for these")
				}
				if anyContains(lower, urgencyTokens) {
					score += 0.2
					reasons = append(reasons, "Urgency tactic detected")
				}
				if anyContains(lower, lotteryTokens) {
					score += 0.4
					reasons = append(reasons, "Lottery / prize claim")
				}
				if anyContains(lower, kycTokens) {
					score += 0.3
					reasons = append(reasons, "Fake KYC-update lure")
				}
				return score, reasons
			}

			scoreVPA := func(vpa string) (float64, []string) {
				score := 0.0
				reasons := []string{}
				v := strings.ToLower(vpa)
				if !strings.Contains(v, "@") {
					return score, reasons
				}
				parts := strings.SplitN(v, "@", 2)
				handle := parts[1]
				// Refund-themed handle is a classic UPI scam.
				if strings.Contains(parts[0], "refund") {
					score += 0.4
					reasons = append(reasons, "VPA local part says 'refund' — common UPI scam pattern")
				}
				// Non-banking handles for transfers (random gmail-style domains).
				if !strings.Contains(handle, "ok") && !strings.Contains(handle, "paytm") && !strings.Contains(handle, "ybl") && !strings.Contains(handle, "axl") && !strings.Contains(handle, "ibl") {
					score += 0.2
					reasons = append(reasons, "VPA handle not on a common Indian PSP — verify before paying")
				}
				return score, reasons
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			reasons := []string{}
			score := 0.0
			if req.URL != "" {
				s, r := scoreURL(req.URL)
				score += s
				reasons = append(reasons, r...)
			}
			if req.Text != "" {
				s, r := scoreText(req.Text)
				score += s
				reasons = append(reasons, r...)
			}
			if req.VPA != "" {
				s, r := scoreVPA(req.VPA)
				score += s
				reasons = append(reasons, r...)
			}
			if score > 1 {
				score = 1
			}
			label := "safe"
			switch {
			case score >= 0.7:
				label = "phishing"
			case score >= 0.4:
				label = "suspicious"
			}

			out := verdict{
				Score:   round2(score),
				Label:   label,
				Reasons: reasons,
				Disclaimer: "Heuristic classifier. Always verify by calling the institution on the " +
					"back-of-card number — never the number in the message.",
			}
			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
