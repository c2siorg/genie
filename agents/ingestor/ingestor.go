// Package ingestor turns a raw CSV upload into a typed batch of transactions
// addressed to the normalizer. It is the first specialist in the Genie
// financial pipeline.
package ingestor

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID            = "ingestor"
	CapIngestCSV  = "ingest_csv"
	NextAgent     = "normalizer"
	TypeRawCSV    = "ingest_csv"
	TypeOutBatch  = "raw_transactions"
)

// Agent parses CSV content from Message.Content and emits a single batch
// message containing the raw rows for the normalizer to canonicalize.
type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "CSV Ingestor" }
func (a *Agent) Capabilities() []string { return []string{CapIngestCSV} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeRawCSV {
		return nil, nil
	}

	txns, err := parseCSV(msg.Content)
	if err != nil {
		return nil, err
	}

	env.Logf("[ingestor] parsed %d rows", len(txns))

	payload, err := finance.MarshalTransactions(txns)
	if err != nil {
		return nil, err
	}
	out := agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOutBatch, payload, msg.Metadata)
	return []agent.Message{out}, nil
}

// parseCSV converts the demo CSV format (date,description,category,amount,type)
// into raw Transaction values. Currency is left blank for the normalizer to
// fill from account metadata.
func parseCSV(content string) ([]finance.Transaction, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("empty csv content")
	}
	r := csv.NewReader(strings.NewReader(content))
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv parse: %w", err)
	}
	if len(rows) < 2 {
		return nil, errors.New("csv must have header + at least one row")
	}

	header := rows[0]
	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	require := func(name string) (int, error) {
		i, ok := colIdx[name]
		if !ok {
			return 0, fmt.Errorf("missing column %q", name)
		}
		return i, nil
	}
	dateCol, err := require("date")
	if err != nil {
		return nil, err
	}
	descCol, err := require("description")
	if err != nil {
		return nil, err
	}
	amountCol, err := require("amount")
	if err != nil {
		return nil, err
	}
	typeCol, err := require("type")
	if err != nil {
		return nil, err
	}
	catCol, hasCat := colIdx["category"]

	out := make([]finance.Transaction, 0, len(rows)-1)
	for i, row := range rows[1:] {
		if len(row) < len(header) {
			return nil, fmt.Errorf("row %d has %d cols, expected %d", i+2, len(row), len(header))
		}
		amtMajor, err := strconv.ParseFloat(strings.TrimSpace(row[amountCol]), 64)
		if err != nil {
			return nil, fmt.Errorf("row %d amount: %w", i+2, err)
		}
		dir := finance.Direction(strings.ToLower(strings.TrimSpace(row[typeCol])))
		cents := int64(amtMajor * 100)
		if dir == finance.DirectionDebit && cents > 0 {
			cents = -cents
		}
		t := finance.Transaction{
			TransactionID: fmt.Sprintf("txn-%04d", i+1),
			Date:          strings.TrimSpace(row[dateCol]),
			Description:   strings.TrimSpace(row[descCol]),
			AmountCents:   cents,
			Direction:     dir,
		}
		if hasCat {
			t.Category = strings.TrimSpace(row[catCol])
		}
		out = append(out, t)
	}
	return out, nil
}
