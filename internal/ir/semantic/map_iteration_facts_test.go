package semantic

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestMapEntryIterationConsumerFacts consumes frontend entry facts without
// synthesizing Iterator.Next or imposing an order. The missing executable map
// representation still fails explicitly before publishing a partial module.
// Rules: rules/collections/collections.md — §14.6;
// rules/compiler/semantic_ir.md — §91 "Semantic plans versus AST".
func TestMapEntryIterationConsumerFacts(t *testing.T) {
	const path = "../../../testdata/sema/map_iteration_valid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	count := 0
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		for _, statement := range fn.Body.Statements {
			loop, ok := statement.(*ast.ForStatement)
			if !ok {
				continue
			}
			fact, known := a.ForIterationDependenciesOf(loop)
			if !known || fact.MapEntries == nil {
				continue
			}
			count++
			if fact.MapEntries.Order != sema.MapIterationOrderUnspecified || !fact.BodyValid {
				t.Fatal(fact)
			}
			if _, protocol := a.ResolvedForIterationOf(loop); protocol {
				t.Fatal("entry loop invented a protocol dispatch plan")
			}
		}
	}
	if count != 7 {
		t.Fatal(count)
	}
	if module, err := Build(program, a, BuildOptions{RequestedModule: "main"}); err == nil || module != nil {
		t.Fatal("map representation boundary lost", module, err)
	}
}
