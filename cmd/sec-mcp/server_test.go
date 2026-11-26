package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sec/internal/platform/cbindings"
)

// fixture uses the canonical project manifest and a comment-preserving binding
// config; no real target or module capability is invented by structural tests.
// Rules: harness design notes §§3(2),5(7),7(2)–(3).
func fixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for _, item := range []struct{ source, destination string }{
		{"../../testdata/projects/model/sec.toml", ".sec/sec.toml"},
		{"../../testdata/tooling/sec_mcp/c-bindings.toml", cbindings.DefaultFile},
	} {
		data, err := os.ReadFile(item.source)
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(root, filepath.FromSlash(item.destination))
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root, filepath.Join(root, filepath.FromSlash(cbindings.DefaultFile))
}

// connect exercises the actual protocol/session machinery with an SDK client,
// rather than calling handlers directly.
// Rules: harness design notes §§2(1)–(4),7(1).
func connect(t *testing.T, opts options) *mcp.ClientSession {
	t.Helper()
	server, closeWorkspace, err := newServer(opts, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeWorkspace() })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "sec-mcp-test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call(t *testing.T, client *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil || result.IsError {
		t.Fatalf("%s: %+v %v", name, result, err)
	}
	return result
}

func decode[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(string(data), err)
	}
	return out
}

func rejected(t *testing.T, client *mcp.ClientSession, name string, args any) {
	t.Helper()
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err == nil && !result.IsError {
		t.Fatalf("unexpected success: %+v", result)
	}
}

// Initialization discovers only read-only tools by default; annotations truthfully
// map shared service metadata and no request can install authorization or a root.
// Rules: harness design notes §§2(1),5(1),5(5)–(7),7(2).
func TestReadOnlyDiscoveryAndParity(t *testing.T) {
	root, file := fixture(t)
	client := connect(t, options{project: root})
	listing, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Tools) != 8 {
		t.Fatal(listing.Tools)
	}
	names := map[string]bool{}
	for _, tool := range listing.Tools {
		names[tool.Name] = true
		a := tool.Annotations
		if a == nil || !a.ReadOnlyHint || a.OpenWorldHint == nil || *a.OpenWorldHint || a.DestructiveHint == nil || *a.DestructiveHint || tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatal(tool)
		}
	}
	if names["c-bindings.apply"] {
		t.Fatal("write advertised without startup approval")
	}
	workspace, err := cbindings.OpenWorkspace(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Close()
	expected, err := workspace.Inspect()
	if err != nil {
		t.Fatal(err)
	}
	actual := decode[cbindings.WorkspaceInspection](t, call(t, client, "c-bindings.inspect", noArguments{}))
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("inspect differs:\n%+v\n%+v", actual, expected)
	}
	validation := decode[validationOutput](t, call(t, client, "c-bindings.validate", noArguments{}))
	if validation.Scope != "structural" || len(validation.Diagnostics) != 0 {
		t.Fatal(validation)
	}
	before, _ := os.ReadFile(file)
	version := "2.0.0"
	op := cbindings.Operation{Kind: cbindings.OpPackageSet, Version: &version}
	planned := decode[planOutput](t, call(t, client, "c-bindings.plan", planArguments{Operation: op}))
	expectedPlan, ds, err := workspace.PlanChange(op)
	if err != nil || len(ds) != 0 || planned.Plan == nil || !reflect.DeepEqual(planned.Plan, expectedPlan) {
		t.Fatal(planned, ds, err)
	}
	diff := decode[diffOutput](t, call(t, client, "c-bindings.diff", savedPlanArguments{Plan: *planned.Plan}))
	if diff.Patch != planned.Plan.Change.Patch || !strings.Contains(diff.Patch, "+version = \"2.0.0\"") {
		t.Fatal(diff)
	}
	rejected(t, client, "c-bindings.apply", savedPlanArguments{Plan: *planned.Plan})
	rejected(t, client, "c-bindings.inspect", map[string]any{"project": root, "file": "../outside"})
	rejected(t, client, "c-bindings.plan", map[string]any{"operation": op, "approved": true})
	rejected(t, client, "shell", map[string]any{"command": "anything"})
	after, _ := os.ReadFile(file)
	if string(after) != string(before) {
		t.Fatal("read-only MCP changed file")
	}
}

