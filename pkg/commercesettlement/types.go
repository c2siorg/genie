// Package commercesettlement implements batch settlement, netting, and
// reconciliation for e-Rupee commerce flows. It aggregates merchant receivables,
// applies bilateral and multilateral netting to reduce settlement volume, and
// executes settlement payments via the Payment Agent + CBDC Bridge.
//
// Typical flow:
//   1. CreateBatch: aggregate merchant receivables for settlement_date
//   2. CalculateNetPositions: bilateral/multilateral netting
//   3. ExecuteBatch: create e-Rupee payments, emit settlement_txn_ids
//   4. QueryByDate: retrieve historical settlement batches
package commercesettlement

import (
	"time"
)

// SettlementStatus represents the current state of a settlement batch.
type SettlementStatus string

const (
	StatusPending  SettlementStatus = "pending"
	StatusNetted   SettlementStatus = "netted"
	StatusSettled  SettlementStatus = "settled"
	StatusFailed   SettlementStatus = "failed"
)

// SettlementBatch represents a batch of settlements scheduled for a given date.
// All amounts are in paise (smallest unit of e-Rupee).
type SettlementBatch struct {
	ID                string                       `json:"id"`
	SettlementDate    time.Time                    `json:"settlement_date"`
	MerchantIDs       []string                     `json:"merchant_ids"`
	TotalAmountPaise  int64                        `json:"total_amount_paise"`
	Status            SettlementStatus             `json:"status"`
	CreatedAt         time.Time                    `json:"created_at"`
	SettledAt         *time.Time                   `json:"settled_at,omitempty"`
	Entries           map[string]*SettlementEntry  `json:"entries"`           // keyed by merchant_id
	NettedPositions   map[string]int64             `json:"netted_positions"`  // net position per merchant (after netting)
	NettingSavings    int64                        `json:"netting_savings_paise"`
	SettlementTxnIDs  []string                     `json:"settlement_txn_ids"`
}

// SettlementEntry represents one merchant's settlement obligation in a batch.
type SettlementEntry struct {
	MerchantID       string `json:"merchant_id"`
	AmountOwedPaise  int64  `json:"amount_owed_paise"` // gross amount owed (before netting)
	SettlementTxnID  string `json:"settlement_txn_id"`  // CBDC transaction ID (populated after settlement)
}

// NettingPool holds entries and computes bilateral/multilateral net positions.
type NettingPool struct {
	Date           time.Time                `json:"date"`
	Entries        []*SettlementEntry       `json:"entries"`
	NetPosition    map[string]int64         `json:"net_position"`  // merchant -> net amount (positive = owes, negative = owed)
	GrossAmount    int64                    `json:"gross_amount"`  // sum of all entries before netting
	NetAmount      int64                    `json:"net_amount"`    // sum of absolute net positions (after netting)
	NettingSavings int64                    `json:"netting_savings_paise"` // gross - net
}

// SettlementReport summarizes a completed settlement batch.
type SettlementReport struct {
	BatchID              string    `json:"batch_id"`
	TotalEntries         int       `json:"total_entries"`
	GrossAmountPaise     int64     `json:"gross_amount_paise"`
	NetAmountPaise       int64     `json:"net_amount_paise"`
	NettingSavingsPaise  int64     `json:"netting_savings_paise"`
	SettledAt            time.Time `json:"settled_at"`
	SettlementTxnCount   int       `json:"settlement_txn_count"`
}

// New creates a new SettlementBatch with generated ID.
func NewSettlementBatch(settlementDate time.Time, merchantIDs []string) *SettlementBatch {
	return &SettlementBatch{
		ID:              generateBatchID(),
		SettlementDate:  settlementDate,
		MerchantIDs:     merchantIDs,
		Status:          StatusPending,
		CreatedAt:       time.Now(),
		Entries:         make(map[string]*SettlementEntry),
		NettedPositions: make(map[string]int64),
	}
}

// AddEntry adds or updates a settlement entry for a merchant.
func (sb *SettlementBatch) AddEntry(merchantID string, amountPaise int64) {
	sb.Entries[merchantID] = &SettlementEntry{
		MerchantID:      merchantID,
		AmountOwedPaise: amountPaise,
	}
	sb.computeTotalAmount()
}

// computeTotalAmount recalculates total from current entries.
func (sb *SettlementBatch) computeTotalAmount() {
	total := int64(0)
	for _, entry := range sb.Entries {
		total += entry.AmountOwedPaise
	}
	sb.TotalAmountPaise = total
}

// generateBatchID creates a settlement batch ID with timestamp and counter.
var batchCounter int64

func generateBatchID() string {
	batchCounter++
	return "batch-" + time.Now().Format("20060102150405") + "-" + randomSuffix(8)
}

// randomSuffix generates a random alphanumeric suffix using simple seed.
func randomSuffix(length int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	seed := time.Now().UnixNano() + batchCounter
	for i := 0; i < length; i++ {
		seed = (seed*1103515245 + 12345) & 0x7fffffff // Simple LCG
		b[i] = chars[seed%int64(len(chars))]
	}
	return string(b)
}
