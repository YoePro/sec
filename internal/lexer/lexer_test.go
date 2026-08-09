package lexer

import "testing"

func TestBareUnderscoreIsDistinctFromIdentifiers(t *testing.T) {
	l := New("_ _name __name")
	want := []Token{
		{Type: UNDERSCORE, Lexeme: "_"},
		{Type: IDENT, Lexeme: "_name"},
		{Type: IDENT, Lexeme: "__name"},
		{Type: EOF},
	}

	for i, expected := range want {
		got := l.NextToken()
		if got.Type != expected.Type || got.Lexeme != expected.Lexeme {
			t.Fatalf("token %d = (%s, %q), want (%s, %q)", i, got.Type, got.Lexeme, expected.Type, expected.Lexeme)
		}
	}
}

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
	input := `= := :<- <- => + - * / % += -= *= /= %= == != < <= > >= && || ! & | ^ ~ << >> &= |= ^= <<= >>= . .. ..< ... , : ; ? @ # () {} []`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{ASSIGN, "="},
		{DECLARE, ":="},
		{MOVE_DECLARE, ":<-"},
		{MOVE_ASSIGN, "<-"},
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

func TestCharLiteral(t *testing.T) {
	input := `'S' '\n' 'AB'`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{CHAR, "'S'"},
		{CHAR, "'\\n'"},
		{CHAR, "'AB'"},
		{EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.typ {
			t.Fatalf("test %d: wrong token type. got=%q want=%q", i, tok.Type, tt.typ)
		}
		if tok.Lexeme != tt.lexeme {
			t.Fatalf("test %d: wrong lexeme. got=%q want=%q", i, tok.Lexeme, tt.lexeme)
		}
	}
}

