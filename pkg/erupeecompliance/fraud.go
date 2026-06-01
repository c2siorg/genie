package erupeecompliance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FraudDetector defines the interface for fraud pattern detection.
type FraudDetector interface {
	// DetectPatterns analyzes transaction history and detects fraud patterns.
	DetectPatterns(ctx context.Context, accountID string, history []TransactionRecord) ([]FraudPattern, float64, string)

	// RecordTransaction records a transaction for fraud analysis.
	RecordTransaction(ctx context.Context, txn TransactionRecord) error

	// GetHistory returns the transaction history for an account.
	GetHistory(ctx context.Context, accountID string, lookbackHours int) []TransactionRecord
}

// InMemoryFraudDetector is a thread-safe in-memory fraud detector.
// Production systems would use persistent storage with efficient querying.
type InMemoryFraudDetector struct {
	mu       sync.RWMutex
	config   *FraudDetectionConfig
	history  map[string][]TransactionRecord // accountID -> transactions
	baseline map[string]*baselineMetrics    // accountID -> baseline stats
}

type baselineMetrics struct {
	avgTxnPerHour  float64
	avgTxnAmount   float64
	lastUpdated    time.Time
}

// NewInMemoryFraudDetector creates a new fraud detector with default config.
func NewInMemoryFraudDetector() *InMemoryFraudDetector {
	return NewInMemoryFraudDetectorWithConfig(DefaultFraudDetectionConfig())
}

// NewInMemoryFraudDetectorWithConfig creates a new fraud detector with custom config.
func NewInMemoryFraudDetectorWithConfig(config *FraudDetectionConfig) *InMemoryFraudDetector {
	return &InMemoryFraudDetector{
		config:   config,
		history:  make(map[string][]TransactionRecord),
		baseline: make(map[string]*baselineMetrics),
	}
}

// RecordTransaction adds a transaction to the fraud detector's history.
func (f *InMemoryFraudDetector) RecordTransaction(ctx context.Context, txn TransactionRecord) error {
	if txn.FromAccountID == "" || txn.TransactionID == "" {
		return fmt.Errorf("invalid transaction: missing required fields")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.history[txn.FromAccountID] = append(f.history[txn.FromAccountID], txn)

	// Keep only last 1000 transactions per account to avoid memory bloat
	if len(f.history[txn.FromAccountID]) > 1000 {
		f.history[txn.FromAccountID] = f.history[txn.FromAccountID][len(f.history[txn.FromAccountID])-1000:]
	}

	return nil
}

// DetectPatterns analyzes transaction history and detects fraud patterns.
// Returns (patterns, fraudScore 0-100, reason).
func (f *InMemoryFraudDetector) DetectPatterns(ctx context.Context, accountID string, history []TransactionRecord) ([]FraudPattern, float64, string) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	patterns := []FraudPattern{}
	fraudScore := 0.0
	reasons := []string{}

	if len(history) == 0 {
		return patterns, fraudScore, "No transaction history"
	}

	// Detect structuring pattern
	if f.detectStructuring(history) {
		patterns = append(patterns, PatternStructuring)
		fraudScore += 30.0
		reasons = append(reasons, fmt.Sprintf(
			"Structuring detected: %d transactions just under ₹%d limit",
			f.countStructuredTxns(history),
			f.config.StructuringLimit/100,
		))
	}

	// Detect round-tripping pattern
	if f.detectRoundTripping(history) {
		patterns = append(patterns, PatternRoundTripping)
		fraudScore += 35.0
		reasons = append(reasons, "Round-tripping detected: send-receive cycles within short timeframe")
	}

	// Detect velocity spike
	if f.detectVelocitySpike(accountID, history) {
		patterns = append(patterns, PatternVelocitySpike)
		fraudScore += 25.0
		reasons = append(reasons, fmt.Sprintf(
			"Velocity spike detected: %.0f%% increase in transaction frequency",
			f.config.VelocitySpikeThreshold,
		))
	}

	// Cap fraud score at 100
	if fraudScore > 100.0 {
		fraudScore = 100.0
	}

	reason := ""
	if len(reasons) > 0 {
		reason = fmt.Sprintf("Fraud risk: %v", reasons)
	} else {
		reason = "No fraud patterns detected"
	}

	return patterns, fraudScore, reason
}

// GetHistory returns transaction history for an account.
func (f *InMemoryFraudDetector) GetHistory(ctx context.Context, accountID string, lookbackHours int) []TransactionRecord {
	f.mu.RLock()
	defer f.mu.RUnlock()

	allTxns := f.history[accountID]
	if len(allTxns) == 0 {
		return []TransactionRecord{}
	}

	// Filter transactions within lookback window
	cutoff := time.Now().UTC().Add(-time.Duration(lookbackHours) * time.Hour)
	result := []TransactionRecord{}

	for _, txn := range allTxns {
		if txn.Timestamp.After(cutoff) {
			result = append(result, txn)
		}
	}

	return result
}

