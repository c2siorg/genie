# HTTP-level authorization for the Genie REST API.
# Used by pkg/opa/middleware.go — separate from message-level authz.
#
# Input document:
#   {
#     "method":  "GET",
#     "path":    ["v1", "governance", "audit"],
#     "headers": {"authorization": "Bearer <token>"},
#     "user":    {"id": "...", "roles": ["admin"], "tenant_id": "..."}
#   }
#
# Governance endpoints require the "admin" role.
# All other authenticated endpoints require the "user" role or higher.
# Health/readyz/disclosures are public.

package genie.http_authz

default allow := false

# Public endpoints — no auth required.
allow if {
	input.path[0] in {"healthz", "readyz"}
}

allow if {
	input.path[0] == "v1"
	input.path[1] == "disclosures"
	input.method == "GET"
}

# Governance endpoints — admin only.
allow if {
	input.path[0] == "v1"
	input.path[1] == "governance"
	"admin" in input.user.roles
}

# Any authenticated user may reach non-governance v1 endpoints.
allow if {
	input.path[0] == "v1"
	input.path[1] != "governance"
	count(input.user.roles) > 0
}

# Reason exposed for audit / 403 responses.
deny_reason := msg if {
	not allow
	input.path[0] == "v1"
	input.path[1] == "governance"
	msg := "governance endpoints require the admin role"
} else := "unauthenticated or insufficient permissions"
