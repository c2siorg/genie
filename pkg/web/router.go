package web

import (
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/handlers"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
	"github.com/go-chi/chi/v5"
)

// Deps bundles the dependencies needed to assemble the HTTP router.
type Deps struct {
	Issuer      *auth.Issuer
	Users       *handlers.Users
	Accounts    *handlers.Accounts
	Documents   *handlers.Documents
	Ask         *handlers.Ask
	AskStream   *handlers.AskStream
	Health      *handlers.Health
	MCPTokens   *handlers.MCPTokens
	MCPServer   http.Handler // optional: mounted at /mcp when non-nil
	Incidents   *handlers.Incidents
	Inventory   *handlers.Inventory
	Disclosures *handlers.Disclosures
	AIBOM       *handlers.AIBOM
	Feedback    *handlers.Feedback
	ChatWS      *handlers.ChatWS
	UI          *handlers.UI
	Elevation   *handlers.Elevation  // optional: time-bound privileged access (PCSE 1.4 analog)
	AgentGov    *handlers.AgentGov   // optional: AGT governance endpoints
	OPAHandler  *handlers.OPAHandler // optional: OPA policy introspection endpoints
	HITL        *handlers.HITLHandler // optional: Human-in-the-Loop approval queue
	Compliance  *handlers.ComplianceHandler // optional: Payment compliance checking
	RateLimit   *mid.RateLimit       // optional global limiter
	Logger      mid.Logger
}

// NewRouter builds the chi router with all middleware and routes wired up.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(mid.RequestID)
	r.Use(mid.Recovery(d.Logger))
	r.Use(mid.AccessLog(d.Logger))
	r.Use(mid.Trace("github.com/c2siorg/genie/pkg/web"))

	// Public routes — no rate limit (k8s probes and disclosure surface
	// must not be throttled).
	r.Group(func(r chi.Router) {
		r.Get("/healthz", d.Health.Live)
		r.Get("/readyz", d.Health.Readiness)
		if d.Disclosures != nil {
			r.Get("/v1/disclosures", d.Disclosures.Get)
		}
		if d.UI != nil {
			r.Get("/", d.UI.IndexHTML)
			r.Mount("/ui", http.StripPrefix("/ui", d.UI))
		}
	})

	MountPprof(r, d.Issuer)

	r.Group(func(r chi.Router) {
		if d.RateLimit != nil {
			r.Use(d.RateLimit.Middleware)
		}
		r.Route("/v1", func(r chi.Router) {
			r.Post("/users", d.Users.Signup)
			r.Post("/users/login", d.Users.Login)

			r.Group(func(r chi.Router) {
				r.Use(mid.Auth(d.Issuer))

				r.Get("/users/me", d.Users.Me)
				r.Get("/accounts", d.Accounts.List)
				r.Post("/accounts", d.Accounts.Create)
				r.Post("/documents", d.Documents.Upload)
				r.Get("/documents", d.Documents.Get)
				r.Post("/ask", d.Ask.Post)
				if d.AskStream != nil {
					r.Post("/ask/stream", d.AskStream.Post)
				}
				if d.MCPTokens != nil {
					r.Post("/mcp/tokens", d.MCPTokens.Store)
				}
				if d.Incidents != nil {
					// Reporting is open to any authenticated user (the form is
					// meant to encourage timely disclosure — para 4.4.63). Listing
					// is admin-only.
					r.Post("/incidents", d.Incidents.Create)
					r.With(mid.RequireRole(auth.RoleAdmin)).Get("/incidents", d.Incidents.List)
				}
				if d.Inventory != nil {
					r.With(mid.RequireRole(auth.RoleAdmin)).Get("/ai-inventory", d.Inventory.List)
				}
				if d.AIBOM != nil {
					r.With(mid.RequireRole(auth.RoleAdmin)).Get("/aibom", d.AIBOM.Get)
				}
				if d.Feedback != nil {
					r.Post("/feedback", d.Feedback.Submit)
				}
				if d.ChatWS != nil {
					r.Get("/chat/ws", d.ChatWS.Serve)
				}
				if d.Elevation != nil {
					// Time-bound privileged access (PCSE §1.4 analog).
					// Request is open to any authenticated user (the
					// service caps TTL and requires admin approval).
					// Approve / Deny / Revoke / List are admin-gated;
					// Get is open with the handler enforcing subject-
					// or-admin access internally.
					r.Route("/elevation/requests", func(r chi.Router) {
						r.Post("/", d.Elevation.Request)
						r.Get("/{id}", d.Elevation.Get)
						r.With(mid.RequireRole(auth.RoleAdmin)).Get("/", d.Elevation.List)
						r.With(mid.RequireRole(auth.RoleAdmin)).Post("/{id}/approve", d.Elevation.Approve)
						r.With(mid.RequireRole(auth.RoleAdmin)).Post("/{id}/deny", d.Elevation.Deny)
						r.With(mid.RequireRole(auth.RoleAdmin)).Post("/{id}/revoke", d.Elevation.Revoke)
					})
				}
				// HITL approval queue — authenticated users can list/decide.
				if d.HITL != nil {
					r.Route("/hitl/approvals", func(r chi.Router) {
						r.Get("/", d.HITL.List)
						r.Get("/{id}", d.HITL.Get)
						r.Post("/{id}/approve", d.HITL.Approve)
						r.Post("/{id}/deny", d.HITL.Deny)
					})
				}


				// Payment Compliance API — check compliance, velocity, fraud history
				if d.Compliance != nil {
					r.Route("/compliance", func(r chi.Router) {
						r.Post("/check", d.Compliance.CheckPayment)
						r.Get("/check/{compliance_check_id}", d.Compliance.GetCheck)
						r.Get("/account/{account_id}/velocity", d.Compliance.GetVelocity)
						r.Get("/account/{account_id}/fraud-history", d.Compliance.GetFraudHistory)
						r.Post("/admin/reset-velocity", d.Compliance.ResetVelocity)
					})
				}
				if d.AgentGov != nil || d.OPAHandler != nil {
					r.With(mid.RequireRole(auth.RoleAdmin)).Route("/governance", func(r chi.Router) {
						if d.AgentGov != nil {
							r.Get("/agents", d.AgentGov.ListAgents)
							r.Get("/trust/{agentID}", d.AgentGov.GetTrust)
							r.Get("/audit", d.AgentGov.GetAudit)
							r.Post("/killswitch", d.AgentGov.ActivateKillSwitch)
							r.Delete("/killswitch", d.AgentGov.ClearKillSwitch)
							r.Get("/killswitch", d.AgentGov.ListKillSwitches)
							r.Get("/slo", d.AgentGov.GetSLO)
							r.Get("/rings/{agentID}", d.AgentGov.GetRing)
						}
						if d.OPAHandler != nil {
							r.Route("/opa", func(r chi.Router) {
								r.Get("/health", d.OPAHandler.Health)
								r.Get("/config", d.OPAHandler.GetConfig)
								r.Post("/evaluate", d.OPAHandler.Evaluate)
								r.Post("/check-http", d.OPAHandler.CheckHTTP)
							})
						}
					})
				}
			})
		})
	})

	if d.MCPServer != nil {
		r.Mount("/mcp", d.MCPServer)
	}

	return r
}
