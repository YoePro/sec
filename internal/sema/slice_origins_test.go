package sema

import (
	"fmt"
	"reflect"
	"sec/internal/ast"
	"testing"
)

// sliceBindingPlans collects the already recorded plans of named source borrows,
// keeping caller contexts distinct when helper parameters reuse the same names.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§36–37,66–67.
func sliceBindingPlans(program *ast.Program, a *Analyzer) map[string]ResolvedSliceCreationPlan {
	out := map[string]ResolvedSliceCreationPlan{}
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		for _, stmt := range fn.Body.Statements {
			binding, ok := stmt.(*ast.LetStatement)
			if !ok {
				continue
			}
			ref, ok := binding.Value.(*ast.RefExpression)
			if !ok {
				continue
			}
			if plan, ok := a.ResolvedSliceCreationPlanOf(ref); ok {
				out[fn.Name.Value+"/"+binding.Name.Value] = plan
			}
		}
	}
	return out
}

// The same ultimate storage and exact range survive parameters, shared copies,
// mutable moves/reborrows, aggregate fields/copies/returns and nested calls.
// Runtime-returned ranges preserve their origin without becoming static proofs.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–43,63–69.
func TestSliceOriginTransferMatrix(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, a, errs := readSliceMatrixInput(t, "slice_origins_valid.sec", width)
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			plans := sliceBindingPlans(program, a)
			source := plans["Observe/source"]
			for name, want := range map[string]string{"nested": "values[3..<5]", "tailRange": "values[3..<7]", "fieldRange": "values[3..<5]", "copiedFieldRange": "values[3..<5]", "unwrappedRange": "values[3..<5]", "methodRange": "values[3..<5]", "referencedFieldRange": "values[3..<5]", "externalFieldRange": "values[2..<7]", "identityRange": "values[2..<7]"} {
				p, ok := plans["Observe/"+name]
				if !ok || p.SourcePlace.String() != want {
					t.Fatalf("%s origin=%s want %s", name, p.SourcePlace.String(), want)
				}
				if !reflect.DeepEqual(p.ReferenceFacts.Facts.StorageIdentities, source.ReferenceFacts.Facts.StorageIdentities) || p.BorrowID == 0 || p.LifetimeID == 0 {
					t.Fatalf("%s lost canonical reference dependencies", name)
				}
			}
			mutable := plans["ObserveMutable/mutableRange"]
			if mutable.Mode != SliceMutable || mutable.SourcePlace.String() != "values[3..<5]" || mutable.SourceLength == nil || mutable.SourceLength.String() != "2" {
				t.Fatalf("mutable transfer=%+v", mutable)
			}
			dynamic := plans["Observe/dynamicRange"]
			again := plans["Observe/dynamicAgainRange"]
			for _, p := range []ResolvedSliceCreationPlan{dynamic, again} {
				if p.SourceLength != nil || p.StaticRange != nil || len(p.SourcePlace.Projections) != 2 || p.SourcePlace.Projections[1].DynamicRange == 0 || p.SourcePlace.Projections[1].SliceStart != nil || p.SourcePlace.Projections[1].SliceEnd != nil {
					t.Fatalf("runtime return fabricated range precision: %+v", p)
				}
				if !Relationship(p.SourcePlace, source.ReferenceFacts.Facts.OriginPlaces[0]).MayOverlap() {
					t.Fatal("unknown returned range became disjoint from source")
				}
			}
			if dynamic.SourcePlace.Projections[1].DynamicRange == again.SourcePlace.Projections[1].DynamicRange {
				t.Fatal("separate calls share a runtime range identity")
			}
			if !Relationship(dynamic.SourcePlace, again.SourcePlace).MayOverlap() {
				t.Fatal("separate runtime returned ranges became disjoint")
			}
			arena := plans["ArenaTransfers/arenaSource"].ReferenceFacts.Facts
			forwarded := plans["ArenaTransfers/arenaForwarded"].ReferenceFacts.Facts
			if arena.EpochDependency == 0 || arena.InvalidationDomain == 0 || forwarded.EpochDependency != arena.EpochDependency || forwarded.InvalidationDomain != arena.InvalidationDomain || !reflect.DeepEqual(forwarded.StorageIdentities, arena.StorageIdentities) || forwarded.AddressSpace != arena.AddressSpace || forwarded.AddressStability != arena.AddressStability {
				t.Fatalf("returned slice lost Arena/address facts: %+v -> %+v", arena, forwarded)
			}
			_, againAnalyzer, errs := readSliceMatrixInput(t, "slice_origins_valid.sec", width)
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			if !reflect.DeepEqual(a.placeIdentities.sliceRanges, againAnalyzer.placeIdentities.sliceRanges) {
				t.Fatal("returned range IDs are nondeterministic")
			}
		})
	}
}
