package sema

import (
	"math/big"
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Defaulted static values keep the canonical semantic default in the published
// initializer, independent of a mutable static's later contents or AST preview.
// Rules: rules/types/default_values.md — §§40-41; rules/declarations/static.md — §§15-16.
func TestStaticDefaultInitializers(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/static_defaults/values.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), "static-defaults.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errs := a.Analyze(program); len(errs) != 0 {
			t.Fatal(errs)
		}
		declarations := a.ResolvedStaticDeclarations()
		if len(declarations) != 7 {
			t.Fatal(len(declarations))
		}
		for _, static := range declarations {
			switch static.Name {
			case "count":
				if static.Initializer.Integer == nil || static.Initializer.Integer.String() != "2" {
					t.Fatal("missing range default")
				}
			case "settings":
				if len(static.Initializer.Components) != 3 || len(static.Initializer.Components[0].Components) != 64 || static.Initializer.Components[1].Integer.String() != "2" {
					t.Fatal("missing struct field defaults")
				}
			case "backing", "initial", "beyondPreview":
				want := 64
				if static.Name == "beyondPreview" {
					want = 4097
				}
				if !static.Initializer.Aggregate || len(static.Initializer.Components) != want {
					t.Fatalf("%s: %d elements", static.Name, len(static.Initializer.Components))
				}
				for _, component := range static.Initializer.Components {
					if component.Integer == nil || component.Integer.Sign() != 0 {
						t.Fatal("nonzero byte default")
					}
				}
				static.Initializer.Components[0].Integer.SetInt64(99)
				again, found := a.ResolvedStaticOf(static.Declaration.Name)
				if !found || again.Initializer.Components[0].Integer.Sign() != 0 {
					t.Fatal("mutable initializer snapshot")
				}
			}
		}
	}
}

// Materialization exhaustion is a compiler resource limit, with no partially
// published initializer facts; it does not fabricate empty/zero initialization.
// Rules: rules/compiler/compile_time_evaluation.md — §§48-49,53-56;
// rules/types/default_values.md — §40.
func TestStaticDefaultMaterializationBudget(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/static_defaults/budget_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "budget_invalid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	errors := a.Analyze(program)
	found := false
	for _, err := range errors {
		found = found || strings.Contains(err.Message, "AggregateValueSize")
	}
	if !found {
		t.Fatal("missing resource-limit diagnostic", errors)
	}
	for _, binding := range a.ResolvedStaticDeclarations() {
		if binding.Initializer.Aggregate {
			t.Fatal("partially published defaults")
		}
	}
}

// Zero-length arrays must never request an element default, and corrupt
// canonical facts must fail as invariants before constructing storage.
// Rules: rules/types/default_values.md — §31;
// rules/compiler/compile_time_evaluation.md — §§32,56-57.
func TestCTEArrayDefaultFacts(t *testing.T) {
	typ := NewFixedArrayType(Type{Name: "opaque", Kind: FunctionType}, big.NewInt(0))
	x := newCTEExecutor(NewAnalyzer(), nil)
	value, ok := x.arrayDefault(DefaultResolution{Kind: ArrayDefault, ArrayLengthDecimal: "0"}, typ, lexer.Token{})
	if !ok || value.Aggregate == nil || len(value.Aggregate.Elements) != 0 {
		t.Fatal("constructed an element of an empty array")
	}
	x = newCTEExecutor(NewAnalyzer(), nil)
	if _, ok := x.arrayDefault(DefaultResolution{Kind: ArrayDefault, ArrayLengthDecimal: "1"}, typ, lexer.Token{}); ok || x.failure == nil || x.failure.Kind != cteCompilerInvariantFailure {
		t.Fatal("accepted mismatched canonical length")
	}
}
