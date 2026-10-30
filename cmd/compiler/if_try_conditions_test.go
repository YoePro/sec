package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sec/internal/testsupport"
	"strings"
	"testing"
)

// TestIfTryConditionDiagnosticsCLI checks that failure propagation does not
// supply truthiness for non-bool successful payloads.
// Rules: rules/control-flow/flowcontrol_if.md — §§11,27–28.
func TestIfTryConditionDiagnosticsCLI(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/control_flow/if_try/non_bool_invalid.sec", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, stderr)
	if code != 3 || len(doc.Occurrences) != 4 {
		t.Fatal(code, stderr)
	}
	for _, occurrence := range doc.Occurrences {
		if occurrence.Primary == nil || !strings.Contains(occurrence.Message.Text, "bool") || !strings.Contains(occurrence.Message.Text, "if condition") {
			t.Fatal(occurrence)
		}
	}
}

// TestIfTryConditionsRunNatively executes propagating, recovering and negated
// if-try conditions over a scalar Result (lowering v11.33); the fixture's
// status names the first failed expectation.
// Rules: rules/control-flow/flowcontrol_if.md — §§11,27–28.
func TestIfTryConditionsRunNatively(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "if-try")
	_, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/control_flow/if_try/valid.sec", "-o", program)
	if code != 0 {
		t.Fatal(code, stderr)
	}
	if err := exec.Command(program).Run(); err != nil {
		t.Fatalf("status: %v", err)
	}
}

// TestIfTryConditionsUnsupportedResultBoundary verifies the current backend
// boundary for a Result payload without a native representation, without
// weakening valid frontend/IR handling or overwriting output.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/compiler/semantic_ir.md — unsupported lowerings.
func TestIfTryConditionsUnsupportedResultBoundary(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	output := filepath.Join(t.TempDir(), "if-try")
	previous := []byte("existing output")
	if err := os.WriteFile(output, previous, 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/control_flow/if_try/unsupported_string_payload.sec", "-o", output)
	if code != 4 || !strings.Contains(stderr, "[unsupported-lowering]") || !strings.Contains(stderr, "sec.result.") {
		t.Fatal(code, stderr)
	}
	current, err := os.ReadFile(output)
	if err != nil || string(current) != string(previous) {
		t.Fatalf("failed build changed output: %q, %v", current, err)
	}
}
