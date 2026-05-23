package schema

import "testing"

func TestSchema_BasicObject(t *testing.T) {
	s, err := Parse([]byte(`{
	  "type":"object",
	  "required":["name","age"],
	  "properties":{
	    "name":{"type":"string","minLength":1},
	    "age":{"type":"integer","minimum":0}
	  }
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ValidateJSON([]byte(`{"name":"Alice","age":30}`)); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
	if err := s.ValidateJSON([]byte(`{"name":"","age":30}`)); err == nil {
		t.Fatal("expected min length failure")
	}
	if err := s.ValidateJSON([]byte(`{"age":30}`)); err == nil {
		t.Fatal("expected missing required failure")
	}
	if err := s.ValidateJSON([]byte(`{"name":"x","age":-1}`)); err == nil {
		t.Fatal("expected minimum failure")
	}
}

func TestSchema_ArrayItems(t *testing.T) {
	s, err := Parse([]byte(`{
	  "type":"array",
	  "items":{"type":"object","required":["rationale"],"properties":{"rationale":{"type":"string","minLength":3}}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ValidateJSON([]byte(`[{"rationale":"because"}]`)); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
	if err := s.ValidateJSON([]byte(`[{"rationale":"x"}]`)); err == nil {
		t.Fatal("expected min length failure")
	}
}
