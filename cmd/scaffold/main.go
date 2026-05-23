// Command scaffold generates the boilerplate for a new Genie specialist agent
// under agents/<name>/. It prints the registration snippet you can paste into
// cmd/api or cmd/genie.
//
// Usage:
//
//	go run ./cmd/scaffold -name forecaster_v2 -capability forecast_v2 -intype enriched_transactions -outtype forecast_v2_result -next financial_supervisor
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type Spec struct {
	Name        string
	Pkg         string
	StructName  string
	Constants   string
	HumanName   string
	Capability  string
	InType      string
	OutType     string
	NextAgent   string
}

const agentTemplate = `// Package {{.Pkg}} is a generated specialist agent skeleton.
// Replace HandleMessage's TODO with real logic.
package {{.Pkg}}

import (
	"context"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "{{.Name}}"
	Capability = "{{.Capability}}"
	TypeIn     = "{{.InType}}"
	TypeOut    = "{{.OutType}}"
	NextAgent  = "{{.NextAgent}}"
)

type {{.StructName}} struct{}

func New() *{{.StructName}} { return &{{.StructName}}{} }

func (a *{{.StructName}}) ID() string             { return ID }
func (a *{{.StructName}}) Name() string           { return "{{.HumanName}}" }
func (a *{{.StructName}}) Capabilities() []string { return []string{Capability} }

func (a *{{.StructName}}) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	env.Logf("[{{.Name}}] handling %s from %s", msg.Type, msg.From)
	// TODO: implement domain logic. Build payload, then publish to NextAgent.
	out := agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, msg.Content, msg.Metadata)
	return []agent.Message{out}, nil
}
`

const testTemplate = `package {{.Pkg}}

import (
	"context"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestHandleMessage_PassThrough(t *testing.T) {
	msg := agent.NewMessage("upstream", ID, agent.RoleAgent, TypeIn, "payload", nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].To != NextAgent {
		t.Fatalf("expected dispatch to %s, got %+v", NextAgent, out)
	}
}
`

func main() {
	var (
		name       = flag.String("name", "", "agent id (also package name): kebab/snake/lower")
		capability = flag.String("capability", "", "capability string")
		inType     = flag.String("intype", "", "message Type this agent handles")
		outType    = flag.String("outtype", "", "message Type this agent emits")
		nextAgent  = flag.String("next", "", "agent id this agent dispatches to")
		human      = flag.String("title", "", "human-readable agent name (defaults to Name)")
		root       = flag.String("root", ".", "repo root")
		dryRun     = flag.Bool("dry-run", false, "print generated files instead of writing")
	)
	flag.Parse()

	if err := generate(*root, Spec{
		Name:       *name,
		Pkg:        *name,
		Capability: *capability,
		InType:     *inType,
		OutType:    *outType,
		NextAgent:  *nextAgent,
		HumanName:  *human,
	}, *dryRun); err != nil {
		fmt.Fprintln(os.Stderr, "scaffold:", err)
		os.Exit(1)
	}
}

func generate(root string, s Spec, dry bool) error {
	if s.Name == "" || s.Capability == "" || s.InType == "" || s.OutType == "" || s.NextAgent == "" {
		return errors.New("name, capability, intype, outtype, next are all required")
	}
	s.StructName = titleCase(s.Name)
	if s.HumanName == "" {
		s.HumanName = s.StructName
	}

	dir := filepath.Join(root, "agents", s.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]string{
		filepath.Join(dir, s.Name+".go"):       agentTemplate,
		filepath.Join(dir, s.Name+"_test.go"): testTemplate,
	}
	for path, tmpl := range files {
		out, err := render(tmpl, s)
		if err != nil {
			return err
		}
		if dry {
			fmt.Printf("--- %s ---\n%s\n", path, out)
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists", path)
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			return err
		}
	}
	fmt.Println("Generated. Register the new agent in cmd/api/main.go and cmd/genie/main.go:")
	fmt.Printf("    register(%s.New())\n", s.Name)
	return nil
}

func render(tmpl string, s Spec) (string, error) {
	t, err := template.New("x").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := t.Execute(&sb, s); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func titleCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "") + "Agent"
}
