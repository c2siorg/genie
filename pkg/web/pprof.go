package web

import (
	"net/http"
	"net/http/pprof"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
	"github.com/go-chi/chi/v5"
)

// MountPprof attaches the standard library pprof handlers under /debug/pprof.
//
// Two reasons it's not on by default:
//  1. The endpoints expose live heap/goroutine data — leaking them lets
//     anyone read process memory layout.
//  2. The CPU and trace profiles can pause the process under heavy load.
//
// Genie therefore mounts pprof behind JWT auth + admin role. For local
// debugging without a JWT, run StartLocalPprof on :6060 — it binds to
// 127.0.0.1 only so containers don't accidentally expose it.
func MountPprof(r chi.Router, issuer *auth.Issuer) {
	r.Route("/debug/pprof", func(r chi.Router) {
		r.Use(mid.Auth(issuer))
		r.Use(mid.RequireRole(auth.RoleAdmin))

		// Index dispatches /debug/pprof/heap, /goroutine, /threadcreate, etc.
		r.HandleFunc("/", pprof.Index)
		r.HandleFunc("/cmdline", pprof.Cmdline)
		r.HandleFunc("/profile", pprof.Profile)
		r.HandleFunc("/symbol", pprof.Symbol)
		r.HandleFunc("/trace", pprof.Trace)
		// Named profiles need their own Handler.
		r.Handle("/allocs", pprof.Handler("allocs"))
		r.Handle("/block", pprof.Handler("block"))
		r.Handle("/goroutine", pprof.Handler("goroutine"))
		r.Handle("/heap", pprof.Handler("heap"))
		r.Handle("/mutex", pprof.Handler("mutex"))
		r.Handle("/threadcreate", pprof.Handler("threadcreate"))
	})
}

// StartLocalPprof launches a localhost-only pprof listener on the given addr
// (use "127.0.0.1:6060"). Returns the server so the caller can Shutdown it.
//
// Bind explicitly to 127.0.0.1 — not :6060 — to keep the endpoint off the
// container's external interface.
func StartLocalPprof(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	return srv
}
