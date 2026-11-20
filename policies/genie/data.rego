# Data classification, residency, and tenant isolation rules.
# Contributes deny_reasons to genie.authz.
#
# Mirrors governance.ClassificationPolicy, governance.DataResidencyPolicy,
# and governance.TenantPolicy.

package genie.authz

# ── Helpers ───────────────────────────────────────────────────────────────────

classification_rank := {
	"public":   0,
	"internal": 1,
	"pii":      2,
	"secret":   3,
}

msg_classification := cls if {
	cls := input.message.metadata.classification
} else := "internal"

msg_region := region if {
	region := input.message.metadata.region
} else := ""

home_region := data.config.home_region

# A region is "safe" when it is the home region or on-prem.
safe_region(r) if r == home_region
safe_region(r) if r == "on-prem"
safe_region("") # empty region means "unspecified" — don't block

# ── Data residency ────────────────────────────────────────────────────────────

# PII and Secret must never leave the home region.
deny_reasons contains reason if {
	msg_classification in {"pii", "secret"}
	not safe_region(msg_region)
	reason := sprintf(
		"data_residency: classification %q must remain in home region %q (message region: %q)",
		[msg_classification, home_region, msg_region],
	)
}

# Internal must not cross borders.
deny_reasons contains reason if {
	msg_classification == "internal"
	msg_region != ""
	not safe_region(msg_region)
	reason := sprintf(
		"data_residency: internal data must not leave home region %q (message region: %q)",
		[home_region, msg_region],
	)
}

# ── Tenant isolation ─────────────────────────────────────────────────────────

tenant_id := tid if {
	tid := input.message.metadata.tenant_id
} else := ""

# Deny messages that carry no tenant_id (except system messages).
deny_reasons contains reason if {
	tenant_id == ""
	input.message.role != "system"
	reason := "tenant_isolation: tenant_id is required in message metadata"
}

# Confused-deputy check: expected_tenant (if set) must match tenant_id.
deny_reasons contains reason if {
	expected := input.message.metadata.expected_tenant
	expected != ""
	expected != tenant_id
	reason := sprintf(
		"tenant_isolation: expected_tenant %q does not match tenant_id %q",
		[expected, tenant_id],
	)
}
