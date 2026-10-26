package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// Full-open reborrows preserve all exact/empty/dynamic source ranges and
// alternative roots, instead of appending a relative identity projection.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§28–30,59–62,66–67.
func TestFullOpenSliceOriginNormalization(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_full_open_origins.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "full-open.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	checked, alternatives, dynamic, empty := 0, 0, 0, 0
	err = walk.Inspect(parsed.Program, func(node any) error {
		ref, ok := node.(*ast.RefExpression)
		if !ok {
			return nil
		}
		slice, ok := ref.Value.(*ast.SliceExpression)
		if !ok || slice.Start != nil || slice.End != nil {
			return nil
		}
		if _, call := slice.Left.(*ast.CallExpression); !call {
			return nil
		}
		plan, ok := a.ResolvedSliceCreationPlanOf(ref)
		if !ok || plan.SourceKind != SliceSourceSlice {
			t.Fatal("missing reborrow plan")
		}
		destination, ok := a.ResolvedPlaceOf(slice)
		if !ok {
			t.Fatal("missing normalized Place")
		}
		before, after := placeOriginAlternatives(plan.SourcePlace), placeOriginAlternatives(destination)
		if len(before) != len(after) {
			t.Fatal("lost alternative origins")
		}
		for index, source := range before {
			result := after[index]
			if source.RootID != result.RootID || source.StorageIdentity != result.StorageIdentity || !samePlaceIdentity(source, result) {
				t.Fatalf("changed origin: source=%+v result=%+v roots=%d/%d storage=%d/%d", source.Projections, result.Projections, source.RootID, result.RootID, source.StorageIdentity, result.StorageIdentity)
			}
			if placeIsStaticallyEmpty(result) {
				empty++
			}
			for _, projection := range result.Projections {
				if projection.DynamicRange != 0 {
					dynamic++
				}
			}
		}
		if len(after) == 2 {
			alternatives++
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked != 4 || alternatives != 1 || dynamic != 1 || empty != 1 {
		t.Fatalf("checked=%d alternatives=%d dynamic=%d empty=%d", checked, alternatives, dynamic, empty)
	}
	invalid, err := os.ReadFile("../../testdata/sema/slice_full_open_returns_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := analyzeSourceRaw(t, string(invalid))
	if len(diagnostics) != 2 {
		t.Fatalf("expected two rejections: %v", diagnostics)
	}
	if !strings.Contains(diagnostics[0].Message, "shared reference never grants mutation authority") || !strings.Contains(diagnostics[1].Message, "cannot return reference to local") {
		t.Fatalf("lost authority/escape rejection: %v", diagnostics)
	}
}
