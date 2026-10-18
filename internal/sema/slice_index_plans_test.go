package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Slice index facts reuse array check/use/transfer/proof enums and preserve
// exact source indices and authority without publishing array lowering facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58.
func TestResolvedSliceIndexPlans(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_index_plans_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), "slice-index.sec"))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) != 0 {
		t.Fatal(parsed.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	plans := map[string][]ResolvedSliceIndexPlan{}
	var query *ast.IndexExpression
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		for _, statement := range fn.Body.Statements {
			for _, index := range indexesInStatement(statement) {
				plan, found := a.ResolvedSliceIndexPlanOf(index)
				if fn.Name.Value == "Array" {
					if found {
						t.Fatal("array access acquired a slice plan")
					}
					continue
				}
				if !found {
					t.Fatalf("%s: missing slice index plan", fn.Name.Value)
				}
				if _, found := a.ResolvedArrayIndexPlanOf(index); found {
					t.Fatal("slice acquired an array plan")
				}
				if _, found := a.ResolvedListIndexPlanOf(index); found {
					t.Fatal("slice acquired a list plan")
				}
				if !sameConcreteType(plan.ErrorType, a.indexErrorType()) || !plan.ErrorType.ErrorAssignable {
					t.Fatalf("noncanonical IndexError: %+v", plan.ErrorType)
				}
				plans[fn.Name.Value] = append(plans[fn.Name.Value], plan)
				if fn.Name.Value == "Huge" {
					query = index
				}
			}
		}
	}
	known := plans["Known"][0]
	if known.SliceLength.String() != "3" || known.ConstantIndex.String() != "1" || known.CheckKind != ArrayIndexProvenSafe || known.ProofKind != ArrayIndexProofConstant || known.Mutable {
		t.Fatalf("known: %+v", known)
	}
	huge := plans["Huge"][0]
	if huge.SliceLength.String() != "9223372036854775809" || huge.ConstantIndex.String() != "9223372036854775808" || huge.IndexType.Name != "uint256" || huge.IndexSigned || huge.CheckKind != ArrayIndexProvenSafe {
		t.Fatalf("wide exact plan: %+v", huge)
	}
	unknown := plans["Unknown"][0]
	if unknown.SliceLength != nil || unknown.CheckKind != ArrayIndexRuntimeCheck || unknown.ProofKind != "" || unknown.FailureMode != ArrayIndexFailureOrdinary {
		t.Fatalf("unknown-length constant is not a proof: %+v", unknown)
	}
	for i, use := range []ArrayIndexUseKind{ArrayIndexRead, ArrayIndexWrite, ArrayIndexBorrow, ArrayIndexMutBorrow} {
		plan := plans["Access"][i]
		if plan.UseKind != use || plan.Action != arrayIndexTransferAction(plan.ElementType, use) || !plan.IndexSigned || plan.IndexType.Name != "int128" || plan.Mutable != (i == 1 || i == 3) {
			t.Fatalf("access %d: %+v", i, plan)
		}
	}
	for name, proof := range map[string]ArrayIndexProofKind{"Ranged": ArrayIndexProofRange, "Guarded": ArrayIndexProofBranch} {
		plan := plans[name][0]
		if plan.CheckKind != ArrayIndexProvenSafe || plan.ProofKind != proof {
			t.Fatalf("%s: %+v", name, plan)
		}
	}
	for _, name := range []string{"Fallible", "Handled"} {
		plan := plans[name][0]
		if plan.FailureMode != ArrayIndexFailureFallible || plan.CheckKind != ArrayIndexRuntimeCheck {
			t.Fatalf("%s: %+v", name, plan)
		}
	}

	// Public queries neither allocate identities nor infer an unknown AST.
	before := len(a.resolvedSliceIndexPlans) + len(a.borrows) + len(a.placeIdentities.borrows) + len(a.placeIdentities.sliceRanges) + len(a.expressionTypes)
	original, _ := a.ResolvedSliceIndexPlanOf(query)
	changed, _ := a.ResolvedSliceIndexPlanOf(query)
	changed.SliceLength.SetInt64(0)
	changed.ConstantIndex.SetInt64(-1)
	changed.ElementType.Name = "Changed"
	if changed.SliceType.Element != nil {
		changed.SliceType.Element.Name = "Changed"
	}
	changed.ErrorType.EnumValues[0] = "Changed"
	changed.ErrorType.EnumConsts["OutOfBounds"].Value.SetInt64(99)
	changed.SourcePlace.Root = "Changed"
	if changed.SourcePlace.Type.Element != nil {
		changed.SourcePlace.Type.Element.Name = "Changed"
	}
	fresh, _ := a.ResolvedSliceIndexPlanOf(query)
	if !reflect.DeepEqual(original, fresh) {
		t.Fatal("query exposes mutable plan data")
	}
	if _, ok := a.ResolvedSliceIndexPlanOf(&ast.IndexExpression{}); ok {
		t.Fatal("unknown query inferred facts")
	}
	if _, ok := a.ResolvedSliceIndexPlanOf(nil); ok {
		t.Fatal("nil query has a plan")
	}
	if before != len(a.resolvedSliceIndexPlans)+len(a.borrows)+len(a.placeIdentities.borrows)+len(a.placeIdentities.sliceRanges)+len(a.expressionTypes) {
		t.Fatal("query mutated analyzer")
	}
	var absent *Analyzer
	if _, ok := absent.ResolvedSliceIndexPlanOf(query); ok {
		t.Fatal("nil analyzer has a plan")
	}
	// Reanalysis starts a fresh fact set; old AST addresses are not retained.
	next := &ast.Program{Statements: program.Statements[:1]}
	if errs := a.Analyze(next); len(errs) != 0 {
		t.Fatal(errs)
	}
	if _, ok := a.ResolvedSliceIndexPlanOf(query); ok {
		t.Fatal("reanalysis retained old AST plan")
	}
}

// Known local slice extents, empty slices and negative wide signed constants
// produce the same owning compile-time bounds diagnostic as fixed arrays.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §52;
// rules/collections/collections.md — §8.3.
func TestSliceIndexConstantBounds(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_index_bounds_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errs := analyzeSourceRaw(t, string(source))
	if len(errs) != 3 {
		t.Fatalf("expected three invalid index diagnostics: %v", errs)
	}
	for _, diagnostic := range errs {
		if diagnostic.ID != diagnostics.IndexOutOfBounds {
			t.Fatalf("unexpected bounds diagnostic: %+v", diagnostic)
		}
	}
}
