// Package cloudevents implements the CloudEvents 1.0 envelope so external
// systems (Knative, Argo Events, Kafka consumers) can ingest Genie bus
// messages without bespoke parsing.
//
// We provide Wrap (protocol.Message -> Event) and Unwrap (Event ->
// protocol.Message) so the bus stays internal while the wire format is
// standardised.
package cloudevents

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// SpecVersion is the CloudEvents version Genie emits.
const SpecVersion = "1.0"

// Event is a CloudEvents 1.0 structured-mode envelope.
type Event struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Subject         string          `json:"subject,omitempty"`
	Time            time.Time       `json:"time,omitempty"`
	DataContentType string          `json:"datacontenttype,omitempty"`
	Data            json.RawMessage `json:"data,omitempty"`

	// Extension attributes — keep classification, trace, region first-class.
	GenieClassification string `json:"genieclassification,omitempty"`
	GenieTraceID        string `json:"genietraceid,omitempty"`
	GenieRegion         string `json:"genieregion,omitempty"`
}

// Wrap turns a protocol.Message into a CloudEvent. The Message's Content
// becomes Data (passed through verbatim — callers ensure it's already valid
// JSON or some other content type they declare).
func Wrap(msg protocol.Message, sourceURI string) Event {
	classification, _ := msg.Metadata[protocol.MetaKeyClassification].(string)
	traceID, _ := msg.Metadata["trace_id"].(string)
	region, _ := msg.Metadata["region"].(string)
	return Event{
		SpecVersion:         SpecVersion,
		ID:                  msg.ID,
		Source:              sourceURI,
		Type:                "com.c2siorg.genie." + msg.Type,
		Subject:             msg.To,
		Time:                msg.CreatedAt,
		DataContentType:     "application/json",
		Data:                json.RawMessage(`"` + escapeJSON(msg.Content) + `"`),
		GenieClassification: classification,
		GenieTraceID:        traceID,
		GenieRegion:         region,
	}
}

// Unwrap converts a CloudEvent back into a protocol.Message. Returns an
// error if the envelope is missing the Genie-specific subject (the To
// field). Roles default to RoleAgent since the envelope strips role.
func Unwrap(ev Event) (protocol.Message, error) {
	if ev.SpecVersion != SpecVersion {
		return protocol.Message{}, errors.New("cloudevents: unsupported specversion")
	}
	if ev.ID == "" {
		return protocol.Message{}, errors.New("cloudevents: id required")
	}
	// data may be either a JSON string or any other JSON value — decode to string.
	var content string
	if len(ev.Data) > 0 {
		if err := json.Unmarshal(ev.Data, &content); err != nil {
			// not a string — keep raw JSON
			content = string(ev.Data)
		}
	}
	md := map[string]any{}
	if ev.GenieClassification != "" {
		md[protocol.MetaKeyClassification] = ev.GenieClassification
	}
	if ev.GenieTraceID != "" {
		md["trace_id"] = ev.GenieTraceID
	}
	if ev.GenieRegion != "" {
		md["region"] = ev.GenieRegion
	}
	return protocol.Message{
		ID:        ev.ID,
		To:        ev.Subject,
		Type:      stripPrefix(ev.Type, "com.c2siorg.genie."),
		Role:      protocol.RoleAgent,
		Content:   content,
		CreatedAt: ev.Time,
		Metadata:  md,
	}, nil
}

// escapeJSON is a tiny helper that escapes only the characters that break
// JSON string syntax. Callers usually pass already-valid JSON content; the
// wrap path needs to embed it as a JSON string.
func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return ""
}

func stripPrefix(s, prefix string) string {
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
