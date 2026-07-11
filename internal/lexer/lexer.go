package lexer

type TokenType string

const (
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"

	IDENT        TokenType = "IDENT"
	INT          TokenType = "INT"
	FLOAT        TokenType = "FLOAT"
	STRING       TokenType = "STRING"
	RAW_STRING   TokenType = "RAW_STRING"
	VOID         TokenType = "VOID"
	BYTES        TokenType = "BYTES"
	INTERPSTRING TokenType = "INTERPSTRING"

	ASM         TokenType = "ASM"
	ASSERT      TokenType = "ASSERT"
	AWAIT       TokenType = "AWAIT"
	BREAK       TokenType = "BREAK"
	CASE        TokenType = "CASE"
	CAPTURE     TokenType = "CAPTURE"
	CONTINUE    TokenType = "CONTINUE"
	DEFAULT     TokenType = "DEFAULT"
	DEFER       TokenType = "DEFER"
	DISCARD     TokenType = "DISCARD"
	ELSE        TokenType = "ELSE"
	ENUM        TokenType = "ENUM"
	FALLTHROUGH TokenType = "FALLTHROUGH"
	FALSE       TokenType = "FALSE"
	FN          TokenType = "FN"
	FOR         TokenType = "FOR"
	GET         TokenType = "GET"
	MODULE      TokenType = "MODULE"
	IF          TokenType = "IF"
	IMPL        TokenType = "IMPL"
	IMPORT      TokenType = "IMPORT"
	IN          TokenType = "IN"
	INTERFACE   TokenType = "INTERFACE"
	LET         TokenType = "LET"
	MATCH       TokenType = "MATCH"
	MUT         TokenType = "MUT"
	PANIC       TokenType = "PANIC"
	PROPERTY    TokenType = "PROPERTY"
	REF         TokenType = "REF"
	RETURN      TokenType = "RETURN"
	REQUIRE     TokenType = "REQUIRE"
	SEC         TokenType = "SEC"
	SET         TokenType = "SET"
	SPAWN       TokenType = "SPAWN"
	STRUCT      TokenType = "STRUCT"
	SWITCH      TokenType = "SWITCH"
	TRUE        TokenType = "TRUE"
	TRY         TokenType = "TRY"
	TYPE        TokenType = "TYPE"
	UNION       TokenType = "UNION"
	UNSAFE      TokenType = "UNSAFE"
	WHERE       TokenType = "WHERE"
	WHILE       TokenType = "WHILE"

	ASSIGN  TokenType = "ASSIGN"
	DECLARE TokenType = "DECLARE"
	ARROW   TokenType = "ARROW"

	PLUS     TokenType = "PLUS"
	MINUS    TokenType = "MINUS"
	ASTERISK TokenType = "ASTERISK"
	SLASH    TokenType = "SLASH"
	PERCENT  TokenType = "PERCENT"

	PLUS_ASSIGN     TokenType = "PLUS_ASSIGN"
	MINUS_ASSIGN    TokenType = "MINUS_ASSIGN"
	ASTERISK_ASSIGN TokenType = "ASTERISK_ASSIGN"
	SLASH_ASSIGN    TokenType = "SLASH_ASSIGN"
	PERCENT_ASSIGN  TokenType = "PERCENT_ASSIGN"

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
	BIT_AND            TokenType = "BIT_AND"     // &
	BIT_OR             TokenType = "BIT_OR"      // |
	BIT_XOR            TokenType = "BIT_XOR"     // ^
	BIT_NOT            TokenType = "BIT_NOT"     // !
	SHIFT_LEFT         TokenType = "SHIFT_LEFT"  // <<
	SHIFT_RIGHT        TokenType = "SHIFT_RIGHT" // >>
	BIT_AND_ASSIGN     TokenType = "BIT_AND_ASSIGN"
	BIT_OR_ASSIGN      TokenType = "BIT_OR_ASSIGN"
	BIT_XOR_ASSIGN     TokenType = "BIT_XOR_ASSIGN"
	SHIFT_LEFT_ASSIGN  TokenType = "SHIFT_LEFT_ASSIGN"
	SHIFT_RIGHT_ASSIGN TokenType = "SHIFT_RIGHT_ASSIGN"

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
)

type Token struct {
	Type   TokenType
	Lexeme string
	Line   int
	Column int
}

type Lexer struct {
	input  []rune
	pos    int
	line   int
	column int
}

