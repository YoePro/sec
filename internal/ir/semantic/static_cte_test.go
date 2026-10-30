package semantic

import (
	"math/big"
	"os"
	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"testing"
)

// P15 static scalar producers consume frozen Sema CTE facts without emitting
// startup calls or expressions; mutable initial values persist across calls.
// Rules: rules/declarations/static.md §§15–16; rules/compiler/semantic_ir.md §28.
func TestStaticScalarCTEProducer(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/static_cte/scalars.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), "static-cte.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errs := a.Analyze(program); len(errs) > 0 {
			t.Fatal(errs)
		}
		// If IR inspected the expression again, Base would become 999. The frozen
		// initializer and every dependency must retain the analyzed value 42.
		for _, stmt := range program.Statements {
			if let, ok := stmt.(*ast.LetStatement); ok && let.Name.Value == "Base" {
				let.Value = &ast.IntegerLiteral{Token: lexer.Token{Lexeme: "999"}, Value: 999}
			}
		}
		m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 15})
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		for _, static := range m.Statics {
			switch static.Name {
			case "main::Base":
				if static.Initializer.Integer.String() != "42" {
					t.Fatal("IR reevaluated initializer")
				}
			case "main::Forward":
				if static.Initializer.Integer.String() != "52" {
					t.Fatal("lost forward initializer")
				}
			}
		}
		for _, fn := range m.Functions {
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					if op.Kind == OpDirectCall {
						t.Fatal("hidden initializer call")
					}
				}
			}
		}
		for name, mutate := range map[string]func(*Module){
			"conflicting payloads":  func(m *Module) { value := true; m.Statics[0].Initializer.Bool = &value },
			"target-width overflow": func(m *Module) { m.Statics[0].Initializer.Integer = new(big.Int).Lsh(big.NewInt(1), uint(width-1)) },
		} {
			original := m.Statics[0].Initializer
			mutate(m)
			if err := Verify(m); err == nil {
				t.Fatal("accepted " + name)
			}
			m.Statics[0].Initializer = original
		}
	}
}
