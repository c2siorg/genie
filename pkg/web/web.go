// Package web is Genie's HTTP layer. It builds a chi-based router with
// JWT + RBAC middleware and routes that translate REST calls onto the
// multi-agent bus.
//
// Boundaries:
//   - This package depends on auth, governance, comm, registry, crypto,
//     storage. It does NOT depend on any specific agent — the agent set is
//     injected via the bus.
//   - HTTP handlers do not reach into protocol.Message; they go through the
//     small "Publish + wait" helper here so retries/correlation stay in one
//     place.
//
// The concrete request/response helpers live in the handlers subpackage
// (pkg/web/handlers), which owns its own JSON encode/decode plumbing.
package web
