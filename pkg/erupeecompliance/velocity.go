package erupeecompliance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// VelocityMonitor defines the interface for transaction velocity checking.
type VelocityMonitor interface {
	// RecordTransaction records a transaction for velocity tracking.
	RecordTransaction(ctx context.Context, accountID string, amount int64, timestamp time.Time) error

	// CheckVelocity checks if an account has exceeded velocity limits.
	// Returns (allowed bool, reason string).
	CheckVelocity(ctx context.Context, accountID string) (bool, string)

	// CheckAndRecord atomically checks velocity limits and, only if allowed,
	// records the transaction. This is the TOCTOU-safe primitive that compliance
	// gates must use — separate CheckVelocity + RecordTransaction calls have a
	// window where two concurrent payments can both slip past a limit.
	CheckAndRecord(ctx context.Context, accountID string, amount int64, now time.Time) (allowed bool, reason string)

	// GetRecord returns the current velocity record for an account.
	GetRecord(ctx context.Context, accountID string) (*VelocityRecord, error)

	// Reset clears velocity records for an account (admin operation).
	Reset(ctx context.Context, accountID string) error
}

// InMemoryVelocityMonitor is a thread-safe in-memory velocity monitor.
// Production systems would use Redis or similar.
type InMemoryVelocityMonitor struct {
	mu     sync.RWMutex
	config *VelocityConfig
	hourly map[string]*VelocityRecord // hourly records
	daily  map[string]*VelocityRecord // daily records
}

// NewInMemoryVelocityMonitor creates a new velocity monitor with default config.
func NewInMemoryVelocityMonitor() *InMemoryVelocityMonitor {
	return NewInMemoryVelocityMonitorWithConfig(DefaultVelocityConfig())
}

// NewInMemoryVelocityMonitorWithConfig creates a new velocity monitor with custom config.
func NewInMemoryVelocityMonitorWithConfig(config *VelocityConfig) *InMemoryVelocityMonitor {
	return &InMemoryVelocityMonitor{
		config: config,
		hourly: make(map[string]*VelocityRecord),
		daily:  make(map[string]*VelocityRecord),
	}
}

