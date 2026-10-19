package sema

import (
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Exact normalization preserves omitted versus unknown endpoints and rejects
// the canonical failure before incrementing an inclusive bound. Wide values
// and input detachment protect both Place and creation-plan consumers.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§13–19,25.
func TestSliceNormalizationBeforeInclusiveIncrement(t *testing.T) {
	wide := new(big.Int).Lsh(big.NewInt(1), 256)
	last := new(big.Int).Sub(wide, big.NewInt(1))
	cases := []struct {
		name       string
		facts      SliceRangeEndpoints
		start, end string
		failure    SliceRangeFailure
	}{
		{"wide-inclusive", SliceRangeEndpoints{Start: last, End: last, Inclusive: true, Length: wide}, last.String(), wide.String(), SliceRangeValid},
		{"wide-exclusive", SliceRangeEndpoints{Start: last, End: wide, Length: wide}, last.String(), wide.String(), SliceRangeValid},
		{"wide-open", SliceRangeEndpoints{StartOmitted: true, EndOmitted: true, Length: wide}, "0", wide.String(), SliceRangeValid},
		{"empty", SliceRangeEndpoints{StartOmitted: true, EndOmitted: true, Length: new(big.Int)}, "0", "0", SliceRangeValid},
		{"unknown-full", SliceRangeEndpoints{StartOmitted: true, EndOmitted: true}, "0", "", SliceRangeValid},
		{"unknown-start", SliceRangeEndpoints{End: big.NewInt(2)}, "", "2", SliceRangeValid},
		{"negative-first", SliceRangeEndpoints{Start: big.NewInt(-1), End: last, Inclusive: true, Length: last}, "", "", SliceRangeInvalidRange},
		{"descending-first", SliceRangeEndpoints{Start: wide, End: last, Inclusive: true, Length: last}, "", "", SliceRangeStartAfterEnd},
		{"inclusive-at-length", SliceRangeEndpoints{StartOmitted: true, End: wide, Inclusive: true, Length: wide}, "", "", SliceRangeOutOfBounds},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copies := []*big.Int{clonePlaceConstantIndex(tc.facts.Start), clonePlaceConstantIndex(tc.facts.End), clonePlaceConstantIndex(tc.facts.Length)}
			start, end, failure := normalizeSliceRange(tc.facts)
			if failure.Failure != tc.failure {
				t.Fatalf("failure=%+v, want %s", failure, tc.failure)
			}
			for i, pair := range []struct {
				got  *big.Int
				want string
			}{{start, tc.start}, {end, tc.end}} {
				if pair.want == "" {
					if pair.got != nil {
						t.Fatalf("bound %d=%s, want unknown", i, pair.got)
					}
				} else if pair.got == nil || pair.got.String() != pair.want {
					t.Fatalf("bound %d=%v, want %s", i, pair.got, pair.want)
				}
				if pair.got != nil {
					pair.got.SetInt64(999)
				}
			}
			if !reflect.DeepEqual(copies, []*big.Int{tc.facts.Start, tc.facts.End, tc.facts.Length}) {
				t.Fatal("normalization or output mutation changed original inputs")
			}
		})
	}
}

// Analysis publishes only original operands in evaluation order; omitted bounds
// are not scheduled and try does not reorder them. Queries detach the schedule
// and neither re-infer calls nor allocate additional borrow/range identities.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§15–17,35–37.
func TestSliceOperandEvaluationPlans(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/slice_evaluation_order_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "slice-order.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(a.resolvedSliceCreationPlans) != 7 {
		t.Fatalf("plans=%d", len(a.resolvedSliceCreationPlans))
	}
	calls, borrows, ranges := len(a.resolvedCalls), len(a.borrows), len(a.placeIdentities.sliceRanges)
	for expr := range a.resolvedSliceCreationPlans {
		original, ok := a.ResolvedSliceCreationPlanOf(expr)
		if !ok || !original.HasValidEvaluationOrder() {
			t.Fatalf("missing ordered plan: %+v", original)
		}
		source := expr.Value.(*ast.SliceExpression)
		if _, call := source.Left.(*ast.CallExpression); call {
			if !reflect.DeepEqual(original.EvaluationOrder, []SliceOperand{SliceOperandSource, SliceOperandStart, SliceOperandEnd}) {
				t.Fatal(original.EvaluationOrder)
			}
			for _, operand := range []ast.Expression{source.Left, source.Start, source.End} {
				if _, ok := a.resolvedCalls[operand.(*ast.CallExpression)]; !ok {
					t.Fatal("original call operand lost")
				}
			}
		}
		changed, _ := a.ResolvedSliceCreationPlanOf(expr)
		changed.EvaluationOrder[0] = SliceOperandEnd
		if changed.HasValidEvaluationOrder() {
			t.Fatal("reordered source accepted")
		}
		fresh, _ := a.ResolvedSliceCreationPlanOf(expr)
		if !reflect.DeepEqual(original, fresh) {
			t.Fatal("query schedule aliases analyzer")
		}
		duplicate := original
		duplicate.EvaluationOrder = append(append([]SliceOperand(nil), original.EvaluationOrder...), SliceOperandSource)
		if duplicate.HasValidEvaluationOrder() {
			t.Fatal("duplicate evaluation accepted")
		}
		missing := original
		missing.EvaluationOrder = missing.EvaluationOrder[1:]
		if missing.HasValidEvaluationOrder() {
			t.Fatal("missing source evaluation accepted")
		}
	}
	if len(a.resolvedCalls) != calls || len(a.borrows) != borrows || len(a.placeIdentities.sliceRanges) != ranges {
		t.Fatal("query re-analyzed operands")
	}
}

// Source and endpoint analysis obeys availability changes in source order;
// obtaining a known returned-reference origin cannot elevate shared authority.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §17;
// rules/memory/borrowing.md — §§5.3,6–7,12.
func TestSliceOperandEvaluationChecksAvailabilityAndAuthority(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/slice_evaluation_order_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "slice-order-invalid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errs := a.Analyze(program)
	if len(errs) == 0 {
		t.Fatal("invalid operand order/authority was accepted")
	}
	authority, availability := 0, 0
	for _, e := range errs {
		switch {
		case strings.Contains(e.Message, "cannot create mutable reference to immutable place"):
			authority++
		case strings.Contains(e.Message, "cannot borrow unavailable place ticket; it was consumed by call"):
			availability++
		default:
			t.Fatalf("unexpected diagnostic: %v", e)
		}
	}
	if authority != 1 || availability != 2 || len(a.resolvedSliceCreationPlans) != 0 {
		t.Fatalf("authority=%d availability=%d invalid plans=%d", authority, availability, len(a.resolvedSliceCreationPlans))
	}
}
