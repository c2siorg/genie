package cloudevents

import (
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

func TestWrapUnwrap_Roundtrip(t *testing.T) {
	msg := protocol.Message{
		ID:        "m-1",
		From:      "ingestor",
		To:        "normalizer",
		Type:      "raw_transactions",
		Role:      protocol.RoleAgent,
		Content:   `{"hello":"world"}`,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		Metadata: map[string]any{
			"trace_id":                       "tr-1",
			"region":                         "in",
			protocol.MetaKeyClassification:   "pii",
		},
	}
	ev := Wrap(msg, "genie://bus")
	if ev.Subject != "normalizer" || ev.GenieTraceID != "tr-1" || ev.GenieClassification != "pii" {
		t.Fatalf("unexpected envelope: %+v", ev)
	}
	if ev.Type != "com.c2siorg.genie.raw_transactions" {
		t.Fatalf("type: %q", ev.Type)
	}
	got, err := Unwrap(ev)
	if err != nil {
		t.Fatal(err)
	}
	if got.To != msg.To || got.Type != msg.Type || got.Content != msg.Content {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if got.Metadata[protocol.MetaKeyClassification] != "pii" {
		t.Errorf("classification not preserved: %+v", got.Metadata)
	}
}
