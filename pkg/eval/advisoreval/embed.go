package advisoreval

import (
	"embed"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/golden"
)

// dataFS embeds the advisor eval cases so they travel with the binary/tests.
//
//go:embed data/*.json
var dataFS embed.FS

// LoadEmbedded loads the advisor eval dataset. It reuses the golden loader, so
// the same invariants apply (no pre-baked output, unique IDs, count match).
func LoadEmbedded() ([]golden.Case, []golden.Dataset, error) {
	return golden.LoadGoldenFS(dataFS, "data")
}
