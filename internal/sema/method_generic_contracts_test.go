package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMethodGenericTemplateContracts checks method bodies before concrete
// demands: owner and method contracts are independent; structural member shape
// and a concrete type offering a member never enlarge a template's guarantees.
// Rules: rules/declarations/generics.md — §§9,12–15,33;
// rules/compiler/monomorphization.md — §§5.1–5.3.
func TestMethodGenericTemplateContracts(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		count int
	}{
		{"method_contracts_valid.sec", 0},
		{"method_contracts_invalid.sec", 7},
	} {
		source, err := os.ReadFile("../../testdata/sema/generics/" + fixture.name)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", fixture.name, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(source), fixture.name))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errors := a.Analyze(program)
					if len(errors) != fixture.count {
						t.Fatalf("got %d diagnostics, want %d: %v", len(errors), fixture.count, errors)
					}
					if fixture.count != 0 {
						want := map[int]string{
							16: "unknown function or type value.Name",
							20: "unknown function or type value.Name",
							24: "unknown function or type value.Extra",
							28: "unknown function or type value.Name",
							32: "operator + requires numeric operands",
							36: "requires mutable receiver",
							44: "unknown function or type value.Extra",
						}
						for _, diagnostic := range errors {
							fragment, ok := want[diagnostic.Line]
							if !ok || !strings.Contains(diagnostic.Message, fragment) || diagnostic.File != fixture.name || diagnostic.Column == 0 {
								t.Errorf("unexpected template diagnostic: %+v", diagnostic)
							}
							delete(want, diagnostic.Line)
						}
						if len(want) != 0 {
							t.Errorf("missing template failures: %v", want)
						}
					}
					concreteDemands := 0
					constraintCalls := 0
					for _, statement := range program.Statements {
						if function, ok := statement.(*ast.FunctionDeclaration); ok && function.Name.Value == "Demand" {
							for _, statement := range function.Body.Statements {
								var expression ast.Expression
								if discard, ok := statement.(*ast.DiscardStatement); ok {
									expression = discard.Value
								}
								if returned, ok := statement.(*ast.ReturnStatement); ok {
									expression = returned.Value
								}
								call, ok := expression.(*ast.CallExpression)
								if !ok {
									continue
								}
								target, ok := a.ResolvedCallTarget(call)
								if !ok || len(target.Function.Parameters) != 1 || target.Function.Parameters[0].Type.Name != "Concrete" {
									t.Fatalf("concrete demand missing: %+v", target)
								}
								concreteDemands++
							}
						}
						impl, ok := statement.(*ast.ImplStatement)
						if !ok || fixture.count != 0 {
							continue
						}
						for _, member := range impl.Members {
							method, ok := member.(*ast.FunctionDeclaration)
							if !ok {
								continue
							}
							for _, statement := range method.Body.Statements {
								var expression ast.Expression
								if returned, ok := statement.(*ast.ReturnStatement); ok {
									expression = returned.Value
								}
								if discard, ok := statement.(*ast.DiscardStatement); ok {
									expression = discard.Value
								}
								if evaluated, ok := statement.(*ast.ExpressionStatement); ok {
									expression = evaluated.Expression
								}
								call, ok := expression.(*ast.CallExpression)
								if !ok {
									continue
								}
								if _, ok := call.Callee.(*ast.MemberExpression); !ok {
									continue
								}
								target, ok := a.ResolvedCallTarget(call)
								if !ok || target.Function.ImplTarget != "T" && target.Function.ImplTarget != "U" {
									t.Fatalf("call lost constraint receiver: %+v", target)
								}
								constraintCalls++
							}
						}
					}
					if fixture.count != 0 && concreteDemands != 2 {
						t.Fatalf("concrete calls = %d, want 2", concreteDemands)
					}
					if fixture.count == 0 && constraintCalls != 8 {
						t.Fatalf("constraint calls = %d, want 8", constraintCalls)
					}
				})
			}
		}
	}
}
