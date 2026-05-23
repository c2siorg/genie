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
	Issuer    *auth.Issuer
	Users     *handlers.Users
	Accounts  *handlers.Accounts
	Documents *handlers.Documents
	Ask       *handlers.Ask
	Health    *handlers.Health
	Logger    mid.Logger
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
		})
	})

	return r
}
