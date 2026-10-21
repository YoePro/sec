package sema

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
)

// Core resolution snapshots detach types and allocate no semantic identity;
// analyzer reuse must not retain an earlier source tree's length decisions.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§37,47–49.
func TestResolvedSliceLengthPlansAreDetached(t *testing.T) {
	program, a, errs := readSliceMatrixInput(t, "../semantic_ir/slice_source_length.sec", 64)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	var expressions []ast.Expression
	properties, globals := 0, 0
	if err := walk.Inspect(program, func(node any) error {
		expr, ok := node.(ast.Expression)
		if !ok {
			return nil
		}
		plan, ok := a.ResolvedSliceLengthPlanOf(expr)
		if !ok {
			return nil
		}
		expressions = append(expressions, expr)
		switch plan.CompilerKnownID {
		case "CKM-LEN-SLICE":
			properties++
		case "CKF-LEN":
			globals++
		default:
			t.Fatal("length was not resolved through the core registry")
		}
		if plan.SourceType.Kind != ReferenceType || plan.SourceType.Element == nil || plan.SourceType.Element.Kind != SliceType || plan.ResultType.Kind != UintType || plan.ResultType.Name != "uint" {
			t.Fatal("invalid length type facts")
		}
		plan.SourceType.Element.Name = "changed"
		plan.ResultType.Name = "changed"
		fresh, _ := a.ResolvedSliceLengthPlanOf(expr)
		if fresh.SourceType.Element.Name == "changed" || fresh.ResultType.Name == "changed" {
			t.Fatal("length query mutated canonical facts")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if properties != 4 || globals != 6 {
		t.Fatalf("core forms: %d properties, %d globals", properties, globals)
	}
	before := a.placeIdentities.clone()
	for _, expr := range expressions {
		a.ResolvedSliceLengthPlanOf(expr)
	}
	if len(before.sliceRanges) != len(a.placeIdentities.sliceRanges) || len(before.borrows) != len(a.placeIdentities.borrows) {
		t.Fatal("length query allocated identities")
	}
	a.Analyze(&ast.Program{})
	for _, expr := range expressions {
		if _, ok := a.ResolvedSliceLengthPlanOf(expr); ok {
			t.Fatal("stale length plan survived analyzer reuse")
		}
	}
}
