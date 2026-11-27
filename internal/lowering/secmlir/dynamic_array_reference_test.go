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

// Reference-reached dynamic owners use schema-14 observation and guarded
// element Places with ordinary CFG; it adds no allocation or ownership copy.
// Rules: rules/foundations/operators.md — §67;
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§40-45,110.
func TestDynamicArrayReferenceEmission(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/dynamic_references/observe.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), "dynamic-reference.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		analyzer := sema.NewAnalyzerWithScalarPlan(testPlan(width))
		if errors := analyzer.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		module, err := semantic.Build(program, analyzer, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 18})
		if err != nil {
			t.Fatal(err)
		}
		options := EmitOptions{DialectVersion: 14}
		output, err := EmitWithOptions(module, testPlan(width), options)
		if err != nil {
			t.Fatal(err)
		}
		again, err := EmitWithOptions(module, testPlan(width), options)
		if err != nil || !bytes.Equal(output, again) {
			t.Fatal("unstable search emission", err)
		}
		text := string(output)
		for _, want := range []string{`"sec.place.deref"`, `"sec.dynamic_array.len"`, `"sec.dynamic_array.index_in_bounds"`, `"sec.dynamic_array.element_place"`, `bounds_kind = "runtime-check", bounds_proof = "guarded"`, `backing_domain = `} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %s", want)
			}
		}
		if strings.Count(text, `"sec.dynamic_array.element_place"`) != 5 {
			t.Fatal("missing reference-reached index/search forms")
		}
		for _, forbidden := range []string{`"sec.dynamic_array.allocate"`, `"sec.dynamic_array.grow_backing"`, `"sec.own.semantic_copy"`} {
			if strings.Contains(text, forbidden) {
				t.Fatal("search allocated or copied owner", forbidden)
			}
		}
		if output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13}); err == nil || len(output) != 0 {
			t.Fatal("dynamic membership bypassed schema boundary")
		}
	}
}
