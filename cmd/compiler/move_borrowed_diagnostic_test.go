package main

import (
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// Every frontend gate transports the selected borrow contract, help and real
// declaration locations; no backend tool is needed to reject an invalid move.
// Rules: rules/declarations/functions.md — §§7,9–10;
// rules/memory/copy_move.md — §28.1; rules/tooling/diagnostics.md — §§8–9,14.
func TestMoveIntoBorrowedParameterCLI(t *testing.T) {
	const base = "../../testdata/sema/move_borrowed/"
	_, output, code := runCheckWithoutTools(t, "check", base+"calls_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "sema", "emit-ir"} {
		args := []string{command, base + "calls_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", "-")
		}
		stdout, output, code := runCLIForDiagnostics(t, args...)
		if code != 3 || stdout != "" {
			t.Fatal(command, code, stdout, output)
		}
		document := decodeOccurrenceDocument(t, output)
		if len(document.Occurrences) != 8 {
			t.Fatal(command, output)
		}
		for _, e := range document.Occurrences {
			if e.ID == nil || *e.ID != diagnostics.MoveIntoBorrowedParameter || len(e.Help) != 1 || !strings.Contains(e.Message.Text, "without transferring ownership") || !strings.Contains(e.Help[0].Text, "to borrow the value") || e.Primary.Span.End.Column <= e.Primary.Span.Start.Column+2 {
				t.Fatalf("%s lost mentor metadata: %+v", command, e)
			}
			if len(e.Related) != 1 || e.Related[0].Message == nil || e.Related[0].Message.Text != "borrowed parameter declared here" {
				t.Fatalf("%s lost parameter location: %+v", command, e)
			}
		}
	}
}
