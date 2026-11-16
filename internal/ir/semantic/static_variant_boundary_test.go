package semantic

import (
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Sema's typed static variants reach Semantic IR without an older aggregate
// or scalar encoder dropping a tag or payload (SVV-2): every static of the
// fixture carries its active variant tree, verified against its type.
// Rules: rules/compiler/compile_time_evaluation.md — §32(3–4);
// rules/compiler/semantic_ir.md — §28.
func TestStaticVariantFactsReachSemanticIR(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/sema/static_variants_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), "static_variants_valid.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		for _, pkg := range []uint8{17, 18, 19} {
			module, err := Build(program, a, BuildOptions{RequestedModule: "static_variants_valid", MaxPackage: pkg})
			if err != nil {
				t.Fatalf("width %d package %d: %v", width, pkg, err)
			}
			if err := Verify(module); err != nil {
				t.Fatalf("width %d package %d: %v", width, pkg, err)
			}
			if len(module.Statics) != 17 {
				t.Fatalf("width %d package %d: %d statics", width, pkg, len(module.Statics))
			}
			for _, static := range module.Statics {
				if static.Initializer.Variant == nil {
					t.Fatalf("width %d package %d: %s has no variant initializer", width, pkg, static.Name)
				}
			}
			if !strings.Contains(Format(module), "init=Err(.Bad)") {
				t.Fatal("error-enum payload was not preserved")
			}
		}
	}
}
