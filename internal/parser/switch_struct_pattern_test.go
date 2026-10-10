package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestSwitchStructPatternRecovery verifies bounded recovery for prohibited
// struct-shaped patterns, keeping comma alternatives and later clauses intact.
// Rules: rules/control-flow/flowcontrol_switch.md §§28, 35, 40;
// rules/compiler/parser_recovery.md — "Bounded damage", "Parser diagnostic IDs".
func TestSwitchStructPatternRecovery(t *testing.T) {
	files, err := filepath.Glob("../../testdata/parser/switch_patterns/*_invalid.sec")
	if err != nil || len(files) != 7 {
		t.Fatalf("fixtures: %v, %v", files, err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			input, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			result := New(lexer.New(string(input))).Parse()
			if !result.HasErrors || result.Fatal || len(result.Diagnostics) != 1 {
				t.Fatalf("expected one recoverable diagnostic, got %+v", result.Diagnostics)
			}
			d := result.Diagnostics[0]
			if d.ID != compilerdiagnostics.ParserInvalidPattern || (d.Primary.Lexeme != "Move" && d.Primary.Lexeme != "Event") {
				t.Fatalf("wrong diagnostic source/identity: %+v", d)
			}
			if d.Message != "switch does not support pattern binding; use match" || !strings.Contains(d.Help, "Use match") {
				t.Fatalf("missing pattern advice: %+v", d)
			}
			if len(result.Program.Statements) != 3 {
				t.Fatalf("lost following declaration: %d statements", len(result.Program.Statements))
			}
			fn := result.Program.Statements[1].(*ast.FunctionDeclaration)
			sw := fn.Body.Statements[0].(*ast.SwitchStatement)
			if len(sw.Cases) != 2 || len(sw.Cases[0].Items) != 3 || sw.Default == nil {
				t.Fatalf("lost case alternatives or clauses: %+v", sw)
			}
			invalid, ok := sw.Cases[0].Items[1].(*ast.SwitchValueCase).Value.(*ast.InvalidExpression)
			if !ok || invalid.Recovery == nil || invalid.Recovery.End.Type != lexer.RBRACE || invalid.Recovery.Skipped == 0 {
				t.Fatalf("missing invalid pattern recovery: %#v", invalid)
			}
			for i, clause := range append(sw.Cases, sw.Default) {
				if clause.Body == nil || len(clause.Body.Statements) != 1 {
					t.Fatalf("lost clause body: %+v", clause)
				}
				ret := clause.Body.Statements[0].(*ast.ReturnStatement)
				if ret.Value.(*ast.IntegerLiteral).Value != int64((i+1)*10) {
					t.Fatalf("changed clause body: %+v", ret)
				}
			}
		})
	}
}

// TestSwitchStructValuesRemainExpressions protects explicit fields, spreads,
// nested values and empty literals from being classified as patterns.
// Rules: rules/control-flow/flowcontrol_switch.md §7;
// rules/foundations/grammar.md — "StructLiteralItem".
func TestSwitchStructValuesRemainExpressions(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/switch_patterns/struct_values.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(input))).Parse()
	if result.HasErrors {
		t.Fatalf("value cases rejected: %+v", result.Diagnostics)
	}
	fn := result.Program.Statements[1].(*ast.FunctionDeclaration)
	sw := fn.Body.Statements[0].(*ast.SwitchStatement)
	if len(sw.Cases) != 4 || len(sw.Cases[0].Items) != 2 {
		t.Fatalf("lost value alternatives: %+v", sw)
	}
	for _, clause := range sw.Cases {
		for _, item := range clause.Items {
			if _, ok := item.(*ast.SwitchValueCase).Value.(*ast.StructLiteral); !ok {
				t.Fatalf("value is not a struct literal: %+v", item)
			}
		}
	}
}