func TestCharAndRuneNumericSuffixes(t *testing.T) {
	input := `65t 0r 0b1000001t 0o101r 0x41r 0x41t`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{INT, "65t"},
		{INT, "0r"},
		{INT, "0b1000001t"},
		{INT, "0o101r"},
		{INT, "0x41r"},
		{INT, "0x41t"},
		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestUnicodeIdentifiers(t *testing.T) {
	input := `unit Ω decimal physical
let Σ := Ω(1)
let μs := 1`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{UNIT, "unit"},
		{IDENT, "Ω"},
		{IDENT, "decimal"},
		{IDENT, "physical"},
		{LET, "let"},
		{IDENT, "Σ"},
		{DECLARE, ":="},
		{IDENT, "Ω"},
		{LPAREN, "("},
		{INT, "1"},
		{RPAREN, ")"},
		{LET, "let"},
		{IDENT, "μs"},
		{DECLARE, ":="},
		{INT, "1"},
		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestKeywords(t *testing.T) {
	input := `module import require sec self extern extends fn free let mut type unit struct interface impl implements for while in if else switch case default fallthrough break cancel continue match where return true false try defer discard ref unsafe asm arena after property get select set enum union spawn static await nil None Some`

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
		{EXTENDS, "extends"},
		{FN, "fn"},
		{FREE, "free"},
		{LET, "let"},
		{MUT, "mut"},
		{TYPE, "type"},
		{UNIT, "unit"},
		{STRUCT, "struct"},
		{INTERFACE, "interface"},
		{IMPL, "impl"},
		{IMPLEMENTS, "implements"},
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
		{CANCEL, "cancel"},
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
		{IDENT, "arena"},
		{AFTER, "after"},
		{PROPERTY, "property"},
		{GET, "get"},
		{SELECT, "select"},
		{SET, "set"},
		{ENUM, "enum"},
		{UNION, "union"},
		{SPAWN, "spawn"},
		{STATIC, "static"},
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
	input := `123 45.67 .1 1..10 1..<10 1.. ..10 10i 10u 10g 10m 1.5g 1.5m .5g .5m 0b1000 0o10 0x8 0x8u 0x10g 0x10m`

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
		{FLOAT, "10g"},
		{FLOAT, "10m"},
		{FLOAT, "1.5g"},
		{FLOAT, "1.5m"},
		{FLOAT, ".5g"},
		{FLOAT, ".5m"},
		{INT, "0b1000"},
		{INT, "0o10"},
		{INT, "0x8"},
		{INT, "0x8u"},
		{FLOAT, "0x10g"},
		{FLOAT, "0x10m"},
		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestNumericDigitSeparators(t *testing.T) {
	input := `1_000_000 0b1111_0000 0o755_000 0xFFFF_FFFF 1_234.567_890 1.25e1_000 42_000u`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{INT, "1_000_000"},
		{INT, "0b1111_0000"},
		{INT, "0o755_000"},
		{INT, "0xFFFF_FFFF"},
		{FLOAT, "1_234.567_890"},
		{FLOAT, "1.25e1_000"},
		{INT, "42_000u"},
		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestInvalidNumericDigitSeparatorsAreIllegalTokens(t *testing.T) {
	for _, input := range []string{
		"100_",
		"1__000",
		"0x_FF",
		"0xFF_",
		"1_.5",
		"1._5",
		"1e_3",
		"1e3_",
	} {
		t.Run(input, func(t *testing.T) {
			l := New(input)
			tok := l.NextToken()
			if tok.Type != ILLEGAL || tok.Lexeme != input {
				t.Fatalf("wrong malformed numeric token. got=%q %q want=%q %q", tok.Type, tok.Lexeme, ILLEGAL, input)
			}
			if next := l.NextToken(); next.Type != EOF {
				t.Fatalf("malformed numeric token did not recover at EOF: %+v", next)
			}
		})
	}
}

func TestScientificExponentLiterals(t *testing.T) {
	input := `1e3 1E3 1.5e-2 .5E+4 1e3g 1e3m`

	tests := []struct {
		typ    TokenType
		lexeme string
	}{
		{FLOAT, "1e3"},
		{FLOAT, "1E3"},
		{FLOAT, "1.5e-2"},
		{FLOAT, ".5E+4"},
		{FLOAT, "1e3g"},
		{FLOAT, "1e3m"},
		{EOF, ""},
	}

	assertTokens(t, input, tests)
}

func TestLegacyNumericSuffixesAreSingleIllegalTokens(t *testing.T) {
	for _, input := range []string{"65c", "10d", "10f", "1.5d", "1.5f", "1e3d", "1e3f", "0b10c", "0o10d"} {
		t.Run(input, func(t *testing.T) {
			l := New(input)
			token := l.NextToken()
			if token.Type != ILLEGAL || token.Lexeme != input {
				t.Fatalf("legacy literal = %q %q, want %q %q", token.Type, token.Lexeme, ILLEGAL, input)
			}
		})
	}
}

func TestFractionalLiteralsRejectIntegerOnlySuffixes(t *testing.T) {
	for _, input := range []string{"1.5i", "1.5u", "1.5t", "1.5r", ".5t", "1e3r"} {
		t.Run(input, func(t *testing.T) {
			token := New(input).NextToken()
			if token.Type != ILLEGAL || token.Lexeme != input {
				t.Fatalf("invalid fractional literal = %q %q, want %q %q", token.Type, token.Lexeme, ILLEGAL, input)
			}
		})
	}
}

func TestMalformedScientificExponentIsOneIllegalToken(t *testing.T) {
	for _, input := range []string{"1e", "1e+", "1.5E-", ".5e+"} {
		t.Run(input, func(t *testing.T) {
			l := New(input)
			tok := l.NextToken()
			if tok.Type != ILLEGAL || tok.Lexeme != input {
				t.Fatalf("wrong malformed exponent token. got=%q %q want=%q %q", tok.Type, tok.Lexeme, ILLEGAL, input)
			}
			if next := l.NextToken(); next.Type != EOF {
				t.Fatalf("malformed exponent did not recover at EOF: %+v", next)
			}
		})
	}
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
