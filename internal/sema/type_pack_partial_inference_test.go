package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Argument-only candidates remain uncached; a unique winner receives missing
// result-related types and complete constraints before any call transfer commits.
// Rules: rules/declarations/generics.md §§14,18,20,29;
// rules/declarations/functions.md §38A(4)(5),(6)(1–2),(9).
func TestTypePackPartialOverloadInference(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, test := range []struct{ name, errorText string }{
			{"overloaded_result_context.sec", ""},
			{"overloaded_result_once.sec", ""},
			{"overloaded_result_none.sec", ""},
			{"overloaded_result_ambiguous_invalid.sec", "ambiguous call"},
			{"overloaded_result_missing_invalid.sec", "cannot infer generic arguments"},
			{"overloaded_result_constraint_invalid.sec", "does not satisfy constraint"},
			{"overloaded_result_fixed_invalid.sec", "must return bool, got int"},
		} {
			data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + test.name)
			if err != nil {
				t.Fatal(err)
			}
			parsed := parser.New(lexer.New(string(data))).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			errors := a.Analyze(parsed.Program)
			if test.errorText != "" {
				if len(errors) != 1 || errors[0].ID == diagnostics.GenericTypePackExpansionUnavailable || !strings.Contains(errors[0].Message, test.errorText) {
					t.Fatal(width, test.name, errors)
				}
				if test.name != "overloaded_result_fixed_invalid.sec" {
					ast.Visit(parsed.Program, func(node any) bool {
						if call, ok := node.(*ast.CallExpression); ok {
							if _, selected := a.ResolvedCallTarget(call); selected {
								t.Fatal("failed inference retained target", test.name)
							}
						}
						return true
					})
				}
			} else {
				if len(errors) != 0 {
					t.Fatal(width, test.name, errors)
				}
				calls := 0
				ast.Visit(parsed.Program, func(node any) bool {
					call, ok := node.(*ast.CallExpression)
					if !ok {
						return true
					}
					target, known := a.ResolvedCallTarget(call)
					if !known {
						t.Fatal("missing committed target", test.name)
					}
					if target.Function.GenericTypePack == "" {
						return true
					}
					calls++
					if typePackPatternNeedsInference(target.Function.ReturnType) {
						t.Fatal("unresolved selected return", target)
					}
					if _, known := a.ResolvedTypeOf(call); !known {
						t.Fatal("missing selected expression type")
					}
					return true
				})
				want := 4
				if test.name == "overloaded_result_once.sec" || test.name == "overloaded_result_none.sec" {
					want = 1
				}
				if calls != want {
					t.Fatal(test.name, calls)
				}
			}
			for _, instance := range a.genericFuncInstances {
				if typePackPatternNeedsInference(instance.ReturnType) {
					t.Fatal("partial candidate entered instance cache", instance)
				}
			}
			for _, template := range a.Functions()["Context"] {
				if len(template.GenericSubstitution) != 0 {
					t.Fatal("template mutated by candidate completion")
				}
			}
		}
	}
}
