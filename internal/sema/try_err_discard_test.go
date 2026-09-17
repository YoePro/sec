package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTryErrDiscardRejectsLifecycleError checks that Err(_) cannot silently
// abandon a possible task handle inside a concrete error union.
// Rules: rules/errors/errorhandling.md — §17 "Err(_)";
// rules/control-flow/discard.md — "Recursive discardability".
func TestTryErrDiscardRejectsLifecycleError(t *testing.T) {
	path := "../../testdata/sema/try_err_discard_non_discardable_invalid.sec"
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
		t.Fatalf("discard diagnostics = %+v", errors)
	}
	if len(analyzer.resolvedTryPlans) != 0 {
		t.Fatalf("invalid handler produced resolved try plan: %+v", analyzer.resolvedTryPlans)
	}
}
