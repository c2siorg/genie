// Package opa integrates Open Policy Agent into Genie's governance stack.
//
// # Architecture
//
// The OPA engine implements governance.Policy so it slots anywhere the
// existing struct-based policies do — including inside the CompositePolicy
// that is built by pkg/policy.BuildComposite.
//
//	composite := policy.BuildCompositeWithOPA(ledger, opaEngine)
//	orchestrator.WithPolicy(composite)
//
// # Rego policy structure
//
// Rego files live under policies/genie/. Multiple files share the
// package genie.authz and each contributes entries to the shared
// deny_reasons partial set:
//
//	policies/genie/
//	  authz.rego    — entry point: allow := count(deny_reasons)==0
//	  rbac.rego     — role-based access control
//	  data.rego     — data classification + residency + tenant isolation
//	  content.rego  — length limits, PII, prompt injection, explainability
//	  rings.rego    — agent ring/capability enforcement
//	  http_authz.rego — HTTP request-level authz (used by Middleware)
//
// # Query contract
//
// The engine evaluates "data.genie.authz" and expects the result document to
// contain:
//
//	{
//	    "allow":        true | false,
//	    "deny_reasons": ["...", ...]   // empty set → allow
//	}
//
// # Input document
//
// Built by messageToInput (engine.go) from a protocol.Message:
//
//	{
//	    "message": {
//	        "id":       "...",
//	        "from":     "...",
//	        "to":       "...",
//	        "role":     "...",
//	        "type":     "...",
//	        "content":  "...",
//	        "metadata": { ... }
//	    }
//	}
//
// # Data document
//
// Policy configuration is loaded into OPA's in-memory store under "config"
// via PolicyConfig (engine.go). The Rego files reference it as data.config.*
// so the same rules work with different configurations (dev / staging / prod).
//
// # Fail-closed
//
// Any OPA evaluation error, or an empty result set, returns DecisionDeny.
// Governance must not silently pass on infrastructure failure.
package opa
