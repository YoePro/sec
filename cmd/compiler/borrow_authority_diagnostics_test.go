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
		ids := []string{"S1161", "S1162", "S1159", "S1160", "S1163", "S1163", "S1161"}
		for index, occurrence := range doc.Occurrences {
			if occurrence.ID == nil || *occurrence.ID != ids[index] || occurrence.Unregistered || occurrence.Name == nil {
				t.Fatalf("%s lost stable borrow identity: %+v", command, occurrence)
			}
			if len(occurrence.Help) != 1 || !strings.Contains(occurrence.Help[0].Text, "is available (still owned), but an active borrow blocks this access") || len(occurrence.Related) != 2 || occurrence.Related[0].Span.Start.Line != occurrence.Primary.Span.Start.Line-1 {
				t.Fatalf("%s lost explanation or borrow origin: %+v", command, occurrence)
			}
		}
		_, human, code := runCLIForDiagnostics(t, args...)
		if code != 3 || strings.Count(human, "not borrow access authority") != 7 {
			t.Fatal(command, code, human)
		}
		for _, id := range ids {
			if !strings.Contains(human, "["+id+"]") {
				t.Fatalf("%s lost human diagnostic ID %s: %s", command, id, human)
			}
		}
	}
}

// Arena carrier escape and invalid mutation authority must retain their IDs
// through all frontend gates; ordinary reference escape keeps its established ID.
// Rules: rules/memory/borrowing.md — §§7,23,32–33;
// rules/tooling/diagnostics.md — §§5,14,15.
func TestAdditionalBorrowDiagnosticIDsCLI(t *testing.T) {
	file := "../../testdata/tooling/borrow_additional_ids_invalid.sec"
	want := []string{"S1165", "S1165", "S1167", "S1167", "S1128", "S1165", "S1163"}
	for _, command := range []string{"sema", "check", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
		args := []string{command, file, "--diagnostic-format=json"}
		if strings.HasPrefix(command, "emit-") {
			args = append(args, "-o", "-")
		}
		stdout, output, code := runCLIForDiagnostics(t, args...)
		doc := decodeOccurrenceDocument(t, output)
		if code != 3 || stdout != "" || len(doc.Occurrences) != len(want) {
			t.Fatal(command, code, stdout, output)
		}
		for index, occurrence := range doc.Occurrences {
			if occurrence.ID == nil || *occurrence.ID != want[index] || occurrence.Unregistered || occurrence.Primary == nil || len(occurrence.Help) != 1 {
				t.Fatalf("%s: %+v", command, occurrence)
			}
		}
	}
}