func New(input string) *Lexer {
	return &Lexer{input: []rune(input), line: 1, column: 1}
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespaceAndComments()

	line := l.line
	column := l.column
	ch := l.peek()

	if ch == 0 {
		return Token{Type: EOF, Line: line, Column: column}
	}

	if isLetter(ch) {
		lit := l.readIdentifier()
		return Token{Type: lookupIdent(lit), Lexeme: lit, Line: line, Column: column}
	}

	if isDigit(ch) {
		lit, typ := l.readNumber()
		return Token{Type: typ, Lexeme: lit, Line: line, Column: column}
	}

	if ch == '$' && l.peekNext() == '"' {
		return l.readPrefixedString(INTERPSTRING)
	}

	if ch == '"' {
		return l.readPlainString()
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

func (l *Lexer) skipWhitespaceAndComments() {
	for isWhitespace(l.peek()) {
		l.advance()
	}
}

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

func (l *Lexer) readLineComment() Token {
	line := l.line
	column := l.column
	start := l.pos

	l.advance()
	l.advance()

	for l.peek() != '\n' && l.peek() != 0 {
		l.advance()
	}

	return Token{Type: COMMENT, Lexeme: string(l.input[start:l.pos]), Line: line, Column: column}
}

func (l *Lexer) readBlockComment() Token {
	line := l.line
	column := l.column
	start := l.pos
	depth := 0

	for {
		if l.peek() == 0 {
			return Token{Type: ILLEGAL, Lexeme: string(l.input[start:l.pos]), Line: line, Column: column}
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
				return Token{Type: COMMENT, Lexeme: string(l.input[start:l.pos]), Line: line, Column: column}
			}
			continue
		}

		l.advance()
	}
}

func (l *Lexer) readIdentifier() string {
	start := l.pos

	for isLetter(l.peek()) || isDigit(l.peek()) {
		l.advance()
	}

	return string(l.input[start:l.pos])
}

func (l *Lexer) readNumber() (string, TokenType) {
	start := l.pos
	typ := INT

	for isDigit(l.peek()) {
		l.advance()
	}

	if l.peek() == '.' && isDigit(l.peekNext()) {
		typ = FLOAT
		l.advance()

		for isDigit(l.peek()) {
			l.advance()
		}
	}

	return string(l.input[start:l.pos]), typ
}

func (l *Lexer) readLeadingDotNumber() Token {
	line := l.line
	column := l.column
	start := l.pos

	l.advance()
	for isDigit(l.peek()) {
		l.advance()
	}

	return Token{Type: FLOAT, Lexeme: string(l.input[start:l.pos]), Line: line, Column: column}
}

func (l *Lexer) readPlainString() Token {
	line := l.line
	column := l.column
	lit, ok := l.readStringBody(false)

	if !ok {
		return Token{Type: ILLEGAL, Lexeme: lit, Line: line, Column: column}
	}

	return Token{Type: STRING, Lexeme: lit, Line: line, Column: column}
}

func (l *Lexer) readRawString() Token {
	line := l.line
	column := l.column
	start := l.pos

	l.advance()
	for l.peek() != '`' && l.peek() != 0 {
		l.advance()
	}

	if l.peek() != '`' {
		return Token{Type: ILLEGAL, Lexeme: string(l.input[start:l.pos]), Line: line, Column: column}
	}

	l.advance()
	return Token{Type: RAW_STRING, Lexeme: string(l.input[start:l.pos]), Line: line, Column: column}
}

func (l *Lexer) readPrefixedString(typ TokenType) Token {
	line := l.line
	column := l.column
	start := l.pos

	l.advance()

	lit, ok := l.readStringBody(true)
	if !ok {
		return Token{Type: ILLEGAL, Lexeme: string(l.input[start:l.pos]), Line: line, Column: column}
	}

	return Token{Type: typ, Lexeme: "$" + lit, Line: line, Column: column}
}

func (l *Lexer) readStringBody(prefixed bool) (string, bool) {
	start := l.pos

	if l.peek() != '"' {
		return "", false
	}

	l.advance()

	for {
		ch := l.peek()

		if ch == 0 || ch == '\n' {
			return string(l.input[start:l.pos]), false
		}

		if ch == '\\' {
			l.advance()
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

func (l *Lexer) readOne(typ TokenType) Token {
	line := l.line
	column := l.column
	ch := l.peek()
	l.advance()

	return Token{Type: typ, Lexeme: string(ch), Line: line, Column: column}
}

func (l *Lexer) readTwo(typ TokenType) Token {
	line := l.line
	column := l.column
	first := l.peek()
	l.advance()
	second := l.peek()
	l.advance()

	return Token{Type: typ, Lexeme: string([]rune{first, second}), Line: line, Column: column}
}

func (l *Lexer) readThree(typ TokenType) Token {
	line := l.line
	column := l.column
	first := l.peek()
	l.advance()
	second := l.peek()
	l.advance()
	third := l.peek()
	l.advance()

	return Token{Type: typ, Lexeme: string([]rune{first, second, third}), Line: line, Column: column}
}

func (l *Lexer) peek() rune {
	return l.peekOffset(0)
}

func (l *Lexer) peekNext() rune {
	return l.peekOffset(1)
}

func (l *Lexer) peekOffset(offset int) rune {
	index := l.pos + offset
	if index >= len(l.input) {
		return 0
	}
	return l.input[index]
}

func (l *Lexer) advance() rune {
	ch := l.peek()
	if ch == 0 {
		return 0
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

func lookupIdent(s string) TokenType {
	switch s {
	case "asm":
		return ASM
	case "assert":
		return ASSERT
	case "await":
		return AWAIT
	case "break":
		return BREAK
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
	case "fn":
		return FN
	case "for":
		return FOR
	case "get":
		return GET
	case "if":
		return IF
	case "impl":
		return IMPL
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
	case "set":
		return SET
	case "spawn":
		return SPAWN
	case "struct":
		return STRUCT
	case "switch":
		return SWITCH
	case "type":
		return TYPE
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

func isLetter(ch rune) bool {
	return ch == '_' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}
