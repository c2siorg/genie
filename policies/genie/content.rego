# Content-safety rules: length limits, required metadata, PII patterns,
# prompt-injection patterns, explainability enforcement.
# Contributes deny_reasons to genie.authz.
#
# Mirrors governance.MaxContentLengthPolicy, governance.RequiredMetadataPolicy,
# governance.PIIBlockPolicy, governance.PromptInjectionPolicy, and
# governance.ExplainabilityPolicy.

package genie.authz

# ── Content length ────────────────────────────────────────────────────────────

deny_reasons contains reason if {
	max_len := data.config.max_content_length
	max_len > 0
	count(input.message.content) > max_len
	reason := sprintf(
		"content_length: message length %d exceeds max %d bytes",
		[count(input.message.content), max_len],
	)
}

# ── Required metadata keys ────────────────────────────────────────────────────

# Each entry in data.config.required_metadata maps a message type to a list
# of metadata keys that MUST be present and non-empty.
deny_reasons contains reason if {
	required_keys := data.config.required_metadata[input.message.type]
	key := required_keys[_]
	val := object.get(input.message.metadata, key, "")
	val == ""
	reason := sprintf(
		"required_metadata: key %q must be present for message type %q",
		[key, input.message.type],
	)
}

# ── PII patterns ──────────────────────────────────────────────────────────────
# Blocked when data.config.block_pii == true AND the content matches a known
# PII pattern AND the escape hatch (pii_acknowledged=true) is not set.

pii_acknowledged if {
	input.message.metadata.pii_acknowledged == "true"
}

pii_acknowledged if {
	input.message.metadata.pii_acknowledged == true
}

# 12+ consecutive digits (Aadhaar / PAN-adjacent patterns).
deny_reasons contains reason if {
	data.config.block_pii == true
	not pii_acknowledged
	regex.match(`\b\d{12,}\b`, input.message.content)
	reason := "pii_block: message content appears to contain a numeric PII pattern (12+ consecutive digits)"
}

# Email address pattern.
deny_reasons contains reason if {
	data.config.block_pii == true
	not pii_acknowledged
	regex.match(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`, input.message.content)
	reason := "pii_block: message content appears to contain an email address"
}

# ── Prompt injection ─────────────────────────────────────────────────────────

injection_phrases := [
	"ignore previous instructions",
	"ignore all previous",
	"disregard prior",
	"reveal system prompt",
	"reveal your instructions",
	"you are now",
	"act as if",
	"pretend you are",
	"jailbreak",
	"[system]",
	"<|im_start|>",
	"<|system|>",
]

deny_reasons contains reason if {
	data.config.block_prompt_injection == true
	phrase := injection_phrases[_]
	lower_content := lower(input.message.content)
	contains(lower_content, phrase)
	reason := sprintf("prompt_injection: content contains injection phrase %q", [phrase])
}

# ── Explainability (RBI FREE-AI Rec 22) ───────────────────────────────────────
# Messages for types in data.config.explainability_applies_to must carry a
# non-empty "rationale" field inside their JSON content.

deny_reasons contains reason if {
	applies_to := data.config.explainability_applies_to
	input.message.type in applies_to
	payload := json.unmarshal(input.message.content)
	not payload.rationale
	reason := sprintf(
		"explainability: message type %q requires a 'rationale' field in its JSON content (RBI FREE-AI Rec 22)",
		[input.message.type],
	)
}
