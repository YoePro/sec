package sema

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// errorhandling.md § 31 and lsp.md (try hover): a try converts only the
// failures it handles; a call whose target may panic inside the protected
// expression remains a panic path, while converted checked arithmetic and
// panic-free calls leave none.
func TestResolvedTryRemainingPanics(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

enum Failure error {
    Missing,
}

fn Risky(value: int32) Result[int32, Failure] {
    assert value != 7
    return Ok(value)
}

fn Safe(value: int32) Result[int32, Failure] {
    return Ok(value)
}

fn Sum(a: int32, b: int32) Result[int32, ArithmeticError] {
    let total := try a + b
    return Ok(total)
}

fn Use(a: int32) Result[int32, Failure] {
    let risky := try Risky(a)
    let safe := try Safe(a)
    return Ok(risky + safe)
}
`)
	assertSemaErrors(t, errors, nil)
	remaining := map[string][]TryRemainingPanic{}
	for expr := range analyzer.tryProtections {
		panics, ok := analyzer.ResolvedTryRemainingPanicsOf(expr)
		if !ok {
			t.Fatalf("try %s has no remaining-panic fact", expr.String())
		}
		remaining[tryOperandName(expr)] = panics
	}
	if len(remaining["a + b"]) != 0 || len(remaining["Safe"]) != 0 {
		t.Fatalf("converted arithmetic and a non-panicking call leave no panic path: %+v", remaining)
	}
	risky := remaining["Risky"]
	if len(risky) != 1 || risky[0].Callee != "Risky" || len(risky[0].PanicReasonIDs) != 1 || risky[0].PanicReasonIDs[0] != diagnostics.PanicReasonAssertionFailure {
		t.Fatalf("try Risky(a) keeps Risky's assertion panic, got %+v", risky)
	}
}

func tryOperandName(expr *ast.TryExpression) string {
	if call, ok := expr.Expression.(*ast.CallExpression); ok {
		return call.Callee.String()
	}
	return expr.Expression.String()
}
