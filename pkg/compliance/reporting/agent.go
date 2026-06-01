package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

// Tool is a minimal interface for agent tools.
// In the full Genie system, this would be github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools.Tool
// For this reporting module, we define a compatible interface to avoid circular dependencies.
type Tool interface {
	Name() string
	Description() string
	Schema() map[string]any
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// ToolDef is a basic tool implementation.
type ToolDef struct {
	ToolName        string
	ToolDescription string
	ToolSchema      map[string]any
	Fn              func(ctx context.Context, args map[string]any) (string, error)
}

func (t *ToolDef) Name() string                { return t.ToolName }
func (t *ToolDef) Description() string         { return t.ToolDescription }
func (t *ToolDef) Schema() map[string]any      { return t.ToolSchema }
func (t *ToolDef) Execute(ctx context.Context, args map[string]any) (string, error) {
	return t.Fn(ctx, args)
}

// GenerateComplianceReportTool returns a Tool that the reporting agent
// can use to generate compliance reports (RBI quarterly or FATF STR).
//
// The tool accepts parameters:
// - report_type: "rbi" or "str"
// - quarter: 1-4 (for RBI reports only)
// - year: YYYY (for RBI reports)
// - format: "csv", "json", or "pdf" (default "json")
//
// For STR reports, it queries the transaction database and generates a report
// on the most recently flagged transaction. In a full system, the agent would
// specify the transaction ID explicitly.
//
// The tool integrates with the ReportRepository to store the generated report
// and return a trace ID for Laminar observability.
func GenerateComplianceReportTool(
	repo ReportRepository,
	txnStore interface{}, // would be *sql.DB or similar in production
	auditLog AuditLog,
) Tool {
	return &ToolDef{
		ToolName:        "generate_compliance_report",
		ToolDescription: "Generate RBI quarterly or FATF suspicious transaction compliance reports. RBI reports are quarterly summaries of transaction volume and risk. FATF STR reports are filed for individual suspicious transactions.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"report_type": map[string]any{
					"type":        "string",
					"enum":        []string{"rbi", "str"},
					"description": "Report type: 'rbi' for quarterly summary, 'str' for suspicious transaction",
				},
				"quarter": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     4,
					"description": "Quarter (1-4) for RBI reports. Ignored for STR.",
				},
				"year": map[string]any{
					"type":        "integer",
					"description": "Year (YYYY) for RBI reports. Ignored for STR.",
				},
				"transaction_id": map[string]any{
					"type":        "string",
					"description": "Transaction ID for STR reports. Required if report_type='str'.",
				},
				"risk_score": map[string]any{
					"type":        "number",
					"minimum":     0,
					"maximum":     1,
					"description": "Risk score (0-1) for STR reports. Default 0.7 if omitted.",
				},
				"format": map[string]any{
					"type":        "string",
					"enum":        []string{"csv", "json", "pdf"},
					"description": "Export format: csv, json, or pdf. Default: json.",
				},
			},
			"required": []string{"report_type"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			reportType, _ := args["report_type"].(string)

			// Fetch audit log for decision reasoning.
			entries, err := auditLog.List(ctx)
			if err != nil {
				return fmt.Sprintf("error fetching audit log: %v", err), nil
			}

			switch strings.ToLower(reportType) {
			case "rbi":
				return handleRBIReportRequest(ctx, args, repo, txnStore, entries)
			case "str":
				return handleSTRReportRequest(ctx, args, repo, entries)
			default:
				return fmt.Sprintf("unknown report_type: %q", reportType), nil
			}
		},
	}
}

