package secmlir

import (
	"os"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestOverloadNominalIdentity preserves distinct same-arity nominal signatures
// even when every physical parameter is an eight-bit integer.
// Rules: rules/declarations/functions.md — §§19–20;
// rules/compiler/semantic_ir.md — §61; rules/mlir/sec_mlir_lowering.md — semantic identity preservation.
func TestOverloadNominalIdentity(t *testing.T) {
	const file = "../../../testdata/build/native/overload_nominal_identity.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), file))
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
		output, err := Emit(module, testPlan(width))
		if err != nil {
			t.Fatal(err)
		}
		symbols, err := resolvedFunctionSymbols(module)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, id := range []semantic.FunctionID{"main::Choose(main::Signed,main::Unsigned)", "main::Choose(main::Unsigned,main::Signed)", "main::Choose(int8,uint8)"} {
			symbol := symbols[id]
			if symbol == "" || seen[symbol] {
				t.Fatal("collapsed overload", id, symbols)
			}
			seen[symbol] = true
			if !strings.Contains(string(output), "func.func @"+symbol+"(") || !strings.Contains(string(output), "callee = @"+symbol+"}") {
				t.Fatal("call/definition identity lost", id)
			}
		}
	}
}
