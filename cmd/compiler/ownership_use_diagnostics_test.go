package main

import (
	"strings"
	"testing"
)

// TestOwnershipUseOriginCLI retains mandatory IDs, explanation and both
// terminal-action/initial-owner links at every compiler frontend gate.
// Rules: rules/memory/copy_move.md — §28.1; rules/memory/destruction.md — §33.2(1).
func TestOwnershipUseOriginCLI(t *testing.T) {
	for _, command := range []string{"sema", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
		args := []string{command, "../../testdata/tooling/ownership_use_origin_invalid.sec", "--diagnostic-format=json"}
		if strings.HasPrefix(command, "emit-") {
			args = append(args, "-o", "-")
		}
		stdout, output, code := runCLIForDiagnostics(t, args...)
		if code != 3 || stdout != "" {
			t.Fatal(command, code, output)
		}
		got := decodeOccurrenceDocument(t, output)
		if len(got.Occurrences) != 9 {
			t.Fatal(got)
		}
		for i, value := range got.Occurrences {
			if value.ID == nil || len(value.Help) != 1 || len(value.Related) != 2 || value.Related[0].Span.Start.Line != []int{10, 16, 21, 26, 31, 36, 41, 45, 52}[i] || value.Related[1].Span.Start.Line != []int{9, 15, 20, 25, 30, 35, 40, 44, 51}[i] {
				t.Fatal(value)
			}
			if value.Related[1].Message.Text != "source ownership established here" || !strings.HasPrefix(value.Related[0].Message.Text, "ownership ") {
				t.Fatal(value)
			}
			if i == 4 && !strings.Contains(value.Help[0].Text, "OkRef") || i == 5 && !strings.Contains(value.Help[0].Text, "ErrRef") {
				t.Fatal(value)
			}
		}
	}
}
