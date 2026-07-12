package catalog

import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// DeepResearchSpec ports agents/deep_research.
//
// The legacy agent is a multi-turn ReAct research worker for the Indian banking
// corpus (RBI circulars, Sahamati Account Aggregator specs, FATF and FIU-IND
// guidance, macro releases). Where macro_research is one-shot, deep_research
// iterates: search -> synthesise -> re-search -> cite, wrapping pkg/reasoning.ReAct
// with a fixed toolbelt of corpus resolvers and returning a cited Brief with a
// reasoning trace. Because its work is advisory, explanatory and LLM-backed (a
// reasoning loop over external corpora, not a table lookup or arithmetic), it is
// ported as an advisory/LLM Spec. Risk is medium per the legacy RiskLevel().
func DeepResearchSpec() afg.Spec {
	return afg.Spec{
		ID:   "deep_research",
		Risk: "medium",
		Instructions: "You are the Genie Deep Research agent, a precise multi-turn financial researcher " +
			"for the Indian banking and regulatory corpus — RBI circulars and master directions, the " +
			"Sahamati Account Aggregator (AA) specifications, FATF and FIU-IND guidance, and macro-economic " +
			"releases. Unlike a one-shot lookup, you iterate: search the relevant corpus, synthesise what you " +
			"find, re-search to fill gaps, and cite every claim. Given a research question (and an optional " +
			"filter over source corpora such as rbi, sahamati, or fiu_ind), gather facts before answering and " +
			"attach a citation — source title, and URL and quote where available — to each material claim; " +
			"never assert a regulatory fact without pinning it to its source. Produce a concise, well-structured " +
			"brief: a plain-language summary, the list of citations, and where useful the corpora you consulted. " +
			"Use Indian English and correct regulator terminology (RBI, SEBI, FIU-IND, STR, CTR, CCR, Account " +
			"Aggregator, FIP, FIU, Sahamati, KYC, PAN, IFSC), and Indian money conventions (rupees, lakh, crore) " +
			"where apt. Your research briefs are advisory and informational only, produced under the guidance of " +
			"the RBI FREE-AI report; they are not legal, tax, or regulatory advice. A human must verify every " +
			"citation and quotation against the original RBI, Sahamati, or FIU-IND publication before relying on it.",
	}
}
