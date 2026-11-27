package secmlir

import (
	"fmt"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// The real source-to-MLIR boundary admits concrete generic calls on both scalar
// plans, but emits nothing when executable type facts are later made unresolved,
// including metadata not reachable from a function's immediate signature.
// Rules: rules/compiler/generics_lowering.md — §§4–5,7–8.
func TestGenericConcretenessEmissionBoundary(t *testing.T) {
	const source = `module main
 type Box[T] struct { Value: T }
 fn Id[T](value: T) T { return value }
 fn Read(value: Box[int]) int { return Id[int](value.Value) }
 `
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(source, "generic_closure.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil || len(output) == 0 {
				t.Fatalf("concrete specialization rejected: %v", err)
			}
			concrete := false
			for _, fn := range module.Functions {
				if fn.Specialization != nil {
					concrete = true
				}
			}
			if !concrete {
				t.Fatal("fixture did not demand a specialization")
			}
			module.Types.Intern(semantic.Type{Kind: "generic", Name: "Pending"})
			for _, schema := range []uint32{11, 12, 13} {
				output, err = EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: schema})
				if err == nil || output != nil || !strings.Contains(err.Error(), "generic concreteness:") {
					t.Fatalf("schema %d emitted unresolved executable types: %s, %v", schema, output, err)
				}
			}
		})
	}
}
