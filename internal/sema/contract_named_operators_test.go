package sema

import (
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestContractNamedOperators verifies exact decimal/string operators, selected
// floating intermediate widths and values shared by contract/default consumers.
// Rules: rules/types/contracts.md — Contract arguments; rules/compiler/compile_time_evaluation.md — §2(3);
// rules/foundations/operators.md — Decimal arithmetic, Floating arithmetic, String concatenation.
func TestContractNamedOperators(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/contract_named_operators_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	expected := map[string]string{"Amount": "5/2", "Rounded": "0", "WideAmount": "18446744073709551616", "Remainder": "-5/4"}
	for name, want := range expected {
		v := a.types[name].ExplicitDefault
		if v == nil || v.Exact == nil || v.Exact.RatString() != want {
			t.Fatalf("%s default = %+v, want %s", name, v, want)
		}
	}
	if a.types["Name"].ExplicitDefault.String != "sec-lang" || !a.types["Flag"].ExplicitDefault.Bool {
		t.Fatal("text/comparison defaults lost")
	}
	for _, bits := range []uint16{32, 64} {
		source := "module main\nlet Large: float := 16777216g\nlet One: float := 1g\ntype Selected float default Large + One - Large\n"
		p := parser.New(lexer.New(source))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		target := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: bits})
		if errors := target.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		want := "1"
		if bits == 32 {
			want = "0"
		}
		if v := target.types["Selected"].ExplicitDefault; v == nil || v.Exact.RatString() != want {
			t.Fatalf("%d-bit result=%+v, want %s", bits, v, want)
		}
	}
}

// TestContractNamedOperatorsRejectUnestablishedValues preserves failures rather
// than rounding exact decimals or evaluating runtime bindings.
// Rules: rules/compiler/compile_time_evaluation.md — §§2(3),47(4); rules/types/contracts.md — Diagnostics.
func TestContractNamedOperatorsRejectUnestablishedValues(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/contract_named_operators_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 6 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		if e.ID == "" || e.PreviousLine == 0 {
			t.Fatalf("missing failure identity/provenance: %+v", e)
		}
	}
}
