package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules: rules/foundations/operators.md — "Boolean equality", "Compiler semantic facts".
func TestBooleanEqualityPublishesResolvedFacts(t *testing.T) {
	for _, file := range []string{"valid.sec", "operands_invalid.sec"} {
		t.Run(file, func(t *testing.T) {
			path := "../../testdata/semantic_ir/boolean_equality/" + file
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), path))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzer()
			errors := a.Analyze(program)
			invalid := file == "operands_invalid.sec"
			if (len(errors) > 0) != invalid {
				t.Fatalf("diagnostics = %v, invalid=%v", errors, invalid)
			}
			count := 0
			for _, statement := range program.Statements {
				fn, ok := statement.(*ast.FunctionDeclaration)
				if !ok || fn.Body == nil {
					continue
				}
				for _, statement := range fn.Body.Statements {
					ret, ok := statement.(*ast.ReturnStatement)
					if !ok {
						continue
					}
					expr, ok := ret.Value.(*ast.InfixExpression)
					if !ok {
						continue
					}
					fact, found := a.ResolvedOperatorOf(expr)
					if invalid {
						if found {
							t.Fatalf("invalid operands published a fact: %+v", fact)
						}
						continue
					}
					want := ResolvedBoolCompareEQ
					if expr.Operator == "!=" {
						want = ResolvedBoolCompareNE
					}
					if !found || fact.Kind != want || fact.LeftType.Name != "bool" || fact.RightType == nil || fact.RightType.Name != "bool" || fact.ResultType.Name != "bool" || fact.RuntimeCheck || fact.FailureBehavior != OperatorDoesNotFail {
						t.Fatalf("resolved bool equality = %+v, found=%v", fact, found)
					}
					count++
				}
			}
			if !invalid && count != 7 {
				t.Fatalf("equality facts = %d, want 7", count)
			}
		})
	}
}
