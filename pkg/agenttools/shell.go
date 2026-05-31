package agenttools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Shell and code-execution tools (lesson 08).
//
// Safety note: RunCommand executes arbitrary commands with the current
// user's full permissions. This is intentional for local development so
// the code stays simple. Production deployments must add sandboxing
// (container isolation, network restrictions, resource limits).
//
// Composite vs orchestrated: ExecuteCode bundles write-temp-file + run + cleanup
// into one tool call. The agent gets one clean result; no temp-file management.

// ─── RunCommand ────────────────────────────────────────────────────────────

// RunCommand returns a Tool that executes a shell command via /bin/sh.
func RunCommand() Tool {
	return &ToolDef{
		ToolName:        "run_command",
		ToolDescription: "Execute a shell command and return its output. Use for system operations, git commands, package management, and running existing scripts.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute",
				},
				"timeout_seconds": map[string]any{
					"type":        "integer",
					"description": "Maximum seconds to wait (default 30)",
					"default":     30,
				},
			},
			"required": []string{"command"},
		},
		Fn: runCommandFn,
	}
}

func runCommandFn(ctx context.Context, args map[string]any) (string, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return "error: command argument is required", nil
	}

	timeoutSec := 30
	if ts, ok := args["timeout_seconds"].(float64); ok && ts > 0 {
		timeoutSec = int(ts)
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "/bin/sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var out strings.Builder
	if stdout.Len() > 0 {
		out.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		out.WriteString(stderr.String())
	}

	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("command timed out after %ds", timeoutSec), nil
		}
		if out.Len() == 0 {
			return fmt.Sprintf("command failed: %v", err), nil
		}
		return fmt.Sprintf("command failed (exit non-zero):\n%s", out.String()), nil
	}

	if out.Len() == 0 {
		return "command completed successfully (no output)", nil
	}
	return out.String(), nil
}

// ─── ExecuteCode (composite tool) ─────────────────────────────────────────

// Language identifies the runtime for code execution.
type Language string

const (
	LangPython     Language = "python"
	LangGo         Language = "go"
	LangBash       Language = "bash"
	LangJavaScript Language = "javascript"
)

// ExecuteCode returns a composite Tool that writes code to a temp file and
// executes it with the appropriate runtime. Supports Python, Go, Bash, JS.
// Temp files are cleaned up in all cases.
func ExecuteCode() Tool {
	return &ToolDef{
		ToolName:        "execute_code",
		ToolDescription: "Execute code and return its output. Supports python, go, bash, and javascript (node). Ideal for computations, data processing, and algorithms.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{
					"type":        "string",
					"description": "The code to execute",
				},
				"language": map[string]any{
					"type":        "string",
					"enum":        []string{"python", "go", "bash", "javascript"},
					"description": "The programming language",
					"default":     "python",
				},
				"timeout_seconds": map[string]any{
					"type":    "integer",
					"default": 30,
				},
			},
			"required": []string{"code"},
		},
		Fn: executeCodeFn,
	}
}

var langExts = map[Language]string{
	LangPython:     ".py",
	LangGo:         ".go",
	LangBash:       ".sh",
	LangJavaScript: ".js",
}

var langRunners = map[Language]string{
	LangPython:     "python3",
	LangBash:       "bash",
	LangJavaScript: "node",
	// Go is handled specially: go run <file>
}

func executeCodeFn(ctx context.Context, args map[string]any) (string, error) {
	code, _ := args["code"].(string)
	if code == "" {
		return "error: code argument is required", nil
	}

	langStr, _ := args["language"].(string)
	if langStr == "" {
		langStr = "python"
	}
	lang := Language(langStr)

	timeoutSec := 30
	if ts, ok := args["timeout_seconds"].(float64); ok && ts > 0 {
		timeoutSec = int(ts)
	}

	ext, ok := langExts[lang]
	if !ok {
		return fmt.Sprintf("unsupported language: %q (supported: python, go, bash, javascript)", lang), nil
	}

	// Write to a temp file.
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("genie-exec-*%s", ext))
	if err != nil {
		return fmt.Sprintf("error creating temp file: %v", err), nil
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(code); err != nil {
		tmpFile.Close()
		return fmt.Sprintf("error writing code: %v", err), nil
	}
	tmpFile.Close()

	// Build the run command.
	var runCmd string
	if lang == LangGo {
		runCmd = fmt.Sprintf("go run %s", tmpPath)
	} else if runner, ok := langRunners[lang]; ok {
		runCmd = fmt.Sprintf("%s %s", runner, tmpPath)
	}

	return runCommandFn(ctx, map[string]any{
		"command":         runCmd,
		"timeout_seconds": float64(timeoutSec),
	})
}

// ShellTools returns a Registry with RunCommand and ExecuteCode.
func ShellTools() *Registry {
	r := NewRegistry()
	r.Register(RunCommand())
	r.Register(ExecuteCode())
	return r
}

// AllTools returns a Registry with file + shell tools.
func AllTools() *Registry {
	r := FileTools()
	r.Register(RunCommand())
	r.Register(ExecuteCode())
	// Web search stub (lesson 07)
	r.Register(WebSearchStub())
	return r
}

// stubPath avoids unused import
var _ = filepath.Join