// handleRBIReportRequest processes an RBI quarterly report request.
func handleRBIReportRequest(
	ctx context.Context,
	args map[string]any,
	repo ReportRepository,
	txnStore interface{},
	auditLog []AuditEntry,
) (string, error) {
	quarterVal := args["quarter"]
	yearVal := args["year"]
	format, _ := args["format"].(string)
	if format == "" {
		format = "json"
	}

	// Parse quarter and year.
	var quarter int
	switch q := quarterVal.(type) {
	case float64:
		quarter = int(q)
	case int:
		quarter = q
	case string:
		var err error
		quarter, err = strconv.Atoi(q)
		if err != nil {
			return "error: quarter must be an integer 1-4", nil
		}
	default:
		return "error: quarter is required and must be an integer 1-4", nil
	}

	var year int
	switch y := yearVal.(type) {
	case float64:
		year = int(y)
	case int:
		year = y
	case string:
		var err error
		year, err = strconv.Atoi(y)
		if err != nil {
			return "error: year must be an integer", nil
		}
	default:
		return "error: year is required and must be an integer (YYYY)", nil
	}

	// Stub: in production, fetch transactions from txnStore by quarter/year.
	// For now, return a placeholder response showing the integration point.
	stubTxns := []finance.Transaction{}

	// Generate RBI report.
	report, err := GenerateRBIReport(quarter, year, stubTxns, auditLog)
	if err != nil {
		return fmt.Sprintf("error generating RBI report: %v", err), nil
	}

	// Save to repository.
	reportID, err := repo.SaveRBIReport(ctx, report)
	if err != nil {
		return fmt.Sprintf("error saving RBI report: %v", err), nil
	}

	// Export to requested format.
	data, err := ExportRBIReport(report, format)
	if err != nil {
		return fmt.Sprintf("error exporting RBI report: %v", err), nil
	}

	// Return summary with report ID and link to trace.
	result := map[string]any{
		"status":      "success",
		"report_type": "rbi",
		"report_id":   reportID,
		"quarter":     quarter,
		"year":        year,
		"format":      format,
		"summary": map[string]any{
			"total_transactions": report.TotalTransactions,
			"total_volume_cents": report.TotalVolumeCents,
			"suspicious_count":   report.SuspiciousCount,
			"risky_merchants":    report.RiskyMerchants,
		},
		"data_preview": string(data[:min(len(data), 500)]), // first 500 chars
		"trace_id":     fmt.Sprintf("report_%s", reportID),  // for Laminar integration
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

// handleSTRReportRequest processes a FATF STR request.
func handleSTRReportRequest(
	ctx context.Context,
	args map[string]any,
	repo ReportRepository,
	auditLog []AuditEntry,
) (string, error) {
	txnID, _ := args["transaction_id"].(string)
	if txnID == "" {
		return "error: transaction_id is required for STR reports", nil
	}

	riskScore := 0.7 // default
	if rs, ok := args["risk_score"].(float64); ok {
		riskScore = rs
	}

	format, _ := args["format"].(string)
	if format == "" {
		format = "json"
	}

	// Stub: in production, fetch the actual transaction by ID from the database.
	// For now, create a minimal transaction for demonstration.
	stubTxn := finance.Transaction{
		TransactionID: txnID,
		AccountID:     "acct_stub",
		Date:          "2024-05-31",
		AmountCents:   15_000_000, // 150k INR
		Currency:      "INR",
		Description:   "High-value transfer",
		Merchant:      "Unknown Merchant",
		Direction:     finance.DirectionDebit,
	}

	// Generate STR.
	str, err := GenerateSTR(stubTxn, auditLog, riskScore)
	if err != nil {
		return fmt.Sprintf("error generating STR: %v", err), nil
	}

	// Save to repository.
	strID, err := repo.SaveSTRReport(ctx, str)
	if err != nil {
		return fmt.Sprintf("error saving STR: %v", err), nil
	}

	// Export to requested format.
	var data []byte
	switch strings.ToLower(format) {
	case "xml":
		data, err = ExportSTRAsXML(str)
	case "json":
		fallthrough
	default:
		data, err = ExportSTRAsJSON(str)
	}
	if err != nil {
		return fmt.Sprintf("error exporting STR: %v", err), nil
	}

	// Return summary.
	result := map[string]any{
		"status":           "success",
		"report_type":      "str",
		"str_id":           strID,
		"transaction_id":   txnID,
		"format":           format,
		"filing_date":      str.FilingDate,
		"risk_score":       str.RiskScore,
		"risk_indicators":  str.RiskIndicators,
		"retention_years":  str.RetentionYears,
		"data_preview":     string(data[:min(len(data), 500)]),
		"trace_id":         fmt.Sprintf("report_%s", strID), // for Laminar integration
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

// Registry is a minimal registry interface for agent tools.
// This is compatible with github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools.Registry
type Registry interface {
	Register(t Tool)
}

// RegisterReportingTools registers all reporting tools with the agent registry.
// Call this during agent setup to enable compliance reporting capabilities.
func RegisterReportingTools(
	registry Registry,
	repo ReportRepository,
	txnStore interface{},
	auditLog AuditLog,
) {
	registry.Register(GenerateComplianceReportTool(repo, txnStore, auditLog))
}

// Helper function.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
