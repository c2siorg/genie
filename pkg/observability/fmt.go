package observability

import "fmt"

// fmtSprintf is a thin wrapper kept in its own file so the rest of the
// package can use sprintf without importing fmt directly. Helps keep
// generated code that depends on observability lean.
func fmtSprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
