package lexer

import (
	"fmt"
	"unicode/utf8"

	compilerdiagnostics "sec/internal/diagnostics"
)

// Token retains a token's exact spelling, scalar position, and half-open byte
// range in the original UTF-8 source.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §1.3 "Unicode scalar values"
//   - rules/foundations/lexical_structure.md — §25 "Required tests"
type Token struct {
	Type      TokenType
	Lexeme    string
	File      string
	Line      int
	Column    int
	EndLine   int
	EndColumn int
	ByteStart int
	ByteEnd   int
}

// EndPosition returns the token's one-based exclusive scalar position. Tokens
// produced by Lexer carry it directly; manually constructed compatibility
// tokens fall back to the same LF, CRLF, and bare-CR traversal as the lexer.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§1.3 and 2
//   - rules/tooling/diagnostics.md — §9 "Source locations"
func (t Token) EndPosition() (line int, column int) {
	if t.EndLine > 0 && t.EndColumn > 0 {
		return t.EndLine, t.EndColumn
	}
	line, column = t.Line, t.Column
	runes := []rune(t.Lexeme)
	for index := 0; index < len(runes); index++ {
		switch runes[index] {
		case '\r':
			line++
			column = 1
			if index+1 < len(runes) && runes[index+1] == '\n' {
				index++
			}
		case '\n':
			line++
			column = 1
		default:
			column++
		}
	}
	return line, column
}

// Diagnostic is a lexical error discovered while decoding or tokenizing the
// source. Primary identifies the exact offending source character or byte.
type Diagnostic struct {
	ID      string
	Message string
	Primary Token
}

// Lexer owns both the decoded scalar stream and its boundary map back to the
// original UTF-8 bytes.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§1.1, 1.3
type Lexer struct {
	input       []rune
	byteOffsets []int
	file        string
	pos         int
	line        int
	column      int
	diagnostics []Diagnostic
}

// State is the complete rewind point required by speculative parser lookahead.
type State struct {
	Pos         int
	Line        int
	Column      int
	Diagnostics int
}

// New creates a lexer for source without a file identity.
func New(input string) *Lexer {
	return NewWithFile(input, "")
}

// NewWithFile decodes source while retaining original byte boundaries.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§1.1, 1.3
func NewWithFile(input string, file string) *Lexer {
	decoded, byteOffsets, diagnostics := decodeSource(input, file)
	return &Lexer{input: decoded, byteOffsets: byteOffsets, file: file, line: 1, column: 1, diagnostics: diagnostics}
}

// Diagnostics returns an isolated copy of lexical diagnostics accumulated so
// far. Encoding and BOM diagnostics are available immediately.
func (l *Lexer) Diagnostics() []Diagnostic {
	result := make([]Diagnostic, len(l.diagnostics))
	copy(result, l.diagnostics)
	return result
}

// decodeSource validates UTF-8 and constructs a boundary map from decoded
// rune indexes to original-source byte offsets. CRLF remains two source runes
// but one physical line; an initial BOM is omitted from the decoded stream.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§1.1, 1.3, 2, 20
//   - rules/corrections/applied/correction-20260823.md — "Required correction"
func decodeSource(input string, file string) ([]rune, []int, []Diagnostic) {
	decoded := make([]rune, 0, utf8.RuneCountInString(input))
	byteOffsets := []int{0}
	diagnostics := []Diagnostic{}
	line, column := 1, 1
	byteOffset := 0
	first := true
	previousWasCR := false
	for len(input) > 0 {
		r, size := utf8.DecodeRuneInString(input)
		lexeme := input[:size]
		if r == utf8.RuneError && size == 1 {
			diagnostics = append(diagnostics, Diagnostic{
				ID:      compilerdiagnostics.LexerInvalidUTF8,
				Message: fmt.Sprintf("invalid UTF-8 byte 0x%02X at %d:%d", input[0], line, column),
				Primary: Token{Type: ILLEGAL, Lexeme: lexeme, File: file, Line: line, Column: column, EndLine: line, EndColumn: column + 1, ByteStart: byteOffset, ByteEnd: byteOffset + size},
			})
			decoded = append(decoded, utf8.RuneError)
		} else if r == '\uFEFF' {
			if first {
				byteOffset += size
				byteOffsets[0] = byteOffset
				input = input[size:]
				first = false
				continue
			}
			diagnostics = append(diagnostics, Diagnostic{
				ID:      compilerdiagnostics.LexerUnexpectedByteOrderMark,
				Message: fmt.Sprintf("unexpected byte-order mark U+FEFF at %d:%d; it is permitted only at the start of a source file", line, column),
				Primary: Token{Type: ILLEGAL, Lexeme: lexeme, File: file, Line: line, Column: column, EndLine: line, EndColumn: column + 1, ByteStart: byteOffset, ByteEnd: byteOffset + size},
			})
			decoded = append(decoded, ' ')
		} else {
			decoded = append(decoded, r)
		}
		byteOffset += size
		byteOffsets = append(byteOffsets, byteOffset)
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
	return decoded, byteOffsets, diagnostics
}

// Snapshot captures speculative lexer state without copying immutable source
// or byte-boundary data.
func (l *Lexer) Snapshot() State {
	return State{Pos: l.pos, Line: l.line, Column: l.column, Diagnostics: len(l.diagnostics)}
}

// Restore rewinds token position and diagnostics to a prior snapshot.
func (l *Lexer) Restore(state State) {
	l.pos = state.Pos
	l.line = state.Line
	l.column = state.Column
	if state.Diagnostics >= 0 && state.Diagnostics <= len(l.diagnostics) {
		l.diagnostics = l.diagnostics[:state.Diagnostics]
	}
}

// token anchors a token to exact half-open scalar and UTF-8 byte ranges in the
// original source. The decoded-rune cursor deliberately remains separate so
// invalid bytes and a skipped initial BOM retain their original coordinates.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §1.3 "Unicode scalar values"
//   - rules/foundations/lexical_structure.md — §20 "Lexical errors"
//   - rules/foundations/lexical_structure.md — §25 "Required tests"
//   - rules/tooling/diagnostics.md — §9 "Source locations"
func (l *Lexer) token(typ TokenType, lexeme string, line int, column int) Token {
	endRune := l.pos
	startRune := endRune - utf8.RuneCountInString(lexeme)
	token := Token{
		Type: typ, Lexeme: lexeme, File: l.file, Line: line, Column: column,
		EndLine: l.line, EndColumn: l.column,
	}
	if startRune < 0 || endRune >= len(l.byteOffsets) {
		return token
	}
	token.ByteStart = l.byteOffsets[startRune]
	token.ByteEnd = l.byteOffsets[endRune]
	return token
}
