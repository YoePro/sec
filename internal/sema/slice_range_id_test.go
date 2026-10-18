package sema

import (
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Dynamic ranges are evaluation-site identities, not endpoint spellings;
// static ranges need none, and public Place queries neither allocate nor alias.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 26–28.
func TestSliceRangeIDsAreStableAndQueriesDetached(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_range_id_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	var expected []SliceRangeID
	analyzer := NewAnalyzer()
	for iteration := 0; iteration < 3; iteration++ {
		text := string(source)
		if iteration == 1 {
			// Equal-width renames preserve evaluation sites, not endpoint text.
			text = strings.NewReplacer("start", "begin", "finish", "ending").Replace(text)
		}
		p := parser.New(lexer.NewWithFile(text, "slice-ranges.sec"))
		parsed := p.Parse()
		if parsed.HasErrors {
			t.Fatal(p.Errors())
		}
		if iteration == 2 {
			analyzer = NewAnalyzer()
		}
		analyzer.placeIdentities.sliceRange("previous-analysis-only")
		if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
			t.Fatal(errors)
		}
		function := parsed.Program.Statements[1].(*ast.FunctionDeclaration)
		var ids []SliceRangeID
		count := len(analyzer.placeIdentities.sliceRanges)
		for index := 0; index < 4; index++ {
			node := function.Body.Statements[index].(*ast.LetStatement).Value.(*ast.RefExpression).Value
			place, ok := analyzer.ResolvedPlaceOf(node)
			if !ok || len(place.Projections) == 0 {
				t.Fatalf("slice %d has no analyzed Place", index)
			}
			ids = append(ids, place.Projections[0].DynamicRange)
			if index == 3 && (len(place.Projections) != 2 || place.Projections[0].DynamicRange != ids[0]) {
				t.Fatalf("nested slice lost its parent range: %+v", place.Projections)
			}
			original := clonePlace(place)
			place.Projections[0].DynamicRange = 999
			place.Projections[0].SliceStart = big.NewInt(999)
			again, _ := analyzer.ResolvedPlaceOf(node)
			if !reflect.DeepEqual(original, again) {
				t.Fatal("query returned analyzer-owned projection storage")
			}
		}
		if ids[0] != 1 || ids[1] != 2 || ids[2] != 0 {
			t.Fatalf("dynamic sites/static range identities = %v", ids)
		}
		if len(analyzer.placeIdentities.sliceRanges) != count {
			t.Fatal("queries allocated range identities")
		}
		if iteration == 0 {
			expected = ids
		} else if !reflect.DeepEqual(expected, ids) {
			t.Fatalf("reanalysis changed IDs: %v != %v", expected, ids)
		}
	}
}

// Symbolic ranges survive nesting/indexing and remain conservatively overlapping;
// the identity is compile-time-only and never exported as portable metadata.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 26–30.
func TestSliceRangeIdentityCompositionAndAllocation(t *testing.T) {
	var allocator placeIdentityAllocator
	if allocator.sliceRange("") != 0 || SliceRangeID(0).String() != "slice-range:none" {
		t.Fatal("zero must mean absent")
	}
	first := allocator.sliceRange("evaluation:1")
	second := allocator.sliceRange("evaluation:2")
	if first != 1 || second != 2 || allocator.sliceRange("evaluation:1") != first || first.String() != "slice-range#1" {
		t.Fatal("allocation must be deterministic and interned")
	}
	snapshot := allocator.clone()
	allocator.sliceRange("evaluation:3")
	if len(snapshot.sliceRanges) != 2 {
		t.Fatal("snapshot shares the allocator map")
	}
	parent := PlaceProjection{Kind: PlaceSlice, SliceStart: big.NewInt(0), DynamicRange: first}
	child := PlaceProjection{Kind: PlaceSlice, SliceStart: big.NewInt(1), SliceEnd: big.NewInt(2)}
	nested := appendSliceProjection([]PlaceProjection{parent}, child)
	indexed := appendIndexProjection([]PlaceProjection{parent}, PlaceProjection{Kind: PlaceIndex, ConstantIndex: big.NewInt(1)})
	if len(nested) != 2 || nested[0].DynamicRange != first || len(indexed) != 2 || indexed[0].DynamicRange != first {
		t.Fatal("composition erased a symbolic parent")
	}
	other := parent
	other.DynamicRange = second
	left := Place{Root: "values", Projections: []PlaceProjection{parent}}
	right := Place{Root: "values", Projections: []PlaceProjection{other}}
	if samePlaceIdentity(left, right) || Relationship(left, right) != PlacePotentiallyOverlapping {
		t.Fatal("different dynamic evaluations cannot imply equality or disjointness")
	}
	if Relationship(left, left) != PlacePotentiallyOverlapping {
		t.Fatal("a compile-time range ID alone cannot prove runtime range equality")
	}
	left.Projections = append(left.Projections, child)
	right.Projections = append(right.Projections, PlaceProjection{Kind: PlaceSlice, SliceStart: big.NewInt(3), SliceEnd: big.NewInt(4)})
	if Relationship(left, right) != PlacePotentiallyOverlapping {
		t.Fatal("disjoint relative children cannot prove disjointness through unknown parents")
	}
	encoded, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	var imported PlaceProjection
	if err := json.Unmarshal(encoded, &imported); err != nil || imported.DynamicRange != 0 {
		t.Fatalf("analysis-local identity leaked to persistence: %s %v", encoded, err)
	}
}
