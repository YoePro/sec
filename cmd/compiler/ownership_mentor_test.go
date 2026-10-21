package main

import (
	"strings"
	"testing"
)

// TestOwnershipMentorCLI projects canonical ownership/overlap explanations and
// their declaration/origin links through all compiler frontend gates.
// Rules: rules/memory/copy_move.md — §28; rules/memory/borrowing.md — §32.
func TestOwnershipMentorCLI(t *testing.T) {
	for _, file := range []string{"ownership_call_mentor_invalid.sec", "borrow_overlap_mentor_invalid.sec"} {
		for _, command := range []string{"sema", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
			args := []string{command, "../../testdata/tooling/" + file, "--diagnostic-format=json"}
			if strings.HasPrefix(command, "emit-") {
				args = append(args, "-o", "-")
			}
			stdout, output, code := runCLIForDiagnostics(t, args...)
			if code != 3 || stdout != "" {
				t.Fatal(command, code, stdout, output)
			}
			doc := decodeOccurrenceDocument(t, output)
			count := 10
			if strings.HasPrefix(file, "borrow") {
				count = 5
			}
			if len(doc.Occurrences) != count {
				t.Fatal(doc.Occurrences)
			}
			for i, got := range doc.Occurrences {
				if len(got.Help) != 1 || len(got.Related) == 0 {
					t.Fatal(command, got)
				}
				help := got.Help[0].Text
				if count == 10 {
					if got.ID == nil || *got.ID != "S1007" {
						t.Fatal(got)
					}
					if i == 2 {
						if !strings.Contains(help, "`<-resource`") || !strings.Contains(help, "To require transfer at every call") || !strings.Contains(got.Message.Text, "copies its argument") {
							t.Fatal(got)
						}
					} else if !strings.Contains(help, "Write `<-") {
						t.Fatal(got)
					}
				} else if !strings.Contains(help, "storage") && !strings.Contains(help, "contains") {
					t.Fatal(got)
				}
			}
		}
	}
}
