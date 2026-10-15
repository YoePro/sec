package main

import (
	"strings"
	"testing"
)

// Recursive origin inference must be shared by frontend reports and emission
// gates; Unknown or local returned origins never become permission to lower.
// Rules: rules/memory/lifetime_analysis.md — §28.2–28.3;
// rules/compiler/compiler_pipeline.md — semantic validation before lowering.
func TestRecursiveReferenceSummariesCLI(t *testing.T) {
	for _, command := range []string{"sema", "analyse"} {
		stdout, stderr, code := runCLIForDiagnostics(t, command, "../../testdata/sema/reference_recursive_summaries.sec")
		if code != 0 {
			t.Fatal(command, code, stdout, stderr)
		}
	}
	for _, test := range []struct{ file, message string }{
		{"reference_recursive_unseeded_invalid.sec", "unknown control-flow provenance"},
		{"reference_recursive_local_invalid.sec", "cannot return reference to local"},
	} {
		for _, command := range []string{"sema", "analyse", "emit-ir"} {
			args := []string{command, "../../testdata/sema/" + test.file}
			if command == "emit-ir" {
				args = append(args, "-o", "-")
			}
			stdout, stderr, code := runCLIForDiagnostics(t, args...)
			if code != 3 || !strings.Contains(stderr, test.message) {
				t.Fatal(command, code, stdout, stderr)
			}
		}
	}
}
