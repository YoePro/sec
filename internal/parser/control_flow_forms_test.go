package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Payload binding and destructuring in if and while conditions report one
// focused P2009 diagnostic per form, replace the condition, and keep every
// body and following declaration.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §13 "No pattern binding in `if`", §28 "Required diagnostics"
//   - rules/control-flow/flowcontrol_while.md — §8 "`is` state tests", §30 "Required diagnostics"
//   - rules/compiler/parser_recovery.md — "Bounded damage"
func TestConditionPatternBindingReportsFocusedDiagnostics(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/condition_pattern_binding_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(input))).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("expected recoverable parse error: %+v", result)
	}
	wants := []struct {
		line    int
		context string
	}{{7, "if"}, {13, "while"}, {19, "if"}, {25, "while"}}
	if len(result.Diagnostics) != len(wants) {
		t.Fatalf("diagnostics = %+v, want %d", result.Diagnostics, len(wants))
	}
	for i, want := range wants {
		diagnostic := result.Diagnostics[i]
		message := "pattern binding is not allowed in " + want.context + " condition; use match"
		if diagnostic.ID != "P2009" || diagnostic.Primary.Line != want.line || !strings.HasPrefix(diagnostic.Message, message) {
			t.Fatalf("diagnostic %d = %+v, want P2009 %q at line %d", i, diagnostic, message, want.line)
		}
	}
	if len(result.Program.Statements) != 6 {
		t.Fatalf("lost declarations: %d statements", len(result.Program.Statements))
	}
	for _, statement := range result.Program.Statements[1:5] {
		fn := statement.(*ast.FunctionDeclaration)
		if fn.Body == nil || len(fn.Body.Statements) != 1 {
			t.Fatalf("lost body of %s", fn.Name.Value)
		}
		switch stmt := fn.Body.Statements[0].(type) {
		case *ast.IfStatement:
			if stmt.Consequence == nil || len(stmt.Consequence.Statements) != 1 || stmt.OptionBinding != nil {
				t.Fatalf("%s if = %+v", fn.Name.Value, stmt)
			}
		case *ast.WhileStatement:
			if stmt.Body == nil || len(stmt.Body.Statements) != 1 {
				t.Fatalf("%s while = %+v", fn.Name.Value, stmt)
			}
		default:
			t.Fatalf("%s statement = %T", fn.Name.Value, stmt)
		}
	}
}

// Non-binding state tests parse into StateTestExpression for if and while,
// including qualified variants, payload variants without parentheses, and
// `is empty`, while the existing Option and availability forms are unchanged.
//
// Rules:
//   - rules/declarations/unions.md — §8.1 "Active variant test", §8.2 "Empty-state test"
//   - rules/control-flow/flowcontrol_if.md — §12 "State tests with `is`"
//   - rules/control-flow/flowcontrol_while.md — §8 "`is` state tests"
func TestConditionStateTestsParse(t *testing.T) {
	input := `module main
fn F() void {
    if state is Idle {
    }
    if state is State.Running {
    }
    while state is empty {
    }
    if result is Ok {
    }
    while value is Some {
    }
    if value is Some(inner) {
    }
    if value is None {
    }
    if place is not available {
    }
}
`
	result := New(lexer.New(input)).Parse()
	if result.HasErrors {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	body := result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements
	condition := func(index int) ast.Expression {
		switch stmt := body[index].(type) {
		case *ast.IfStatement:
			return stmt.Condition
		case *ast.WhileStatement:
			return stmt.Condition
		}
		t.Fatalf("statement %d = %T", index, body[index])
		return nil
	}
	wants := []string{"state is Idle", "state is State.Running", "state is empty", "result is Ok", "value is Some"}
	for index, want := range wants {
		test, ok := condition(index).(*ast.StateTestExpression)
		if !ok || test.String() != want {
			t.Fatalf("condition %d = %#v, want state test %q", index, condition(index), want)
		}
	}
	if stmt := body[5].(*ast.IfStatement); stmt.OptionBinding == nil || stmt.OptionBinding.Binding.Value != "inner" {
		t.Fatalf("Option binding = %+v", stmt.OptionBinding)
	}
	if _, ok := condition(6).(*ast.MatchExpression); !ok {
		t.Fatalf("is None = %T, want Option absence match", condition(6))
	}
	if availability, ok := condition(7).(*ast.AvailabilityExpression); !ok || !availability.Negated {
		t.Fatalf("is not available = %#v", condition(7))
	}
}

// Loop labels, while in value position, and value-carrying break or continue
// each report one focused diagnostic while the loop, its body, and following
// declarations are retained.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §16 "No labeled loop control in Sec 0.1", §26 "No loop expression value", §27 "Parser requirements"
//   - rules/compiler/parser_recovery.md — "Bounded damage"
func TestUnsupportedWhileFormsReportFocusedDiagnostics(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/while_unsupported_forms_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(input))).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("expected recoverable parse error: %+v", result)
	}
	wants := []struct {
		line    int
		id      string
		message string
	}{
		{7, "P2001", "labeled loops are not supported in Sec 0.1; remove label outer"},
		{14, "P2011", "while is a statement and does not produce a value"},
		{20, "P2001", "break does not carry a value in Sec 0.1"},
		{26, "P2001", "continue does not carry a value in Sec 0.1"},
	}
	if len(result.Diagnostics) != len(wants) {
		t.Fatalf("diagnostics = %+v, want %d", result.Diagnostics, len(wants))
	}
	for i, want := range wants {
		diagnostic := result.Diagnostics[i]
		if diagnostic.ID != want.id || diagnostic.Primary.Line != want.line || !strings.Contains(diagnostic.Message, want.message) {
			t.Fatalf("diagnostic %d = %+v, want %s %q at line %d", i, diagnostic, want.id, want.message, want.line)
		}
	}
	if len(result.Program.Statements) != 6 {
		t.Fatalf("lost declarations: %d statements", len(result.Program.Statements))
	}
	labeled := result.Program.Statements[1].(*ast.FunctionDeclaration)
	if loop, ok := labeled.Body.Statements[0].(*ast.WhileStatement); !ok || len(loop.Body.Statements) != 1 {
		t.Fatalf("labeled loop = %#v", labeled.Body.Statements[0])
	}
	breakLoop := result.Program.Statements[3].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.WhileStatement)
	if _, ok := breakLoop.Body.Statements[0].(*ast.BreakStatement); !ok || len(breakLoop.Body.Statements) != 1 {
		t.Fatalf("break body = %#v", breakLoop.Body.Statements)
	}
	continueLoop := result.Program.Statements[4].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.WhileStatement)
	if _, ok := continueLoop.Body.Statements[0].(*ast.ContinueStatement); !ok || len(continueLoop.Body.Statements) != 1 {
		t.Fatalf("continue body = %#v", continueLoop.Body.Statements)
	}
}

