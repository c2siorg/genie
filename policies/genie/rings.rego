# Agent ring (privilege) enforcement.
# Contributes deny_reasons to genie.authz.
#
# Ring model (mirrors agentgov/setup.go):
#   Ring 0 (admin)      — supervisor, auditor                 — all capabilities
#   Ring 1 (standard)   — analyzer, recommender, …             — message+tool+llm+doc+bus
#   Ring 2 (restricted) — ingestor, normalizer, …              — message+tool+bus
#   Ring 3 (sandboxed)  — aa_fetcher, voice, …                 — message.handle only
#
# data.config.agent_rings maps agentID -> ring number (0-3).
# If an agent has no entry it is treated as ring 3 (most restrictive).
#
# For message-level checks, the "capability" checked is the message type
# concatenated with a ring-capability prefix. This is intentionally looser
# than the AGT runtime check — the OPA policy adds a second, auditable layer.

package genie.authz

# ── Ring helpers ──────────────────────────────────────────────────────────────

agent_ring(agent_id) := ring if {
	ring := data.config.agent_rings[agent_id]
} else := 3 # default: sandboxed

ring_1_capabilities := {
	"message.handle", "tool.call", "llm.complete",
	"document.read", "bus.publish",
}

ring_2_capabilities := {"message.handle", "tool.call", "bus.publish"}

ring_3_capabilities := {"message.handle"}

# capability_permitted returns true if the given ring is allowed the capability.
capability_permitted(ring, _) if ring == 0 # ring 0 is unrestricted

capability_permitted(1, cap) if cap in ring_1_capabilities

capability_permitted(2, cap) if cap in ring_2_capabilities

capability_permitted(3, cap) if cap in ring_3_capabilities

# ── Ring enforcement ─────────────────────────────────────────────────────────

# Map message types to the minimum ring capability required.
# Uses else-chain so only one value is produced for each input.

required_capability(msg_type) := "llm.complete" if {
	llm_types := {"finance_question", "portfolio_request", "recommendations", "analysis"}
	msg_type in llm_types
} else := "tool.call" if {
	tool_types := {"external_fetch", "db_query", "document_ingest"}
	msg_type in tool_types
} else := "message.handle"

deny_reasons contains reason if {
	ring := agent_ring(input.message.to)
	cap := required_capability(input.message.type)
	not capability_permitted(ring, cap)
	reason := sprintf(
		"ring_enforcement: agent %q (ring %d) cannot handle type %q (needs %q)",
		[input.message.to, ring, input.message.type, cap],
	)
}
