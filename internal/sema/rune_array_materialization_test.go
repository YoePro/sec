package sema

import (
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

const runeArraySource = `module main

fn Runes() Result[uint, CollectionError] {
    let text := "héj"
    let runes := try text.ToRuneArray()
    return Ok(runes.Len)
}

fn main() int {
    discard Runes()
    return 0
}
`

// core-library.md § 5.3 (MD-072), package18 §§ 33, 72-73: ToRuneArray
// allocates through the active context, so noalloc rejects it; under a
// hosted profile it has a materialization plan with a shared view of the
// receiver Place, a temporary owner with its own storage and backing domain,
// and a complete growth plan.
func TestRuneArrayMaterializationPlan(t *testing.T) {
	analyze := func(profile string) (*Analyzer, *ast.Program, []Error) {
		p := parser.New(lexer.NewWithFile(runeArraySource, "runes.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		a.SetAllocationProfile(profile)
		return a, program, a.Analyze(program)
	}
	if _, _, errs := analyze("noalloc"); len(errs) != 1 || errs[0].ID != diagnostics.NoAllocViolation {
		t.Fatalf("noalloc errors = %v", errs)
	}
	a, program, errs := analyze("hosted")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	var call *ast.CallExpression
	walkASTValue(reflect.ValueOf(program), func(node any) {
		if candidate, ok := node.(*ast.CallExpression); ok {
			if member, ok := candidate.Callee.(*ast.MemberExpression); ok && member.Property != nil && member.Property.Value == "ToRuneArray" {
				call = candidate
			}
		}
	})
	if call == nil {
		t.Fatal("no ToRuneArray call")
	}
	plan, ok := a.ResolvedRuneArrayMaterializationOf(call)
	if !ok {
		t.Fatal("no materialization plan")
	}
	if plan.Source.Root != "text" || plan.View.MutableAuthority || !plan.View.Checkable() || plan.View.Borrow == 0 {
		t.Fatalf("view = %+v of %+v", plan.View, plan.Source)
	}
	owner := plan.Owner
	if owner.Place.StorageIdentity == 0 || owner.Place.RootID == 0 || owner.BackingDomain == 0 || owner.Place.StorageIdentity == plan.Source.StorageIdentity {
		t.Fatalf("owner = %+v", owner)
	}
	allocation := plan.Allocation
	if allocation.Operation != AllocationStringToRuneArray || allocation.Allocation == 0 || allocation.Transition.Domain != owner.BackingDomain ||
		allocation.ElementType.Kind != RuneType || plan.ResultType.Kind != ResultType {
		t.Fatalf("allocation = %+v", allocation)
	}
}
