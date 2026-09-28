package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// A call reaching EOF without its closing parenthesis retains every completed
// argument and the enclosing return/function while recording a virtual closer.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Argument-list recovery"
//   - rules/compiler/parser_recovery.md — "Recovery goals"
func TestUnterminatedCallRetainsArgumentsAndEnclosingFunction(t *testing.T) {
	const fixture = "../../testdata/parser/unterminated_call_invalid.sec"
	input, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(input), fixture)).Parse()
	if !result.HasErrors {
		t.Fatal("unterminated call must remain a parser error")
	}
	if len(result.Diagnostics) != 2 || result.Diagnostics[0].ID != diagnostics.ParserMissingToken {
		t.Fatalf("diagnostics = %+v, want missing call closer plus unterminated function", result.Diagnostics)
	}

	var function *ast.FunctionDeclaration
	for _, statement := range result.Program.Statements {
		candidate, ok := statement.(*ast.FunctionDeclaration)
		if ok && candidate.Name != nil && candidate.Name.Value == "Value" {
			function = candidate
			break
		}
	}
	if function == nil || function.Body == nil || len(function.Body.Statements) != 1 {
		t.Fatalf("lost enclosing Value function or return body: %#v", function)
	}
	returned, ok := function.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("Value body statement = %T, want return", function.Body.Statements[0])
	}
	call, ok := returned.Value.(*ast.CallExpression)
	if !ok || len(call.Arguments) != 2 {
		t.Fatalf("return value = %#v, want retained two-argument call", returned.Value)
	}
	for index, want := range []string{"1", "2"} {
		integer, ok := call.Arguments[index].(*ast.IntegerLiteral)
		if !ok || integer.Token.Lexeme != want {
			t.Fatalf("argument %d = %#v, want integer %s", index, call.Arguments[index], want)
		}
	}

	foundCloser := false
	for _, event := range result.Recovery {
		if event.Kind == RecoveryInsertMissingToken && len(event.Expected) == 1 &&
			event.Expected[0] == lexer.RPAREN && event.Start.Type == lexer.EOF {
			foundCloser = true
			break
		}
	}
	if !foundCloser {
		t.Fatalf("missing virtual call closer event: %+v", result.Recovery)
	}
}

// EOF immediately after the opener or a separator retains the call with the
// argument positions that were actually present in source.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Argument-list recovery"
//   - rules/compiler/parser_recovery.md — "Progress"
func TestUnterminatedCallEOFAfterOpenOrCommaRetainsCall(t *testing.T) {
	for name, test := range map[string]struct {
		tail      string
		arguments int
	}{
		"after opener": {tail: "Sum(", arguments: 0},
		"after comma":  {tail: "Sum(1,", arguments: 1},
	} {
		t.Run(name, func(t *testing.T) {
			source := "module main\nfn Sum(value: int) int { return value }\nfn Value() int { return " + test.tail
			result := New(lexer.New(source)).Parse()
			if !result.HasErrors {
				t.Fatal("unterminated call must remain a parser error")
			}
			function, ok := result.Program.Statements[len(result.Program.Statements)-1].(*ast.FunctionDeclaration)
			if !ok || function.Body == nil || len(function.Body.Statements) != 1 {
				t.Fatalf("lost Value function: %#v", result.Program.Statements)
			}
			returned, ok := function.Body.Statements[0].(*ast.ReturnStatement)
			if !ok {
				t.Fatalf("Value body statement = %T, want return", function.Body.Statements[0])
			}
			call, ok := returned.Value.(*ast.CallExpression)
			if !ok || len(call.Arguments) != test.arguments {
				t.Fatalf("return value = %#v, want call with %d arguments", returned.Value, test.arguments)
			}
		})
	}
}
