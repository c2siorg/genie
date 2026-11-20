package agenttools

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// File system tools — readFile, writeFile, listFiles, deleteFile.
// These give the agent persistent memory, a scratchpad, and an audit trail
// (see lesson 06). All errors are returned as strings so the agent can recover.

// ─── readFile ──────────────────────────────────────────────────────────────

// ReadFile returns a Tool that reads text files.
func ReadFile() Tool {
	return &ToolDef{
		ToolName:        "read_file",
		ToolDescription: "Read the contents of a file at the specified path. Use this to examine file contents, configuration, or any text data.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The path to the file to read",
				},
			},
			"required": []string{"path"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "error: path argument is required", nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				var pe *fs.PathError
				if errors.As(err, &pe) && errors.Is(pe.Err, fs.ErrNotExist) {
					return fmt.Sprintf("error: file not found: %s", path), nil
				}
				return fmt.Sprintf("error reading file: %v", err), nil
			}
			return string(b), nil
		},
	}
}

// ─── writeFile ─────────────────────────────────────────────────────────────

// WriteFile returns a Tool that writes or overwrites a text file.
// Parent directories are created automatically (mkdir -p semantics).
func WriteFile() Tool {
	return &ToolDef{
		ToolName:        "write_file",
		ToolDescription: "Write content to a file at the specified path. Creates the file (and parent directories) if they don't exist, overwrites if the file exists.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The path to the file to write",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)
			if path == "" {
				return "error: path argument is required", nil
			}
			// Auto-create parent directories.
			if dir := filepath.Dir(path); dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Sprintf("error creating directories: %v", err), nil
				}
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Sprintf("error writing file: %v", err), nil
			}
			return fmt.Sprintf("successfully wrote %d bytes to %s", len(content), path), nil
		},
	}
}

// ─── listFiles ─────────────────────────────────────────────────────────────

// ListFiles returns a Tool that lists directory contents.
func ListFiles() Tool {
	return &ToolDef{
		ToolName:        "list_files",
		ToolDescription: "List all files and directories at the specified path. Use this to navigate the file system and discover what's available.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"directory": map[string]any{
					"type":        "string",
					"description": "The directory path to list. Defaults to current directory.",
					"default":     ".",
				},
			},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			dir, _ := args["directory"].(string)
			if dir == "" {
				dir = "."
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				var pe *fs.PathError
				if errors.As(err, &pe) && errors.Is(pe.Err, fs.ErrNotExist) {
					return fmt.Sprintf("error: directory not found: %s", dir), nil
				}
				return fmt.Sprintf("error listing directory: %v", err), nil
			}
			if len(entries) == 0 {
				return fmt.Sprintf("directory %s is empty", dir), nil
			}
			var lines []string
			for _, e := range entries {
				prefix := "[file]"
				if e.IsDir() {
					prefix = "[dir] "
				}
				lines = append(lines, prefix+" "+e.Name())
			}
			return strings.Join(lines, "\n"), nil
		},
	}
}

// ─── deleteFile ────────────────────────────────────────────────────────────

// DeleteFile returns a Tool that removes a file.
func DeleteFile() Tool {
	return &ToolDef{
		ToolName:        "delete_file",
		ToolDescription: "Delete a file at the specified path. Use with caution — this is irreversible.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The path to the file to delete",
				},
			},
			"required": []string{"path"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "error: path argument is required", nil
			}
			if err := os.Remove(path); err != nil {
				var pe *fs.PathError
				if errors.As(err, &pe) && errors.Is(pe.Err, fs.ErrNotExist) {
					return fmt.Sprintf("error: file not found: %s", path), nil
				}
				return fmt.Sprintf("error deleting file: %v", err), nil
			}
			return fmt.Sprintf("successfully deleted %s", path), nil
		},
	}
}

// FileTools returns a Registry pre-loaded with all four file tools.
func FileTools() *Registry {
	r := NewRegistry()
	r.Register(ReadFile())
	r.Register(WriteFile())
	r.Register(ListFiles())
	r.Register(DeleteFile())
	return r
}
