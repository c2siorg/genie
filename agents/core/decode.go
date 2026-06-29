package core

import (
	"encoding/json"
	"fmt"
)

// DecodeInput decodes an agent's loosely-typed Execute input into a concrete
// target struct, regardless of how the input arrived.
//
// Agents are called two ways:
//   - In-process (MemoryClient / Pipeline): input is a typed Go value — often a
//     pointer to a struct, or the previous agent's output.
//   - Over HTTP/gRPC: input is decoded from JSON, arriving as a
//     map[string]interface{} or (once AgentInput.Payload becomes
//     json.RawMessage per Decision 2) as raw bytes.
//
// DecodeInput normalizes all of these: json.RawMessage and []byte are
// unmarshaled directly (preserving int64 precision); anything else is
// round-tripped through JSON into target. This means an agent has a single,
// uniform decode path and needs no changes when the transport switches Payload
// to json.RawMessage.
//
// target must be a non-nil pointer.
func DecodeInput(input, target interface{}) error {
	if target == nil {
		return fmt.Errorf("decode target is nil")
	}
	switch v := input.(type) {
	case nil:
		return fmt.Errorf("input is nil")
	case json.RawMessage:
		return json.Unmarshal(v, target)
	case []byte:
		return json.Unmarshal(v, target)
	default:
		b, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("marshal input: %w", err)
		}
		return json.Unmarshal(b, target)
	}
}
