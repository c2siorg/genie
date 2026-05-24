package memory

import (
	"context"
	"testing"
)

func TestLongTermRecordAndCurrent(t *testing.T) {
	m := NewLongTermMemory()
	m.Record("u-1", Fact{Key: "primary_bank", Value: "HDFC", Confidence: 0.9, Source: "test"})
	got, ok := m.Current("u-1", "primary_bank")
	if !ok || got.Value != "HDFC" {
		t.Errorf("expected HDFC, got %+v ok=%v", got, ok)
	}
}

func TestLongTermSupersedes(t *testing.T) {
	m := NewLongTermMemory()
	m.Record("u-1", Fact{Key: "primary_bank", Value: "HDFC"})
	m.Record("u-1", Fact{Key: "primary_bank", Value: "ICICI"})
	got, _ := m.Current("u-1", "primary_bank")
	if got.Value != "ICICI" {
		t.Errorf("expected most recent ICICI to win; got %s", got.Value)
	}
	hist := m.History("u-1", "primary_bank")
	if len(hist) != 2 {
		t.Errorf("history should retain both entries; got %d", len(hist))
	}
	if hist[0].SupersededAt == nil {
		t.Errorf("old fact must be marked superseded")
	}
}

func TestPerUserIsolation(t *testing.T) {
	m := NewLongTermMemory()
	m.Record("u-1", Fact{Key: "x", Value: "a"})
	m.Record("u-2", Fact{Key: "x", Value: "b"})
	g1, _ := m.Current("u-1", "x")
	g2, _ := m.Current("u-2", "x")
	if g1.Value != "a" || g2.Value != "b" {
		t.Errorf("users must not leak: u1=%s u2=%s", g1.Value, g2.Value)
	}
}

func TestSearchValue(t *testing.T) {
	m := NewLongTermMemory()
	m.Record("u-1", Fact{Key: "primary_bank", Value: "HDFC"})
	m.Record("u-1", Fact{Key: "secondary_bank", Value: "Axis"})
	hits := m.SearchValue("u-1", "hdfc")
	if len(hits) != 1 || hits[0].Key != "primary_bank" {
		t.Errorf("expected primary_bank hit; got %+v", hits)
	}
}

func TestCurrentAllSkipsSuperseded(t *testing.T) {
	m := NewLongTermMemory()
	m.Record("u-1", Fact{Key: "k", Value: "old"})
	m.Record("u-1", Fact{Key: "k", Value: "new"})
	m.Record("u-1", Fact{Key: "other", Value: "x"})
	current := m.CurrentAll("u-1")
	if len(current) != 2 {
		t.Errorf("expected 2 active facts, got %d", len(current))
	}
}

type fakeConsolidator struct{ out []Fact }

func (f fakeConsolidator) Consolidate(_ context.Context, _ string) ([]Fact, error) {
	return f.out, nil
}

func TestApplyConsolidator(t *testing.T) {
	m := NewLongTermMemory()
	n, err := m.Apply(context.Background(), "u-1", fakeConsolidator{
		out: []Fact{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 facts recorded, got %d", n)
	}
	if got, _ := m.Current("u-1", "a"); got.Value != "1" {
		t.Errorf("consolidator fact missing")
	}
}
