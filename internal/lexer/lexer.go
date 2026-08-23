package lexer

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	compilerdiagnostics "sec/internal/diagnostics"
)

type TokenType string

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
const (
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"

	IDENT        TokenType = "IDENT"
	INT          TokenType = "INT"
	FLOAT        TokenType = "FLOAT"
	STRING       TokenType = "STRING"
	CHAR         TokenType = "CHAR"
	RAW_STRING   TokenType = "RAW_STRING"
	VOID         TokenType = "VOID"
	BYTES        TokenType = "BYTES"
	INTERPSTRING TokenType = "INTERPSTRING"

	ARENA       TokenType = "ARENA"
	AFTER       TokenType = "AFTER"
	ASM         TokenType = "ASM"
	ASSERT      TokenType = "ASSERT"
	AWAIT       TokenType = "AWAIT"
	BREAK       TokenType = "BREAK"
	CANCEL      TokenType = "CANCEL"
	CASE        TokenType = "CASE"
	CAPTURE     TokenType = "CAPTURE"
	CONTINUE    TokenType = "CONTINUE"
	DEFAULT     TokenType = "DEFAULT"
	DEFER       TokenType = "DEFER"
	DISCARD     TokenType = "DISCARD"
	ELSE        TokenType = "ELSE"
	ENUM        TokenType = "ENUM"
	EXTERN      TokenType = "EXTERN"
	EXTENDS     TokenType = "EXTENDS"
	FALLTHROUGH TokenType = "FALLTHROUGH"
	FALSE       TokenType = "FALSE"
	FN          TokenType = "FN"
	FOR         TokenType = "FOR"
	FREE        TokenType = "FREE"
	GET         TokenType = "GET"
	MODULE      TokenType = "MODULE"
	IF          TokenType = "IF"
	IMPL        TokenType = "IMPL"
	IMPLEMENTS  TokenType = "IMPLEMENTS"
	IMPORT      TokenType = "IMPORT"
	IN          TokenType = "IN"
	INTERFACE   TokenType = "INTERFACE"
	LET         TokenType = "LET"
	MATCH       TokenType = "MATCH"
	MUT         TokenType = "MUT"
	NEW         TokenType = "NEW"
	PANIC       TokenType = "PANIC"
	PROPERTY    TokenType = "PROPERTY"
	REF         TokenType = "REF"
	RETURN      TokenType = "RETURN"
	REQUIRE     TokenType = "REQUIRE"
	SEC         TokenType = "SEC"
	SELF        TokenType = "SELF"
	SELECT      TokenType = "SELECT"
	SET         TokenType = "SET"
	SPAWN       TokenType = "SPAWN"
	STATIC      TokenType = "STATIC"
	STRUCT      TokenType = "STRUCT"
	SWITCH      TokenType = "SWITCH"
	TRUE        TokenType = "TRUE"
	TRY         TokenType = "TRY"
	TYPE        TokenType = "TYPE"
	UNIT        TokenType = "UNIT"
	UNION       TokenType = "UNION"
	UNSAFE      TokenType = "UNSAFE"
	WHERE       TokenType = "WHERE"
	WHILE       TokenType = "WHILE"

	ASSIGN        TokenType = "ASSIGN"
	DECLARE       TokenType = "DECLARE"
	ARROW         TokenType = "ARROW"
	CONSUME_ARROW TokenType = "CONSUME_ARROW" // ->

	PLUS     TokenType = "PLUS"
	MINUS    TokenType = "MINUS"
	ASTERISK TokenType = "ASTERISK"
	SLASH    TokenType = "SLASH"
	PERCENT  TokenType = "PERCENT"

	PLUS_ASSIGN     TokenType = "PLUS_ASSIGN"     // +=
	MINUS_ASSIGN    TokenType = "MINUS_ASSIGN"    // -=
	ASTERISK_ASSIGN TokenType = "ASTERISK_ASSIGN" // *=
	SLASH_ASSIGN    TokenType = "SLASH_ASSIGN"    // /=
	PERCENT_ASSIGN  TokenType = "PERCENT_ASSIGN"  // %=

	// Logical
	EQ  TokenType = "EQ"  // ==
	NEQ TokenType = "NEQ" // !=
	LT  TokenType = "LT"  // >
	LTE TokenType = "LTE" // >=
	GT  TokenType = "GT"  // <
	GTE TokenType = "GTE" // <=
	AND TokenType = "AND" // &&
	OR  TokenType = "OR"  // ||
	NOT TokenType = "NOT" // !

	// Bitwise
	BIT_AND            TokenType = "BIT_AND"            // &
	BIT_OR             TokenType = "BIT_OR"             // |
	BIT_XOR            TokenType = "BIT_XOR"            // ^
	BIT_NOT            TokenType = "BIT_NOT"            // !
	SHIFT_LEFT         TokenType = "SHIFT_LEFT"         // <<
	SHIFT_RIGHT        TokenType = "SHIFT_RIGHT"        // >>
	BIT_AND_ASSIGN     TokenType = "BIT_AND_ASSIGN"     //
	BIT_OR_ASSIGN      TokenType = "BIT_OR_ASSIGN"      //
	BIT_XOR_ASSIGN     TokenType = "BIT_XOR_ASSIGN"     //
	SHIFT_LEFT_ASSIGN  TokenType = "SHIFT_LEFT_ASSIGN"  //
	SHIFT_RIGHT_ASSIGN TokenType = "SHIFT_RIGHT_ASSIGN" //

	DOT             TokenType = "DOT"
	RANGE           TokenType = "RANGE"           // ..
	RANGE_KW        TokenType = "RANGE_KW"        // keyword: range in type contracts
	RANGE_EXCLUSIVE TokenType = "RANGE_EXCLUSIVE" // ..<
	SPREAD          TokenType = "SPREAD"          // ...

	COMMA      TokenType = "COMMA"      // ,
	COLON      TokenType = "COLON"      // :
	SEMICOLON  TokenType = "SEMICOLON"  // ;
	QUESTION   TokenType = "QUESTION"   // ?
	UNDERSCORE TokenType = "UNDERSCORE" // _
	AT         TokenType = "AT"         // @
	HASH       TokenType = "HASH"       // #
	LPAREN     TokenType = "LPAREN"     // (
	RPAREN     TokenType = "RPAREN"     // )
	LBRACE     TokenType = "LBRACE"     // {
	RBRACE     TokenType = "RBRACE"     // }
	LBRACKET   TokenType = "LBRACKET"   // [
	RBRACKET   TokenType = "RBRACKET"   // ]

	COMMENT TokenType = "COMMENT"

	MOVE_ASSIGN  TokenType = "MOVE_ASSIGN"  // <-
	MOVE_DECLARE TokenType = "MOVE_DECLARE" // :<-
)

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
type Token struct {
	Type   TokenType
	Lexeme string
	File   string
	Line   int
	Column int
}

