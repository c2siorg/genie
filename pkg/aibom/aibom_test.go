package aibom

import (
	"testing"
	"time"
)

func TestSignVerify_Roundtrip(t *testing.T) {
	signer, err := NewEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	doc := Document{
		GeneratedAt: time.Now().UTC(),
		Components: []Manifest{
			{ID: "ingestor", Name: "ingestor", Kind: "agent", TrainingDataClass: TrainingUnknown},
		},
	}
	sd, err := Sign(doc, signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(sd); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Tamper a field; verification should fail.
	sd.Document.Components[0].Name = "tampered"
	if err := Verify(sd); err == nil {
		t.Fatal("expected verify to detect tamper")
	}
}

func TestCycloneDX_Shape(t *testing.T) {
	doc := Document{
		GeneratedAt: time.Now().UTC(),
		Components: []Manifest{
			{ID: "ollama", Name: "ollama", Kind: "llm", Model: "llama3.2:1b", Region: "on-prem"},
			{ID: "ingestor", Name: "ingestor", Kind: "agent"},
		},
	}
	bom := doc.ToCycloneDX()
	if bom.BomFormat != "CycloneDX" || bom.SpecVersion != "1.6" {
		t.Fatalf("unexpected header: %+v", bom)
	}
	var sawML, sawLib bool
	for _, c := range bom.Components {
		if c.Type == "machine-learning-model" {
			sawML = true
		}
		if c.Type == "library" {
			sawLib = true
		}
	}
	if !sawML || !sawLib {
		t.Fatalf("expected mix of types, got %+v", bom.Components)
	}
}
