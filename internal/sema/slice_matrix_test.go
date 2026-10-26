package sema

import (
	"fmt"
	"os"
	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// readSliceMatrixInput reads actual Sec source for the P16 source conformance
// matrix; the chosen target plan cannot be inferred from the compiler host.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§125–128,134–137.
func readSliceMatrixInput(t *testing.T, file string, width uint16) (*ast.Program, *Analyzer, []Error) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/sema/" + file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
	errs := a.Analyze(program)
	return program, a, errs
}

// All source range forms, host-wide constants, signed/unsigned runtime bounds,
// empty provenance and live disjoint borrows are valid on both target plans.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§125–128,134–137.
func TestSliceSourceMatrixValid(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, a, errs := readSliceMatrixInput(t, "slice_matrix_valid.sec", width)
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			plans := sliceBindingPlans(program, a)
			expected := map[string][2]string{"full": {"0", "8"}, "tail": {"2", "8"}, "inclusiveHead": {"0", "3"}, "exclusiveHead": {"0", "2"}, "inclusive": {"1", "3"}, "exclusive": {"1", "3"}, "empty": {"8", "8"}, "singleton": {"7", "8"}}
			for name, bounds := range expected {
				p, ok := plans["Forms/"+name]
				if !ok || p.StaticRange == nil || p.StaticRange.Start.String() != bounds[0] || p.StaticRange.EndExclusive.String() != bounds[1] || p.CheckKind != SliceRangeProvenSafe {
					t.Fatalf("%s: %+v", name, p)
				}
			}
			dynamic := plans["Dynamic/dynamic"]
			if dynamic.CheckKind != SliceRangeRuntimeCheck || dynamic.StaticRange != nil || !dynamic.Start.Signed || dynamic.End.Signed || dynamic.Start.Type.Name != "int128" || dynamic.End.Type.Name != "uint256" {
				t.Fatalf("runtime check=%s bounds=%s/%s signed=%v/%v", dynamic.CheckKind, dynamic.Start.Type.Name, dynamic.End.Type.Name, dynamic.Start.Signed, dynamic.End.Signed)
			}
			for _, key := range []string{"Empty/first", "Empty/second"} {
				p := plans[key]
				if p.StaticRange == nil || p.StaticRange.Start.Sign() != 0 || p.StaticRange.EndExclusive.Sign() != 0 || p.BorrowID == 0 || len(p.ReferenceFacts.Facts.StorageIdentities) == 0 {
					t.Fatal("empty slice lost valid provenance")
				}
			}
			if Relationship(plans["Disjoint/left"].ReferenceFacts.Facts.OriginPlaces[0], plans["Disjoint/right"].ReferenceFacts.Facts.OriginPlaces[0]) != PlaceDisjoint {
				t.Fatal("nested static mutable borrows are not disjoint")
			}
		})
	}
	program, a, errs := readSliceMatrixInput(t, "slice_matrix_wide.sec", 64)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	wide := sliceBindingPlans(program, a)["Wide/wide"]
	if wide.StaticRange == nil || wide.StaticRange.Start.String() != "9223372036854775807" || wide.StaticRange.EndExclusive.String() != "9223372036854775808" {
		t.Fatal("wide source endpoints were truncated")
	}
	_, _, errs = readSliceMatrixInput(t, "slice_matrix_wide.sec", 32)
	found := false
	for _, err := range errs {
		found = found || strings.Contains(err.Message, "overflows target uint32")
	}
	if !found {
		t.Fatal("32-bit target array extent gate was lost")
	}
}

// Each independent invalid source function must produce its owning error;
// unrelated diagnostics cannot make a missing borrow/escape rejection pass.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§125–128,134–137.
func TestSliceSourceMatrixInvalid(t *testing.T) {
	expected := map[string]string{
		"Negative": "slice lower bound must be non-negative", "Descending": "descending slice range", "Bounds": "inclusive slice upper bound",
		"SharedToMutable": "shared reference never grants mutation authority", "MutableCopy": "cannot copy move-only value",
		"MovedHolder": "cannot borrow unavailable place", "DiscardedHolder": "cannot borrow unavailable place",
		"StaticOverlap": "while it is already borrowed", "DynamicOverlap": "while it is already borrowed", "EmptyIndex": "slice index 0 is out of bounds",
		"LocalDirect": "cannot return reference to local variable values", "LocalEmpty": "cannot return reference to local variable values",
		"LocalCall": "cannot return reference to local variable values", "LocalCallRange": "cannot return reference to local variable values",
		"LocalAggregate": "cannot return value containing reference to local variable values", "LocalAggregateField": "cannot return reference to local variable values",
		"LocalMethodField": "cannot return reference to local variable values",
		"StaleReturned":    "while dependency values remains live",
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, _, errs := readSliceMatrixInput(t, "slice_matrix_invalid.sec", width)
			seen := map[string]bool{}
			for _, err := range errs {
				var owner *ast.FunctionDeclaration
				for _, s := range program.Statements {
					fn, ok := s.(*ast.FunctionDeclaration)
					if ok && fn.Token.Line <= err.Line && (owner == nil || fn.Token.Line > owner.Token.Line) {
						owner = fn
					}
				}
				if owner == nil {
					t.Fatalf("unowned diagnostic %v", err)
				}
				want, ok := expected[owner.Name.Value]
				if !ok {
					t.Fatalf("helper %s unexpectedly invalid: %v", owner.Name.Value, err)
				}
				if !strings.Contains(err.Message, want) {
					t.Fatalf("%s: diagnostic %q does not contain %q", owner.Name.Value, err.Message, want)
				}
				seen[owner.Name.Value] = true
			}
			for name := range expected {
				if !seen[name] {
					t.Errorf("%s: invalid source accepted", name)
				}
			}
		})
	}
}
