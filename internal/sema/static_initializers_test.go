package sema

import (
	"os"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// Static materialization uses initial values and canonical definition bindings,
// including forward and function-local dependencies. Published values are isolated.
// Rules: rules/declarations/static.md §§3–4,15–16; rules/compiler/compile_time_evaluation.md §47.
func TestResolvedStaticScalarInitializers(t *testing.T) {
	file := "../../testdata/semantic_ir/static_cte/scalars.sec"
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), file))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errs := a.Analyze(program); len(errs) > 0 {
			t.Fatal(errs)
		}
		expected := map[string]string{"Base": "42", "Forward": "52", "Later": "10", "Negative": "-3", "Remainder": "-1", "Bits": "4099", "Count": "42", "InitialCount": "43", "Shadow::Seed": "3", "Shadow::Local": "6", "Step::calls": "6", "Other::Seed": "5", "Other::Local": "10", "WideSeed": "1606938044258990275541962092341162602522202993782792835301376", "Wide": "3213876088517980551083924184682325205044405987565585670602752"}
		for _, binding := range a.ResolvedStaticDeclarations() {
			name := binding.Name
			if binding.Function != "" {
				name = binding.Function + "::" + name
			}
			if name == "Ready" || name == "IsRounded" {
				want := name == "Ready" || width == 32
				if binding.Initializer.Bool == nil || *binding.Initializer.Bool != want {
					t.Fatalf("%d %s: %+v", width, name, binding.Initializer)
				}
				*binding.Initializer.Bool = !want
				actual, _ := a.ResolvedStaticOf(binding.Declaration.Name)
				if *actual.Initializer.Bool != want {
					t.Fatal("mutable bool snapshot")
				}
				continue
			}
			want, exists := expected[name]
			if !exists || binding.Initializer.Integer == nil || binding.Initializer.Integer.String() != want {
				t.Fatalf("%d %s initializer %+v want %s", width, name, binding.Initializer, want)
			}
			binding.Initializer.Integer.SetInt64(999)
			again, _ := a.ResolvedStaticOf(binding.Declaration.Name)
			if again.Initializer.Integer.String() != want {
				t.Fatal("mutable integer snapshot")
			}
			delete(expected, name)
		}
		if len(expected) > 0 {
			t.Fatal("missing initializers", expected)
		}
		if errs := a.Analyze(program); len(errs) > 0 {
			t.Fatal(errs)
		}
		for _, binding := range a.ResolvedStaticDeclarations() {
			if binding.Initializer.Integer != nil && binding.Initializer.Integer.Int64() == 999 {
				t.Fatal("facts leaked across analysis")
			}
		}
	}
}

// Invalid source never publishes materialization facts, nor falls back to a
// runtime initializer. Rules: rules/declarations/static.md §§15–16.
func TestInvalidStaticScalarInitializers(t *testing.T) {
	for _, name := range []string{"overflow_invalid.sec", "cycle_invalid.sec", "capture_invalid.sec", "runtime_invalid.sec"} {
		file := "../../testdata/semantic_ir/static_cte/" + name
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		if errors := a.Analyze(program); len(errors) == 0 {
			t.Fatalf("accepted %s", name)
		}
		for _, binding := range a.ResolvedStaticDeclarations() {
			if binding.Initializer.Integer != nil || binding.Initializer.Bool != nil {
				t.Fatalf("invalid initializer published: %s", name)
			}
		}
	}
}
