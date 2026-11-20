# RBAC rules — contributes deny_reasons to genie.authz.
#
# Config consumed from data.config.rbac:
#   { "finance_question": ["user","advisor","admin"], ... }
#
# Admin bypass: if data.config.admin_bypass == true AND user has "admin" role,
# RBAC checks are skipped entirely (same behaviour as governance.RBACPolicy).

package genie.authz

# ── Helpers ───────────────────────────────────────────────────────────────────

# Extract user roles safely; handles both []string (Go JSON arrays) and missing keys.
user_roles := roles if {
	roles := input.message.metadata.user_roles
} else := []

# Admin bypass: if admin_bypass is enabled and the user carries the admin role,
# skip all RBAC checks.
admin_bypass if {
	data.config.admin_bypass == true
	"admin" in user_roles
}

# any_role_matches is true if the user carries at least one of the required roles.
any_role_matches(required) if {
	role := user_roles[_]
	role in required
}

# ── RBAC denial ───────────────────────────────────────────────────────────────

# Deny when the message type has a required-role table entry, none of the
# user's roles match, and admin bypass is not active.
deny_reasons contains reason if {
	required := data.config.rbac[input.message.type]
	not admin_bypass
	not any_role_matches(required)
	reason := sprintf(
		"rbac: type %q requires one of %v; user has %v",
		[input.message.type, required, user_roles],
	)
}
