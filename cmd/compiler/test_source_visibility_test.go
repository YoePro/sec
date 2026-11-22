package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSecTestCoreHelperVisibility exercises the actual trusted-core loader and
// test command. Listing may succeed before execution exists; private production
// and sibling-test helpers must still be rejected rather than granted access.
// Rules: rules/tooling/testing.md — §§4,7,27–31;
// rules/projects/modules.md — §6; rules/library/core-library.md — §1.2.
func TestSecTestCoreHelperVisibility(t *testing.T) {
	root := t.TempDir()
	core := filepath.Join(root, "sec", "core")
	for _, dir := range []string{core, filepath.Join(root, "sec", "platform")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	copyFixture := func(name string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("../../testdata/core/test_visibility", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(core, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"helpers.sec", "helpers_test.sec", "sibling_test.sec"} {
		copyFixture(file)
	}
	for _, list := range []bool{true, false} {
		args := []string{"test", "--json"}
		wantCode, wantStatus := testExitExecutionUnavailable, testExecutionUnavailable
		if list {
			args = append(args, "--list")
			wantCode, wantStatus = testExitSuccess, testListed
		}
		stdout, stderr, code := runCLIForDiagnostics(t, append(args, core)...)
		if code != wantCode {
			t.Fatalf("list=%v exit %d: %s\n%s", list, code, stdout, stderr)
		}
		var report testRunReport
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatal(err, stdout)
		}
		if len(report.Modules) != 1 || !report.Modules[0].Compiled || report.Summary.Selected != 2 || report.Summary.Passed != 0 {
			t.Fatalf("unexpected report: %+v", report)
		}
		for _, test := range report.Modules[0].Tests {
			if test.Module != "core" || test.Status != wantStatus {
				t.Fatal(test)
			}
		}
	}
	copyFixture("visibility_invalid_test.sec")
	_, stderr, code := runCLIForDiagnostics(t, "test", "--list", "--json", core)
	if code != testExitCompilationFailure || !strings.Contains(stderr, "function __privateProduction is not accessible") ||
		!strings.Contains(stderr, "function __privateTest is not accessible") || strings.Contains(stderr, "function _shared") {
		t.Fatalf("privacy boundary exit %d: %s", code, stderr)
	}
}
