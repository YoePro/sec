package main

import (
	"strings"
	"testing"
)

// TestBorrowAuthorityDiagnosticsCLI checks human and structured diagnostics at
// every frontend gate: ownership presence is not presented as access permission.
// Rules: rules/memory/borrowing.md — §32(7–8),§33(1);
// rules/tooling/diagnostics.md — §§9,17.
func TestBorrowAuthorityDiagnosticsCLI(t *testing.T) {
	file := "../../testdata/tooling/borrow_authority_invalid.sec"
	for _, command := range []string{"sema", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
		args := []string{command, file}
		if strings.HasPrefix(command, "emit-") {
			args = append(args, "-o", "-")
		}
		stdout, output, code := runCLIForDiagnostics(t, append(args, "--diagnostic-format=json")...)
		doc := decodeOccurrenceDocument(t, output)
		if code != 3 || stdout != "" || len(doc.Occurrences) != 7 || doc.Summary.Errors != 7 {
			t.Fatalf("%s: %d %s %s", command, code, stdout, output)
		}
		for _, occurrence := range doc.Occurrences {
			if len(occurrence.Help) != 1 || !strings.Contains(occurrence.Help[0].Text, "is available (still owned), but an active borrow blocks this access") || len(occurrence.Related) != 2 || occurrence.Related[0].Span.Start.Line != occurrence.Primary.Span.Start.Line-1 {
				t.Fatalf("%s lost explanation or borrow origin: %+v", command, occurrence)
			}
		}
		_, human, code := runCLIForDiagnostics(t, args...)
		if code != 3 || strings.Count(human, "not borrow access authority") != 7 {
			t.Fatal(command, code, human)
		}
	}
}
