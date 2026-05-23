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
	RateLimit   *mid.RateLimit // optional global limiter
	Logger      mid.Logger
}

// NewRouter builds the chi router with all middleware and routes wired up.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(mid.RequestID)
	r.Use(mid.Recovery(d.Logger))
	r.Use(mid.AccessLog(d.Logger))
	r.Use(mid.Trace("github.com/c2siorg/genie/pkg/web"))

	r.Get("/healthz", d.Health.Live)
	r.Get("/readyz", d.Health.Readiness)
	if d.Disclosures != nil {
		// Public disclosure surface — no auth, no rate limit needed.
		r.Get("/v1/disclosures", d.Disclosures.Get)
	}

	if d.UI != nil {
		// Mount the embedded single-page UI under /ui/ and redirect root.
		r.Get("/", d.UI.IndexHTML)
		r.Mount("/ui", http.StripPrefix("/ui", d.UI))
	}

	if d.RateLimit != nil {
		r.Use(d.RateLimit.Middleware)
	}

	MountPprof(r, d.Issuer)

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
		})
	})

	if d.MCPServer != nil {
		r.Mount("/mcp", d.MCPServer)
	}

	return r
}
