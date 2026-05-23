// Package finance holds the canonical data types and helpers shared by Genie
// financial specialist agents. Keeping these here (instead of inside any one
// agent package) avoids the temptation for agents to import each other.
package finance

import (
	"encoding/json"
	"strings"
	"time"
)

// Direction represents whether a transaction is money in or money out.
type Direction string

const (
	DirectionCredit Direction = "credit"
	DirectionDebit  Direction = "debit"
)

// Transaction is the canonical normalized form used by every downstream agent.
//
// Amount is stored as integer minor units (e.g. paise/cents) to avoid float
// rounding errors. Sign convention: credits are positive, debits are negative.
type Transaction struct {
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	Date          string    `json:"date"` // ISO-8601 YYYY-MM-DD
	AmountCents   int64     `json:"amount_cents"`
	Currency      string    `json:"currency"`
	Description   string    `json:"description"`
	Merchant      string    `json:"merchant,omitempty"`
	Category      string    `json:"category,omitempty"`
	Direction     Direction `json:"direction,omitempty"`
}

// ParsedDate parses Transaction.Date as YYYY-MM-DD.
func (t Transaction) ParsedDate() (time.Time, error) {
	return time.Parse("2006-01-02", t.Date)
}

// MarshalTransactions returns the JSON payload (an object with a transactions
// array) used as Message.Content for batches.
func MarshalTransactions(txns []Transaction) (string, error) {
	b, err := json.Marshal(struct {
		Transactions []Transaction `json:"transactions"`
	}{Transactions: txns})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// UnmarshalTransactions parses the JSON payload back into a slice.
func UnmarshalTransactions(content string) ([]Transaction, error) {
	var w struct {
		Transactions []Transaction `json:"transactions"`
	}
	if err := json.Unmarshal([]byte(content), &w); err != nil {
		return nil, err
	}
	return w.Transactions, nil
}

// NormalizeMerchant strips punctuation and lowercases the merchant slug so
// "Swiggy Order #4823" and "SWIGGY*ORDER" both become "swiggy".
func NormalizeMerchant(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	// Keep the first whitespace/punctuation-separated token.
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '*' || r == '#' || r == '/' || r == '_' || r == '-' {
			return s[:i]
		}
	}
	return s
}
