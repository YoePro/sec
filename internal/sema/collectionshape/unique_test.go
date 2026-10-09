package collectionshape_test

import (
	"math"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/sema/collectionshape"
)

// TestFirstDuplicateUsesSemanticEquality checks direct values without assuming
// their host representation or hash identity defines language equality.
// Rules: rules/types/contracts.md — String and collection contracts (unique).
func TestFirstDuplicateUsesSemanticEquality(t *testing.T) {
	i, j, found := collectionshape.FirstDuplicate([]string{"A", "b", "a", "B"}, strings.EqualFold)
	if !found || i != 2 || j != 0 {
		t.Fatalf("semantic duplicate = %d,%d,%v", i, j, found)
	}
	for _, values := range [][]int{nil, {}, {1}} {
		if _, _, found := collectionshape.FirstDuplicate(values, func(int, int) bool { t.Fatal("unneeded equality"); return false }); found {
			t.Fatal("fabricated duplicate")
		}
	}
	values := []float64{math.NaN(), math.NaN(), math.Copysign(0, -1), 0}
	if i, j, found := collectionshape.FirstDuplicate(values, func(x, y float64) bool { return x == y }); !found || i != 3 || j != 2 {
		t.Fatalf("NaN/signed-zero equality = %d,%d,%v", i, j, found)
	}
	nested := [][]int{{1, 1}, {2, 2}}
	equal := func(x, y []int) bool {
		if len(x) != len(y) {
			return false
		}
		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}
		return true
	}
	if _, _, found := collectionshape.FirstDuplicate(nested, equal); found {
		t.Fatal("recursively enforced uniqueness")
	}
	nested = append(nested, []int{1, 1})
	if i, j, found := collectionshape.FirstDuplicate(nested, equal); !found || i != 2 || j != 0 {
		t.Fatalf("aggregate equality = %d,%d,%v", i, j, found)
	}
}

// TestUniqueContractSemanticProofs verifies scalar constant folding, nominal
// members, direct aggregate equality and non-comparable elements at every depth.
// Rules: rules/types/contracts.md — Applicability, unique, Diagnostics;
// rules/collections/collections.md — §10 Equality.
func TestUniqueContractSemanticProofs(t *testing.T) {
	for _, file := range []string{"unique_semantics_valid.sec", "unique_semantics_invalid.sec", "unique_elements_invalid.sec"} {
		source := fixture(t, file)
		for _, width := range []uint16{32, 64} {
			for _, depth := range []sema.AnalysisDepth{sema.AnalysisInteractive, sema.AnalysisStandard, sema.AnalysisDeep} {
				p := parser.New(lexer.New(source))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				errors := sema.NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth).Analyze(program)
				want := 0
				if file == "unique_semantics_invalid.sec" {
					want = 7
				}
				if file == "unique_elements_invalid.sec" {
					want = 4
				}
				if len(errors) != want {
					t.Fatalf("%s: %v; want %d", file, errors, want)
				}
				for _, e := range errors {
					wantID := diagnostics.ValueViolatesContract
					if file == "unique_elements_invalid.sec" {
						wantID = diagnostics.InapplicableContract
					}
					if e.ID != wantID || e.Line == 0 || e.Column == 0 {
						t.Fatalf("unexpected diagnostic instead of unique failure: %#v", e)
					}
					if len(e.Related) == 0 && e.PreviousLine == 0 {
						t.Fatalf("missing defining provenance: %v", e)
					}
				}
			}
		}
	}
}
