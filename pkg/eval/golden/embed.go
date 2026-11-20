package golden

import "embed"

// dataFS embeds the on-disk golden dataset so it travels with any binary that
// imports this package (the eval-golden CLI, CI, tests) without a working-dir
// dependency.
//
//go:embed data/*.json
var dataFS embed.FS

// LoadEmbedded loads the golden dataset baked into the binary at build time.
// It is the canonical entry point for the gate and CLI; LoadGoldenDir remains
// available for ad-hoc on-disk datasets.
func LoadEmbedded() ([]Case, []Dataset, error) {
	return LoadGoldenFS(dataFS, "data")
}
