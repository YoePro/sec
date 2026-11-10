package secmlir

import (
	"bytes"
	"os"
	"sec/internal/ast"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestOverloadSpecializationSymbols keeps a sole demanded specialization of
// an overload family independent of declaration order, target width and IR
// ordering, and preserves the same symbol at its exact selected call.
// Rules: rules/declarations/functions.md — §§19–20;
// rules/compiler/monomorphization.md — §§4.1,15.3;
// rules/compiler/semantic_ir.md — §27A.
func TestOverloadSpecializationSymbols(t *testing.T) {
	const file = "../../../testdata/build/native/overload_specialization_identity.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var baseline string
	for _, width := range []uint16{32, 64} {
		for _, reverseSource := range []bool{false, true} {
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			if reverseSource {
				var functions []ast.Statement
				var header []ast.Statement
				for _, stmt := range program.Statements {
					if _, ok := stmt.(*ast.FunctionDeclaration); ok {
						functions = append(functions, stmt)
					} else {
						header = append(header, stmt)
					}
				}
				for i, j := 0, len(functions)-1; i < j; i, j = i+1, j-1 {
					functions[i], functions[j] = functions[j], functions[i]
				}
				program.Statements = append(header, functions...)
			}
			a := sema.NewAnalyzerWithScalarPlan(testPlan(width))
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			id := semantic.FunctionID("main::Pick[T=int8](int8)")
			symbols, err := resolvedFunctionSymbols(module)
			if err != nil || symbols[id] == "" {
				t.Fatal(symbols, err)
			}
			if baseline == "" {
				baseline = symbols[id]
			} else if symbols[id] != baseline {
				t.Fatal("source/target changed specialization symbol", symbols[id], baseline)
			}
			first, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatal(err)
			}
			again, err := Emit(module, testPlan(width))
			if err != nil || !bytes.Equal(first, again) {
				t.Fatal("repeated emission changed symbols", err)
			}
			if !strings.Contains(string(first), "callee = @"+baseline+"}") || !strings.Contains(string(first), "func.func @"+baseline+"(") {
				t.Fatal("call/definition disagree", string(first))
			}
			for i, j := 0, len(module.Functions)-1; i < j; i, j = i+1, j-1 {
				module.Functions[i], module.Functions[j] = module.Functions[j], module.Functions[i]
			}
			reordered, err := resolvedFunctionSymbols(module)
			if err != nil || reordered[id] != baseline {
				t.Fatal("IR order changed specialization symbol", reordered, baseline, err)
			}
			// A different module's same-spelled callable must receive a distinct
			// local symbol based on its producer-owned canonical FunctionID.
			for _, fn := range module.Functions {
				if fn.ID == id {
					other := *fn
					other.ID = "other::Pick[T=int8](int8)"
					module.Functions = append(module.Functions, &other)
					both, err := resolvedFunctionSymbols(module)
					if err != nil || both[id] != baseline || both[other.ID] == baseline {
						t.Fatal("module identity lost", both, err)
					}
					break
				}
			}
		}
	}
}