// Diagnostic is a lexical error discovered while decoding or tokenizing the
// source. Primary identifies the exact offending source character or byte.
type Diagnostic struct {
	ID      string
	Message string
	Primary Token
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
type Lexer struct {
	input       []rune
	file        string
	pos         int
	line        int
	column      int
	diagnostics []Diagnostic
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
type State struct {
	Pos         int
	Line        int
	Column      int
	Diagnostics int
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func New(input string) *Lexer {
	return NewWithFile(input, "")
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func NewWithFile(input string, file string) *Lexer {
	decoded, diagnostics := decodeSource(input, file)
	return &Lexer{input: decoded, file: file, line: 1, column: 1, diagnostics: diagnostics}
}

// 2026-08-10 22:28 CEST: Added strict UTF-8 decoding, initial-BOM removal,
// unexpected-BOM recovery, and Unicode-whitespace diagnostics.
// Diagnostics returns the lexical diagnostics discovered so far. Encoding and
// BOM diagnostics are available immediately; token-context diagnostics appear
// as NextToken advances.
func (l *Lexer) Diagnostics() []Diagnostic {
	result := make([]Diagnostic, len(l.diagnostics))
	copy(result, l.diagnostics)
	return result
}

// decodeSource validates source encoding and establishes diagnostic positions
// under rules/foundations/lexical_structure.md. correction.md requires the
// decoder and token cursor to share LF, CRLF, and bare-CR line semantics.
func decodeSource(input string, file string) ([]rune, []Diagnostic) {
	decoded := make([]rune, 0, utf8.RuneCountInString(input))
	diagnostics := []Diagnostic{}
	line, column := 1, 1
	first := true
	previousWasCR := false
	for len(input) > 0 {
		r, size := utf8.DecodeRuneInString(input)
		lexeme := input[:size]
		if r == utf8.RuneError && size == 1 {
			diagnostics = append(diagnostics, Diagnostic{
				ID:      compilerdiagnostics.LexerInvalidUTF8,
				Message: fmt.Sprintf("invalid UTF-8 byte 0x%02X at %d:%d", input[0], line, column),
				Primary: Token{Type: ILLEGAL, Lexeme: lexeme, File: file, Line: line, Column: column},
			})
			decoded = append(decoded, utf8.RuneError)
		} else if r == '\uFEFF' {
			if first {
				input = input[size:]
				first = false
				continue
			}
			diagnostics = append(diagnostics, Diagnostic{
				ID:      compilerdiagnostics.LexerUnexpectedByteOrderMark,
				Message: fmt.Sprintf("unexpected byte-order mark U+FEFF at %d:%d; it is permitted only at the start of a source file", line, column),
				Primary: Token{Type: ILLEGAL, Lexeme: lexeme, File: file, Line: line, Column: column},
			})
			// Preserve following source columns while allowing tokenization to
			// recover without producing a second generic parser diagnostic.
			decoded = append(decoded, ' ')
		} else {
			decoded = append(decoded, r)
		}
		// rules/foundations/lexical_structure.md, Physical source lines;
		// correction.md: CRLF is one line ending and bare CR is also a line ending.
		if r == '\r' {
			line++
			column = 1
		} else if r == '\n' {
			if !previousWasCR {
				line++
			}
			column = 1
		} else {
			column++
		}
		previousWasCR = r == '\r'
		input = input[size:]
		first = false
	}
	return decoded, diagnostics
}

func (l *Lexer) Snapshot() State {
	return State{Pos: l.pos, Line: l.line, Column: l.column, Diagnostics: len(l.diagnostics)}
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) Restore(state State) {
	l.pos = state.Pos
	l.line = state.Line
	l.column = state.Column
	if state.Diagnostics >= 0 && state.Diagnostics <= len(l.diagnostics) {
		l.diagnostics = l.diagnostics[:state.Diagnostics]
	}
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) NextToken() Token {
	l.skipWhitespaceAndComments()

	line := l.line
	column := l.column
	ch := l.peek()

	if ch == 0 {
		return l.token(EOF, "", line, column)
	}

	if isUnsupportedUnicodeWhitespace(ch) {
		token := l.readOne(ILLEGAL)
		l.diagnostics = append(l.diagnostics, Diagnostic{
			ID:      compilerdiagnostics.LexerUnsupportedWhitespace,
			Message: fmt.Sprintf("unsupported Unicode whitespace U+%04X at %d:%d", ch, line, column),
			Primary: token,
		})
		return token
	}

	// 2026-08-01: Keep the grammar's bare discard/reserved-field symbol
	// distinct from ordinary identifiers such as _name and __name.
	if ch == '_' && !isLetter(l.peekNext()) && !isDigit(l.peekNext()) {
		return l.readOne(UNDERSCORE)
	}

	if isLetter(ch) {
		lit := l.readIdentifier()
		return l.token(lookupIdent(lit), lit, line, column)
	}

	if isDigit(ch) {
		lit, typ := l.readNumber()
		return l.token(typ, lit, line, column)
	}

	if ch == '$' && l.peekNext() == '"' {
		return l.readPrefixedString(INTERPSTRING)
	}

	if ch == '"' {
		return l.readPlainString()
	}

	if ch == '\'' {
		return l.readCharLiteral()
	}

	if ch == '`' {
		return l.readRawString()
	}

	switch ch {
	case '=':
		if l.peekNext() == '=' {
			return l.readTwo(EQ)
		}
		if l.peekNext() == '>' {
			return l.readTwo(ARROW)
		}
		return l.readOne(ASSIGN)

	case ':':
		if l.peekNext() == '<' && l.peekOffset(2) == '-' {
			return l.readThree(MOVE_DECLARE)
		}
		if l.peekNext() == '=' {
			return l.readTwo(DECLARE)
		}
		return l.readOne(COLON)

	case '.':
		if l.peekNext() == '.' && l.peekOffset(2) == '.' {
			return l.readThree(SPREAD)
		}
		if l.peekNext() == '.' && l.peekOffset(2) == '<' {
			return l.readThree(RANGE_EXCLUSIVE)
		}
		if l.peekNext() == '.' {
			return l.readTwo(RANGE)
		}
		if isDigit(l.peekNext()) {
			return l.readLeadingDotNumber()
		}
		return l.readOne(DOT)

	case '+':
		if l.peekNext() == '=' {
			return l.readTwo(PLUS_ASSIGN)
		}
		return l.readOne(PLUS)

	case '-':
		if l.peekNext() == '>' {
			return l.readTwo(CONSUME_ARROW)
		}
		if l.peekNext() == '=' {
			return l.readTwo(MINUS_ASSIGN)
		}
		return l.readOne(MINUS)

	case '*':
		if l.peekNext() == '=' {
			return l.readTwo(ASTERISK_ASSIGN)
		}
		return l.readOne(ASTERISK)

	case '/':
		if l.peekNext() == '/' {
			return l.readLineComment()
		}
		if l.peekNext() == '*' {
			return l.readBlockComment()
		}
		if l.peekNext() == '=' {
			return l.readTwo(SLASH_ASSIGN)
		}
		return l.readOne(SLASH)

	case '%':
		if l.peekNext() == '=' {
			return l.readTwo(PERCENT_ASSIGN)
		}
		return l.readOne(PERCENT)

	case '!':
		if l.peekNext() == '=' {
			return l.readTwo(NEQ)
		}
		return l.readOne(NOT)

	case '<':
		if l.peekNext() == '-' {
			return l.readTwo(MOVE_ASSIGN)
		}
		if l.peekNext() == '<' && l.peekOffset(2) == '=' {
			return l.readThree(SHIFT_LEFT_ASSIGN)
		}
		if l.peekNext() == '<' {
			return l.readTwo(SHIFT_LEFT)
		}
		if l.peekNext() == '=' {
			return l.readTwo(LTE)
		}
		return l.readOne(LT)

	case '>':
		if l.peekNext() == '>' && l.peekOffset(2) == '=' {
			return l.readThree(SHIFT_RIGHT_ASSIGN)
		}
		if l.peekNext() == '>' {
			return l.readTwo(SHIFT_RIGHT)
		}
		if l.peekNext() == '=' {
			return l.readTwo(GTE)
		}
		return l.readOne(GT)

	case '&':
		if l.peekNext() == '&' {
			return l.readTwo(AND)
		}
		if l.peekNext() == '=' {
			return l.readTwo(BIT_AND_ASSIGN)
		}
		return l.readOne(BIT_AND)

	case '|':
		if l.peekNext() == '|' {
			return l.readTwo(OR)
		}
		if l.peekNext() == '=' {
			return l.readTwo(BIT_OR_ASSIGN)
		}
		return l.readOne(BIT_OR)

	case '^':
		if l.peekNext() == '=' {
			return l.readTwo(BIT_XOR_ASSIGN)
		}
		return l.readOne(BIT_XOR)

	case '~':
		return l.readOne(BIT_NOT)

	case ',':
		return l.readOne(COMMA)
	case ';':
		return l.readOne(SEMICOLON)
	case '?':
		return l.readOne(QUESTION)
	case '_':
		return l.readOne(UNDERSCORE)
	case '@':
		return l.readOne(AT)
	case '#':
		return l.readOne(HASH)

	case '(':
		return l.readOne(LPAREN)
	case ')':
		return l.readOne(RPAREN)
	case '{':
		return l.readOne(LBRACE)
	case '}':
		return l.readOne(RBRACE)
	case '[':
		return l.readOne(LBRACKET)
	case ']':
		return l.readOne(RBRACKET)
	}

	return l.readOne(ILLEGAL)
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) skipWhitespaceAndComments() {
	for isWhitespace(l.peek()) {
		l.advance()
	}
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) skipBlockComment() {
	depth := 0

	for {
		if l.peek() == 0 {
			return
		}

		if l.peek() == '/' && l.peekNext() == '*' {
			depth++
			l.advance()
			l.advance()
			continue
		}

		if l.peek() == '*' && l.peekNext() == '/' {
			depth--
			l.advance()
			l.advance()

			if depth == 0 {
				return
			}
			continue
		}

		l.advance()
	}
}

// readLineComment implements rules/foundations/lexical_structure.md line-comment
// boundaries. Per correction.md, LF, CRLF, and bare CR all terminate the token.
func (l *Lexer) readLineComment() Token {
	line := l.line
	column := l.column
	start := l.pos

	l.advance()
	l.advance()

	for !isPhysicalLineEnding(l.peek()) && l.peek() != 0 {
		l.advance()
	}

	return l.token(COMMENT, string(l.input[start:l.pos]), line, column)
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) readBlockComment() Token {
	line := l.line
	column := l.column
	start := l.pos
	depth := 0

	for {
		if l.peek() == 0 {
			return l.token(ILLEGAL, string(l.input[start:l.pos]), line, column)
		}

		if l.peek() == '/' && l.peekNext() == '*' {
			depth++
			l.advance()
			l.advance()
			continue
		}

		if l.peek() == '*' && l.peekNext() == '/' {
			l.advance()
			l.advance()
			depth--

			if depth == 0 {
				return l.token(COMMENT, string(l.input[start:l.pos]), line, column)
			}
			continue
		}

		l.advance()
	}
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) readIdentifier() string {
	start := l.pos

	for isLetter(l.peek()) || isDigit(l.peek()) {
		l.advance()
	}

	return string(l.input[start:l.pos])
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) readNumber() (string, TokenType) {
	start := l.pos
	typ := INT
	valid := true

	if l.peek() == '0' {
		switch l.peekNext() {
		case 'b', 'B':
			l.advance()
			l.advance()
			_, valid = l.readDigitSequence(func(ch rune) bool { return ch == '0' || ch == '1' })
			typ = l.consumeIntegerSuffix(typ)
			if !valid {
				return string(l.input[start:l.pos]), ILLEGAL
			}
			return string(l.input[start:l.pos]), typ
		case 'o', 'O':
			l.advance()
			l.advance()
			_, valid = l.readDigitSequence(func(ch rune) bool { return ch >= '0' && ch <= '7' })
			typ = l.consumeIntegerSuffix(typ)
			if !valid {
				return string(l.input[start:l.pos]), ILLEGAL
			}
			return string(l.input[start:l.pos]), typ
		case 'x', 'X':
			l.advance()
			l.advance()
			_, valid = l.readDigitSequence(isHexDigit)
			typ = l.consumeIntegerSuffix(typ)
			if !valid {
				return string(l.input[start:l.pos]), ILLEGAL
			}
			return string(l.input[start:l.pos]), typ
		}
	}

	_, valid = l.readDigitSequence(isDigit)
	if l.peek() == '.' && (isDigit(l.peekNext()) || l.peekNext() == '_') {
		typ = FLOAT
		l.advance()
		_, fractionValid := l.readDigitSequence(isDigit)
		valid = valid && fractionValid
	}
	if l.peek() == 'e' || l.peek() == 'E' {
		typ = FLOAT
		if !l.readDecimalExponent() {
			valid = false
		}
	}
	if isCanonicalNumericSuffix(l.peek()) {
		suffix := l.peek()
		l.advance()
		if typ == FLOAT && !isFractionalNumericSuffix(suffix) {
			valid = false
		}
		if suffix == 'g' || suffix == 'm' {
			typ = FLOAT
		}
	} else if isLegacyNumericSuffix(l.peek()) {
		l.advance()
		valid = false
	}
	if !valid {
		return string(l.input[start:l.pos]), ILLEGAL
	}

	return string(l.input[start:l.pos]), typ
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) readLeadingDotNumber() Token {
	line := l.line
	column := l.column
	start := l.pos

	l.advance()
	_, valid := l.readDigitSequence(isDigit)
	if l.peek() == 'e' || l.peek() == 'E' {
		if !l.readDecimalExponent() {
			valid = false
		}
	}
	if isFractionalNumericSuffix(l.peek()) {
		l.advance()
	} else if isCanonicalNumericSuffix(l.peek()) || isLegacyNumericSuffix(l.peek()) {
		l.advance()
		valid = false
	}
	if !valid {
		return l.token(ILLEGAL, string(l.input[start:l.pos]), line, column)
	}

	return l.token(FLOAT, string(l.input[start:l.pos]), line, column)
}

// readDecimalExponent consumes an exponent marker, an optional sign, and its
// required decimal digits. The caller retains the complete malformed token.
func (l *Lexer) readDecimalExponent() bool {
	l.advance()
	if l.peek() == '+' || l.peek() == '-' {
		l.advance()
	}
	digits, valid := l.readDigitSequence(isDigit)
	return digits && valid
}

func (l *Lexer) readDigitSequence(validDigit func(rune) bool) (bool, bool) {
	sawDigit := false
	previousWasDigit := false
	valid := true
	for validDigit(l.peek()) || l.peek() == '_' {
		if l.peek() == '_' {
			if !previousWasDigit || !validDigit(l.peekNext()) {
				valid = false
			}
			previousWasDigit = false
			l.advance()
			continue
		}
		sawDigit = true
		previousWasDigit = true
		l.advance()
	}
	return sawDigit, valid && sawDigit
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) readPlainString() Token {
	line := l.line
	column := l.column
	lit, ok := l.readStringBody(false)

	if !ok {
		return l.token(ILLEGAL, lit, line, column)
	}

	return l.token(STRING, lit, line, column)
}

// readCharLiteral implements rules/foundations/lexical_structure.md character
// literal boundaries. Physical line endings are rejected per correction.md.
func (l *Lexer) readCharLiteral() Token {
	line := l.line
	column := l.column
	start := l.pos

	l.advance()
	for {
		ch := l.peek()
		if ch == 0 || isPhysicalLineEnding(ch) {
			return l.token(ILLEGAL, string(l.input[start:l.pos]), line, column)
		}
		if ch == '\\' {
			l.advance()
			if isPhysicalLineEnding(l.peek()) {
				return l.token(ILLEGAL, string(l.input[start:l.pos]), line, column)
			}
			if l.peek() != 0 {
				l.advance()
			}
			continue
		}
		if ch == '\'' {
			l.advance()
			return l.token(CHAR, string(l.input[start:l.pos]), line, column)
		}
		l.advance()
	}
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) readRawString() Token {
	line := l.line
	column := l.column
	start := l.pos

	l.advance()
	for l.peek() != '`' && l.peek() != 0 {
		l.advance()
	}

	if l.peek() != '`' {
		return l.token(ILLEGAL, string(l.input[start:l.pos]), line, column)
	}

	l.advance()
	return l.token(RAW_STRING, string(l.input[start:l.pos]), line, column)
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) readPrefixedString(typ TokenType) Token {
	line := l.line
	column := l.column
	start := l.pos

	l.advance()

	lit, ok := l.readStringBody(true)
	if !ok {
		return l.token(ILLEGAL, string(l.input[start:l.pos]), line, column)
	}

	return l.token(typ, "$"+lit, line, column)
}

// readStringBody implements rules/foundations/lexical_structure.md ordinary and
// interpolated string boundaries. Physical line endings are rejected per correction.md.
func (l *Lexer) readStringBody(prefixed bool) (string, bool) {
	start := l.pos

	if l.peek() != '"' {
		return "", false
	}

	l.advance()

	for {
		ch := l.peek()

		if ch == 0 || isPhysicalLineEnding(ch) {
			return string(l.input[start:l.pos]), false
		}

		if ch == '\\' {
			l.advance()
			if isPhysicalLineEnding(l.peek()) {
				return string(l.input[start:l.pos]), false
			}
			if l.peek() != 0 {
				l.advance()
			}
			continue
		}

		if ch == '"' {
			l.advance()
			return string(l.input[start:l.pos]), true
		}

		l.advance()
	}
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) readOne(typ TokenType) Token {
	line := l.line
	column := l.column
	ch := l.peek()
	l.advance()

	return l.token(typ, string(ch), line, column)
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) readTwo(typ TokenType) Token {
	line := l.line
	column := l.column
	first := l.peek()
	l.advance()
	second := l.peek()
	l.advance()

	return l.token(typ, string([]rune{first, second}), line, column)
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) readThree(typ TokenType) Token {
	line := l.line
	column := l.column
	first := l.peek()
	l.advance()
	second := l.peek()
	l.advance()
	third := l.peek()
	l.advance()

	return l.token(typ, string([]rune{first, second, third}), line, column)
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) token(typ TokenType, lexeme string, line int, column int) Token {
	return Token{Type: typ, Lexeme: lexeme, File: l.file, Line: line, Column: column}
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) peek() rune {
	return l.peekOffset(0)
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) peekNext() rune {
	return l.peekOffset(1)
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func (l *Lexer) peekOffset(offset int) rune {
	index := l.pos + offset
	if index >= len(l.input) {
		return 0
	}
	return l.input[index]
}

// advance maintains source positions according to
// rules/foundations/lexical_structure.md. correction.md requires CRLF to be
// consumed as one physical line ending and bare CR to advance one line.
func (l *Lexer) advance() rune {
	ch := l.peek()
	if ch == 0 {
		return 0
	}

	if ch == '\r' {
		l.pos++
		if l.peek() == '\n' {
			l.pos++
		}
		l.line++
		l.column = 1
		return ch
	}

	l.pos++
	if ch == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}

	return ch
}

// isPhysicalLineEnding is the canonical lexer boundary from
// rules/foundations/lexical_structure.md and correction.md.
func isPhysicalLineEnding(ch rune) bool {
	return ch == '\n' || ch == '\r'
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func lookupIdent(s string) TokenType {
	switch s {
	case "after":
		return AFTER
	case "asm":
		return ASM
	case "assert":
		return ASSERT
	case "await":
		return AWAIT
	case "break":
		return BREAK
	case "cancel":
		return CANCEL
	case "case":
		return CASE
	case "capture":
		return CAPTURE
	case "continue":
		return CONTINUE
	case "default":
		return DEFAULT
	case "defer":
		return DEFER
	case "discard":
		return DISCARD
	case "else":
		return ELSE
	case "enum":
		return ENUM
	case "false":
		return FALSE
	case "fallthrough":
		return FALLTHROUGH
	case "extern":
		return EXTERN
	case "extends":
		return EXTENDS
	case "fn":
		return FN
	case "for":
		return FOR
	case "free":
		return FREE
	case "get":
		return GET
	case "if":
		return IF
	case "impl":
		return IMPL
	case "implements":
		return IMPLEMENTS
	case "import":
		return IMPORT
	case "in":
		return IN
	case "interface":
		return INTERFACE
	case "let":
		return LET
	case "match":
		return MATCH
	case "module":
		return MODULE
	case "mut":
		return MUT
	case "new":
		return NEW
	case "panic":
		return PANIC
	case "property":
		return PROPERTY
	case "range":
		return RANGE_KW
	case "ref":
		return REF
	case "return":
		return RETURN
	case "require":
		return REQUIRE
	case "sec":
		return SEC
	case "self":
		return SELF
	case "select":
		return SELECT
	case "spawn":
		return SPAWN
	case "static":
		return STATIC
	case "struct":
		return STRUCT
	case "switch":
		return SWITCH
	case "type":
		return TYPE
	case "unit":
		return UNIT
	case "true":
		return TRUE
	case "try":
		return TRY
	case "union":
		return UNION
	case "unsafe":
		return UNSAFE
	case "where":
		return WHERE
	case "while":
		return WHILE

	default:
		return IDENT
	}
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func isLetter(ch rune) bool {
	return ch == '_' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		unicode.IsLetter(ch)
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func isHexDigit(ch rune) bool {
	return isDigit(ch) ||
		(ch >= 'a' && ch <= 'f') ||
		(ch >= 'A' && ch <= 'F')
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func isCanonicalNumericSuffix(ch rune) bool {
	switch ch {
	case 'i', 'u', 'g', 'm', 't', 'r':
		return true
	default:
		return false
	}
}

func isFractionalNumericSuffix(ch rune) bool {
	return ch == 'g' || ch == 'm'
}

func isLegacyNumericSuffix(ch rune) bool {
	return ch == 'c' || ch == 'd' || ch == 'f'
}

func (l *Lexer) consumeIntegerSuffix(typ TokenType) TokenType {
	if isCanonicalNumericSuffix(l.peek()) {
		suffix := l.peek()
		l.advance()
		if suffix == 'g' || suffix == 'm' {
			return FLOAT
		}
		return typ
	}
	if isLegacyNumericSuffix(l.peek()) {
		l.advance()
		return ILLEGAL
	}
	return typ
}

// Transferred to sec - ALL changes *MUST* be visible and commented with date, time and what has changed.
func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isUnsupportedUnicodeWhitespace(ch rune) bool {
	return unicode.IsSpace(ch) && !isWhitespace(ch)
}
