package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestInterpolationPartsAndSourcePositions(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/interpolation_parts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(input), "parts.sec")).Parse()
	if result.HasErrors {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	fn := result.Program.Statements[1].(*ast.FunctionDeclaration)
	literal := fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.InterpolatedStringLiteral)
	if len(literal.Parts) != 5 {
		t.Fatalf("parts: %+v", literal.Parts)
	}
	if literal.Parts[0].Text != "α {ok} " || literal.Parts[2].Text != " :: " || literal.Parts[4].Text != ` end\n` {
		t.Fatalf("incorrect text parts: %+v", literal.Parts)
	}
	line := strings.Split(string(input), "\n")[3]
	for _, part := range literal.Parts {
		if part.Token.File != "parts.sec" || part.Token.Line != 4 || part.End.Line != 4 || part.End.Column <= part.Token.Column {
			t.Fatalf("incorrect part range: %+v", part)
		}
		span := string([]rune(line)[part.Token.Column-1 : part.End.Column-1])
		if part.Expression == nil && span != part.Token.Lexeme {
			t.Fatalf("lost text source: %+v", part)
		}
		if part.Expression != nil && (!strings.HasPrefix(span, "{") || !strings.HasSuffix(span, "}")) {
			t.Fatalf("incorrect expression range: %q", span)
		}
	}
	sum, ok := literal.Parts[1].Expression.(*ast.InfixExpression)
	if !ok || sum.Operator != "+" {
		t.Fatalf("expression not parsed: %#v", literal.Parts[1].Expression)
	}
	left := sum.Left.(*ast.IntegerLiteral)
	if left.Token.Column != 1+len([]rune(line[:strings.Index(line, "1 +")])) {
		t.Fatalf("wrong operand position: %+v", left.Token)
	}
	nested := literal.Parts[3].Expression.(*ast.InterpolatedStringLiteral)
	name := nested.Parts[1].Expression.(*ast.Identifier)
	if name.Value != "value" || name.Token.Column != 1+len([]rune(line[:strings.Index(line, "value")])) {
		t.Fatalf("wrong nested expression: %+v", name)
	}
	if literal.Value != literal.Token.Lexeme || literal.String() != literal.Token.Lexeme {
		t.Fatal("original spelling lost")
	}
}

func TestInterpolationExpressionRecovery(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/interpolation_parts_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(input), "parts.sec")).Parse()
	if !result.HasErrors || result.Fatal || len(result.Diagnostics) != 3 {
		t.Fatalf("diagnostics: %+v", result)
	}
	fn := result.Program.Statements[1].(*ast.FunctionDeclaration)
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("lost following declaration: %+v", fn.Body)
	}
	literal := fn.Body.Statements[0].(*ast.LetStatement).Value.(*ast.InterpolatedStringLiteral)
	invalid, valid := 0, 0
	for _, part := range literal.Parts {
		switch expression := part.Expression.(type) {
		case *ast.InvalidExpression:
			invalid++
			if expression.Recovery == nil {
				t.Fatal("invalid expression lacks recovery metadata")
			}
		case *ast.IntegerLiteral:
			valid++
			if expression.Value != 42 {
				t.Fatalf("wrong surviving expression: %+v", expression)
			}
		}
	}
	if invalid != 3 || valid != 1 {
		t.Fatalf("invalid=%d valid=%d", invalid, valid)
	}
	for _, d := range result.Diagnostics {
		if d.Primary.File != "parts.sec" || d.Primary.Line != 4 || d.Primary.Column < 20 {
			t.Fatalf("fragment-relative diagnostic: %+v", d)
		}
	}
}
