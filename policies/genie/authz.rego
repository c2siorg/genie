# package genie.authz — top-level authorization decision for a Genie message.
#
# Input document shape (built by pkg/opa/engine.go:messageToInput):
#
#   {
#     "message": {
#       "id": "...", "from": "...", "to": "...",
#       "role": "...", "type": "...", "content": "...",
#       "metadata": {
#         "classification": "internal",
#         "user_roles":     ["user"],
#         "user_id":        "...",
#         "tenant_id":      "...",
#         "region":         "in"
#       }
#     }
#   }
#
# Data document (loaded by engine from PolicyConfig, available as data.config):
#
#   {
#     "config": {
#       "rbac":                       { "finance_question": ["user","advisor","admin"], ... },
#       "home_region":                "in",
#       "allow_cross_border_for_public": true,
#       "admin_bypass":               true,
#       "max_content_length":         262144,
#       "required_metadata":          { "finance_question": ["user_id","trace_id"], ... },
#       "agent_rings":                { "supervisor": 0, "analyzer": 1, ... }
#     }
#   }
#
# Returns:
#   {
#     "allow":        true | false,
#     "deny_reasons": ["rbac: ...", "data_residency: ...", ...]
#   }
#
# Other sub-packages (rbac.rego, data.rego, content.rego, rings.rego) each
# contribute entries to the shared `deny_reasons` partial set. OPA evaluates
# all contributions and merges them automatically.

package genie.authz

# ── Defaults ─────────────────────────────────────────────────────────────────

default allow := false

# ── Entry point ───────────────────────────────────────────────────────────────

# Allow when nothing denied it.
allow if count(deny_reasons) == 0
