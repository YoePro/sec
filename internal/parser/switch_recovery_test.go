package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestUnterminatedSwitchRetainsClauses(t *testing.T) {
	for _, name := range []string{"unterminated_switch", "unterminated_empty_switch"} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile("../../testdata/parser/" + name + "_invalid.sec")
			if err != nil {
				t.Fatal(err)
			}
			result := New(lexer.New(string(input))).Parse()
			if !result.HasErrors || result.Fatal {
				t.Fatalf("expected recoverable parse error: %+v", result)
			}
			found := false
			for _, d := range result.Diagnostics {
				if strings.Contains(d.Message, "unterminated switch body") && d.Primary.Type == lexer.EOF {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing switch diagnostic: %+v", result.Diagnostics)
			}
			if len(result.Program.Statements) != 2 {
				t.Fatalf("lost enclosing function: %+v", result.Program)
			}
			fn, ok := result.Program.Statements[1].(*ast.FunctionDeclaration)
			if !ok || fn.Body == nil || len(fn.Body.Statements) != 1 {
				t.Fatalf("lost function body: %#v", result.Program.Statements[1])
			}
			sw, ok := fn.Body.Statements[0].(*ast.SwitchStatement)
			if !ok {
				t.Fatalf("lost switch: %T", fn.Body.Statements[0])
			}
			if name == "unterminated_empty_switch" {
				if sw.Subject != nil || len(sw.Cases) != 0 || sw.Default != nil {
					t.Fatalf("fabricated switch contents: %+v", sw)
				}
				return
			}
			if len(sw.Cases) != 2 || sw.Default == nil {
				t.Fatalf("lost clauses: %+v", sw)
			}
			for i, clause := range append(sw.Cases, sw.Default) {
				if clause.Body == nil || len(clause.Body.Statements) != 1 {
					t.Fatalf("lost clause body: %+v", clause)
				}
				ret, ok := clause.Body.Statements[0].(*ast.ReturnStatement)
				if !ok {
					t.Fatalf("lost return: %T", clause.Body.Statements[0])
				}
				value, ok := ret.Value.(*ast.IntegerLiteral)
				if !ok || value.Value != int64((i+1)*10) {
					t.Fatalf("incorrect clause order/value: %#v", ret.Value)
				}
			}
		})
	}
}

// switch in value position reports the focused statement-only diagnostic once,
// skips the whole switch region, and preserves the enclosing and following
// declarations.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §30 "Switch is not an expression"
//   - rules/compiler/parser_recovery.md — "Parser diagnostic IDs", "Bounded damage"
func TestSwitchInExpressionPositionReportsStatementOnlyDiagnostic(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/switch_expression_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(input))).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("expected recoverable parse error: %+v", result)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want exactly one", result.Diagnostics)
	}
	diagnostic := result.Diagnostics[0]
	if diagnostic.ID != "P2011" || !strings.Contains(diagnostic.Message, "switch is a statement and does not produce a value") || diagnostic.Primary.Type != lexer.SWITCH {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
	if len(result.Program.Statements) != 3 {
		t.Fatalf("lost declarations: %d statements", len(result.Program.Statements))
	}
	fn, ok := result.Program.Statements[1].(*ast.FunctionDeclaration)
	if !ok || fn.Body == nil || len(fn.Body.Statements) != 2 {
		t.Fatalf("lost enclosing function body: %#v", result.Program.Statements[1])
	}
	let, ok := fn.Body.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("lost let statement: %T", fn.Body.Statements[0])
	}
	invalid, ok := let.Value.(*ast.InvalidExpression)
	if !ok || invalid.Recovery == nil || invalid.Recovery.End.Type != lexer.RBRACE {
		t.Fatalf("let value = %#v, want invalid expression spanning the switch", let.Value)
	}
	if _, ok := fn.Body.Statements[1].(*ast.ReturnStatement); !ok {
		t.Fatalf("lost following return: %T", fn.Body.Statements[1])
	}
}

// A where guard on a switch case reports the focused guard diagnostic while
// the clause body and later default clause are retained.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §29 "No switch case guards"
//   - rules/compiler/parser_recovery.md — "Parser diagnostic IDs", "Bounded damage"
func TestSwitchCaseGuardReportsFocusedDiagnosticAndRetainsClauses(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/switch_case_guard_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(input))).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("expected recoverable parse error: %+v", result)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want exactly one", result.Diagnostics)
	}
	diagnostic := result.Diagnostics[0]
	if diagnostic.ID != "P2011" || !strings.Contains(diagnostic.Message, "switch case guards are not part of Sec 0.1") || diagnostic.Primary.Type != lexer.WHERE {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
	if len(result.Program.Statements) != 3 {
		t.Fatalf("lost declarations: %d statements", len(result.Program.Statements))
	}
	fn := result.Program.Statements[1].(*ast.FunctionDeclaration)
	sw, ok := fn.Body.Statements[0].(*ast.SwitchStatement)
	if !ok {
		t.Fatalf("lost switch: %T", fn.Body.Statements[0])
	}
	if len(sw.Cases) != 1 || len(sw.Cases[0].Items) != 1 || sw.Cases[0].Body == nil || len(sw.Cases[0].Body.Statements) != 1 {
		t.Fatalf("lost guarded clause: %+v", sw.Cases)
	}
	if sw.Default == nil || sw.Default.Body == nil || len(sw.Default.Body.Statements) != 1 {
		t.Fatalf("lost default clause: %+v", sw.Default)
	}
}

// A bare `_` case item is retained as an identifier value item so Sema can
// report the switch-specific wildcard diagnostic.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §28 "No pattern matching", §35 "Parser requirements"
func TestSwitchWildcardItemIsRetainedForSema(t *testing.T) {
	input := "module main\nfn F(v: int) void {\n    switch v {\n    case 1, _:\n        return\n    }\n}\n"
	result := New(lexer.New(input)).Parse()
	if result.HasErrors {
		t.Fatalf("unexpected parse errors: %+v", result.Diagnostics)
	}
	fn := result.Program.Statements[1].(*ast.FunctionDeclaration)
	sw := fn.Body.Statements[0].(*ast.SwitchStatement)
	if len(sw.Cases) != 1 || len(sw.Cases[0].Items) != 2 {
		t.Fatalf("case items = %+v", sw.Cases)
	}
	item, ok := sw.Cases[0].Items[1].(*ast.SwitchValueCase)
	if !ok {
		t.Fatalf("wildcard item = %T", sw.Cases[0].Items[1])
	}
	if identifier, ok := item.Value.(*ast.Identifier); !ok || identifier.Value != "_" {
		t.Fatalf("wildcard value = %#v", item.Value)
	}
}
