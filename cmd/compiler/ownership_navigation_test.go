package main

import (
	"strings"
	"testing"
)

// TestOwnershipNavigationCLI retains named semantic links and repair help in
// the canonical diagnostic occurrence schema at every compiler frontend gate.
// Rules: rules/memory/destruction.md — §33.2(2);
// rules/memory/borrowing.md — §§32(4,8),33(3).
func TestOwnershipNavigationCLI(t *testing.T) {
	for _, file := range []string{"custom_free_partial_move_invalid.sec", "borrow_holder_transfer_invalid.sec"} {
		for _, command := range []string{"sema", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
			args := []string{command, "../../testdata/tooling/" + file, "--diagnostic-format=json"}
			if strings.HasPrefix(command, "emit-") {
				args = append(args, "-o", "-")
			}
			stdout, output, code := runCLIForDiagnostics(t, args...)
			if code != 3 || stdout != "" {
				t.Fatal(command, code, stdout, output)
			}
			occurrences := decodeOccurrenceDocument(t, output).Occurrences
			if file == "custom_free_partial_move_invalid.sec" {
				if len(occurrences) != 1 {
					t.Fatal(occurrences)
				}
				got := occurrences[0]
				if got.ID == nil || *got.ID != "S1085" || len(got.Related) != 1 || got.Related[0].Span.Start.Line != 6 || got.Related[0].Message.Text != "custom free requires complete self" || len(got.Help) != 1 || !strings.Contains(got.Help[0].Text, "requires complete self") || !strings.Contains(got.Help[0].Text, "redesign ownership") {
					t.Fatal(got)
				}
			} else {
				if len(occurrences) != 2 {
					t.Fatal(occurrences)
				}
				for i, got := range occurrences {
					if len(got.Related) != 2 || got.Related[0].Span.Start.Line != []int{4, 15}[i] || got.Related[1].Span.Start.Line != []int{5, 11}[i] || !strings.Contains(got.Related[1].Message.Text, "keeps authority live") || (i == 1 && !strings.Contains(got.Related[0].Message.Text, "previous loop iteration")) {
						t.Fatal(got)
					}
				}
			}
		}
	}
}
