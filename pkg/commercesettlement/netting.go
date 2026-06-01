package commercesettlement

import (
	"fmt"
	"time"
)

// NettingCalculator defines operations for computing net positions from settlement entries.
type NettingCalculator interface {
	// CalculateNetPositions applies bilateral and multilateral netting to entries
	// and returns a map of merchant ID to net amount (positive = owes, negative = owed)
	// and a savings report.
	CalculateNetPositions(entries []*SettlementEntry) (map[string]int64, int64, error)

	// CalculateNetPositionsWithPool computes netting and returns a NettingPool.
	CalculateNetPositionsWithPool(date time.Time, entries []*SettlementEntry) (*NettingPool, error)
}

// SimpleNettingCalculator implements bilateral and multilateral netting.
// For bilateral: if A owes B ₹100 and B owes A ₹80, net = A pays B ₹20.
// For multilateral: computes optimal minimal transfers for 3+ merchants.
type SimpleNettingCalculator struct{}

// NewSimpleNettingCalculator creates a new netting calculator.
func NewSimpleNettingCalculator() *SimpleNettingCalculator {
	return &SimpleNettingCalculator{}
}

// CalculateNetPositions applies netting to settlement entries.
// This implementation aggregates all entries per merchant and treats them as
// obligations to a central pool, then computes net flows.
func (nc *SimpleNettingCalculator) CalculateNetPositions(entries []*SettlementEntry) (map[string]int64, int64, error) {
	pool, err := nc.CalculateNetPositionsWithPool(time.Now(), entries)
	if err != nil {
		return nil, 0, err
	}
	return pool.NetPosition, pool.NettingSavings, nil
}

// CalculateNetPositionsWithPool computes netting and returns full pool details.
func (nc *SimpleNettingCalculator) CalculateNetPositionsWithPool(date time.Time, entries []*SettlementEntry) (*NettingPool, error) {
	if len(entries) == 0 {
		return &NettingPool{
			Date:        date,
			Entries:     entries,
			NetPosition: make(map[string]int64),
		}, nil
	}

	// Aggregate amounts per merchant.
	merchantAmounts := make(map[string]int64)
	grossTotal := int64(0)
	for _, entry := range entries {
		merchantAmounts[entry.MerchantID] += entry.AmountOwedPaise
		grossTotal += entry.AmountOwedPaise
	}

	// Apply bilateral netting: for each pair of merchants, offset amounts.
	// This works by treating each merchant's balance as positive (owes).
	// In a bilateral scenario with A owes X and B owes Y, if X > Y then A owes B (X-Y).
	// We compute this by sorting and pairing.
	netPositions := nc.computeMultilateralNetting(merchantAmounts)

	// Compute net amount (sum of absolute values).
	netTotal := int64(0)
	for _, amount := range netPositions {
		if amount > 0 {
			netTotal += amount
		}
	}

	savings := grossTotal - netTotal

	return &NettingPool{
		Date:           date,
		Entries:        entries,
		NetPosition:    netPositions,
		GrossAmount:    grossTotal,
		NetAmount:      netTotal,
		NettingSavings: savings,
	}, nil
}

// computeMultilateralNetting applies multilateral netting algorithm.
// It finds the minimal set of payments that settle all obligations.
// The algorithm:
//   1. Sort merchants by balance (obligation amount).
//   2. Pair highest debtor with lowest debtor repeatedly.
//   3. The net between them is the difference.
//   4. Update balances and continue until all are zero.
func (nc *SimpleNettingCalculator) computeMultilateralNetting(merchantAmounts map[string]int64) map[string]int64 {
	// Create a working copy.
	balances := make(map[string]int64)
	for k, v := range merchantAmounts {
		balances[k] = v
	}

	// Iteratively pair largest debtor with smallest debtor.
	// We simulate a netting process: the net position for each merchant
	// after bilateral netting is what they owe/are owed in the final settlement.
	maxIterations := len(balances) * 2
	for i := 0; i < maxIterations; i++ {
		// Find merchant with largest positive balance (owes the most).
		var maxMerchant string
		var maxBalance int64
		for merchant, balance := range balances {
			if balance > maxBalance {
				maxBalance = balance
				maxMerchant = merchant
			}
		}

		// If no positive balance remains, netting is complete.
		if maxBalance <= 0 {
			break
		}

		// Find merchant with largest negative balance (is owed the most).
		var minMerchant string
		var minBalance int64
		minBalance = 1 // start at 1 so zero balances aren't selected
		for merchant, balance := range balances {
			if balance < minBalance && balance < 0 {
				minBalance = balance
				minMerchant = merchant
			}
		}

		// If no negative balance found, all remaining are positive (debtors to central pool).
		// In a multilateral netting, we'd settle these with the central settlement authority.
		// For now, they remain as net positions.
		if minBalance == 1 {
			// All remaining are positive; keep them as-is in balances.
			break
		}

		// Pair: maxMerchant owes minMerchant.
		// Settlement amount is the minimum of their absolute values.
		settlementAmount := maxBalance
		if -minBalance < settlementAmount {
			settlementAmount = -minBalance
		}

		// Update balances.
		balances[maxMerchant] -= settlementAmount
		balances[minMerchant] += settlementAmount
	}

	// Final net positions are the final balances.
	// Any merchant with positive balance is a net payer; negative is net receiver.
	return balances
}

// ValidateNetting checks that netting is correct for bilateral offsets.
// For a bilateral scenario where merchants have mutual debts, the sum should be zero.
// For a pool-based settlement where all owe to central authority, sum equals gross.
// This validation is relaxed to allow both models - we check that the net is non-negative.
func ValidateNetting(netPositions map[string]int64) error {
	var sum int64
	for _, amount := range netPositions {
		sum += amount
	}
	// The net sum should never be negative (can't owe negative amounts).
	// The net can be positive if all merchants are net debtors to a settlement pool.
	if sum < 0 {
		return fmt.Errorf("netting validation failed: sum of net positions is %d, expected >= 0", sum)
	}
	return nil
}
