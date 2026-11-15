package secmlir

import (
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Strict policies and nominal source scalar names must survive emission;
// emission must reject tampered IR rather than dropping or repairing its facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§18,20–27,64–69,74–76.
func TestEmitFloatSemanticPolicies(t *testing.T) {
	p := parser.New(lexer.New(`module main
fn Ops(x: float, y: float) bool {
 let z: float := 1.25
 discard z
 discard x % y
 return x != y
}
fn main() int { return 0 }
`))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 32})
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	m, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{RequestedModule: "main"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := Emit(m, testPlan(32))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`sec.float_source_scalar = "float"`, `sec.float_format = 32 : i32`, `sec.rounding = "nearest-even"`, `sec.fast_math = "none"`, `sec.contraction = "off"`, `sec.denormal = "preserve"`, `sec.comparison_predicate = "une"`, `sec.remainder_kind = "truncation"`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	for _, fn := range m.Functions {
		for _, b := range fn.Blocks {
			for i := range b.Operations {
				op := &b.Operations[i]
				if op.Kind == semantic.OpFloatBinary {
					op.FloatSemantics.FastMath = "fast"
					if _, err := Emit(m, testPlan(32)); err == nil || !strings.Contains(err.Error(), "float semantic facts") {
						t.Fatalf("emitter accepted or repaired bad policy: %v", err)
					}
					return
				}
			}
		}
	}
	t.Fatal("missing remainder")
}
