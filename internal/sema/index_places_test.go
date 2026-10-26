package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Call-result indexing preserves the actual source holder/root/projections,
// retains disjointness and rejects read-only authority and owning temporaries.
// Rules: rules/memory/borrowing.md — §§12,14.4–14.5,15;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§55,58–62,64–67.
func TestSliceCallAssignmentPlaces(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/slice_call_index_assignment.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(source), "slice_call_index_assignment.sec"))
			parsed := p.Parse()
			if parsed.HasErrors {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := a.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			count, local := 0, 0
			if err := walk.Inspect(parsed.Program, func(node any) error {
				assignment, ok := node.(*ast.AssignmentStatement)
				if !ok {
					return nil
				}
				index, ok := assignment.Target.(*ast.IndexExpression)
				if !ok {
					return nil
				}
				if _, ok := index.Left.(*ast.CallExpression); !ok {
					return nil
				}
				place, ok := a.ResolvedPlaceOf(index)
				if !ok || !place.Mutable || !place.Addressable || place.RootID == 0 || len(place.Projections) == 0 {
					t.Fatalf("missing canonical writable Place: %s known=%t mutable=%t addressable=%t root=%d storage=%d", place, ok, place.Mutable, place.Addressable, place.RootID, place.StorageIdentity)
				}
				plan, ok := a.ResolvedSliceIndexPlanOf(index)
				if !ok || !plan.Mutable || plan.UseKind != ArrayIndexWrite || plan.SourcePlace.RootID != place.RootID || plan.SourcePlace.StorageIdentity != place.StorageIdentity {
					t.Fatalf("missing authoritative write plan: %+v", plan)
				}
				if place.ReferenceHolder == "view" {
					local++
				}
				if place.Root != "values" && place.Root != "data" {
					t.Fatalf("call got an invented storage root: %+v", place)
				}
				count++
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if count != 4 || local != 1 {
				t.Fatalf("canonical assignments=%d local granting holders=%d", count, local)
			}
		})
	}
}

// Granting ancestors may authorize a returned reference, while shared children
// and siblings keep their conflict checks. Owned call results gain no Place.
// Rules: rules/memory/borrowing.md — §§4,12,14.4–14.5,15.
func TestSliceCallAssignmentRestrictions(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_call_assignment_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	if len(errors) != 4 {
		t.Fatalf("expected four rejected writes: %v", errors)
	}
	for index, want := range []string{"immutable index target", "immutable index target", "already borrowed", "already borrowed"} {
		if !strings.Contains(errors[index].Message, want) {
			t.Fatalf("diagnostic %d: %v, want %q", index, errors[index], want)
		}
	}
}

// Each alternative returned origin retains its own granting holder and exact
// projected index; a representative cannot replace the whole finite origin set.
// Rules: rules/memory/borrowing.md — §§12,13.1,14.4,17(5);
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§59–62,66–67.
func TestSliceCallAssignmentAlternativePlaces(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_call_assignment_alternatives_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "alternatives.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	checked := false
	if err := walk.Inspect(parsed.Program, func(node any) error {
		assignment, ok := node.(*ast.AssignmentStatement)
		if !ok {
			return nil
		}
		place, ok := a.ResolvedPlaceOf(assignment.Target)
		if !ok {
			t.Fatal("no canonical index Place")
		}
		origins := placeOriginAlternatives(place)
		if len(origins) != 2 {
			t.Fatalf("lost return origin alternatives: %v", origins)
		}
		indices := map[string]string{}
		for _, origin := range origins {
			if !origin.Mutable || origin.Root != "data" || origin.RootID == 0 {
				t.Fatalf("lost alternative authority/root: %v", origin)
			}
			last := origin.Projections[len(origin.Projections)-1]
			if last.ConstantIndex == nil {
				t.Fatalf("lost exact projected index: %v", origin)
			}
			indices[origin.ReferenceHolder] = last.ConstantIndex.String()
		}
		if indices["left"] != "0" || indices["right"] != "2" {
			t.Fatalf("lost per-origin grants/ranges: %v", indices)
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("fixture contains no assignment")
	}
}
