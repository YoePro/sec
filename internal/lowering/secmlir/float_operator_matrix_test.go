package secmlir

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// The native arithmetic source also emits all six source predicates and all
// five arithmetic kinds for f32, f64 and platform float on both scalar plans.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§20–27,74–77,86;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md — §2.
func TestEmitFloatOperatorSourceMatrix(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/build/native/float_operator_matrix.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), "float_operator_matrix.sec"))
		parsed := p.Parse()
		if parsed.HasErrors {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(parsed.Program); len(errors) != 0 {
			t.Fatal(errors)
		}
		module, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"float_operator_matrix.sec"}})
		if err != nil {
			t.Fatal(err)
		}
		output, err := Emit(module, testPlan(width))
		if err != nil {
			t.Fatal(err)
		}
		text := string(output)
		for _, kind := range []string{"add", "sub", "mul", "div", "rem"} {
			if count := strings.Count(text, `"sec.float.binary"`); count < 3 {
				t.Fatalf("plan %d missing arithmetic", width)
			}
			if count := strings.Count(text, `kind = "`+kind+`"`); count < 3 {
				t.Fatalf("plan %d: %s found %d times, want all three source types", width, kind, count)
			}
		}
		for _, predicate := range []string{"eq", "ne", "lt", "le", "gt", "ge"} {
			if count := strings.Count(text, `predicate = "`+predicate+`"`); count < 3 {
				t.Fatalf("plan %d: predicate %s found %d times", width, predicate, count)
			}
		}
		for _, forbidden := range []string{"fastmath<fast>", `remainder_kind = "ieee"`, `predicate = "one"`} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("plan %d weakens float semantics: %s", width, forbidden)
			}
		}
	}
}
