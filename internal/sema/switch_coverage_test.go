package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Enum aliases share one runtime value class. A later alias case is therefore
// a duplicate and its diagnostic must identify the first equivalent case.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §20 "Duplicate compile-time values"
//   - rules/declarations/enums.md — §7 "Value aliases"
//   - rules/corrections/applied/correction24-20260823.md — "Bug 3 — enum aliases bypass duplicate switch-case detection"
func TestSwitchEnumAliasDuplicateIdentifiesEarlierEquivalentCase(t *testing.T) {
	input := `
module main

enum State int {
    Ready = 1,
    AlsoReady = 1,
    Busy = 2,
}

fn Select(state: State) void {
    switch state {
    case State.Ready:
        return
    case State.AlsoReady:
        return
    }
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	if len(errors) != 1 {
		t.Fatalf("errors = %+v, want one duplicate alias diagnostic", errors)
	}
	diagnostic := errors[0]
	if diagnostic.ID != diagnostics.DuplicateSwitchCase ||
		!strings.Contains(diagnostic.Message, "duplicate switch case enum underlying value") {
		t.Fatalf("duplicate alias diagnostic = %+v", diagnostic)
	}
	if diagnostic.PreviousLine != 12 || diagnostic.PreviousColumn != 15 {
		t.Fatalf("previous alias location = %d:%d, want 12:15", diagnostic.PreviousLine, diagnostic.PreviousColumn)
	}

	warnings := analyzer.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "omits known values: Busy") || strings.Contains(warnings[0].Message, "AlsoReady") {
		t.Fatalf("value-class warning = %+v, want only Busy missing", warnings)
	}
}
