// Package phishing_classifier scores a URL / SMS / email body for the
// likelihood of being a financial phishing attempt. Rule-stack first so
// every verdict is auditable, with an optional LLM layer for ambiguous
// text (the supervisor wires that on top — this package stays deterministic).
//
// Signals:
//   * URL — punycode, IP literal, brand-impersonation lookalike, long
//     subdomain chain, untrusted TLD, presence of @ or %
//   * Text — urgency tokens, request-credential tokens, prize / lottery
//     tokens, KYC-update tokens, OTP-share tokens
//   * UPI VPA — non-bank handle, lookalike merchant, contains "refund"
package phishing_classifier

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "phishing_classifier"
	Capability = "classify_phishing"
	TypeIn     = "phishing_check"
	TypeOut    = "phishing_verdict"
	NextAgent  = "financial_supervisor"
)

// Request is the wire payload. Any field empty is skipped.
type Request struct {
	URL  string `json:"url,omitempty"`
	Text string `json:"text,omitempty"`
	VPA  string `json:"upi_vpa,omitempty"`
}

// Verdict is the wire output.
type Verdict struct {
	Score       float64  `json:"score_0_1"`
	Label       string   `json:"label"` // "safe" | "suspicious" | "phishing"
	Reasons     []string `json:"reasons"`
	Disclaimer  string   `json:"disclaimer"`
}

// Trusted Indian banking domains (illustrative).
var trustedDomains = map[string]bool{
	"sbi.co.in": true, "hdfcbank.com": true, "icicibank.com": true,
	"axisbank.com": true, "kotak.com": true, "rbi.org.in": true,
	"npci.org.in": true, "uidai.gov.in": true,
}

var urgencyTokens = []string{
	"urgent", "immediately", "expires today", "account blocked",
	"verify now", "act now", "final notice", "last chance",
}
var credentialTokens = []string{
	"share otp", "share password", "share pin", "share cvv",
	"upi pin", "verify your mpin", "share aadhaar otp",
}
var lotteryTokens = []string{
	"you have won", "lottery", "lucky draw", "free reward", "claim prize",
}
var kycTokens = []string{
	"kyc update", "kyc expiring", "kyc deactivated", "re-kyc",
}

// Agent implements agent.Agent.
type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Phishing Classifier" }
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
	res := a.Classify(req)
	env.Logf("[phishing] score=%.2f label=%s", res.Score, res.Label)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Classify scores the request across URL, text and VPA signals.
func (a *Agent) Classify(req Request) Verdict {
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
	return Verdict{
		Score:      round2(score),
		Label:      label,
		Reasons:    reasons,
		Disclaimer: "Heuristic classifier. Always verify by calling the institution on the back-of-card number — never the number in the message.",
	}
}

func scoreURL(raw string) (float64, []string) {
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

	// 2. Punycode
	if strings.Contains(host, "xn--") {
		score += 0.3
		reasons = append(reasons, "URL contains punycode — likely homograph impersonation")
	}

	// 3. Untrusted look-alike: contains a trusted brand string but is not on a trusted domain
	for trusted := range trustedDomains {
		brand := strings.Split(trusted, ".")[0]
		if strings.Contains(host, brand) && !endsWithDomain(host, trusted) {
			score += 0.5
			reasons = append(reasons, "Domain mimics "+trusted+" without being it")
			break
		}
	}

	// 4. Long subdomain chain
	if strings.Count(host, ".") > 3 {
		score += 0.2
		reasons = append(reasons, "Unusually deep subdomain chain")
	}

	// 5. @ in URL (credential injection)
	if strings.Contains(raw, "@") {
		score += 0.3
		reasons = append(reasons, "URL contains an @ — credential confusion attack")
	}

	return score, reasons
}

func endsWithDomain(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func scoreText(text string) (float64, []string) {
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

func scoreVPA(vpa string) (float64, []string) {
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

func anyContains(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
