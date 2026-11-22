package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTryNegativeLiteral excludes proven-safe signed literals from a protected
// call's failure set while retaining dynamic negation and exactly one S1093.
// Rules: rules/errors/runtime_checks.md — §§5.1,7;
// rules/errors/errorhandling.md — §§9,11–13.
func TestTryNegativeLiteral(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			for _, mode := range []string{"valid", "invalid"} {
				path := "../../testdata/sema/try_negative_literal_" + mode + ".sec"
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				p := parser.New(lexer.NewWithFile(string(data), path))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				errors := a.Analyze(program)
				want := 0
				if mode == "invalid" {
					want = 1
				}
				if len(errors) != want {
					t.Fatalf("%d/%s/%s: %+v", width, depth, mode, errors)
				}
				if mode == "invalid" && errors[0].ID != "S1093" {
					t.Fatal(errors)
				}
				ast.Visit(program, func(node any) bool {
					prefix, ok := node.(*ast.PrefixExpression)
					if !ok || prefix.Operator != "-" {
						return true
					}
					fact, ok := a.ResolvedOperatorOf(prefix)
					if !ok || fact.RuntimeCheck != (mode == "invalid") {
						t.Fatal(mode, fact)
					}
					return true
				})
			}
		}
	}
}
