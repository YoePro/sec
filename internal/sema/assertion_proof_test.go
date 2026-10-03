package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Assertions the compiler proves true contribute no panic effect and are
// valid in @noPanic code: range contracts and integer type ranges, constant
// comparisons, logical composition, and an identical dominating condition
// not invalidated by mutation. Unproven assertions keep MayPanic.
//
// Rules:
//   - rules/errors/panic.md — § 15.6 "Assertion refinement", § 15.8 "Assertions in @noPanic"
func TestAssertionProofBeyondLiteralTrue(t *testing.T) {
	source := `module main

type NonNegative int range 0..1000000
type Small int range 1..9

@noPanic
fn ContractRange(value: NonNegative) int {
    assert value >= 0
    return 0
}

@noPanic
fn Composed(small: Small, count: uint) int {
    assert small >= 1 && small <= 9 && count >= 0
    assert small >= 1 || small == 3
    return 0
}

@noPanic
fn Dominated(value: int) int {
    if value > 3 {
        assert value > 3
    }
    return 0
}

fn Invalidated(value: int) int {
    let mut current := value
    if current > 3 {
        current = 0
        assert current > 3
    }
    return current
}

fn Unproven(value: int) int {
    assert value > 3
    return value
}
`
	program := parser.New(lexer.New(source)).ParseProgram()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
	graph := analyzer.CallGraph()
	for name, wantPanic := range map[string]bool{
		"ContractRange": false,
		"Composed":      false,
		"Dominated":     false,
		"Invalidated":   true,
		"Unproven":      true,
	} {
		if got := graph.EffectSummary(callGraphNodeIDByName(t, graph, name)).MayPanic; got != wantPanic {
			t.Errorf("%s MayPanic = %v, want %v", name, got, wantPanic)
		}
	}
	proven := 0
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok || function.Name.Value != "Composed" {
			continue
		}
		for _, inner := range function.Body.Statements {
			if assertion, ok := inner.(*ast.AssertStatement); ok {
				if fact, found := analyzer.ResolvedAssertionOf(assertion); found && fact.Proven {
					proven++
				}
			}
		}
	}
	if proven != 2 {
		t.Fatalf("proven assertions in Composed = %d, want 2", proven)
	}
}

// Dominating comparisons prove symbolic relations between the same two values
// (`a < b` implies `a <= b`, `a != b`, and the mirrored `b > a`), narrow a
// value's interval when compared against constants, and those refined
// intervals carry through `+` and `-`. A relation that is not implied, a
// different pair, and a fact invalidated by assignment keep MayPanic.
//
// Rules:
//   - rules/errors/panic.md — § 15.6 "Assertion refinement", § 15.8 "Assertions in @noPanic"
func TestAssertionProofUsesSymbolicRelationsAndRefinedIntervals(t *testing.T) {
	source := `module main

@noPanic
fn Implied(low: int, high: int) int {
    if low < high {
        assert low <= high
        assert low != high
        assert high > low
    }
    return 0
}

@noPanic
fn FromConjunction(low: int, high: int, enabled: bool) int {
    if enabled && low <= high {
        assert high >= low
    }
    return 0
}

@noPanic
fn Refined(value: int) int {
    if value < 10 {
        assert value <= 20
    }
    return 0
}

fn RefinedArithmetic(value: int) int {
    if value >= 0 && value < 100 {
        assert value + 1 > 0
        assert value - 1 < 99
    }
    return 0
}

fn NotImplied(low: int, high: int) int {
    if low <= high {
        assert low < high
    }
    return 0
}

fn DifferentPair(low: int, high: int, other: int) int {
    if low < high {
        assert low < other
    }
    return 0
}

fn RelationInvalidated(low: int, high: int) int {
    let mut current := low
    if current < high {
        current = high
        assert current < high
    }
    return current
}
`
	program := parser.New(lexer.New(source)).ParseProgram()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
	graph := analyzer.CallGraph()
	for name, wantPanic := range map[string]bool{
		"Implied":             false,
		"FromConjunction":     false,
		"Refined":             false,
		"NotImplied":          true,
		"DifferentPair":       true,
		"RelationInvalidated": true,
	} {
		if got := graph.EffectSummary(callGraphNodeIDByName(t, graph, name)).MayPanic; got != wantPanic {
			t.Errorf("%s MayPanic = %v, want %v", name, got, wantPanic)
		}
	}
	// The arithmetic itself keeps its own checked-overflow effect; the
	// assertions over the refined `+`/`-` intervals are proven.
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok || function.Name.Value != "RefinedArithmetic" {
			continue
		}
		guarded := function.Body.Statements[0].(*ast.IfStatement)
		for _, nested := range guarded.Consequence.Statements {
			assertion, found := analyzer.ResolvedAssertionOf(nested.(*ast.AssertStatement))
			if !found || !assertion.Proven {
				t.Errorf("assertion %s proven = %v, want proven", nested.(*ast.AssertStatement).Condition.String(), assertion.Proven)
			}
		}
	}
}

// A non-bool assertion condition is a stable S1102 type error whose mentor
// help names the explicit test for the condition's type, since Sec applies
// no truthiness conversion.
//
// Rules:
//   - rules/errors/panic.md — § 15.2 "Condition typing", § 28(2), § 28(4)
func TestAssertConditionTypeErrorIsMentorDiagnostic(t *testing.T) {
	errors := analyzeSource(t, `
module main

fn Check(count: int, name: string, maybe: Option[int]) void {
    assert count
    assert name
    assert maybe
}
`)
	wantHelp := []string{"assert count != 0", "assert name.Len != 0", "assert maybe is not None"}
	if len(errors) != len(wantHelp) {
		t.Fatalf("errors = %v", errors)
	}
	for index, help := range wantHelp {
		if errors[index].ID != diagnostics.AssertConditionNotBool || !strings.Contains(errors[index].Help, help) {
			t.Fatalf("error %d = %+v, want S1102 suggesting %q", index, errors[index], help)
		}
	}
}
