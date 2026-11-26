package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sec/internal/platform/cbindingplan"
	"sec/internal/platform/cbindings"
	"sec/internal/platform/projectconfig"
	"sec/internal/platform/target"
)

func sameJSON(t *testing.T, a, b any) {
	t.Helper()
	normalize := func(v any) any {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var out any
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if !reflect.DeepEqual(normalize(a), normalize(b)) {
		t.Fatalf("semantic mismatch:\n%+v\n%+v", a, b)
	}
}

// CLI and real stdio MCP requests must produce the same plans, edits,
// diagnostics and target/variant provenance. Builds only a temporary executable.
// Rules: harness design notes §7(1)-(4).
func TestHarnessCLIMCPAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess acceptance builds the temporary CLI")
	}
	binary := filepath.Join(t.TempDir(), "sec")
	build := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/compiler")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	root, bindingFile := fixture(t)
	manifestFile := filepath.Join(root, projectconfig.File)
	// Select explicit target/runtime facts rather than deriving them from the host.
	manifest, _ := os.ReadFile(manifestFile)
	text := strings.Replace(string(manifest), "[variant.linux-amd64]\nos = \"linux\"\narch = \"amd64\"", "[variant.linux-amd64]\nos = \"linux\"\narch = \"amd64\"\nabi = \"gnu\"\nc_runtime = \"glibc\"\n\n[variant.linux-amd64.c_bindings]\nid = \"demo\"\nversion = \"=1.0.0\"\nenvironment = \"linux\"", 1)
	if err := os.WriteFile(manifestFile, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	for _, module := range []string{"c/common", "c/linux"} {
		dir := filepath.Join(root, "sec/platform", module)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(module)
		source, err := os.ReadFile("../../testdata/tooling/sec_mcp/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".sec"), source, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cli := func(expect int, args ...string) any {
		t.Helper()
		command := exec.Command(binary, args...)
		command.Dir = root
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		err := command.Run()
		code := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if code != expect {
			t.Fatalf("%v exit %d: %s %s", args, code, stdout.String(), stderr.String())
		}
		var value any
		if stdout.Len() > 0 {
			if err := json.Unmarshal(stdout.Bytes(), &value); err != nil {
				t.Fatal(stdout.String(), err)
			}
		}
		return value
	}
	projectCLI := func(expect int, verb string, args ...string) any {
		t.Helper()
		return cli(expect, append([]string{"platform", "project", verb, "--project", root}, args...)...)
	}
	client, closeClient := stdioClient(t, "--project", root)
	sameJSON(t, projectCLI(0, "inspect"), call(t, client, "project.inspect", noArguments{}).StructuredContent)
	resolution := decode[projectconfig.Resolution](t, call(t, client, "project.resolve", projectSelectionArguments{Target: "sec", Variant: "linux-amd64"}))
	sameJSON(t, projectCLI(0, "resolve", "--target", "sec", "--variant", "linux-amd64"), resolution)
	canonical, problems := cbindingplan.Resolve(root, "linux-amd64", target.Target{OS: "linux", Arch: "amd64"})
	if len(problems) > 0 || canonical == nil {
		t.Fatal(problems)
	}
	sameJSON(t, canonical, resolution.Binding)
	bindingCLI := cli(0, "platform", "c-bindings", "resolve", "--variant", "linux-amd64", "--json").(map[string]any)
	sameJSON(t, bindingCLI["binding"], resolution.Binding.Resolution)
	if resolution.Selection.Facts == nil || resolution.Selection.Facts.Profile.ProviderIdentity != "" || resolution.Binding.Resolution.Fingerprint == "" || len(resolution.Binding.Files) != 2 {
		t.Fatal(resolution)
	}
	mode, profile := "noalloc", "hosted"
	op := projectconfig.Operation{Target: "sec", Variant: "linux-amd64", Profile: &profile, Mode: &mode}
	encoded, _ := json.Marshal(op)
	proposal := decode[projectPlanOutput](t, call(t, client, "project.plan", projectOperationArguments{op}))
	sameJSON(t, projectCLI(0, "plan", "--operation", string(encoded)), proposal)
	sameJSON(t, projectCLI(0, "plan", "--operation", string(encoded)), proposal) // deterministic JSON
	planFile := filepath.Join(t.TempDir(), "plan.json")
	saved, _ := json.Marshal(proposal.Plan)
	os.WriteFile(planFile, saved, 0600)
	sameJSON(t, projectCLI(0, "diff", "--plan", planFile), call(t, client, "project.diff", projectPlanArguments{*proposal.Plan}).StructuredContent)
	rejected(t, client, "project.apply", projectPlanArguments{*proposal.Plan})
	for _, invalid := range []string{
		`{"variant":"unknown","mode":"noalloc"}`,
		`{"variant":"../outside","mode":"noalloc"}`,
		`{"variant":"linux-amd64","profile":"baremetal"}`,
		`{"variant":"linux-amd64","mode":"hosted","provider":"sec.system-heap.v1"}`,
	} {
		var operation projectconfig.Operation
		json.Unmarshal([]byte(invalid), &operation)
		result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "project.plan", Arguments: projectOperationArguments{operation}})
		if err != nil || !result.IsError {
			t.Fatal(result, err)
		}
		sameJSON(t, projectCLI(2, "plan", "--operation", invalid), result.StructuredContent)
		unchanged, _ := os.ReadFile(manifestFile)
		if string(unchanged) != text {
			t.Fatal("rejected edit wrote")
		}
	}
	closeClient()
	cliResult := projectCLI(0, "apply-plan", "--plan", planFile, "--approve", proposal.Plan.PlanSHA256)
	cliBytes, _ := os.ReadFile(manifestFile)
	if err := os.WriteFile(manifestFile, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	approved, closeApproved := stdioClient(t, "--project", root, "--approve", proposal.Plan.PlanSHA256)
	sameJSON(t, cliResult, call(t, approved, "project.apply", projectPlanArguments{*proposal.Plan}).StructuredContent)
	actual, _ := os.ReadFile(manifestFile)
	if !bytes.Equal(actual, cliBytes) || !bytes.Contains(actual, []byte("# The human name")) {
		t.Fatal("different edits or comment loss")
	}
	rejected(t, approved, "project.apply", projectPlanArguments{*proposal.Plan})
	projectCLI(2, "apply-plan", "--plan", planFile, "--approve", proposal.Plan.PlanSHA256)
	resolved := decode[projectconfig.Resolution](t, call(t, approved, "project.resolve", projectSelectionArguments{Target: "sec", Variant: "linux-amd64"}))
	if resolved.Selection.Facts.Selection.ModeScope != "target.sec.variant.linux-amd64.allocation" || resolved.Selection.Facts.Profile.AllocationPolicy != "noalloc" {
		t.Fatal(resolved)
	}
	closeApproved()
	// Equivalent C-binding edits use their established CLI command and same editor.
	version := "2.0.0"
	bindingClient := connect(t, options{project: root})
	bindingProposal := decode[planOutput](t, call(t, bindingClient, "c-bindings.plan", planArguments{cbindings.Operation{Kind: cbindings.OpPackageSet, Version: &version}}))
	raw := cli(0, "platform", "c-bindings", "package", "set", "--version", version, "--file", bindingFile, "--plan", "--json")
	data, _ := json.Marshal(raw)
	var cliPlan cbindings.ChangePlan
	json.Unmarshal(data, &cliPlan)
	cliPlan.File = bindingProposal.Plan.Change.File
	cliPlan.PlanSHA256 = bindingProposal.Plan.Change.PlanSHA256
	sameJSON(t, cliPlan, bindingProposal.Plan.Change)
	originalBindings, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatal(err)
	}
	for name, invalid := range map[string]string{
		"invalid TOML":     "schema_version = [",
		"unknown key":      "unknown_key = true\n" + string(originalBindings),
		"duplicate names":  string(originalBindings) + "\n[environments.linux]\nos='linux'\n",
		"invalid ABI":      strings.Replace(string(originalBindings), "abi = \"gnu\"", "abi = \"\"", 1),
		"module traversal": strings.Replace(string(originalBindings), "c/common", "../outside", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(bindingFile, []byte(invalid), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := bindingClient.CallTool(context.Background(), &mcp.CallToolParams{Name: "c-bindings.validate", Arguments: noArguments{}})
			if err != nil || !result.IsError {
				t.Fatal(result, err)
			}
			out := decode[validationOutput](t, result)
			cliOut := cli(2, "platform", "c-bindings", "validate", "--file", bindingFile, "--json").(map[string]any)
			sameJSON(t, cliOut["diagnostics"], out.Diagnostics)
			bytesNow, _ := os.ReadFile(bindingFile)
			if string(bytesNow) != invalid {
				t.Fatal("validation rewrote input")
			}
		})
	}
	if err := os.WriteFile(bindingFile, originalBindings, 0600); err != nil {
		t.Fatal(err)
	}
	// Missing module selection is rejected identically by the shared plan owner.
	if err := os.RemoveAll(filepath.Join(root, "sec/platform/c/linux")); err != nil {
		t.Fatal(err)
	}
	failed, err := bindingClient.CallTool(context.Background(), &mcp.CallToolParams{Name: "project.resolve", Arguments: projectSelectionArguments{Target: "sec", Variant: "linux-amd64"}})
	if err != nil || !failed.IsError {
		t.Fatal(failed, err)
	}
	sameJSON(t, projectCLI(2, "resolve", "--target", "sec", "--variant", "linux-amd64"), failed.StructuredContent)

}
