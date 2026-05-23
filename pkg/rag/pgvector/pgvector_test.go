package pgvector

import (
	"strings"
	"testing"
)

func TestVectorLiteral(t *testing.T) {
	got := vectorLiteral([]float32{1.5, 0, -0.25})
	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
		t.Fatalf("not bracketed: %q", got)
	}
	if !strings.Contains(got, "1.5") || !strings.Contains(got, "-0.25") {
		t.Fatalf("values missing: %q", got)
	}
	// Empty slice should still bracket.
	got = vectorLiteral(nil)
	if got != "[]" {
		t.Fatalf("empty vector literal: %q", got)
	}
}
