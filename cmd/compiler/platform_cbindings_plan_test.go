package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/platform/cbindings"
)

// Every existing typed mutation can be reviewed and separately approved via
// the CLI, with the same deterministic service plan and canonical edit result.
// Rules: harness design notes §§2(2)–(4),5(2)–(7),7(1)–(3); cli.md §8.
func TestPlatformCBindingsChangePlans(t *testing.T) {
	file := filepath.Join(t.TempDir(), ".sec", "platform", "c-bindings.toml")
	if code, out := runCBindings(t, "init", "--id", "demo", "--version", "1.0.0", "--source-root", "sec/platform", "--file", file); code != 0 {
		t.Fatal(code, out)
	}
	steps := [][]string{
		{"package", "set", "--version", "2.0.0"},
		{"environment", "add", "linux", "--os", "linux", "--arch", "amd64", "--abi", "gnu", "--c-runtime", "glibc"},
		{"environment", "set", "linux", "--c-runtime-versions", ">=2.31"},
		{"module", "add", "c/common", "--common"},
		{"module", "remove", "c/common", "--common"},
		{"environment", "remove", "linux"},
	}
	for _, step := range steps {
		t.Run(strings.Join(step[:2], " "), func(t *testing.T) {
			before, _ := os.ReadFile(file)
			args := append(append([]string{}, step...), "--file", file, "--plan", "--json")
			code, out := runCBindings(t, args...)
			if code != 0 {
				t.Fatal(code, out)
			}
			var plan cbindings.ChangePlan
			if err := json.Unmarshal([]byte(out), &plan); err != nil {
				t.Fatal(out, err)
			}
			after, _ := os.ReadFile(file)
			if string(before) != string(after) || plan.BeforeSHA256 != cbindings.SHA256(before) {
				t.Fatal("planning wrote file")
			}
			canonical, ds, err := cbindings.PlanChange(file, plan.Operation)
			if err != nil || len(ds) != 0 || canonical.PlanSHA256 != plan.PlanSHA256 {
				t.Fatal(canonical, ds, err)
			}
			if _, repeat := runCBindings(t, args...); repeat != out {
				t.Fatal("unstable JSON")
			}
			planFile := filepath.Join(t.TempDir(), "plan.json")
			if err := os.WriteFile(planFile, []byte(out), 0600); err != nil {
				t.Fatal(err)
			}
			code, out = runCBindings(t, "apply-plan", planFile, "--json")
			if code != 2 || !strings.Contains(out, "c-bindings.plan-not-approved") {
				t.Fatal(code, out)
			}
			code, out = runCBindings(t, "apply-plan", planFile, "--approve", plan.PlanSHA256, "--json")
			if code != 0 {
				t.Fatal(code, out)
			}
			var result cBindingsResult
			if err := json.Unmarshal([]byte(out), &result); err != nil || !result.Changed || result.AfterSHA == nil || *result.AfterSHA != plan.AfterSHA256 {
				t.Fatal(out, err)
			}
			after, _ = os.ReadFile(file)
			want := cbindings.Apply(string(before), plan.Operation)
			if string(after) != want.Text {
				t.Fatal("different CLI/service edit")
			}
			code, out = runCBindings(t, "apply-plan", planFile, "--approve", plan.PlanSHA256, "--json")
			if code != 2 || !strings.Contains(out, "c-bindings.stale-file") {
				t.Fatal(code, out)
			}
		})
	}
	// Freshly prepared idempotent mutations keep the original file identity.
	args := []string{"environment", "remove", "absent", "--file", file, "--plan", "--json"}
	code, out := runCBindings(t, args...)
	var plan cbindings.ChangePlan
	if code != 0 || json.Unmarshal([]byte(out), &plan) != nil || plan.Changed || plan.Patch != "" {
		t.Fatal(code, out)
	}
	if code, out := runCBindings(t, "package", "set", "--version", "3.0.0", "--file", file, "--plan"); code != 0 || !strings.Contains(out, "Read-only proposal") || !strings.Contains(out, "--- a/c-bindings.toml") {
		t.Fatal(code, out)
	}
}

