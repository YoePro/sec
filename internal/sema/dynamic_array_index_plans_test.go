package sema

import (
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// P18-09/10: every owning-array index publishes a plan that keeps its
// runtime bounds check (ordinary panic.bounds with the dynamic-array-index
// operation, or IndexError under a valid try), records the use and transfer
// action from its context, never changes the backing domain, marks element
// borrows epoch-dependent, and names the indexed owner's backing domain.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 40–45
func TestDynamicArrayIndexPlans(t *testing.T) {
	const source = `module main

fn Use(index: uint) Result[int, CollectionError] {
    let mut values: int[]
    try values.Append(1)
    let read := values[index]
    values[0] = 3
    let shared := ref values[index]
    let copied := *shared
    let exclusive := ref mut values[index]
    *exclusive = 4
    return Ok(read + copied)
}

fn Fallible(values: int[], index: uint) Result[int, IndexError] {
    let value := try values[index]
    return Ok(value)
}
`
	parsed := parser.New(lexer.New(source))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) > 0 {
		t.Fatal(parsed.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	var indexes []*ast.IndexExpression
	walkASTValue(reflect.ValueOf(program), func(node any) {
		if index, ok := node.(*ast.IndexExpression); ok {
			indexes = append(indexes, index)
		}
	})
	if len(indexes) != 5 {
		t.Fatalf("found %d index expressions", len(indexes))
	}
	want := []struct {
		use      ArrayIndexUseKind
		action   ResolvedArrayTransferAction
		failure  ArrayIndexFailureMode
		epoch    bool
		constant string
	}{
		{ArrayIndexRead, ArrayTransferCopyTrivial, ArrayIndexFailureOrdinary, false, ""},
		{ArrayIndexWrite, ArrayTransferConstructDirect, ArrayIndexFailureOrdinary, false, "0"},
		{ArrayIndexBorrow, ArrayTransferBorrowShared, ArrayIndexFailureOrdinary, true, ""},
		{ArrayIndexMutBorrow, ArrayTransferBorrowMutable, ArrayIndexFailureOrdinary, true, ""},
		{ArrayIndexRead, ArrayTransferCopyTrivial, ArrayIndexFailureFallible, false, ""},
	}
	var domain InvalidationDomainID
	for i, expected := range want {
		plan, ok := analyzer.ResolvedDynamicArrayIndexPlanOf(indexes[i])
		if !ok {
			t.Fatalf("index %d has no dynamic-array plan", i)
		}
		constant := ""
		if plan.ConstantIndex != nil {
			constant = plan.ConstantIndex.String()
		}
		if plan.UseKind != expected.use || plan.Action != expected.action || plan.FailureMode != expected.failure ||
			plan.EpochDependent != expected.epoch || constant != expected.constant {
			t.Errorf("index %d plan %+v", i, plan)
		}
		if plan.CheckKind != ArrayIndexRuntimeCheck || plan.ProofKind != "" || plan.PanicOperation != DynamicArrayIndexPanicOperation ||
			plan.BackingTransition != BackingTransitionNone || plan.ElementType.Kind != IntType || plan.IndexSigned != (i == 1) ||
			plan.ErrorType.Name != "IndexError" {
			t.Errorf("index %d invariant facts %+v", i, plan)
		}
		if i < 4 {
			if plan.Owner == nil || plan.Owner.BackingDomain == 0 || domain != 0 && plan.Owner.BackingDomain != domain {
				t.Errorf("index %d owner %+v", i, plan.Owner)
			} else {
				domain = plan.Owner.BackingDomain
			}
		}
	}
	first, _ := analyzer.ResolvedDynamicArrayIndexPlanOf(indexes[1])
	first.ConstantIndex.SetInt64(7)
	first.Owner.Place.Root = "mutated"
	if again, _ := analyzer.ResolvedDynamicArrayIndexPlanOf(indexes[1]); again.ConstantIndex.Int64() != 0 || again.Owner.Place.Root == "mutated" {
		t.Fatal("dynamic-array index plans alias analyzer state")
	}
}

// P18-10/11/12: a move-only element is never moved out by runtime index
// (borrow it instead), a structural mutation is rejected while an element or
// slice borrow of the old backing epoch lives, an unproven index violates
// @noPanic while try satisfies it, and Append fails through its Result.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 38, 43, 48, 71, 113–115
func TestDynamicArrayIndexRestrictionsAndEffects(t *testing.T) {
	prelude := `module main

type Res struct {
    id: int,
}

impl Res {
    free {
    }
}

`
	cases := []struct {
		name string
		body string
		want string
	}{
		{"move-out by runtime index", `fn F(handles: Res[], i: uint) int {
    let taken := handles[i]
    return 0
}`, "cannot copy non-copyable element handles[*] out of a runtime index"},
		{"explicit indexed extraction", `fn F(handles: Res[], i: uint) int {
    let taken :<- handles[i]
    return 0
}`, "explicit indexed extraction is not implemented"},
		{"append across element borrow", `fn F() Result[void, CollectionError] {
    let mut values: int[]
    try values.Append(1)
    let first := ref values[0]
    try values.Append(2)
    let read := *first
    return Ok()
}`, "cannot Append values while it is shared borrowed"},
		{"append across slice borrow", `fn F() Result[void, CollectionError] {
    let mut values: int[]
    try values.Append(1)
    let view := ref values[..]
    try values.Append(2)
    let n := view.Len
    return Ok()
}`, "cannot Append values while it is shared borrowed"},
		{"noPanic rejects unproven index", `@noPanic
fn F(values: int[], i: uint) int {
    return values[i]
}`, "does not satisfy @noPanic: reachable may-panic-bounds effect"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prelude+test.body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) {
					return
				}
			}
			t.Fatalf("no diagnostic containing %q; got %v", test.want, errors)
		})
	}

	valid := analyzeSourceRaw(t, prelude+`@noPanic
fn ReadTry(values: int[], i: uint) Result[int, IndexError] {
    let x := try values[i]
    return Ok(x)
}

@noPanic
fn Grow(i: int) Result[void, CollectionError] {
    let mut values: int[]
    try values.Append(i)
    return Ok()
}

fn Borrow(handles: Res[], i: uint) int {
    let handle := ref handles[i]
    return handle.id
}
`)
	if len(valid) != 0 {
		t.Fatalf("try indexing, fallible Append and element borrows are valid: %v", valid)
	}
}
