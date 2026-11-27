package secmlir

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Canonical defaults reach existing static globals, including nominal scalar
// identity and aggregate values, without startup code or a new dialect schema.
// Rules: rules/types/default_values.md — §§40-41;
// rules/compiler/semantic_ir.md — §28; rules/declarations/static.md — §§15-16.
func TestStaticDefaultsEmitGlobals(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/static_defaults/values.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), "static-defaults.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(testPlan(width))
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 17})
		if err != nil {
			t.Fatal(err)
		}
		options := EmitOptions{DialectVersion: 13}
		output, err := EmitWithOptions(module, testPlan(width), options)
		if err != nil {
			t.Fatal(err)
		}
		again, err := EmitWithOptions(module, testPlan(width), options)
		if err != nil || !bytes.Equal(output, again) {
			t.Fatal("nondeterministic default emission", err)
		}
		if strings.Count(string(output), `"sec.static.global"`) != 7 || !strings.Contains(string(output), `initial = "2"`) {
			t.Fatal("missing canonical globals")
		}
		if strings.Contains(string(output), `"sec.call"`) {
			t.Fatal("hidden initialization call")
		}
		if !strings.Contains(string(output), `!sec.named<"main::Positive", !sec.int>`) {
			t.Fatal("static verification erased nominal type identity")
		}
		verifyPackage15MLIR(t, "static_defaults", output)
	}
}
