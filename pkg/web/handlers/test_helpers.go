// test_helpers.go — Shared test utilities for handlers.
package handlers

import (
	"context"

	"github.com/go-chi/chi/v5"
)

// setupChiContext sets up a chi routing context with URL parameters for testing.
// This simulates chi's URL parameter extraction.
func setupChiContext(ctx context.Context, paramName, paramValue string) context.Context {
	chiCtx := &chi.Context{}
	chiCtx.URLParams = chi.RouteParams{
		Keys:   []string{paramName},
		Values: []string{paramValue},
	}
	return context.WithValue(ctx, chi.RouteCtxKey, chiCtx)
}

// setupMultipleChiParams sets up multiple URL parameters in the chi context.
func setupMultipleChiParams(ctx context.Context, params map[string]string) context.Context {
	chiCtx := &chi.Context{}
	keys := make([]string, 0, len(params))
	values := make([]string, 0, len(params))
	for k, v := range params {
		keys = append(keys, k)
		values = append(values, v)
	}
	chiCtx.URLParams = chi.RouteParams{
		Keys:   keys,
		Values: values,
	}
	return context.WithValue(ctx, chi.RouteCtxKey, chiCtx)
}
