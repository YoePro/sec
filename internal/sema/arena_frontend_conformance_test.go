package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestArenaFrontendConformance exercises the complete P19 frontend matrix using
// defined constructors, including non-lexical last use, owner return, zero
// backing/count, ownership termination and retained nested dependencies.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §149;
// rules/memory/arena.md — §§4–13,19–29,36–46,48–51.
func TestArenaFrontendConformance(t *testing.T) {
	for _, file := range []string{"arena_frontend_valid.sec", "arena_frontend_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/sema/" + file)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{0, 32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", file, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(data), file))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errors := a.Analyze(program)
					if file == "arena_frontend_valid.sec" {
						if len(errors) != 0 {
							t.Fatal(errors)
						}
						kinds := map[ArenaOperationKind]bool{}
						constructors := map[string]bool{}
						for _, plan := range a.arenaOperationPlans {
							kinds[plan.Operation] = true
							if plan.Operation == ArenaOperationCreate {
								constructors[plan.Constructor] = true
							}
						}
						if len(kinds) != 5 || len(constructors) != 3 {
							t.Fatal("valid matrix lost operation/constructor coverage", kinds, constructors)
						}
						return
					}
					want := map[string]string{
						"CopyOwner":            "cannot copy move-only value",
						"UseAfterRelease":      "was released here and is no longer available",
						"DoubleRelease":        "was released here and is no longer available",
						"ReturnLocalReference": "arena",
						"ResetLiveReference":   "cannot call Arena.Reset",
						"ResetLiveSlice":       "cannot call Arena.Reset",
						"ResetParent":          "cannot call Arena.Reset",
					}
					if len(errors) != len(want) {
						t.Fatalf("got %d errors, want %d: %v", len(errors), len(want), errors)
					}
					for _, statement := range program.Statements {
						function, ok := statement.(*ast.FunctionDeclaration)
						if !ok {
							continue
						}
						name := function.Name.Value
						found := 0
						for _, diagnostic := range errors {
							if diagnostic.Line < function.Token.Line || diagnostic.Line > function.EndToken.Line {
								continue
							}
							found++
							if !strings.Contains(diagnostic.Message, want[name]) {
								t.Fatalf("%s: wrong diagnostic %v", name, diagnostic)
							}
							if strings.HasPrefix(name, "Reset") {
								if diagnostic.ID != diagnostics.ArenaResetLiveDependency || diagnostic.PreviousLine == 0 {
									t.Fatal("reset must report the live dependency", diagnostic)
								}
							}
							for _, plan := range a.arenaOperationPlans {
								if plan.Source.Line == diagnostic.Line && (plan.Operation == ArenaOperationReset || plan.Operation == ArenaOperationRelease) {
									t.Fatal("rejected invalidation published an operation plan", plan)
								}
							}
						}
						if found != 1 {
							t.Fatalf("%s: got %d diagnostics, want one", name, found)
						}
					}
				})
			}
		}
	}
}
