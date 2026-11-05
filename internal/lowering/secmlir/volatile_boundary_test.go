package secmlir

import (
	"errors"
	"os"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Raw volatile access must fail before raw-pointer type erasure or partial
// output. A physical plan is not implied by unsafe authorization or bit width.
// Rules: rules/platform/volatile.md §§9–12; rules/memory/copy_move.md §19.
func TestVolatileEmissionRequiresPhysicalPlan(t *testing.T) {
	const path = "../../../testdata/semantic_ir/volatile_snapshot/main.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		plan := testPlan(width)
		p := parser.New(lexer.NewWithFile(string(source), path))
		parsed := p.Parse()
		if parsed.HasErrors {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(plan)
		if errs := a.Analyze(parsed.Program); len(errs) != 0 {
			t.Fatal(errs)
		}
		module, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 23})
		if err != nil {
			t.Fatal(err)
		}
		original := append([]*semantic.Function(nil), module.Functions...)
		for _, kind := range []string{"read", "write"} {
			module.Functions = append([]*semantic.Function(nil), original...)
			if kind == "write" {
				for i, fn := range module.Functions {
					if fn.Name == "Ordered" {
						module.Functions[0], module.Functions[i] = module.Functions[i], module.Functions[0]
						break
					}
				}
			}
			output, err := EmitWithOptions(module, plan, EmitOptions{DialectVersion: 13})
			var unsupported *UnsupportedLoweringError
			if len(output) != 0 || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "volatile."+kind+" needs a verified physical volatile access plan") {
				t.Fatalf("%d-bit %s output=%q error=%v", width, kind, output, err)
			}
		}
	}
}
