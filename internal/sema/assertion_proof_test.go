package sema

import (
	"testing"

	"sec/internal/ast"
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
