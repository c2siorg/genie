package observability

import (
	"context"
	"log/slog"
	"os"
)

// SlogLogger satisfies pkg/web/mid.Logger and observability.Logger using
// log/slog. JSON output goes to stdout by default; swap the handler to switch
// formats. This is the production logger; older StdLogger remains for the
// CLI demo.
type SlogLogger struct {
	L *slog.Logger
}

// NewSlogLogger returns a JSON slog handler configured with sensible defaults.
func NewSlogLogger(level slog.Level) *SlogLogger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return &SlogLogger{L: slog.New(h)}
}

// Printf implements observability.Logger for backwards compatibility.
func (s *SlogLogger) Printf(format string, v ...any) {
	s.L.Info("legacy.printf", slog.String("msg", sprintf(format, v...)))
}

// Info satisfies pkg/web/mid.Logger.
func (s *SlogLogger) Info(msg string, args ...any) { s.L.Info(msg, args...) }

// Error satisfies pkg/web/mid.Logger.
func (s *SlogLogger) Error(msg string, args ...any) { s.L.Error(msg, args...) }

// LogAttrs lets callers attach structured attrs in hot paths.
func (s *SlogLogger) LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	s.L.LogAttrs(ctx, level, msg, attrs...)
}

// sprintf is a tiny indirection so we can avoid importing fmt for printf-style
// usage from the legacy Logger interface. Kept here to keep formatting in one
// place if we ever want to swap for something else.
func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmtSprintf(format, args...)
}
