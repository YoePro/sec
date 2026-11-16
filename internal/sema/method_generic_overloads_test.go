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

// TestMethodGenericOverloads ranks method inputs before completing result-only
// inference, preserving constraints, owner arguments and fixed/variadic rules.
// Rules: rules/declarations/functions.md — §§19–20;
// rules/declarations/generics.md — §§9,18,20,29.
func TestMethodGenericOverloads(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		count int
	}{
		{"method_overloads_valid.sec", 0},
		{"method_overloads_invalid.sec", 4},
		{"method_overload_duplicates_invalid.sec", 2},
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
					if fixture.name == "method_overloads_invalid.sec" {
						ambiguous, resultMismatch := 0, 0
						for _, diagnostic := range errors {
							if strings.Contains(diagnostic.Message, "ambiguous") {
								ambiguous++
							} else if strings.Contains(diagnostic.Message, "must return") {
								resultMismatch++
							} else {
								t.Errorf("unexpected diagnostic: %+v", diagnostic)
							}
						}
						if ambiguous != 3 || resultMismatch != 1 {
							t.Fatalf("ambiguity/result failures = %d/%d: %v", ambiguous, resultMismatch, errors)
						}
					}
					if fixture.name == "method_overload_duplicates_invalid.sec" {
						for _, diagnostic := range errors {
							if diagnostic.ID != diagnostics.DuplicateDeclaration || diagnostic.PreviousLine == 0 {
								t.Errorf("missing duplicate declaration identity: %+v", diagnostic)
							}
						}
					}
					if fixture.count != 0 {
						return
					}
					calls := 0
					for _, statement := range program.Statements {
						fn, ok := statement.(*ast.FunctionDeclaration)
						if !ok {
							continue
						}
						call := fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.CallExpression)
						target, ok := a.ResolvedCallTarget(call)
						if !ok {
							t.Fatalf("%s has no selected method", fn.Name.Value)
						}
						switch fn.Name.Value {
						case "IntegerTag", "StaticIntegerTag":
							if target.Function.Parameters[0].Type.Name != "int" {
								t.Fatal("wrong tag overload", target)
							}
						case "BooleanTag", "StaticBooleanTag":
							if target.Function.Parameters[0].Type.Name != "bool" {
								t.Fatal("wrong tag overload", target)
							}
						case "ConstraintFails":
							if len(target.Function.GenericSubstitution) != 1 {
								t.Fatalf("failed generic constraint selected: %+v", target)
							}
						case "ConstraintSatisfied":
							if target.Function.GenericSubstitution["U"].Name != "Label" {
								t.Fatal("lost explicit conformance", target)
							}
						case "FixedArity":
							if target.Function.Parameters[0].Variadic {
								t.Fatal("variadic overload displaced exact fixed overload")
							}
						case "SelectedResult":
							if target.Function.Parameters[0].Ref || target.Function.GenericSubstitution["V"].Name != "string" {
								t.Fatal("result context selected inputs or lost final inference", target)
							}
						}
						if target.Function.GenericSubstitution["T"].Name != "int" {
							t.Fatal("owner environment lost", target)
						}
						calls++
					}
					if calls != 8 {
						t.Fatalf("selected %d calls, want 8", calls)
					}
				})
			}
		}
	}
}