// RecordTransaction records a transaction for an account.
func (v *InMemoryVelocityMonitor) RecordTransaction(ctx context.Context, accountID string, amount int64, timestamp time.Time) error {
	if accountID == "" {
		return fmt.Errorf("account ID cannot be empty")
	}
	if amount < 0 {
		return fmt.Errorf("amount cannot be negative")
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// Update hourly record
	v.updateRecord(v.hourly, accountID, amount, timestamp, "hourly")

	// Update daily record
	v.updateRecord(v.daily, accountID, amount, timestamp, "daily")

	return nil
}

// updateRecord updates a velocity record, resetting if period has passed.
func (v *InMemoryVelocityMonitor) updateRecord(records map[string]*VelocityRecord, accountID string, amount int64, timestamp time.Time, period string) {
	record, exists := records[accountID]
	if !exists {
		// Create new record
		record = &VelocityRecord{
			AccountID:        accountID,
			TransactionCount: 1,
			TotalAmount:      amount,
			Period:           period,
			LastReset:        timestamp,
			UpdatedAt:        timestamp,
		}
		records[accountID] = record
		return
	}

	// Check if record needs to be reset based on period
	var resetTime time.Duration

	switch period {
	case "hourly":
		resetTime = 1 * time.Hour
	case "daily":
		resetTime = 24 * time.Hour
	}

	if timestamp.Sub(record.LastReset) > resetTime {
		// Reset the record
		record.TransactionCount = 1
		record.TotalAmount = amount
		record.LastReset = timestamp
	} else {
		// Update existing record
		record.TransactionCount++
		record.TotalAmount += amount
	}

	record.UpdatedAt = timestamp
}

// CheckVelocity checks if an account has exceeded velocity limits.
func (v *InMemoryVelocityMonitor) CheckVelocity(ctx context.Context, accountID string) (bool, string) {
	v.mu.RLock()
	hourlyRecord, hourlyExists := v.hourly[accountID]
	dailyRecord, dailyExists := v.daily[accountID]
	v.mu.RUnlock()

	// Check hourly transaction count
	if hourlyExists {
		if hourlyRecord.TransactionCount > v.config.MaxTransactionsPerHour {
			return false, fmt.Sprintf(
				"Hourly transaction limit exceeded: %d > %d",
				hourlyRecord.TransactionCount,
				v.config.MaxTransactionsPerHour,
			)
		}

		// Check hourly amount limit
		if hourlyRecord.TotalAmount > v.config.MaxAmountPerHour {
			return false, fmt.Sprintf(
				"Hourly amount limit exceeded: %.2f INR > %.2f INR",
				float64(hourlyRecord.TotalAmount)/100.0,
				float64(v.config.MaxAmountPerHour)/100.0,
			)
		}
	}

	// Check daily amount limit
	if dailyExists {
		if dailyRecord.TotalAmount > v.config.MaxAmountPerDay {
			return false, fmt.Sprintf(
				"Daily amount limit exceeded: %.2f INR > %.2f INR",
				float64(dailyRecord.TotalAmount)/100.0,
				float64(v.config.MaxAmountPerDay)/100.0,
			)
		}
	}

	return true, "Within velocity limits"
}

// GetRecord returns the current velocity record for an account.
func (v *InMemoryVelocityMonitor) GetRecord(ctx context.Context, accountID string) (*VelocityRecord, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Return hourly record as the primary metric
	if record, exists := v.hourly[accountID]; exists {
		return record, nil
	}

	return nil, fmt.Errorf("no velocity record found for account %s", accountID)
}

// Reset clears velocity records for an account.
func (v *InMemoryVelocityMonitor) Reset(ctx context.Context, accountID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	delete(v.hourly, accountID)
	delete(v.daily, accountID)

	return nil
}

// GetRecordSafe returns the velocity record with defaults if not found.
func (v *InMemoryVelocityMonitor) GetRecordSafe(ctx context.Context, accountID string) *VelocityRecord {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if record, exists := v.hourly[accountID]; exists {
		return record
	}

	// Return empty record with current timestamp
	return &VelocityRecord{
		AccountID:        accountID,
		TransactionCount: 0,
		TotalAmount:      0,
		Period:           "hourly",
		LastReset:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
}

// WouldExceed reports whether recording `amount` for accountID at time `now`
// WOULD breach the hourly amount limit, the daily amount limit, or the hourly
// transaction-count limit — without mutating any state.
//
// This is the PROSPECTIVE check a pre-authorization gate needs. CheckVelocity
// only evaluates activity that has ALREADY been recorded, so a fresh account's
// first oversized payment slips past it; WouldExceed projects the limits as if
// the payment had been recorded, so a single large payment is rejected before
// it is committed.
func (v *InMemoryVelocityMonitor) WouldExceed(ctx context.Context, accountID string, amount int64, now time.Time) (bool, string) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.wouldExceedLocked(accountID, amount, now)
}

// wouldExceedLocked is the lock-free core of WouldExceed. Callers must hold at
// least a read lock.
func (v *InMemoryVelocityMonitor) wouldExceedLocked(accountID string, amount int64, now time.Time) (bool, string) {
	hAmount, hCount := projectVelocity(v.hourly[accountID], amount, now, time.Hour)
	if hCount > v.config.MaxTransactionsPerHour {
		return true, fmt.Sprintf("Hourly transaction limit exceeded: %d > %d", hCount, v.config.MaxTransactionsPerHour)
	}
	if hAmount > v.config.MaxAmountPerHour {
		return true, fmt.Sprintf("Hourly amount limit exceeded: %.2f INR > %.2f INR",
			float64(hAmount)/100.0, float64(v.config.MaxAmountPerHour)/100.0)
	}

	dAmount, _ := projectVelocity(v.daily[accountID], amount, now, 24*time.Hour)
	if dAmount > v.config.MaxAmountPerDay {
		return true, fmt.Sprintf("Daily amount limit exceeded: %.2f INR > %.2f INR",
			float64(dAmount)/100.0, float64(v.config.MaxAmountPerDay)/100.0)
	}

	return false, "Within velocity limits"
}

// CheckAndRecord atomically evaluates whether `amount` for accountID would
// breach velocity limits at time `now` and, ONLY if it would not, records it.
// Returns (allowed, reason).
//
// Check-and-record happen under a single write lock, so concurrent callers
// cannot both slip past a limit (there is no TOCTOU window between a separate
// WouldExceed and RecordTransaction). This is the gate-safe primitive a
// pre-payment compliance control should use.
func (v *InMemoryVelocityMonitor) CheckAndRecord(ctx context.Context, accountID string, amount int64, now time.Time) (bool, string) {
	if accountID == "" {
		return false, "account ID cannot be empty"
	}
	// A pre-payment control must reject non-positive amounts: a zero-value
	// "transaction" carries no money yet still consumes the per-hour count
	// budget, so it must not be silently recorded.
	if amount <= 0 {
		return false, "amount must be positive"
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if exceeded, reason := v.wouldExceedLocked(accountID, amount, now); exceeded {
		return false, reason
	}

	// Allowed → record under the same lock so the next caller sees this txn.
	v.updateRecord(v.hourly, accountID, amount, now, "hourly")
	v.updateRecord(v.daily, accountID, amount, now, "daily")
	return true, "Within velocity limits"
}

// projectVelocity returns the (amount, count) an account would have in the
// given window if `amount` were recorded at `now`, honoring window resets. A
// nil record (no prior activity) or an expired window both mean this would be
// the first transaction in a fresh window.
func projectVelocity(rec *VelocityRecord, amount int64, now time.Time, window time.Duration) (int64, int) {
	if rec == nil || now.Sub(rec.LastReset) > window {
		return amount, 1
	}
	return rec.TotalAmount + amount, rec.TransactionCount + 1
}
