package catalog

import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// EducatorSpec ports agents/educator (the Financial Educator, registered as
// "financial_educator").
//
// The legacy agent answers "explain finance concept X" style requests. It is
// modelled on the ADK Financial Advisor sample: backed by a static glossary of
// personal-finance terms (SIP, EMI, compound interest, emergency fund, asset
// allocation, PPF, NPS, index fund) with an explicit note to "swap with an LLM
// call in production", and it optionally attaches RAG source citations when a
// rag.Index is configured. Because the work is advisory, explanatory and
// intended to be LLM-backed (not a table lookup, arithmetic, or threshold
// classification), it is ported as an advisory/LLM Spec. No RiskLevel() is
// declared on the legacy agent and its work is purely informational education,
// so risk is low.
func EducatorSpec() afg.Spec {
	return afg.Spec{
		ID:   "financial_educator",
		Risk: "low",
		Instructions: "You are the Genie Financial Educator, a plain-language teacher of personal-finance " +
			"and investing concepts for an Indian audience. Given a finance concept or question (for example " +
			"SIP, EMI, compound interest, emergency fund, asset allocation, PPF, NPS, or index fund), explain " +
			"it clearly and concisely in Indian English, using correct regulator and market terminology (RBI, " +
			"SEBI, mutual fund, Nifty 50, PPF, NPS) and Indian money conventions (rupees, lakh, crore) where " +
			"apt. Prefer accessible, jargon-light definitions; where a term is unfamiliar, say so plainly and " +
			"invite the user to ask about a related concept rather than inventing details. When grounding " +
			"knowledge is available, cite your sources so the reader can verify each claim, and never present " +
			"an unsourced fact as authoritative. Your explanations are educational and informational only, " +
			"produced in line with the RBI FREE-AI report; they are not personalised investment, tax, or " +
			"financial advice, and the user should consult a qualified adviser before acting on any concept.",
	}
}
