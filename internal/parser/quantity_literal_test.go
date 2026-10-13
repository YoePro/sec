package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// A numeric literal immediately followed by a complete unit annotation on the
// same line is one quantity literal; a separated or incomplete `<` stays an
// ordinary comparison, and a leading sign is a prefix expression.
//
// Rules:
//   - rules/foundations/grammar.md — "Quantity literals"
func TestQuantityLiteralParsing(t *testing.T) {
	for source, want := range map[string]string{
		"5<s>":           "5<s>",
		"0.5<ns>":        "0.5<ns>",
		"100<m/s^2>":     "100<m/s^2>",
		"-5<s>":          "(-5<s>)",
		"5<s> + 250<ms>": "(5<s> + 250<ms>)",
		"5 < s":          "(5 < s)",
		"5<count":        "(5 < count)",
		"5<6":            "(5 < 6)",
	} {
		p := New(lexer.NewWithFile("module main\n\nfn F() void {\n    let x := "+source+"\n}\n", "q.sec"))
		program := p.Parse().Program
		if len(p.Errors()) != 0 {
			t.Errorf("%s: parse errors %v", source, p.Errors())
			continue
		}
		fn := program.Statements[len(program.Statements)-1].(*ast.FunctionDeclaration)
		value := fn.Body.Statements[0].(*ast.LetStatement).Value
		if value.String() != want {
			t.Errorf("%s: parsed as %s, want %s", source, value.String(), want)
		}
	}
}
