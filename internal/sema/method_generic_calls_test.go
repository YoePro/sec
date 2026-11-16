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

// TestMethodGenericCallArguments keeps owner bindings separate from explicit
// method prefixes and argument/result inference, including the signature cache.
// Rules: rules/declarations/generics.md — §§9,18,20,25.
func TestMethodGenericCallArguments(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/generics/method_calls_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("%d/%s", width, depth), func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(source), "method_calls_valid.sec"))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				calls := 0
				for _, statement := range program.Statements {
					fn, ok := statement.(*ast.FunctionDeclaration)
					if !ok {
						continue
					}
					call := fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.CallExpression)
					fact, ok := a.ResolvedCallTarget(call)
					if !ok {
						t.Fatalf("%s: no selected method", fn.Name.Value)
					}
					owner := fact.Function.GenericSubstitution["T"]
					wantOwner := "int"
					if fn.Name.Value == "SimpleInstance" || fn.Name.Value == "SimpleStatic" {
						wantOwner = ""
					}
					if fn.Name.Value == "OtherOwner" || fn.Name.Value == "SameMethodArgsOtherOwner" {
						wantOwner = "bool"
					}
					if owner.Name != wantOwner {
						t.Fatalf("%s owner = %s, want %s; %+v", fn.Name.Value, owner.Name, wantOwner, fact.Function)
					}
					wantU := "bool"
					if fn.Name.Value == "Prefix" || fn.Name.Value == "StaticPrefix" || fn.Name.Value == "OtherOwner" || fn.Name.Value == "SimpleStatic" {
						wantU = "string"
					}
					if fact.Function.GenericSubstitution["U"].Name != wantU {
						t.Fatalf("%s method U = %+v, want %s", fn.Name.Value, fact.Function.GenericSubstitution["U"], wantU)
					}
					if fn.Name.Value == "Prefix" || fn.Name.Value == "StaticPrefix" || fn.Name.Value == "SimpleStatic" {
						if fact.Function.GenericSubstitution["V"].Name != "bool" {
							t.Fatalf("%s failed to infer V", fn.Name.Value)
						}
					}
					if fn.Name.Value == "Expected" || fn.Name.Value == "ExpectedInferred" {
						if fact.Function.GenericSubstitution["V"].Name != "string" {
							t.Fatalf("%s ignored expected result", fn.Name.Value)
						}
					}
					if len(fact.Function.GenericParameters) != 0 || fact.Function.ReturnType.Kind == GenericType {
						t.Fatalf("%s unresolved signature: %+v", fn.Name.Value, fact.Function)
					}
					calls++
				}
				if calls != 13 {
					t.Fatalf("selected calls = %d, want 13", calls)
				}
			})
		}
	}
}

// TestMethodGenericCallFailures keeps fixed owner/explicit arguments
// authoritative and diagnoses unresolved arguments and failed constraints.
// Rules: rules/declarations/generics.md — §§14,18,20;
// rules/declarations/static.md — §8.
func TestMethodGenericCallFailures(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/generics/method_calls_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "method_calls_invalid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errors := NewAnalyzer().Analyze(program)
	if len(errors) != 8 {
		t.Fatalf("got %d diagnostics, want 8: %v", len(errors), errors)
	}
	seen := map[int]bool{}
	for _, diagnostic := range errors {
		if diagnostic.Line < 12 || diagnostic.Line > 19 || seen[diagnostic.Line] {
			t.Errorf("unexpected/duplicate failure: %+v", diagnostic)
		}
		seen[diagnostic.Line] = true
		if diagnostic.Line == 14 && !strings.Contains(diagnostic.Message, "requires 1 explicit generic arguments") {
			t.Errorf("owner parameter counted in method arity: %+v", diagnostic)
		}
		if diagnostic.Line == 16 && !strings.Contains(diagnostic.Message, "does not satisfy constraint Named for U") {
			t.Errorf("method constraint lost: %+v", diagnostic)
		}
	}
}
