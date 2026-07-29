package bq

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestJSONLSinkOneLinePerEvent(t *testing.T) {
	var buf bytes.Buffer
	s := NewJSONLSink(&buf)
	err := s.Append(context.Background(), []Event{
		{Kind: KindAgentHandle, AgentID: "ingestor", Success: true, DurationMs: 12},
		{Kind: KindLLMCall, LLMProvider: "ollama", PromptTokens: 100, Success: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var first Event
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.AgentID != "ingestor" {
		t.Errorf("expected ingestor on first line")
	}
}

func TestJSONLSinkSetsOccurredAt(t *testing.T) {
	var buf bytes.Buffer
	_ = NewJSONLSink(&buf).Append(context.Background(), []Event{{Kind: KindAgentHandle, AgentID: "a"}})
	var e Event
	_ = json.Unmarshal(buf.Bytes(), &e)
	if e.OccurredAt.IsZero() {
		t.Errorf("expected OccurredAt to be set automatically")
	}
}

type captureSink struct {
	mu    sync.Mutex
	calls [][]Event
}

func (c *captureSink) Append(_ context.Context, events []Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, append([]Event{}, events...))
	return nil
}

func TestBufferAutoFlushesAtMax(t *testing.T) {
	sink := &captureSink{}
	b := NewBuffer(sink, 2)
	_ = b.Record(context.Background(), Event{Kind: KindAgentHandle, AgentID: "x"})
	_ = b.Record(context.Background(), Event{Kind: KindAgentHandle, AgentID: "y"}) // triggers flush
	if len(sink.calls) != 1 || len(sink.calls[0]) != 2 {
		t.Errorf("expected one auto-flush of 2 events; got %+v", sink.calls)
	}
}

func TestBufferManualFlush(t *testing.T) {
	sink := &captureSink{}
	b := NewBuffer(sink, 100)
	_ = b.Record(context.Background(), Event{Kind: KindAgentHandle, AgentID: "x"})
	_ = b.Flush(context.Background())
	if len(sink.calls) != 1 || len(sink.calls[0]) != 1 {
		t.Errorf("expected one manual flush of 1 event; got %+v", sink.calls)
	}
}

func TestBufferFlushEmpty(t *testing.T) {
	sink := &captureSink{}
	b := NewBuffer(sink, 10)
	if err := b.Flush(context.Background()); err != nil {
		t.Errorf("flush of empty buffer should be a noop; got %v", err)
	}
	if len(sink.calls) != 0 {
		t.Errorf("empty flush should not call sink")
	}
}
