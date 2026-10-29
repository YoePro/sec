package main

import (
	"os"
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

// TestIfTryConditionsUnsupportedResultBoundary verifies the current backend
// boundary without weakening valid frontend/IR handling or overwriting output.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/compiler/semantic_ir.md — unsupported lowerings.
func TestIfTryConditionsUnsupportedResultBoundary(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	output := filepath.Join(t.TempDir(), "if-try")
	previous := []byte("existing output")
	if err := os.WriteFile(output, previous, 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/control_flow/if_try/valid.sec", "-o", output)
	if code != 4 || !strings.Contains(stderr, "[unsupported-lowering]") || !strings.Contains(stderr, "sec.result.") {
		t.Fatal(code, stderr)
	}
	current, err := os.ReadFile(output)
	if err != nil || string(current) != string(previous) {
		t.Fatalf("failed build changed output: %q, %v", current, err)
	}
}
