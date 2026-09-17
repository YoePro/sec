package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMatchErrDiscardRejectsLifecycleError verifies that a match Err(_)
// cannot silently abandon a task handle inside a concrete error union.
// Rules: rules/control-flow/flowcontrol_match.md — §7 "Result errors must not be hidden";
// rules/errors/errorhandling.md — §17 "Err(_)".
func TestMatchErrDiscardRejectsLifecycleError(t *testing.T) {
	path := "../../testdata/sema/match_err_discard_non_discardable_invalid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if parsed.HasErrors {
		t.Fatalf("parser diagnostics = %+v", parsed.Diagnostics)
	}
	analyzer := NewAnalyzer()
	errors := analyzer.Analyze(parsed.Program)
	if len(errors) != 1 || errors[0].ID != diagnostics.NonDiscardableValue ||
		!strings.Contains(errors[0].Message, "Err(_) cannot ignore Failure") ||
		!strings.Contains(errors[0].Help, "Bind the error") {
		t.Fatalf("match discard diagnostics = %+v", errors)
	}
	if len(analyzer.resolvedMatchPlans) != 0 {
		t.Fatalf("invalid match produced resolved plan: %+v", analyzer.resolvedMatchPlans)
	}
}
