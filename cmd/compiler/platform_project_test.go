package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/platform/projectconfig"
)

// Typed project configuration flags cannot bypass the shared service or treat
// request data as approval. Help exposes the new syntax without project I/O.
func TestPlatformProjectCLI(t *testing.T) {
	root := t.TempDir()
	data, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, ".sec"), 0700)
	os.WriteFile(filepath.Join(root, projectconfig.File), data, 0600)
	run := func(want int, args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runPlatformCommand(append([]string{"project"}, args...), &stdout, &stderr)
		if code != want {
			t.Fatalf("%v: code%d: %s %s", args, code, stdout.String(), stderr.String())
		}
		return stdout.String()
	}
	if !strings.Contains(run(0, "--help"), "apply-plan") {
		t.Fatal("help missing")
	}
	for _, args := range [][]string{
		{"inspect"}, {"unknown", "--project", root}, {"inspect", "--project", root, "--approve", "abc"},
		{"plan", "--project", root, "--operation", `{"variant":"linux-amd64","mode":"noalloc","approved":true}`},
		{"plan", "--project", root, "--operation", `{} {}`},
		{"inspect", "--project", root, "unexpected"},
	} {
		run(1, args...)
	}
	proposed := run(0, "plan", "--project", root, "--operation", `{"variant":"linux-amd64","mode":"noalloc"}`)
	var proposal struct {
		Plan projectconfig.Plan `json:"plan"`
	}
	if err := json.Unmarshal([]byte(proposed), &proposal); err != nil {
		t.Fatal(err)
	}
	planFile := filepath.Join(t.TempDir(), "plan.json")
	encoded, _ := json.Marshal(proposal.Plan)
	os.WriteFile(planFile, encoded, 0600)
	run(1, "apply-plan", "--project", root, "--plan", planFile)
	unchanged, _ := os.ReadFile(filepath.Join(root, projectconfig.File))
	if !bytes.Equal(unchanged, data) {
		t.Fatal("no approval wrote")
	}
	run(0, "diff", "--project", root, "--plan", planFile)
	run(0, "apply-plan", "--project", root, "--plan", planFile, "--approve", proposal.Plan.PlanSHA256)
	run(2, "apply-plan", "--project", root, "--plan", planFile, "--approve", proposal.Plan.PlanSHA256)
}

// The workspace's negative result must agree with compiler pre-analysis against
// the same MD-139 snapshot, including unsupported provider activation.
func TestProjectWorkspaceCompilerValidationParity(t *testing.T) {
	linux, _ := parseCompilerTarget("linux-amd64")
	definition, _ := findTargetDefinition(linux)
	for _, extra := range []string{"profile = \"baremetal\"\n", "\n[variant.linux-amd64.allocation]\nmode = \"hosted\"\nprovider = \"sec.system-heap.v1\"\n"} {
		data, err := os.ReadFile("../../testdata/projects/model/sec.toml")
		if err != nil {
			t.Fatal(err)
		}
		// Append selections to the existing Linux Variant table using explicit
		// target-specific overrides, preserving a fully valid project identity.
		text := string(data) + "\n[target.sec.variant.linux-amd64]\n" + extra
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, ".sec"), 0700)
		if err := os.WriteFile(filepath.Join(root, projectconfig.File), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		oldTarget := cliPlanTarget
		cliPlanTarget = "sec"

		problems := validateCLIPlanSelection([]string{filepath.Join(root, "cmd/sec/app.sec")}, linux, definition.Definition)
		cliPlanTarget = oldTarget
		_, diagnostics := projectconfig.Resolve(text)
		if len(problems) == 0 || len(diagnostics) == 0 {
			t.Fatal(problems, diagnostics)
		}
		for _, problem := range problems {
			found := false
			for _, canonical := range diagnostics {
				if strings.Contains(canonical.Message, problem) {
					found = true
				}
			}
			if !found {
				t.Fatal(problems, diagnostics)
			}
		}
	}
}
