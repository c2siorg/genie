package kyc

import (
	"context"
	"time"

	"github.com/c2siorg/genie/pkg/compliance"
	"github.com/c2siorg/genie/pkg/kyc"
)

// SanctionsCheckerAgent screens customer name/DOB/jurisdiction against OFAC SDN, UN, and MHA watchlists.
type SanctionsCheckerAgent struct {
	sanctions SanctionsService
	cache     *MCPResultCache
	auditLog  compliance.AuditLog
}

// CheckSanctions screens name against multiple watchlists.
// Returns hit status, list of matched records, and risk level.
func (a *SanctionsCheckerAgent) CheckSanctions(
	ctx context.Context,
	name string,
	dob *time.Time,
	jurisdiction string,
) (*kyc.SanctionsResult, error) {
	// TODO: implement
	panic("not implemented")
}

// SanctionsService defines the interface for sanctions screening.
type SanctionsService interface {
	// Screen performs sanctions screening against configured watchlists.
	Screen(ctx context.Context, name string, dob *time.Time, jurisdiction string) (*SanctionsScreeningResult, error)
}

// SanctionsScreeningResult holds the output of sanctions screening.
type SanctionsScreeningResult struct {
	IsSanctioned      bool
	Hits              []SanctionsHit
	WatchlistsChecked []string
	RiskLevel         string // "green" | "yellow" | "red"
	LastCheckedAt     time.Time
	CheckerID         string
	Note              string
}

// SanctionsHit represents a match against a watchlist.
type SanctionsHit struct {
	Watchlist     string
	MatchedName   string
	Confidence    float64
	RecordID      string
	DateOfListing time.Time
	Type          string // "individual" | "entity"
}

// MCPResultCache provides caching for MCP sanctions check results.
type MCPResultCache struct {
	// TODO: implement
}

// MCPResultCache methods will be implemented during full development.
