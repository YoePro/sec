package sema

import (
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestFloatingUnitConversionPlans verifies finite proofs at native and explicit
// widths, using named contracts, dominating guards and immutable constants.
// Rules: rules/types/units.md — Exact fixed conversions; No hidden precision loss.
func TestFloatingUnitConversionPlans(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/float_unit_conversion/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		errors := a.Analyze(program)
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		if len(a.unitConversionPlans) != 12 {
			t.Fatal(a.unitConversionPlans)
		}
		for expr := range a.unitConversionPlans {
			plan, ok := a.UnitConversionPlanOf(expr)
			if !ok || plan.Proof != UnitConversionFloatingValueRange || plan.FloatMinimum == nil || plan.FloatMaximum == nil || plan.Factor.RatString() != "2" {
				t.Fatal(plan)
			}
			minimum := plan.FloatMinimum.RatString()
			plan.FloatMinimum.SetInt64(99)
			again, _ := a.UnitConversionPlanOf(expr)
			if again.FloatMinimum.RatString() != minimum {
				t.Fatal("mutable proof leaked", again)
			}
		}
	}
}

// TestFloatingUnitConversionRejections retains source errors for unproven,
// mutated, lossy, overflowing and carrier-changing values.
// Rules: rules/types/units.md — No hidden precision loss; Same named unit.
func TestFloatingUnitConversionRejections(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/float_unit_conversion/rejections_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 8 {
		t.Fatal(errors)
	}
	if len(a.unitConversionPlans) != 0 {
		t.Fatal(a.unitConversionPlans)
	}
}
