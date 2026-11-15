package secmlir

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Emit every core member from verified source facts on 32/64-bit plans. Clamp
// remains a checked candidate plus failure, not a total minimum/maximum alias.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§28–36,69,74–78,87;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md §2.
func TestEmitFloatCoreSourceMatrix(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/float_core_members.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(data), "float_core_members.sec"))
			parsed := p.Parse()
			if parsed.HasErrors {
				t.Fatal(p.Errors())
			}
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errs := a.Analyze(parsed.Program); len(errs) != 0 {
				t.Fatal(errs)
			}
			m, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"float_core_members.sec"}})
			if err != nil {
				t.Fatal(err)
			}
			output, err := Emit(m, testPlan(width))
			if err != nil {
				t.Fatal(err)
			}
			text := string(output)
			for _, op := range []string{"abs", "minimum", "maximum", "clamp_checked"} {
				if n := strings.Count(text, `"sec.float.`+op+`"`); n != 3 {
					t.Fatalf("%s emitted %d times, want one per source type", op, n)
				}
			}
			for op, kinds := range map[string][]string{"property": {"min", "max", "epsilon", "infinity", "negative-infinity", "nan"}, "classify": {"finite", "infinite", "nan", "normal", "subnormal"}, "round": {"floor", "ceiling", "truncate", "nearest-even"}} {
				for _, kind := range kinds {
					count := 0
					for _, line := range strings.Split(text, "\n") {
						if strings.Contains(line, `"sec.float.`+op+`"`) && strings.Contains(line, `kind = "`+kind+`"`) {
							count++
						}
					}
					if count != 3 {
						t.Fatalf("%s %s emitted %d times, want all three types", op, kind, count)
					}
				}
			}
		})
	}
}
