package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sec/internal/platform/projectconfig"
)

// MCP schemas and shared integrity checks reject path/policy overrides,
// cross-domain approvals and mutated proposals before any file can be written.
func TestProjectRequestIsolation(t *testing.T) {
	root, _ := fixture(t)
	file := filepath.Join(root, projectconfig.File)
	before, _ := os.ReadFile(file)
	client := connect(t, options{project: root})
	mode := "noalloc"
	out := decode[projectPlanOutput](t, call(t, client, "project.plan", projectOperationArguments{projectconfig.Operation{Variant: "linux-amd64", Mode: &mode}}))
	for _, name := range []string{"project.inspect", "project.resolve", "project.plan", "project.diff", "project.apply"} {
		rejected(t, client, name, map[string]any{"root": "../outside", "approved": true})
	}
	approved := connect(t, options{project: root, approved: approvals{out.Plan.PlanSHA256: true}})
	rejected(t, approved, "project.apply", map[string]any{"plan": out.Plan, "approved": true})
	rejected(t, approved, "c-bindings.apply", savedPlanArguments{})
	forged := *out.Plan
	forged.ProjectRoot = t.TempDir()
	rejected(t, approved, "project.diff", projectPlanArguments{forged})
	forged = *out.Plan
	forged.File = "../outside"
	rejected(t, approved, "project.apply", projectPlanArguments{forged})
	forged = *out.Plan
	forged.Resolutions = nil
	rejected(t, approved, "project.apply", projectPlanArguments{forged})
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Fatal("rejected request changed manifest")
	}
	// Invalid live input remains diagnosed over the protocol without writes.
	invalid := string(before) + "\n[variant.linux-amd64.allocation]\nunknown = true\n"
	if err := os.WriteFile(file, []byte(invalid), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "project.inspect", Arguments: noArguments{}})
	if err != nil || !result.IsError {
		t.Fatal(result, err)
	}
}
