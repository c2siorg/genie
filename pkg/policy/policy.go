// Package policy loads the Annexure V "Board-Approved AI Policy" from YAML
// and assembles a runtime governance.Policy composite from it.
//
// Putting the policy in config — not code — is what the RBI FREE-AI report
// asks for in Recommendation 14: the board approves the policy, not the
// engineers. Engineers ship the loader; the board owns the YAML.
package policy

import (
	"errors"
	"fmt"
	"os"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/sovereignty"
	"gopkg.in/yaml.v3"
)

// AIPolicy mirrors Annexure V — Suggested Outline of Board Policy on AI.
// Each section is a separate struct so future board revisions can extend
// without breaking older files.
type AIPolicy struct {
	Version       string         `yaml:"version"`
	BoardApproved string         `yaml:"board_approved_on"` // ISO date
	Owner         string         `yaml:"owner"`             // e.g. CIO, CRO
	Purpose       string         `yaml:"purpose"`
	Principles    []string       `yaml:"principles"`
	Governance    Governance     `yaml:"governance"`
	Risk          RiskAppetite   `yaml:"risk"`
	Data          DataLifecycle  `yaml:"data"`
	Consumer      Consumer       `yaml:"consumer"`
	Sovereignty   SovereigntyDef `yaml:"sovereignty"`
	Consent       Consent        `yaml:"consent"`
	Explain       Explain        `yaml:"explainability"`
	Limits        Limits         `yaml:"limits"`
}

type Governance struct {
	AdminBypass bool                `yaml:"admin_bypass"`
	RBAC        map[string][]string `yaml:"rbac"` // type -> any-of roles
}

type RiskAppetite struct {
	MaxContentLengthBytes int `yaml:"max_content_length_bytes"`
}

type DataLifecycle struct {
	RetentionDays         int  `yaml:"retention_days"`
	BlockPII              bool `yaml:"block_pii"`
	BlockPromptInjection  bool `yaml:"block_prompt_injection"`
}

type Consumer struct {
	AIDisclosureBanner string `yaml:"ai_disclosure_banner"`
}

type SovereigntyDef struct {
	HomeRegion                string `yaml:"home_region"` // "in"|"us"|"eu"|"on-prem"
	AllowCrossBorderForPublic bool   `yaml:"allow_cross_border_for_public"`
}

type Consent struct {
	// TypeToCategory maps message type -> consent category required.
	TypeToCategory map[string]string `yaml:"type_to_category"`
}

type Explain struct {
	AppliesTo []string `yaml:"applies_to"`
}

type Limits struct {
	RequiredMetadata map[string][]string `yaml:"required_metadata"`
}

// Load parses a YAML file from disk.
func Load(path string) (*AIPolicy, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ai-policy: %w", err)
	}
	return Parse(body)
}

// Parse decodes YAML bytes into an AIPolicy and applies defaults.
func Parse(body []byte) (*AIPolicy, error) {
	var p AIPolicy
	if err := yaml.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("parse ai-policy: %w", err)
	}
	if p.Version == "" {
		return nil, errors.New("ai-policy: version is required")
	}
	if p.Risk.MaxContentLengthBytes == 0 {
		p.Risk.MaxContentLengthBytes = 256 * 1024
	}
	if p.Sovereignty.HomeRegion == "" {
		p.Sovereignty.HomeRegion = string(sovereignty.RegionIN)
	}
	return &p, nil
}

// BuildComposite turns an AIPolicy into a runtime governance.Policy composite,
// pulling in the consent ledger when consent gating is configured.
func (p *AIPolicy) BuildComposite(consents compliance.Ledger) governance.Policy {
	policies := []governance.Policy{
		governance.MaxContentLengthPolicy{Max: p.Risk.MaxContentLengthBytes},
	}
	if len(p.Limits.RequiredMetadata) > 0 {
		for typeName, required := range p.Limits.RequiredMetadata {
			policies = append(policies, governance.RequiredMetadataPolicy{
				AppliesTo: []string{typeName},
				Required:  required,
			})
		}
	}
	if len(p.Governance.RBAC) > 0 {
		policies = append(policies, governance.RBACPolicy{
			RequiredRolesByType: p.Governance.RBAC,
			AdminBypass:         p.Governance.AdminBypass,
		})
	}
	// Default ceiling is Internal so any unknown recipient cannot accept PII
	// without being explicitly whitelisted. Aligned with the RBI report's
	// "minimum baseline" stance.
	policies = append(policies, governance.ClassificationPolicy{
		DefaultCeiling: protocol.ClassInternal,
	})
	policies = append(policies, governance.NewResidencyPolicy(sovereignty.Region(p.Sovereignty.HomeRegion)))
	if p.Data.BlockPII {
		policies = append(policies, governance.PIIBlockPolicy{})
	}
	if p.Data.BlockPromptInjection {
		policies = append(policies, governance.PromptInjectionPolicy{})
	}
	if consents != nil && len(p.Consent.TypeToCategory) > 0 {
		mapping := make(map[string]compliance.ConsentCategory, len(p.Consent.TypeToCategory))
		for k, v := range p.Consent.TypeToCategory {
			mapping[k] = compliance.ConsentCategory(v)
		}
		policies = append(policies, governance.ConsentPolicy{Ledger: consents, TypeToCat: mapping})
	}
	if len(p.Explain.AppliesTo) > 0 {
		policies = append(policies, governance.ExplainabilityPolicy{AppliesTo: p.Explain.AppliesTo})
	}
	return governance.NewComposite(policies...)
}
