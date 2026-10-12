package cst

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestHeaderSpacingRolesAreParserOwned checks exact modifier ranges and
// separates hyphenated header words from ordinary subtraction without loss.
// Rules: rules/tooling/formatter.md — §5(2–6),§16(18–19),§22(1).
func TestHeaderSpacingRolesAreParserOwned(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/header_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	register := program.Statements[2].(*ast.TypeDeclStatement).RegisterType
	if len(register.ModifierTokens) != 2 || register.ModifierTokens[0].Start.Lexeme != "msb" || register.ModifierTokens[0].End.Lexeme != "first" || register.ModifierTokens[1].End.Lexeme != "endian" {
		t.Fatalf("lost parser ranges: %+v", register.ModifierTokens)
	}
	document := Build(text, "")
	document.ApplyProgramRoles(program)
	if document.Text() != text {
		t.Fatal("role projection changed bytes")
	}
	compact, binary := 0, 0
	for _, element := range document.Elements {
		if element.Token.Type != lexer.MINUS {
			continue
		}
		if element.HasRole(HeaderCompoundHyphen) {
			compact++
		} else if element.HasRole(BinaryOperator) {
			binary++
		}
	}
	if compact != 2 || binary != 1 {
		t.Fatalf("compact=%d binary=%d", compact, binary)
	}
}

// TestInvalidHeaderModifierGetsNoCompactRole keeps rejected syntax outside
// parser-proven register-header spacing without guessing from token spelling.
// Rules: rules/tooling/formatter.md — §5(5),§25(1–4).
func TestInvalidHeaderModifierGetsNoCompactRole(t *testing.T) {
	data, err := os.ReadFile("../../testdata/cst/header_modifier_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("invalid modifier accepted")
	}
	document := Build(string(data), "")
	document.ApplyProgramRoles(program)
	for _, element := range document.Elements {
		if element.HasRole(HeaderCompoundHyphen) {
			t.Fatal("invalid syntax acquired a valid header role")
		}
	}
	if document.Text() != string(data) {
		t.Fatal("lost malformed source bytes")
	}
}

// TestInvalidModuleHeaderGetsNoSpacingRole preserves a module header rejected
// by the parser instead of claiming that an accepted first word proves it valid.
// Rules: rules/tooling/formatter.md — §5(5),§25(1–4).
func TestInvalidModuleHeaderGetsNoSpacingRole(t *testing.T) {
	data, err := os.ReadFile("../../testdata/cst/header_module_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("invalid module accepted")
	}
	document := Build(string(data), "")
	document.ApplyProgramRoles(program)
	for _, element := range document.Elements {
		if element.Token.Type == lexer.MODULE && element.HasRole(SpacedKeyword) {
			t.Fatal("invalid module acquired a valid header role")
		}
	}
	if document.Text() != string(data) {
		t.Fatal("lost malformed module bytes")
	}
}
