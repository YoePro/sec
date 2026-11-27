package semantic

import (
	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/sema"
	"testing"
)

// Defaulted statics consume Sema's canonical initializer and never require
// startup functions or reinterpret a compact default's syntax preview.
// Rules: rules/types/default_values.md — §§40-41;
// rules/declarations/static.md — §§15-16; rules/compiler/semantic_ir.md — §28.
func TestStaticDefaultsMaterialize(t *testing.T) {
	program := sliceBoundaryProgram(t, "semantic_ir/static_defaults/values.sec")
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	// The analyzed facts, rather than synthesized compatibility syntax, own
	// initialization. Clearing the preview must not change any static value.
	for _, statement := range program.Statements {
		if declaration, ok := statement.(*ast.LetStatement); ok && declaration.SynthesizedDefault {
			if preview, array := declaration.Value.(*ast.ArrayLiteral); array {
				preview.Elements = nil
			}
		}
	}
	module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	serialized, err := Serialize(module)
	if err != nil {
		t.Fatal(err)
	}
	module, err = Deserialize(serialized)
	if err != nil {
		t.Fatal(err)
	}
	if len(module.Statics) != 7 {
		t.Fatalf("statics: %d", len(module.Statics))
	}
	for _, static := range module.Statics {
		switch static.Name {
		case "main::backing", "main::initial":
			if !static.Initializer.Aggregate || len(static.Initializer.Components) != 64 {
				t.Fatal(static)
			}
		case "main::beyondPreview":
			if len(static.Initializer.Components) != 4097 {
				t.Fatal("truncated default", len(static.Initializer.Components))
			}
		case "main::count":
			if static.Initializer.Integer == nil || static.Initializer.Integer.String() != "2" {
				t.Fatal("invented zero instead of range default", static)
			}
		case "main::settings":
			if len(static.Initializer.Components) != 3 || len(static.Initializer.Components[0].Components) != 64 || static.Initializer.Components[1].Integer.String() != "2" || *static.Initializer.Components[2].Bool {
				t.Fatal(static)
			}
		case "main::empty":
			if !static.Initializer.Aggregate || len(static.Initializer.Components) != 0 {
				t.Fatal(static)
			}
		case "main::nested":
			if len(static.Initializer.Components) == 0 || len(static.Initializer.Components[0].Components) == 0 {
				t.Fatal(static)
			}
		}
	}
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpDirectCall {
					t.Fatal("hidden runtime initialization")
				}
			}
		}
	}
}
