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
	Elevation   *handlers.Elevation         // optional: time-bound privileged access (PCSE 1.4 analog)
	AgentGov    *handlers.AgentGov          // optional: AGT governance endpoints
	OPAHandler  *handlers.OPAHandler        // optional: OPA policy introspection endpoints
	HITL        *handlers.HITLHandler       // optional: Human-in-the-Loop approval queue
	Settlement  *handlers.SettlementHandler // optional: Settlement Coordinator
	AML         *handlers.AMLHandler        // optional: AML Risk Scoring
	Consent     *handlers.ConsentHandler    // optional: Consent Registry
	Lineage     *handlers.LineageHandler    // optional: Data Lineage Tracker
	// E-Rupee Commerce APIs
	Payment    *handlers.Payment           // optional: e-Rupee Payment Agent
	Commerce   *handlers.CommerceHandler   // optional: Commerce Workflow Engine
	Merchant   *handlers.MerchantHandler   // optional: Merchant Onboarding
	Compliance *handlers.ComplianceHandler // optional: Payment Compliance
	CBDC       *handlers.CBDCHandler       // optional: CBDC Ledger & Settlement
	RateLimit  *mid.RateLimit              // optional global limiter
	Logger     mid.Logger
	// CSRFEnforce turns the cookie-scoped CSRF middleware from report-only into
	// hard enforcement (403 on a missing/invalid token for cookie-authed,
	// state-changing requests). Default false — flip once the SPA sends the
	// X-CSRF-Token header. Has no effect on Bearer-token API traffic.
	CSRFEnforce bool
}

// NewRouter builds the chi router with all middleware and routes wired up.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(mid.RequestID)
	r.Use(mid.Recovery(d.Logger))
	r.Use(mid.AccessLog(d.Logger))
	r.Use(mid.Trace("github.com/c2siorg/genie/pkg/web"))
	// Defensive response headers (CSP, X-Frame-Options: DENY, nosniff,
	// Referrer-Policy, Permissions-Policy) on every response, including errors.
	// The default CSP is same-origin ('self'); verify the embedded SPA loads
	// clean in staging before promoting (a CSP tweak is one line).
	r.Use(mid.SecurityHeaders())
	// Cookie-scoped CSRF protection. Self-scopes to cookie-authenticated,
	// state-changing requests — a no-op for Bearer-token API traffic and
	// uncredentialed requests — so it is safe to install globally. Report-only
	// until d.CSRFEnforce is set (see GENIE_CSRF_ENFORCE).
	if d.Issuer != nil {
		r.Use(mid.CookieScopedCSRF(d.Issuer, d.CSRFEnforce, d.Logger))
	}

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

				// Finance Module APIs (Settlement, AML, Consent, Lineage)
				if d.Settlement != nil {
					r.Route("/settlement", func(r chi.Router) {
						r.Route("/request", func(r chi.Router) {
							r.Post("/", d.Settlement.CreateRequest)
							r.Get("/{request_id}", d.Settlement.GetRequest)
							r.Post("/{request_id}/execute", d.Settlement.ExecuteSettlement)
							r.Get("/{request_id}/audit", d.Settlement.GetAuditLog)
						})
					})
				}

				if d.AML != nil {
					r.Route("/aml", func(r chi.Router) {
						r.Post("/score", d.AML.ScoreTransaction)
						r.Get("/score/{score_id}", d.AML.GetScore)
						r.Get("/history/{txn_id}", d.AML.GetHistory)
					})
				}

				if d.Consent != nil {
					r.Route("/consent", func(r chi.Router) {
						r.Post("/grant", d.Consent.GrantConsent)
						r.Post("/revoke", d.Consent.RevokeConsent)
						r.Get("/list", d.Consent.ListGrants)
						r.Get("/audit/{consent_id}", d.Consent.GetAuditDecisions)
					})
				}

				if d.Lineage != nil {
					r.Route("/lineage", func(r chi.Router) {
						r.Post("/query", d.Lineage.QueryLineage)
						r.Post("/verify", d.Lineage.VerifyIntegrity)
						r.Get("/export/{entity_id}", d.Lineage.ExportAuditTrail)
					})
				}

				// E-Rupee Commerce APIs
				if d.Payment != nil {
					r.Route("/payment", func(r chi.Router) {
						r.Post("/initiate", d.Payment.InitiatePayment)
						r.Get("/{payment_id}", d.Payment.GetPayment)
					})
					r.Route("/account", func(r chi.Router) {
						r.Post("/", d.Payment.CreateAccount)
						r.Get("/{account_id}", d.Payment.GetAccount)
					})
					r.Route("/transaction", func(r chi.Router) {
						r.Get("/{transaction_id}", d.Payment.ListTransactions)
					})
				}

				if d.Commerce != nil {
					r.Route("/commerce/order", func(r chi.Router) {
						r.Post("/", d.Commerce.CreateOrder)
						r.Get("/{order_id}", d.Commerce.GetOrder)
						r.Post("/{order_id}/execute", d.Commerce.ExecuteWorkflow)
						r.Get("/{order_id}/audit", d.Commerce.GetAuditLog)
					})
				}

				if d.Merchant != nil {
					r.Route("/merchant", func(r chi.Router) {
						r.Post("/onboard", d.Merchant.OnboardMerchant)
						r.Get("/{merchant_id}", d.Merchant.GetMerchant)
						r.Get("/{merchant_id}/onboarding", d.Merchant.GetOnboardingStatus)
						r.Post("/{merchant_id}/approve", d.Merchant.ApproveMerchant)
						r.Post("/{merchant_id}/limits", d.Merchant.UpdateLimits)
					})
				}

				if d.Compliance != nil {
					r.Route("/compliance", func(r chi.Router) {
						r.Post("/check", d.Compliance.CheckPayment)
						r.Get("/check/{check_id}", d.Compliance.GetCheck)
						r.Get("/account/{account_id}/velocity", d.Compliance.GetVelocity)
						r.Get("/account/{account_id}/fraud-history", d.Compliance.GetFraudHistory)
						r.Post("/admin/reset-velocity", d.Compliance.ResetVelocity)
					})
				}

				if d.CBDC != nil {
					r.Route("/cbdc", func(r chi.Router) {
						r.Post("/transaction", d.CBDC.InitiateTransaction)
						r.Get("/transaction/{transaction_id}", d.CBDC.GetTransaction)
						r.Get("/block/{height}", d.CBDC.GetBlock)
						r.Get("/limits/{account_id}", d.CBDC.GetLimits)
						r.Get("/health", d.CBDC.Health)
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
