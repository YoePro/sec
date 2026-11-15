package semantic

import (
	"fmt"
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Every source member must retain its primitive source type and selected width,
// including checked Clamp, on both target plans. Native edge values are checked
// separately; this matrix prevents the source-to-IR mapping from being bypassed.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§28–36,63–69,87;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md §2.
func TestFloatCoreSourceMatrix(t *testing.T) {
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
			module, err := Build(parsed.Program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{"float_core_members.sec"}})
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(module); err != nil {
				t.Fatal(err)
			}
			expected := map[string]uint16{"CoreF32": 32, "CoreF64": 64, "CorePlatform": width}
			for _, fn := range module.Functions {
				bits, ok := expected[fn.Name]
				if !ok {
					continue
				}
				delete(expected, fn.Name)
				sourceType := map[string]string{"CoreF32": "float32", "CoreF64": "float64", "CorePlatform": "float"}[fn.Name]
				seen := map[string]int{}
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						switch op.Kind {
						case OpFloatAbs, OpFloatMinimum, OpFloatMaximum, OpFloatClampChecked, OpFloatClassify, OpFloatRound, OpFloatProperty:
							seen[string(op.Kind)+":"+op.FloatCore]++
							// Classification produces bool; all other candidates/properties produce T.
							result, found := module.Types.Lookup(op.Results[0].Type)
							if op.Kind != OpFloatClassify && (!found || result.BitWidth != bits || result.Name != sourceType) {
								t.Fatalf("%s wrong float width for %s: %+v", fn.Name, op.Kind, op.Results)
							}
						}
					}
				}
				for _, kind := range []OpKind{OpFloatAbs, OpFloatMinimum, OpFloatMaximum, OpFloatClampChecked} {
					if seen[string(kind)+":"] != 1 {
						t.Fatalf("%s missing %s: %v", fn.Name, kind, seen)
					}
				}
				for kind, variants := range map[OpKind][]string{
					OpFloatProperty: {"min", "max", "epsilon", "infinity", "negative-infinity", "nan"},
					OpFloatClassify: {"finite", "infinite", "nan", "normal", "subnormal"},
					OpFloatRound:    {"floor", "ceiling", "truncate", "nearest-even"},
				} {
					for _, v := range variants {
						if seen[string(kind)+":"+v] != 1 {
							t.Fatalf("%s missing %s %s: %v", fn.Name, kind, v, seen)
						}
					}
				}
			}
			if len(expected) != 0 {
				t.Fatalf("missing source types: %v", expected)
			}
		})
	}
}
