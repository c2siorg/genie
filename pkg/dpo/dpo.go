// Package dpo exports collected user feedback as DPO/RLAIF-ready preference
// pairs (chosen vs rejected). Pairs are emitted as JSONL — one example per
// line — matching the de-facto OpenAI / Hugging Face training format.
package dpo

import (
	"context"
	"encoding/json"
	"io"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/synth"
)

// Example is one (chosen, rejected) preference pair.
type Example struct {
	Prompt   string `json:"prompt"`
	Chosen   string `json:"chosen"`
	Rejected string `json:"rejected"`
}

// Export pulls feedback from the store and writes JSONL pairs to w. The
// mapping is intentionally simple:
//
//   - "edit" feedback: Chosen = Preferred, Rejected = Original.
//   - "down" feedback: Rejected = Original, Chosen = "" (caller can
//     pair with a separate high-quality answer in a later pass).
//   - "up" feedback is skipped — there's no rejected counterpart yet.
//
// Returns the number of examples written.
func Export(ctx context.Context, store synth.FeedbackStore, w io.Writer, limit int) (int, error) {
	entries, err := store.List(ctx, limit)
	if err != nil {
		return 0, err
	}
	enc := json.NewEncoder(w)
	written := 0
	for _, fb := range entries {
		var ex Example
		switch fb.Kind {
		case synth.FeedbackEdit:
			if fb.Preferred == "" || fb.Original == "" {
				continue
			}
			ex = Example{Chosen: fb.Preferred, Rejected: fb.Original}
		case synth.FeedbackDown:
			if fb.Original == "" {
				continue
			}
			ex = Example{Rejected: fb.Original}
		default:
			continue
		}
		// Use the feedback trace as the prompt placeholder so downstream
		// joiners can recover the original conversation.
		ex.Prompt = fb.TraceID
		if err := enc.Encode(ex); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}