// Loop bindings retain their written ref or ref mut mode, while consuming for
// forms and reference modes on a discard report one focused diagnostic each
// and keep the loop body.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §4 "Loop bindings", §9 "Discard bindings", §36 "No consuming `for` in Sec 0.1"
//   - rules/compiler/parser_recovery.md — "Bounded damage"
func TestForBindingModesAndConsumingForms(t *testing.T) {
	valid := "module main\nfn F(items: int[3]) void {\n    for ref item in items {\n    }\n    for index, ref mut item in items {\n    }\n}\n"
	result := New(lexer.New(valid)).Parse()
	if result.HasErrors {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	body := result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements
	shared := body[0].(*ast.ForStatement).Bindings
	if len(shared) != 1 || shared[0].Mode != ast.ForBindingRef || shared[0].Name != "item" || shared[0].ModeToken.Type != lexer.REF {
		t.Fatalf("shared binding = %+v", shared)
	}
	mutable := body[1].(*ast.ForStatement).Bindings
	if len(mutable) != 2 || mutable[0].Mode != ast.ForBindingValue || mutable[1].Mode != ast.ForBindingRefMut || mutable[1].Name != "item" {
		t.Fatalf("mutable bindings = %+v", mutable)
	}

	input, err := os.ReadFile("../../testdata/parser/for_binding_modes_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result = New(lexer.New(string(input))).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("expected recoverable parse error: %+v", result)
	}
	wants := []struct {
		line    int
		message string
	}{
		{6, "consuming for iteration is not part of Sec 0.1"},
		{12, "consuming for iteration is not part of Sec 0.1"},
		{18, "consuming for iteration is not part of Sec 0.1"},
		{24, "a discard loop binding takes no ref mode"},
	}
	if len(result.Diagnostics) != len(wants) {
		t.Fatalf("diagnostics = %+v, want %d", result.Diagnostics, len(wants))
	}
	for i, want := range wants {
		diagnostic := result.Diagnostics[i]
		if diagnostic.Primary.Line != want.line || !strings.Contains(diagnostic.Message, want.message) {
			t.Fatalf("diagnostic %d = %+v, want %q at line %d", i, diagnostic, want.message, want.line)
		}
	}
	if len(result.Program.Statements) != 6 {
		t.Fatalf("lost declarations: %d statements", len(result.Program.Statements))
	}
	for _, statement := range result.Program.Statements[1:5] {
		fn := statement.(*ast.FunctionDeclaration)
		loop, ok := fn.Body.Statements[0].(*ast.ForStatement)
		if !ok || loop.Body == nil || len(loop.Body.Statements) != 1 || loop.Iterable == nil {
			t.Fatalf("%s loop = %#v", fn.Name.Value, fn.Body.Statements[0])
		}
	}
}