// Saved input hashes, review text and explicit approval cannot be bypassed by
// stale input, modified JSON, malformed documents or placing approval in JSON.
// Rules: harness design notes §§5(3)–(7),7(2)–(3).
func TestPlatformCBindingsChangePlanRejections(t *testing.T) {
	for _, kind := range []string{"stale", "modified-preview", "unknown-field", "trailing-json", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "c-bindings.toml")
			code, out := runCBindings(t, "init", "--id", "demo", "--version", "1.0.0", "--source-root", "sec/platform", "--file", file)
			if code != 0 {
				t.Fatal(out)
			}
			code, out = runCBindings(t, "package", "set", "--version", "2.0.0", "--file", file, "--plan", "--json")
			if code != 0 {
				t.Fatal(out)
			}
			var plan cbindings.ChangePlan
			if err := json.Unmarshal([]byte(out), &plan); err != nil {
				t.Fatal(err)
			}
			wantCode := 1
			switch kind {
			case "stale":
				data, _ := os.ReadFile(file)
				os.WriteFile(file, append(data, []byte("# concurrent\n")...), 0600)
				wantCode = 2
			case "modified-preview":
				plan.Patch = "wrong"
				data, _ := json.Marshal(plan)
				out = string(data)
				wantCode = 2
			case "unknown-field":
				out = strings.TrimSpace(out)
				out = strings.TrimSuffix(out, "}") + `,"approved":true}`
			case "trailing-json":
				out += "{}"
			case "malformed":
				out = "{"
			}
			planFile := filepath.Join(root, "plan.json")
			os.WriteFile(planFile, []byte(out), 0600)
			before, _ := os.ReadFile(file)
			code, out = runCBindings(t, "apply-plan", planFile, "--approve", plan.PlanSHA256, "--json")
			after, _ := os.ReadFile(file)
			if code != wantCode || string(after) != string(before) {
				t.Fatal(code, out, "target changed")
			}
		})
	}
	if code, out := runCBindings(t, "show", "--plan"); code != 1 || !strings.Contains(out, "--plan applies only") {
		t.Fatal(code, out)
	}
}

// Help discovers both phases; a read-only plan reaches the executable command
// dispatcher rather than being available only through in-process helper tests.
// Rules: rules/tooling/cli.md §§2,8(7).
func TestCBindingsPlanPublicCLI(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "c-bindings.toml")
	code, out := runCBindings(t, "init", "--id", "demo", "--version", "1.0.0", "--source-root", "sec/platform", "--file", file)
	if code != 0 {
		t.Fatal(code, out)
	}
	stdout, stderr, status := runCLIForDiagnostics(t, "help", "platform")
	if status != 0 || !strings.Contains(stdout+stderr, "--plan") || !strings.Contains(stdout+stderr, "apply-plan") {
		t.Fatal(stdout, stderr, status)
	}
	stdout, stderr, status = runCLIForDiagnostics(t, "platform", "c-bindings", "package", "set", "--version", "2.0.0", "--file", file, "--plan", "--json")
	var plan cbindings.ChangePlan
	if status != 0 || json.Unmarshal([]byte(stdout), &plan) != nil || plan.Format != "sec.c-bindings-change-plan" {
		t.Fatal(stdout, stderr, status)
	}
	// Apply through the public command boundary as well.
	planFile := filepath.Join(root, "change.json")
	os.WriteFile(planFile, []byte(stdout), 0600)
	stdout, stderr, status = runCLIForDiagnostics(t, "platform", "c-bindings", "apply-plan", planFile, "--approve", plan.PlanSHA256, "--json")
	if status != 0 || !strings.Contains(stdout, `"changed": true`) {
		t.Fatal(stdout, stderr, status)
	}
}
