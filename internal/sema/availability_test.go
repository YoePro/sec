package sema

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAvailabilityTestsResolveStaticOwnedAndMovedPlaces verifies that queries
// observe ownership without reading the tested Place or inventing provenance.
//
// Rules:
//   - rules/memory/ownership.md — §§5 and 21
//   - rules/corrections/applied/correction30-20260828.md — §§1–3
func TestAvailabilityTestsResolveStaticOwnedAndMovedPlaces(t *testing.T) {
	source := `module main
@noCopy
type Resource struct { Value: int }
fn Owned(resource: Resource) int {
    if resource is available { return resource.Value }
    return 0
}
fn Moved(resource: Resource) int {
    let moved :<- resource
    if resource is not available { return moved.Value }
    return 0
}
`
	p := parser.New(lexer.New(source))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %#v", result.Diagnostics)
	}
	a := NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %#v", errors)
	}
	owned := result.Program.Statements[2].(*ast.FunctionDeclaration)
	moved := result.Program.Statements[3].(*ast.FunctionDeclaration)
	positive := owned.Body.Statements[0].(*ast.IfStatement).Condition.(*ast.AvailabilityExpression)
	negative := moved.Body.Statements[1].(*ast.IfStatement).Condition.(*ast.AvailabilityExpression)
	positiveFact, ok := a.ResolvedAvailabilityTestOf(positive)
	if !ok || !positiveFact.StaticallyKnown || !positiveFact.Value || positiveFact.Negated {
		t.Fatalf("positive availability = %#v, %t", positiveFact, ok)
	}
	negativeFact, ok := a.ResolvedAvailabilityTestOf(negative)
	if !ok || !negativeFact.StaticallyKnown || !negativeFact.Value || !negativeFact.Negated {
		t.Fatalf("negative availability = %#v, %t", negativeFact, ok)
	}
}

// TestConditionalAvailabilityRefinesBothContinuingPaths exercises the current
// frontend's binary Available|Unavailable state at a join. The positive branch
// and the fallthrough after a terminating negative branch both regain legal
// access without converting ownership into Option/null semantics.
//
// Rules:
//   - rules/memory/ownership.md — §§20–21
//   - rules/corrections/applied/correction30-20260828.md — §§2–3
func TestConditionalAvailabilityRefinesBothContinuingPaths(t *testing.T) {
	source := `module main
@noCopy
type Resource struct { Value: int }
fn Check(resource: Resource, consume: bool) int {
    if consume {
        let moved :<- resource
        discard moved
    }
    if resource is available {
        return resource.Value
    }
    return 0
}
fn CheckNegative(resource: Resource, consume: bool) int {
    if consume {
        let moved :<- resource
        discard moved
    }
    if resource is not available {
        return 0
    }
    return resource.Value
}
`
	p := parser.New(lexer.New(source))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %#v", result.Diagnostics)
	}
	a := NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %#v", errors)
	}
	for _, statementIndex := range []int{2, 3} {
		function := result.Program.Statements[statementIndex].(*ast.FunctionDeclaration)
		test := function.Body.Statements[1].(*ast.IfStatement).Condition.(*ast.AvailabilityExpression)
		fact, ok := a.ResolvedAvailabilityTestOf(test)
		if !ok || fact.StaticallyKnown {
			t.Fatalf("%s conditional availability = %#v, %t", function.Name.Value, fact, ok)
		}
	}
}

// TestPartialAggregateNegativeAvailabilityPreservesSibling proves the exact
// Place rule: the whole aggregate is not available, while an owned sibling
// remains usable inside the negative branch.
//
// Rules:
//   - rules/memory/ownership.md — §§5.4 and 21
//   - rules/corrections/applied/correction30-20260828.md — §§1–3
func TestPartialAggregateNegativeAvailabilityPreservesSibling(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main
@noCopy
type Resource struct { Value: int }
type Pair struct { First: Resource, Second: int }
fn Check(pair: Pair) int {
    let first :<- pair.First
    if pair is not available { return pair.Second }
    return 0
}
`)
	if len(errors) != 0 {
		t.Fatalf("sema: %#v", errors)
	}
}
