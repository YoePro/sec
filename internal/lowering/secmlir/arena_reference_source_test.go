package secmlir

import (
	"bytes"
	"os"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
	"sec/internal/sema"
	"strings"
	"testing"
)

// Source Arena.New references retain P15 dynamic validity through schema 15;
// physical references and Arena execution are separate lowering boundaries.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §102;
// rules/mlir/packages/sec-mlir-dialect_package15.md — §§59-63.
func TestArenaReferenceSourceEmission(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/arena_references/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"amd64", "armv7"} {
		definition, _ := target.Find(target.Target{OS: "linux", Arch: arch})
		plan, err := definition.ScalarPlan()
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(source), "arena-reference.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(plan)
		a.SetAllocationProfile("hosted")
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		m, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 19})
		if err != nil {
			t.Fatal(err)
		}
		options := EmitOptions{DialectVersion: 15}
		wire, err := EmitWithOptions(m, plan, options)
		if err != nil {
			t.Fatal(err)
		}
		again, err := EmitWithOptions(m, plan, options)
		if err != nil || !bytes.Equal(wire, again) {
			t.Fatal("nondeterministic emission", err)
		}
		for _, want := range []string{`"sec.arena.new"`, `"sec.ref.is_valid"`, `"sec.place.deref"`, `"sec.fail.reference_generation"`, `dynamic-epoch`} {
			if !strings.Contains(string(wire), want) {
				t.Fatalf("missing %s", want)
			}
		}
		if strings.Count(string(wire), `"sec.ref.is_valid"`) != 5 {
			t.Fatal("source validity checks missing/duplicated")
		}
	}
}
