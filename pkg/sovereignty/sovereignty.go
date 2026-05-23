// Package sovereignty enforces data-residency rules. India's DPDP Act and
// the RBI payments-data-localization circular require certain data (PII,
// payment data) to stay within India. Genie expresses that with three
// primitives:
//
//   - Region tags on outbound providers and on messages
//   - A ProviderRegistry that lists which external providers may receive
//     which classifications
//   - A DataResidencyPolicy that runs on the bus and denies cross-border
//     hops for sensitive payloads
//
// Wiring this as a Policy keeps residency enforcement where every other
// safety check already lives (governance) rather than scattering checks
// through agent code.
package sovereignty

import (
	"strings"
	"sync"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// Region identifies a hosting locale. We use ISO 3166-1 alpha-2 (lowercase)
// plus a sentinel "on-prem" for self-hosted deployments.
type Region string

const (
	RegionIN     Region = "in"
	RegionUS     Region = "us"
	RegionEU     Region = "eu"
	RegionOnPrem Region = "on-prem"
)

// MetaKeyRegion is the metadata key holding the message's residency tag.
const MetaKeyRegion = "region"

// Provider describes an external service Genie may call (LLM, MCP, payment API).
type Provider struct {
	Name             string
	Region           Region
	// AllowedClassifications enumerates which data classifications may be
	// sent to this provider. Empty list means none (default deny).
	AllowedClassifications []protocol.Classification
}

// ProviderRegistry is the allowlist of providers.
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry builds an empty registry.
func NewRegistry() *ProviderRegistry {
	return &ProviderRegistry{providers: map[string]Provider{}}
}

// Register adds or replaces a provider.
func (r *ProviderRegistry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[strings.ToLower(p.Name)] = p
}

// Get returns a provider by name (case-insensitive).
func (r *ProviderRegistry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[strings.ToLower(name)]
	return p, ok
}

// Allowed returns true if the provider may receive the given classification.
func (r *ProviderRegistry) Allowed(name string, c protocol.Classification) bool {
	p, ok := r.Get(name)
	if !ok {
		return false
	}
	for _, x := range p.AllowedClassifications {
		if x == c {
			return true
		}
	}
	return false
}