// All six typed mutation kinds use exactly the same canonical service plan and
// edit semantics; apply is visible only after explicit exact startup approval.
// Rules: harness design notes §§2(2)–(4),5(2)–(7),7(1)–(3).
func TestApprovedMutationParity(t *testing.T) {
	value := func(s string) *string { return &s }
	for _, op := range []cbindings.Operation{
		{Kind: cbindings.OpPackageSet, Version: value("2.0.0")},
		{Kind: cbindings.OpEnvironmentAdd, Environment: "linux-extra", OS: value("linux"), Arch: value("amd64"), ABI: value("gnu"), CRuntime: value("glibc")},
		{Kind: cbindings.OpEnvironmentSet, Environment: "linux", CRuntimeVersions: value(">=2.31")},
		{Kind: cbindings.OpEnvironmentRemove, Environment: "linux"},
		{Kind: cbindings.OpModuleAdd, Common: true, Module: "c/extra"},
		{Kind: cbindings.OpModuleRemove, Common: true, Module: "c/common"},
	} {
		t.Run(string(op.Kind), func(t *testing.T) {
			root, file := fixture(t)
			workspace, err := cbindings.OpenWorkspace(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer workspace.Close()
			plan, ds, err := workspace.PlanChange(op)
			if err != nil || len(ds) != 0 {
				t.Fatal(ds, err)
			}
			client := connect(t, options{project: root, approved: approvals{plan.PlanSHA256: true}})
			listing, err := client.ListTools(context.Background(), nil)
			if err != nil || len(listing.Tools) != 10 {
				t.Fatal(listing, err)
			}
			for _, tool := range listing.Tools {
				if tool.Name == "c-bindings.apply" && (tool.Annotations.ReadOnlyHint || !*tool.Annotations.DestructiveHint || *tool.Annotations.OpenWorldHint) {
					t.Fatal(tool)
				}
			}
			actualPlan := decode[planOutput](t, call(t, client, "c-bindings.plan", planArguments{Operation: op}))
			if !reflect.DeepEqual(actualPlan.Plan, plan) {
				t.Fatal("transport changed plan")
			}
			before, _ := os.ReadFile(file)
			expected := cbindings.Apply(string(before), op)
			rejected(t, client, "c-bindings.apply", map[string]any{"plan": plan, "approved": true})
			applied := decode[applyOutput](t, call(t, client, "c-bindings.apply", savedPlanArguments{Plan: *plan}))
			after, _ := os.ReadFile(file)
			if !applied.Changed || len(applied.Diagnostics) != 0 || string(after) != expected.Text {
				t.Fatal(applied, string(after))
			}
			rejected(t, client, "c-bindings.apply", savedPlanArguments{Plan: *plan})
		})
	}
}

// Approval is launcher policy, not JSON data. Unknown selectors, stale files,
// forged/cross-project plans and symlink escapes never turn into writes.
// Rules: harness design notes §§5(3)–(7),7(2)–(3).
func TestProtocolRejectionsPreserveFiles(t *testing.T) {
	for _, kind := range []string{"wrong-approval", "stale", "modified", "foreign-project", "foreign-file", "symlink", "policy-snapshot"} {
		t.Run(kind, func(t *testing.T) {
			root, file := fixture(t)
			workspace, err := cbindings.OpenWorkspace(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer workspace.Close()
			version := "2.0.0"
			plan, _, err := workspace.PlanChange(cbindings.Operation{Kind: cbindings.OpPackageSet, Version: &version})
			if err != nil {
				t.Fatal(err)
			}
			approval := plan.PlanSHA256
			switch kind {
			case "wrong-approval", "policy-snapshot":
				approval = strings.Repeat("0", 64)
			case "stale":
				data, _ := os.ReadFile(file)
				os.WriteFile(file, append(data, []byte("# edited after review\n")...), 0600)
			case "modified":
				plan.Change.Patch = "forged"
			case "foreign-project":
				plan.ProjectRoot = t.TempDir()
			case "foreign-file":
				plan.Change.File = "../escape"
			case "symlink":
				external := filepath.Join(t.TempDir(), "external.toml")
				data, _ := os.ReadFile(file)
				os.WriteFile(external, data, 0600)
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, file); err != nil {
					t.Skip(err)
				}
			}
			before, _ := os.ReadFile(file)
			policy := approvals{approval: true}
			client := connect(t, options{project: root, approved: policy})
			if kind == "policy-snapshot" {
				policy[plan.PlanSHA256] = true
			}
			rejected(t, client, "c-bindings.apply", savedPlanArguments{Plan: *plan})
			after, _ := os.ReadFile(file)
			if string(before) != string(after) {
				t.Fatal("rejected MCP request wrote file")
			}
		})
	}
}

// Canonical schema diagnostics cross the transport without editor-only rules;
// malformed operations never activate a provider or target capability.
// Rules: harness design notes §§3(2),5(7),7(1)–(3).
func TestCanonicalValidationErrors(t *testing.T) {
	root, file := fixture(t)
	client := connect(t, options{project: root})
	before, _ := os.ReadFile(file)
	arch := "invented-architecture"
	for _, op := range []cbindings.Operation{
		{Kind: cbindings.OpModuleAdd, Common: true, Module: "../escape"},
		{Kind: cbindings.OpEnvironmentSet, Environment: "linux", Arch: &arch},
		{Kind: "activate-provider"},
	} {
		result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "c-bindings.plan", Arguments: planArguments{Operation: op}})
		if err != nil || !result.IsError {
			t.Fatal(result, err)
		}
		out := decode[planOutput](t, result)
		expected := cbindings.Apply(string(before), op)
		if out.Plan != nil || !reflect.DeepEqual(out.Diagnostics, expected.Diagnostics) {
			t.Fatal(out, expected)
		}
	}
	os.WriteFile(file, []byte("schema_version = ["), 0600)
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "c-bindings.validate", Arguments: noArguments{}})
	if err != nil || !result.IsError {
		t.Fatal(result, err)
	}
	out := decode[validationOutput](t, result)
	if out.Scope != "structural" || len(out.Diagnostics) == 0 {
		t.Fatal(out)
	}
}
