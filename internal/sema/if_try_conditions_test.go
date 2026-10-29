package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestIfTryConditionsRequireBooleanSuccess verifies propagation, local recovery
// and negation against compiler-owned condition types, never payload truthiness.
// Rules: rules/control-flow/flowcontrol_if.md — §§11,27–28;
// rules/errors/errorhandling.md — try in conditions.
func TestIfTryConditionsRequireBooleanSuccess(t *testing.T) {
	for _, name := range []string{"valid", "non_bool_invalid"} {
		source, err := os.ReadFile("../../testdata/control_flow/if_try/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(source), name+".sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if name == "valid" {
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			count := 0
			_ = walk.Inspect(program, func(node any) error {
				condition, ok := node.(*ast.IfStatement)
				if !ok {
					return nil
				}
				if _, ok := condition.Condition.(*ast.TryExpression); !ok {
					return nil
				}
				typ, found := a.ResolvedTypeOf(condition.Condition)
				if !found || typ.Kind != BoolType {
					t.Fatalf("if try condition type: %+v", typ)
				}
				count++
				return nil
			})
			if count != 3 {
				t.Fatalf("want 3 direct if try conditions, got %d", count)
			}
		} else {
			if len(errors) != 4 {
				t.Fatalf("want four non-bool rejections: %v", errors)
			}
			for i, want := range []string{"int", "string", "Configuration", "int"} {
				diagnostic := errors[i]
				prefix := "try expression in if condition must produce bool, got "
				if i == 3 {
					prefix = "if condition must be bool, got "
				}
				if !strings.Contains(diagnostic.Message, prefix+want) || diagnostic.Line != 9+i {
					t.Fatalf("wrong non-bool diagnostic: %+v", diagnostic)
				}
			}
		}
	}
}
