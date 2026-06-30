// This file provides order settlement reconciliation and verification logic.
// It validates that orders match their CBDC ledger commits and lineage records,
// ensuring no double-spends and maintaining audit trail integrity.
//
// Author: Phase 2 Integration Testing Implementation
// License: MIT (see root LICENSE file)
package commerce

import (
	"context"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/cbdc"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/lineage"
)

// ReconciliationCheck verifies that an order's settlement is complete and correct.
type ReconciliationCheck struct {
	OrderID          string
	AmountMatched    bool
	LineageComplete  bool
	NoDuplicateSpend bool
	SettlementStatus string
	VerifiedAt       time.Time
	Issues           []string
}

// VerifyOrderSettlement checks that an order is backed by a real CBDC ledger
// entry of the correct amount and an intact lineage chain.
//
// settlementLedgerID is the payment ID under which the order's settlement was
// committed to the ledger (e.g. the settlement batch key). It is supplied
// explicitly because the order itself does not yet persist its ledger linkage —
// passing it makes the order→ledger correspondence auditable rather than
// assumed. (Persisting the linkage on the order is the durable follow-up.)
//
// Unlike the previous version — which hard-coded AmountMatched and
// NoDuplicateSpend to true — this queries the ledger and only sets those flags
// when the ledger actually confirms them.
func VerifyOrderSettlement(ctx context.Context, orderID, settlementLedgerID string, ledger cbdc.Ledger, lineageRec lineage.Recorder, orderMgr OrderManager) (*ReconciliationCheck, error) {
	if orderID == "" {
		return nil, fmt.Errorf("order_id required")
	}

	check := &ReconciliationCheck{
		OrderID:    orderID,
		VerifiedAt: time.Now().UTC(),
		Issues:     []string{},
	}

	// Step 1: Get the order
	order, err := orderMgr.GetOrder(orderID)
	if err != nil || order == nil {
		check.Issues = append(check.Issues, "order not found")
		return check, nil
	}

	// Step 2: Verify lineage captures full workflow
	if lineageRec == nil {
		check.Issues = append(check.Issues, "lineage recorder not initialized")
	} else {
		result := lineageRec.Verify(ctx)
		if result.Valid {
			check.LineageComplete = true
		} else {
			check.Issues = append(check.Issues, fmt.Sprintf("lineage integrity check failed at %s: %v", result.BrokenAt, result.Error))
		}
	}

	// Step 3+4: Verify the CBDC ledger actually holds the settlement entry, and
	// that its amount matches the order. These are REAL queries against the
	// ledger — not assumptions.
	switch {
	case ledger == nil:
		check.Issues = append(check.Issues, "CBDC ledger not initialized")
	case settlementLedgerID == "":
		check.Issues = append(check.Issues, "settlement ledger id not provided; cannot verify ledger commit")
	default:
		entry, found, lErr := ledger.GetTransactionByPaymentID(settlementLedgerID)
		switch {
		case lErr != nil:
			check.Issues = append(check.Issues, fmt.Sprintf("ledger query failed for %s: %v", settlementLedgerID, lErr))
		case !found || entry == nil:
			check.Issues = append(check.Issues, fmt.Sprintf("no ledger entry for settlement %s (order not settled)", settlementLedgerID))
		default:
			// The ledger rejects duplicate payment IDs at commit time
			// (double-spend prevention), so a single found, non-rejected entry
			// for this settlement ID confirms it settled exactly once.
			if entry.Status == cbdc.StatusRejected {
				check.Issues = append(check.Issues, fmt.Sprintf("ledger entry %s is rejected", settlementLedgerID))
			} else {
				check.NoDuplicateSpend = true
			}

			if entry.AmountPaise == order.TotalPaise {
				check.AmountMatched = true
			} else {
				check.Issues = append(check.Issues, fmt.Sprintf(
					"ledger amount mismatch: order=%d paise, ledger=%d paise", order.TotalPaise, entry.AmountPaise))
			}
		}
	}

	// Step 5: Verify settlement status
	switch order.Status {
	case StatusFulfilled:
		check.SettlementStatus = "settled"
	case StatusPaymentFailed:
		check.SettlementStatus = "failed"
	case StatusComplianceBlocked:
		check.SettlementStatus = "blocked"
		check.Issues = append(check.Issues, "order was blocked by compliance and never settled")
	default:
		check.SettlementStatus = string(order.Status)
		check.Issues = append(check.Issues, fmt.Sprintf("unexpected order status: %v", order.Status))
	}

	return check, nil
}

// IsReconciled returns true if the check has no issues.
func (r *ReconciliationCheck) IsReconciled() bool {
	return len(r.Issues) == 0 && r.AmountMatched && r.LineageComplete && r.NoDuplicateSpend
}