// detectStructuring identifies attempts to avoid transaction limits by making
// multiple smaller transactions (e.g., 5+ txns just under ₹10k in 1 day).
func (f *InMemoryFraudDetector) detectStructuring(history []TransactionRecord) bool {
	if len(history) < f.config.StructuringThreshold {
		return false
	}

	// Count transactions under the limit in the last 24 hours
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	underLimitCount := 0

	for _, txn := range history {
		if txn.Timestamp.After(cutoff) && txn.Amount < f.config.StructuringLimit {
			underLimitCount++
		}
	}

	return underLimitCount >= f.config.StructuringThreshold
}

// countStructuredTxns returns the number of sub-limit transactions in last 24h.
func (f *InMemoryFraudDetector) countStructuredTxns(history []TransactionRecord) int {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	count := 0

	for _, txn := range history {
		if txn.Timestamp.After(cutoff) && txn.Amount < f.config.StructuringLimit {
			count++
		}
	}

	return count
}

// detectRoundTripping identifies send→receive cycles within a short timeframe.
// Looks for: account A sends to B, then receives from B shortly after (repeat pattern).
func (f *InMemoryFraudDetector) detectRoundTripping(history []TransactionRecord) bool {
	if len(history) < 4 { // Need at least 2 send-receive pairs
		return false
	}

	window := time.Duration(f.config.RoundTripWindowSeconds) * time.Second
	roundTripCount := 0

	for i, txn := range history {
		if txn.FromAccountID == "" {
			continue
		}

		// Look for a matching receive transaction within window
		for j := i + 1; j < len(history); j++ {
			other := history[j]
			// Check if other is a receive from the same account we just sent to
			if other.ToAccountID == txn.FromAccountID && other.FromAccountID == txn.ToAccountID {
				if other.Timestamp.Sub(txn.Timestamp) <= window {
					roundTripCount++
					break
				}
			}
		}
	}

	return roundTripCount >= f.config.RoundTripThreshold
}

// detectVelocitySpike identifies unusual increases in transaction frequency.
// Compares recent frequency to baseline (100%+ increase = spike).
func (f *InMemoryFraudDetector) detectVelocitySpike(accountID string, history []TransactionRecord) bool {
	if len(history) < 4 {
		return false
	}

	// Calculate recent transaction rate (last 2 hours)
	recentWindow := 2 * time.Hour
	cutoffRecent := time.Now().UTC().Add(-recentWindow)
	recentCount := 0.0

	for _, txn := range history {
		if txn.Timestamp.After(cutoffRecent) {
			recentCount++
		}
	}

	recentRate := recentCount / recentWindow.Hours()

	// Get or calculate baseline (previous 7 days excluding recent 2 hours)
	baseline := f.getOrComputeBaseline(accountID, history)

	// Check if recent rate is significantly higher than baseline
	if baseline > 0 {
		percentIncrease := ((recentRate - baseline) / baseline) * 100.0
		return percentIncrease >= f.config.VelocitySpikeThreshold
	}

	return false
}

// getOrComputeBaseline computes transaction frequency from historical data.
func (f *InMemoryFraudDetector) getOrComputeBaseline(accountID string, history []TransactionRecord) float64 {
	// Use last 7 days (excluding the most recent 2 hours) as baseline
	baselineWindow := 7 * 24 * time.Hour
	recentWindow := 2 * time.Hour
	baselineCutoff := time.Now().UTC().Add(-baselineWindow)
	recentCutoff := time.Now().UTC().Add(-recentWindow)

	count := 0.0
	for _, txn := range history {
		if txn.Timestamp.After(baselineCutoff) && txn.Timestamp.Before(recentCutoff) {
			count++
		}
	}

	if count == 0 {
		return 0
	}

	baselineHours := (baselineWindow - recentWindow).Hours()
	return count / baselineHours
}

// CheckNewAccountHighValue checks if a payment is from a new account with high value.
// Returns (isNewAccountHighValue, score, reason).
func CheckNewAccountHighValue(payment PaymentRequest, config *FraudDetectionConfig) (bool, float64, string) {
	accountAgeSeconds := payment.AccountAgeSeconds
	if accountAgeSeconds == 0 {
		return false, 0, "Account age unknown"
	}

	// Check if account is "new"
	if accountAgeSeconds < config.NewAccountAgeSeconds {
		// Check if payment is high value
		if payment.Amount > config.NewAccountHighValue {
			score := 40.0
			reason := fmt.Sprintf(
				"New account (age %d seconds) with high-value payment (%.2f INR)",
				accountAgeSeconds,
				float64(payment.Amount)/100.0,
			)
			return true, score, reason
		}
	}

	return false, 0, "Account age or amount threshold not met"
}
