package agenttools_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
)

// ─── Registry ─────────────────────────────────────────────────────────────

func TestRegistry_Execute_UnknownTool(t *testing.T) {
	r := agenttools.NewRegistry()
	result, err := r.Execute(context.Background(), "nonexistent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("unknown tool should return an error string")
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		if rec := recover(); rec == nil {
			t.Error("duplicate registration should panic")
		}
	}()
	r := agenttools.NewRegistry()
	r.Register(agenttools.ReadFile())
	r.Register(agenttools.ReadFile()) // should panic
}

func TestRegistry_Definitions(t *testing.T) {
	r := agenttools.FileTools()
	defs := r.Definitions()
	if len(defs) != 4 {
		t.Errorf("FileTools should have 4 definitions, got %d", len(defs))
	}
}

// ─── File tools ────────────────────────────────────────────────────────────

func TestReadFile_Success(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "hello.txt")
	_ = os.WriteFile(path, []byte("hello world"), 0o644)

	tool := agenttools.ReadFile()
	result, err := tool.Execute(context.Background(), map[string]any{"path": path})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world" {
		t.Errorf("unexpected content: %q", result)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	tool := agenttools.ReadFile()
	result, _ := tool.Execute(context.Background(), map[string]any{"path": "/does/not/exist.txt"})
	if result == "" {
		t.Error("missing file should return error string")
	}
}

func TestWriteFile_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sub", "out.txt")

	tool := agenttools.WriteFile()
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":    path,
		"content": "test content",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("write should return confirmation")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "test content" {
		t.Errorf("unexpected content: %q", string(b))
	}
}

func TestListFiles_ReturnsEntries(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "a.txt"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(tmp, "b.txt"), nil, 0o644)
	_ = os.MkdirAll(filepath.Join(tmp, "sub"), 0o755)

	tool := agenttools.ListFiles()
	result, err := tool.Execute(context.Background(), map[string]any{"directory": tmp})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("list should return entries")
	}
}

func TestDeleteFile_RemovesFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "del.txt")
	_ = os.WriteFile(path, []byte("bye"), 0o644)

	tool := agenttools.DeleteFile()
	_, err := tool.Execute(context.Background(), map[string]any{"path": path})
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("file should be deleted")
	}
}

func TestDeleteFile_NotFound(t *testing.T) {
	tool := agenttools.DeleteFile()
	result, _ := tool.Execute(context.Background(), map[string]any{"path": "/no/such/file"})
	if result == "" {
		t.Error("missing file should return error string")
	}
}

// ─── Shell tools ───────────────────────────────────────────────────────────

func TestRunCommand_Success(t *testing.T) {
	tool := agenttools.RunCommand()
	result, err := tool.Execute(context.Background(), map[string]any{"command": "echo hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("echo should return output")
	}
}

func TestRunCommand_Failure(t *testing.T) {
	tool := agenttools.RunCommand()
	result, _ := tool.Execute(context.Background(), map[string]any{"command": "exit 1"})
	if result == "" {
		t.Error("failed command should return error string")
	}
}

func TestExecuteCode_Python(t *testing.T) {
	if _, err := os.LookupEnv("SKIP_SHELL_TESTS"); err {
		t.Skip("SKIP_SHELL_TESTS set")
	}
	tool := agenttools.ExecuteCode()
	result, execErr := tool.Execute(context.Background(), map[string]any{
		"code":     `print("hello from python")`,
		"language": "python",
	})
	if execErr != nil {
		t.Fatal(execErr)
	}
	if result == "" {
		t.Error("python execution should produce output")
	}
}
