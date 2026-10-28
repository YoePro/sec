package sema

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// Reject a move marker at selected borrowed contracts before committing any
// transfer, retaining the caller's value for the following legal borrow/use.
// Rules: rules/declarations/functions.md — §§9–10;
// rules/memory/borrowing.md — §15; rules/memory/copy_move.md — §28.1.
func TestMoveIntoBorrowedParameterDiagnostic(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/move_borrowed/calls_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 8 {
		t.Fatalf("want 8 ownership errors with no follow-up move errors: %v", errors)
	}
	for i, e := range errors {
		if e.ID != diagnostics.MoveIntoBorrowedParameter {
			t.Errorf("unstructured borrowed parameter error: %+v", e)
		}
		if e.Help == "" || !strings.Contains(e.Message, "without transferring ownership") || !strings.Contains(e.Help, "to borrow the value") || e.EndColumn <= e.Column+2 {
			t.Errorf("incomplete mentor diagnostic: %+v", e)
		}
		if (i < 5 || i == 7) && (len(e.Related) != 1 || e.Related[0].Line <= 0 || e.Related[0].EndColumn <= e.Related[0].Column) {
			t.Errorf("missing parameter declaration: %+v", e)
		}
		if i == 1 && !strings.Contains(e.Message, "ref mut Bytes") {
			t.Errorf("lost mutable contract: %+v", e)
		}
		if i == 4 && !strings.Contains(e.Help, "reader.Read(values)") {
			t.Errorf("lost receiver spelling: %+v", e)
		}
		if i == 7 && (!strings.Contains(e.Help, "<-first") || !strings.Contains(e.Help, "second")) {
			t.Errorf("lost other argument's ownership marker: %s", e.Help)
		}
	}
	valid, err := os.ReadFile("../../testdata/sema/move_borrowed/calls_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(valid)); len(errors) != 0 {
		t.Fatalf("legal borrows/transfers rejected: %v", errors)
	}
}
