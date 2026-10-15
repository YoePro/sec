package main

import (
	"strings"
	"testing"
)

// Retained syntax and new skip events must never let malformed source reach
// analysis, Semantic IR or either backend through an ordinary compilation gate.
// Rules: rules/compiler/parser_recovery.md — "Error blocks code generation", "Compiler result".
func TestSpecializedParserRecoveryBlocksCompilation(t *testing.T) {
	for _, file := range []string{"specialized_skip_ranges_invalid.sec", "specialized_skip_boundaries_invalid.sec"} {
		for _, command := range []string{"sema", "check", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
			args := []string{command, "../../testdata/parser/" + file, "--diagnostic-format=json"}
			if strings.HasPrefix(command, "emit-") {
				args = append(args, "-o", "-")
			}
			stdout, output, code := runCLIForDiagnostics(t, args...)
			document := decodeOccurrenceDocument(t, output)
			if code != 2 || stdout != "" || document.Summary.Errors == 0 {
				t.Fatalf("%s %s: code=%d stdout=%s diagnostics=%s", command, file, code, stdout, output)
			}
			for _, occurrence := range document.Occurrences {
				if occurrence.ID == nil || occurrence.Unregistered || occurrence.Primary == nil {
					t.Fatalf("%s %s lost structured parser diagnostics: %+v", command, file, occurrence)
				}
			}
		}
	}
}
