package hitl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// CLIApprover implements synchronous HITL via stdin/stdout.
// The agent loop blocks until the user types y/yes (approve) or anything else
// (deny). This is the simplest possible HITL — ideal for local development,
// CLI tools, and interactive sessions where the user is present.
//
// Example:
//
//	approver := hitl.NewCLIApprover(os.Stdin, os.Stderr)
//	executor.Approver = approver
type CLIApprover struct {
	in  io.Reader
	out io.Writer
}

// NewCLIApprover creates a CLIApprover that reads decisions from in and
// prints prompts to out. Pass os.Stdin / os.Stderr for the default terminal.
func NewCLIApprover(in io.Reader, out io.Writer) *CLIApprover {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stderr
	}
	return &CLIApprover{in: in, out: out}
}

// RequestApproval prints the tool name and args, then waits for a y/yes
// response. Any other input (including empty) is treated as denial.
// Cancelling ctx before the user responds returns (false, ctx.Err()).
func (a *CLIApprover) RequestApproval(ctx context.Context, req ApprovalRequest) (bool, error) {
	// Render args as compact JSON for readability.
	argsJSON, _ := json.Marshal(req.Args)

	fmt.Fprintf(a.out, "\n╔══ HITL Approval Required ═══════════════════════════════╗\n")
	fmt.Fprintf(a.out, "  Tool    : %s\n", req.ToolName)
	fmt.Fprintf(a.out, "  Args    : %s\n", argsJSON)
	if req.AgentID != "" {
		fmt.Fprintf(a.out, "  Agent   : %s\n", req.AgentID)
	}
	if req.RiskScore > 0 {
		fmt.Fprintf(a.out, "  Risk    : %.2f\n", req.RiskScore)
	}
	fmt.Fprintf(a.out, "╚══════════════════════════════════════════════════════════╝\n")
	fmt.Fprintf(a.out, "Approve? [y/N]: ")

	// Read the response on a goroutine so we can respect ctx cancellation.
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(a.in)
		if scanner.Scan() {
			ch <- result{line: scanner.Text()}
		} else {
			ch <- result{err: scanner.Err()}
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Fprintf(a.out, "\n[HITL] context cancelled — treating as denial\n")
		return false, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return false, r.err
		}
		resp := strings.TrimSpace(strings.ToLower(r.line))
		approved := resp == "y" || resp == "yes"
		if approved {
			fmt.Fprintf(a.out, "[HITL] ✓ approved\n")
		} else {
			fmt.Fprintf(a.out, "[HITL] ✗ denied\n")
		}
		return approved, nil
	}
}
