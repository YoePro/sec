package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
)

// TestSpreadRecoveryKeepsFollowingSource checks prefix, repeated and missing
// source recovery without silently turning malformed spread into a valid
// expansion, losing sibling statements or consuming a following declaration.
// Rules: rules/declarations/spread.md — §§1–2;
// rules/compiler/parser_recovery.md — §§7.3,14.4,63.5,82.
func TestSpreadRecoveryKeepsFollowingSource(t *testing.T) {
	for _, name := range []string{"prefix", "repeated", "missing_source"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/parser/spread_" + name + "_invalid.sec")
			if err != nil {
				t.Fatal(err)
			}
			outcome, finished := robustParse(string(data))
			if !finished || outcome.panicked != nil {
				t.Fatalf("spread recovery did not terminate normally: %+v", outcome)
			}
			result := outcome.result
			if !result.HasErrors {
				t.Fatal("malformed spread was silently accepted")
			}
			broken := functionNamed(result.Program, "Broken")
			after := functionNamed(result.Program, "After")
			if broken == nil || broken.Body == nil || after == nil || after.Body == nil {
				t.Fatal("lost enclosing/following function")
			}
			bindings := map[string]bool{}
			invalid := 0
			retainedSources := 0
			_ = walk.Inspect(broken.Body, func(node any) error {
				switch node := node.(type) {
				case *ast.LetStatement:
					if node.Name != nil {
						bindings[node.Name.Value] = true
					}
				case *ast.InvalidExpression:
					invalid++
					if name == "repeated" && node.Operator.Type == lexer.SPREAD {
						spread, ok := node.Left.(*ast.SpreadExpression)
						if !ok {
							t.Fatalf("repeated marker lost its valid prefix: %+v", node)
						}
						id, ok := spread.Value.(*ast.Identifier)
						if !ok || id.Value != "values" {
							t.Fatalf("repeated marker lost its source: %+v", spread)
						}
						retainedSources++
					}
				}
				return nil
			})
			if !bindings["before"] || !bindings["survivor"] || invalid == 0 {
				t.Fatalf("lost source/invalid expression: bindings=%v invalid=%d diagnostics=%v", bindings, invalid, result.Diagnostics)
			}
			if name == "repeated" && retainedSources != 2 {
				t.Fatalf("both repeated forms must retain their source: %d", retainedSources)
			}
		})
	}
}

// TestSpreadOutOfContextKeepsSourceExpression leaves context legality to Sema
// while retaining the actual source operand and later declarations in the AST.
// Rules: rules/declarations/spread.md — §2;
// rules/compiler/parser_recovery.md — §§7.3,14.4.
func TestSpreadOutOfContextKeepsSourceExpression(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/spread_out_of_context_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	outcome, finished := robustParse(string(data))
	if !finished || outcome.panicked != nil {
		t.Fatalf("parser did not terminate normally: %+v", outcome)
	}
	if outcome.result.HasErrors {
		t.Fatalf("syntactically complete spread should retain a Sema-checkable AST: %v", outcome.result.Diagnostics)
	}
	if functionNamed(outcome.result.Program, "After") == nil {
		t.Fatal("lost After")
	}
	spreads := 0
	_ = walk.Inspect(outcome.result.Program, func(node any) error {
		if spread, ok := node.(*ast.SpreadExpression); ok {
			id, ok := spread.Value.(*ast.Identifier)
			if !ok || id.Value != "values" {
				t.Fatalf("spread lost its real source: %+v", spread)
			}
			spreads++
		}
		return nil
	})
	if spreads != 2 {
		t.Fatalf("want both spread expressions, got %d", spreads)
	}
}
