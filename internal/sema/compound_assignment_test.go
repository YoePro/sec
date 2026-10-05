package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
)

// A compound assignment and its `++`/`--` alias apply the underlying checked
// operator: Sema publishes that operator for the statement and records its
// arithmetic panic effect, so `@noPanic` rejects `i += 1` exactly as it
// rejects `i = i + 1`. Bitwise compound operators cannot fail.
//
// Rules:
//   - rules/foundations/operators.md — "Compound assignment", "Compound arithmetic failure", "Increment and decrement aliases"
//   - rules/errors/panic.md — arithmetic panic effects and @noPanic
func TestCompoundAssignmentPublishesItsCheckedOperator(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
@noPanic
fn Add(x: int) int {
    let mut i := x
    i += 1
    return i
}

@noPanic
fn Increment(x: uint8) uint8 {
    let mut i := x
    i++
    return i
}

@noPanic
fn Mask(x: int) int {
    let mut i := x
    i &= 255
    i ^= 1
    return i
}

fn Index(values: int[4]) int {
    let mut copy := values
    copy[0] -= 1
    return copy[0]
}
`)
	violations := map[string]bool{}
	for _, err := range errors {
		if err.ID != "S1092" {
			t.Errorf("unexpected error: %s", err.Message)
			continue
		}
		for _, name := range []string{"Add", "Increment", "Mask"} {
			if strings.Contains(err.Message, "function "+name+" ") {
				violations[name] = true
			}
		}
	}
	if !violations["Add"] || !violations["Increment"] || violations["Mask"] {
		t.Fatalf("@noPanic violations = %v, want Add and Increment only", violations)
	}

	kinds := map[string]ResolvedOperatorKind{}
	for stmt := range analyzer.resolvedCompoundAssignments {
		resolved, ok := analyzer.ResolvedCompoundAssignmentOf(stmt)
		if !ok {
			t.Fatalf("missing compound fact for %s", stmt.Operator)
		}
		kinds[stmt.Operator+" "+assignmentTargetName(stmt)] = resolved.Kind
		if resolved.RuntimeCheck != (resolved.FailureBehavior == OperatorArithmeticFailure) {
			t.Errorf("%s check metadata = %+v", stmt.Operator, resolved)
		}
	}
	want := map[string]ResolvedOperatorKind{
		"+= i": ResolvedIntegerAddChecked, "&= i": ResolvedIntegerBitAnd, "^= i": ResolvedIntegerBitXor,
		"-= copy": ResolvedIntegerSubtractChecked,
	}
	for key, kind := range want {
		if kinds[key] != kind {
			t.Errorf("%s = %q, want %q (all: %v)", key, kinds[key], kind, kinds)
		}
	}
}

func assignmentTargetName(stmt *ast.AssignmentStatement) string {
	switch target := stmt.Target.(type) {
	case *ast.Identifier:
		return target.Value
	case *ast.IndexExpression:
		if identifier, ok := target.Left.(*ast.Identifier); ok {
			return identifier.Value
		}
	}
	return "?"
}
