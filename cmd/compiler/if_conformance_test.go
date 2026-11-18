package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/testsupport"
)

// TestIfConformanceTruthinessCLI rejects each non-boolean condition family
// at all frontend/emission gates, before any artifact can be emitted.
// Rules: rules/control-flow/flowcontrol_if.md — §§3,26–28.
func TestIfConformanceTruthinessCLI(t *testing.T) {
	for _, command := range []string{"check", "sema", "emit-ir", "emit-mlir", "emit-llvm"} {
		t.Run(command, func(t *testing.T) {
			args := []string{command, "../../testdata/control_flow/if_conformance/truthiness_invalid.sec", "--diagnostic-format=json"}
			if strings.HasPrefix(command, "emit-") {
				args = append(args, "-o", "-")
			}
			stdout, stderr, code := runCLIForDiagnostics(t, args...)
			doc := decodeOccurrenceDocument(t, stderr)
			if code != 3 || stdout != "" || doc.Summary.Errors != 9 || len(doc.Occurrences) != 9 {
				t.Fatal(code, stdout, stderr)
			}
			for i, d := range doc.Occurrences {
				if d.ID == nil || *d.ID != diagnostics.IfConditionNotBool || len(d.Help) == 0 || d.Severity != "error" || d.Primary == nil || d.Primary.Span.Start.Line != 10+i || !strings.Contains(d.Message.Text, "if condition must be bool") {
					t.Fatalf("incomplete or misplaced condition diagnostic: %+v", d)
				}
			}
		})
	}
}

// TestIfConformanceNative checks observable evaluation order, skipped RHS and
// later branches, exactly one selected body, grouping, negation and no-else joins
// through the single Sema -> Semantic IR -> Sec MLIR -> LLVM compilation track.
// Rules: rules/control-flow/flowcontrol_if.md — §§4–6,19,21–22,27.
func TestIfConformanceNative(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	output := filepath.Join(t.TempDir(), "if-conformance")
	_, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/control_flow/if_conformance/native.sec", "-o", output)
	if code != 0 {
		t.Fatal(code, stderr)
	}
	if out, err := exec.Command(output).CombinedOutput(); err != nil {
		t.Fatalf("if conformance: %v\n%s", err, out)
	}
}
