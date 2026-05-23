package aibom

import "time"

// CycloneDX 1.6 ML-BOM representation Genie can emit alongside its native
// Document form. CycloneDX is the de-facto OWASP SBOM spec; the ML-BOM
// extension covers AI components (models, datasets, services).

// CycloneDXBOM is the top-level CycloneDX 1.6 document.
type CycloneDXBOM struct {
	BomFormat   string                `json:"bomFormat"`
	SpecVersion string                `json:"specVersion"`
	SerialNumber string               `json:"serialNumber,omitempty"`
	Version     int                   `json:"version"`
	Metadata    CycloneDXMetadata     `json:"metadata"`
	Components  []CycloneDXComponent  `json:"components"`
}

// CycloneDXMetadata holds the timestamp + tool block.
type CycloneDXMetadata struct {
	Timestamp time.Time `json:"timestamp"`
	Tools     struct {
		Components []CycloneDXComponent `json:"components"`
	} `json:"tools"`
}

// CycloneDXComponent is one entry in the BOM. type=machine-learning-model
// is the CycloneDX 1.6 ML-extension shape.
type CycloneDXComponent struct {
	Type        string             `json:"type"` // "library" | "service" | "machine-learning-model"
	Name        string             `json:"name"`
	Version     string             `json:"version,omitempty"`
	BomRef      string             `json:"bom-ref"`
	Description string             `json:"description,omitempty"`
	Properties  []CycloneDXProperty `json:"properties,omitempty"`
}

// CycloneDXProperty is a free-form name/value pair on a component.
type CycloneDXProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ToCycloneDX renders the Document as a CycloneDX 1.6 ML-BOM.
func (d Document) ToCycloneDX() CycloneDXBOM {
	bom := CycloneDXBOM{
		BomFormat:   "CycloneDX",
		SpecVersion: "1.6",
		Version:     1,
		Metadata: CycloneDXMetadata{
			Timestamp: d.GeneratedAt,
		},
	}
	bom.Metadata.Tools.Components = []CycloneDXComponent{{
		Type:    "library",
		Name:    "genie",
		Version: "0.1.0",
		BomRef:  "tool/genie",
	}}
	for _, m := range d.Components {
		c := CycloneDXComponent{
			Name:        m.Name,
			Version:     m.Version,
			BomRef:      "agent/" + m.ID,
			Description: m.Notes,
		}
		switch m.Kind {
		case "llm", "embedder", "reranker":
			c.Type = "machine-learning-model"
		case "tool":
			c.Type = "service"
		default:
			c.Type = "library"
		}
		c.Properties = []CycloneDXProperty{
			{Name: "genie.kind", Value: m.Kind},
			{Name: "genie.region", Value: m.Region},
			{Name: "genie.training_data_class", Value: string(m.TrainingDataClass)},
			{Name: "genie.risk_class", Value: string(m.RiskClass)},
		}
		if m.Model != "" {
			c.Properties = append(c.Properties, CycloneDXProperty{Name: "genie.model", Value: m.Model})
		}
		if m.Provider != "" {
			c.Properties = append(c.Properties, CycloneDXProperty{Name: "genie.provider", Value: m.Provider})
		}
		bom.Components = append(bom.Components, c)
	}
	return bom
}
