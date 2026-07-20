package lexer

import "testing"

func TestLanguageSample(t *testing.T) {
	input := `
module internal.storage

import (
	"fmt"
	db "modules/database"
)

type Percent int range 0..100

type Reader interface {
	fn read(buffer: []byte) Result[int, IOError]
}

impl Reader for FileReader {
	fn read(buffer: []byte) Result[int, IOError] {
		value := try load() {
			Err(error) => return Err(error)
		}

		return Ok(value)
	}
}
`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{MODULE, "module"},
		{IDENT, "internal"},
		{DOT, "."},
		{IDENT, "storage"},

		{IMPORT, "import"},
		{LPAREN, "("},
		{STRING, `"fmt"`},
		{IDENT, "db"},
		{STRING, `"modules/database"`},
		{RPAREN, ")"},

		{TYPE, "type"},
		{IDENT, "Percent"},
		{IDENT, "int"},
		{RANGE_KW, "range"},
		{INT, "0"},
		{RANGE, ".."},
		{INT, "100"},

		{TYPE, "type"},
		{IDENT, "Reader"},
		{INTERFACE, "interface"},
		{LBRACE, "{"},
		{FN, "fn"},
		{IDENT, "read"},
		{LPAREN, "("},
		{IDENT, "buffer"},
		{COLON, ":"},
		{LBRACKET, "["},
		{RBRACKET, "]"},
		{IDENT, "byte"},
		{RPAREN, ")"},
		{IDENT, "Result"},
		{LBRACKET, "["},
		{IDENT, "int"},
		{COMMA, ","},
		{IDENT, "IOError"},
		{RBRACKET, "]"},
		{RBRACE, "}"},

		{IMPL, "impl"},
		{IDENT, "Reader"},
		{FOR, "for"},
		{IDENT, "FileReader"},
		{LBRACE, "{"},
		{FN, "fn"},
		{IDENT, "read"},

		{LPAREN, "("},
		{IDENT, "buffer"},
		{COLON, ":"},
		{LBRACKET, "["},
		{RBRACKET, "]"},
		{IDENT, "byte"},
		{RPAREN, ")"},
		{IDENT, "Result"},
		{LBRACKET, "["},
		{IDENT, "int"},
		{COMMA, ","},
		{IDENT, "IOError"},
		{RBRACKET, "]"},

		{LBRACE, "{"},
		{IDENT, "value"},
		{DECLARE, ":="},
		{TRY, "try"},
		{IDENT, "load"},
		{LPAREN, "("},
		{RPAREN, ")"},
		{LBRACE, "{"},
		{IDENT, "Err"},
		{LPAREN, "("},
		{IDENT, "error"},
		{RPAREN, ")"},
		{ARROW, "=>"},
		{RETURN, "return"},
		{IDENT, "Err"},
		{LPAREN, "("},
		{IDENT, "error"},
		{RPAREN, ")"},
		{RBRACE, "}"},
		{RETURN, "return"},
		{IDENT, "Ok"},
		{LPAREN, "("},
		{IDENT, "value"},
		{RPAREN, ")"},
		{RBRACE, "}"},
		{RBRACE, "}"},
		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestOperators(t *testing.T) {
	input := `= := => + - * / % += -= *= /= %= == != < <= > >= && || ! & | ^ ~ << >> &= |= ^= <<= >>= . .. ..< ... , : ; ? @ # () {} []`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{ASSIGN, "="},
		{DECLARE, ":="},
		{ARROW, "=>"},
		{PLUS, "+"},
		{MINUS, "-"},
		{ASTERISK, "*"},
		{SLASH, "/"},
		{PERCENT, "%"},
		{PLUS_ASSIGN, "+="},
		{MINUS_ASSIGN, "-="},
		{ASTERISK_ASSIGN, "*="},
		{SLASH_ASSIGN, "/="},
		{PERCENT_ASSIGN, "%="},
		{EQ, "=="},
		{NEQ, "!="},
		{LT, "<"},
		{LTE, "<="},
		{GT, ">"},
		{GTE, ">="},
		{AND, "&&"},
		{OR, "||"},
		{NOT, "!"},
		{BIT_AND, "&"},
		{BIT_OR, "|"},
		{BIT_XOR, "^"},
		{BIT_NOT, "~"},
		{SHIFT_LEFT, "<<"},
		{SHIFT_RIGHT, ">>"},
		{BIT_AND_ASSIGN, "&="},
		{BIT_OR_ASSIGN, "|="},
		{BIT_XOR_ASSIGN, "^="},
		{SHIFT_LEFT_ASSIGN, "<<="},
		{SHIFT_RIGHT_ASSIGN, ">>="},
		{DOT, "."},
		{RANGE, ".."},
		{RANGE_EXCLUSIVE, "..<"},
		{SPREAD, "..."},
		{COMMA, ","},
		{COLON, ":"},
		{SEMICOLON, ";"},
		{QUESTION, "?"},
		{AT, "@"},
		{HASH, "#"},
		{LPAREN, "("},
		{RPAREN, ")"},
		{LBRACE, "{"},
		{RBRACE, "}"},
		{LBRACKET, "["},
		{RBRACKET, "]"},
		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestKeywords(t *testing.T) {
	input := `module import require sec self extern fn let mut type unit struct interface impl for while in if else switch case default fallthrough break continue match where return true false try defer discard ref unsafe asm property get set enum union spawn await nil None Some`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{MODULE, "module"},
		{IMPORT, "import"},
		{REQUIRE, "require"},
		{SEC, "sec"},
		{SELF, "self"},
		{EXTERN, "extern"},
		{FN, "fn"},
		{LET, "let"},
		{MUT, "mut"},
		{TYPE, "type"},
		{UNIT, "unit"},
		{STRUCT, "struct"},
		{INTERFACE, "interface"},
		{IMPL, "impl"},
		{FOR, "for"},
		{WHILE, "while"},
		{IN, "in"},
		{IF, "if"},
		{ELSE, "else"},
		{SWITCH, "switch"},
		{CASE, "case"},
		{DEFAULT, "default"},
		{FALLTHROUGH, "fallthrough"},
		{BREAK, "break"},
		{CONTINUE, "continue"},
		{MATCH, "match"},
		{WHERE, "where"},
		{RETURN, "return"},
		{TRUE, "true"},
		{FALSE, "false"},
		{TRY, "try"},
		{DEFER, "defer"},
		{DISCARD, "discard"},
		{REF, "ref"},
		{UNSAFE, "unsafe"},
		{ASM, "asm"},
		{PROPERTY, "property"},
		{GET, "get"},
		{SET, "set"},
		{ENUM, "enum"},
		{UNION, "union"},
		{SPAWN, "spawn"},
		{AWAIT, "await"},

		// Sec has no nil keyword. None/Some are union variants, not keywords.
		{IDENT, "nil"},
		{IDENT, "None"},
		{IDENT, "Some"},

		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestNumbersAndRanges(t *testing.T) {
	input := `123 45.67 .1 1..10 1..<10 1.. ..10 10i 10u 10f 10d 1.5f 1.5d .5f .5d 0b1000 0o10 0x8 0x8u`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{INT, "123"},
		{FLOAT, "45.67"},
		{FLOAT, ".1"},
		{INT, "1"},
		{RANGE, ".."},
		{INT, "10"},
		{INT, "1"},
		{RANGE_EXCLUSIVE, "..<"},
		{INT, "10"},
		{INT, "1"},
		{RANGE, ".."},
		{RANGE, ".."},
		{INT, "10"},
		{INT, "10i"},
		{INT, "10u"},
		{FLOAT, "10f"},
		{FLOAT, "10d"},
		{FLOAT, "1.5f"},
		{FLOAT, "1.5d"},
		{FLOAT, ".5f"},
		{FLOAT, ".5d"},
		{INT, "0b1000"},
		{INT, "0o10"},
		{INT, "0x8"},
		{INT, "0x8u"},
		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestStrings(t *testing.T) {
	input := "`json:\"id\"` \"hello\\nworld\" $\"Hello {name}\""

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{RAW_STRING, "`json:\"id\"`"},
		{STRING, `"hello\nworld"`},
		{INTERPSTRING, `$"Hello {name}"`},
		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestCommentsAreTokenized(t *testing.T) {
	input := `
let a := 1 // line comment
/*
	outer
	/*
		inner
	*/
	still outer
*/
let b := 2
`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{LET, "let"},
		{IDENT, "a"},
		{DECLARE, ":="},
		{INT, "1"},
		{COMMENT, "// line comment"},
		{COMMENT, "/*\n\touter\n\t/*\n\t\tinner\n\t*/\n\tstill outer\n*/"},
		{LET, "let"},
		{IDENT, "b"},
		{DECLARE, ":="},
		{INT, "2"},
		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestPosition(t *testing.T) {
	input := "module main\nlet x := 1"

	l := New(input)

	tok := l.NextToken()
	if tok.Type != MODULE || tok.Line != 1 || tok.Column != 1 {
		t.Fatalf("wrong position for module: %+v", tok)
	}

	tok = l.NextToken()
	if tok.Type != IDENT || tok.Line != 1 || tok.Column != 8 {
		t.Fatalf("wrong position for main: %+v", tok)
	}

	tok = l.NextToken()
	if tok.Type != LET || tok.Line != 2 || tok.Column != 1 {
		t.Fatalf("wrong position for let: %+v", tok)
	}
}

func TestIllegalUnterminatedString(t *testing.T) {
	input := `"unterminated`

	tok := New(input).NextToken()

	if tok.Type != ILLEGAL {
		t.Fatalf("wrong type. got=%q want=%q", tok.Type, ILLEGAL)
	}
}

func assertTokens(t *testing.T, input string, tests []struct {
	typ    TokenType
	lexeme string
}) {
	t.Helper()

	l := New(input)

	for i, expected := range tests {
		tok := l.NextToken()

		if tok.Type != expected.typ {
			t.Fatalf("test %d: wrong type. got=%q want=%q lexeme=%q", i, tok.Type, expected.typ, tok.Lexeme)
		}

		if tok.Lexeme != expected.lexeme {
			t.Fatalf("test %d: wrong lexeme. got=%q want=%q", i, tok.Lexeme, expected.lexeme)
		}
	}
}
