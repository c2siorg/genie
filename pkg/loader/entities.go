package loader

import (
	"regexp"
	"strings"
)

// Entity is a recognised span — merchant, amount, currency, date, etc.
type Entity struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

var (
	rxAmount   = regexp.MustCompile(`(?i)(?:₹|inr|usd|eur|\$)\s?\d+(?:[.,]\d+)?(?:\s?(?:l|lakh|cr|crore|k))?`)
	rxDate     = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
	rxAccount  = regexp.MustCompile(`\b\d{12,18}\b`)
	rxMerchant = regexp.MustCompile(`(?i)\b(swiggy|zomato|amazon|flipkart|uber|ola|netflix|spotify|jio|airtel|bigbasket|blinkit|zerodha|groww)\b`)
	rxCurrency = regexp.MustCompile(`(?i)\b(INR|USD|EUR|GBP|JPY|SGD)\b`)
)

// ExtractEntities pulls a small set of finance-relevant entities from text.
// Heuristic only — for production swap in a proper NER model.
func ExtractEntities(text string) []Entity {
	var out []Entity
	for _, m := range rxAmount.FindAllStringIndex(text, -1) {
		out = append(out, Entity{Type: "amount", Value: strings.TrimSpace(text[m[0]:m[1]]), Start: m[0], End: m[1]})
	}
	for _, m := range rxDate.FindAllStringIndex(text, -1) {
		out = append(out, Entity{Type: "date", Value: text[m[0]:m[1]], Start: m[0], End: m[1]})
	}
	for _, m := range rxAccount.FindAllStringIndex(text, -1) {
		out = append(out, Entity{Type: "account", Value: text[m[0]:m[1]], Start: m[0], End: m[1]})
	}
	for _, m := range rxMerchant.FindAllStringIndex(text, -1) {
		out = append(out, Entity{Type: "merchant", Value: strings.ToLower(text[m[0]:m[1]]), Start: m[0], End: m[1]})
	}
	for _, m := range rxCurrency.FindAllStringIndex(text, -1) {
		out = append(out, Entity{Type: "currency", Value: strings.ToUpper(text[m[0]:m[1]]), Start: m[0], End: m[1]})
	}
	return out
}
