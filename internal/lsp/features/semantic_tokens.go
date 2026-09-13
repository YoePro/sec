// Package features contains protocol-independent LSP feature implementations.
package features

import (
	"fmt"
	"strings"
	"unicode/utf16"

	"sec/internal/lexer"
)

var SemanticTokenTypes = []string{"namespace", "type", "class", "enum", "interface", "struct", "typeParameter", "parameter", "variable", "property", "enumMember", "event", "function", "method", "keyword", "modifier", "comment", "string", "number", "operator", "decorator"}
var SemanticTokenModifiers = []string{"declaration", "static", "readonly", "documentation"}

var compilerKnownAttributes = map[string]bool{
	"address":       true,
	"interrupt":     true,
	"interruptSafe": true,
	"isr":           true,
	"noAlloc":       true,
	"noBlock":       true,
	"noCopy":        true,
	"noPanic":       true,
	"target":        true,
	"when":          true,
}

var tokenTypeIndex = func() map[string]int {
	result := map[string]int{}
	for i, name := range SemanticTokenTypes {
		result[name] = i
	}
	return result
}()

var tokenModifierIndex = func() map[string]uint32 {
	result := map[string]uint32{}
	for index, name := range SemanticTokenModifiers {
		result[name] = 1 << index
	}
	return result
}()

// SemanticTokens encodes lexical tokens using compiler-provided semantic names
// and parser-positioned contextual classifications.
//
// Rules:
//   - rules/tooling/lsp.md — "Semantic tokens"
//   - rules/foundations/lexical_structure.md — §23 tooling classification
func SemanticTokens(text, file string, classification map[string]string) []int {
	l := lexer.NewWithFile(text, file)
	sourceLines := physicalLines(text)
	data := []int{}
	previousLine, previousStart := 0, 0
	previousToken := lexer.Token{}
	for {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			break
		}
		kind, modifiers := tokenKind(token, classification)
		if token.Type == lexer.IDENT && previousToken.Type == lexer.AT {
			kind, modifiers = "decorator", 0
			if compilerKnownAttributes[token.Lexeme] {
				kind = "modifier"
			}
		}
		previousToken = token
		index, ok := tokenTypeIndex[kind]
		if !ok {
			continue
		}
		for _, segment := range semanticTokenSegments(token, sourceLines) {
			deltaStart := segment.start
			if segment.line == previousLine {
				deltaStart = segment.start - previousStart
			}
			data = append(data, segment.line-previousLine, deltaStart, segment.length, index, int(modifiers))
			previousLine, previousStart = segment.line, segment.start
		}
	}
	return data
}

type semanticTokenSegment struct {
	line   int
	start  int
	length int
}

// semanticTokenSegments converts one lexer token into protocol-valid,
// single-line UTF-16 segments. A block comment remains one compiler token but
// is deliberately represented by one semantic token on every non-empty
// physical line so coloring cannot disappear after its opening line.
//
// Rules:
//   - rules/tooling/lsp.md — "Semantic tokens"
//   - rules/tooling/lsp.md — "Shared diagnostic model", protocol position encoding
//   - rules/foundations/lexical_structure.md — §§2 and 5
func semanticTokenSegments(token lexer.Token, sourceLines []string) []semanticTokenSegment {
	line := max(token.Line-1, 0)
	start := semanticTokenStart(token, sourceLines)
	parts := physicalLines(token.Lexeme)
	segments := make([]semanticTokenSegment, 0, len(parts))
	for index, part := range parts {
		segmentStart := 0
		if index == 0 {
			segmentStart = start
		}
		length := utf16Length(part)
		if length == 0 {
			continue
		}
		segments = append(segments, semanticTokenSegment{
			line:   line + index,
			start:  segmentStart,
			length: length,
		})
	}
	return segments
}

func semanticTokenStart(token lexer.Token, sourceLines []string) int {
	line := max(token.Line-1, 0)
	scalarColumn := max(token.Column-1, 0)
	if line >= len(sourceLines) {
		return scalarColumn
	}
	runes := []rune(sourceLines[line])
	if scalarColumn > len(runes) {
		scalarColumn = len(runes)
	}
	return utf16Length(string(runes[:scalarColumn]))
}

func physicalLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

func utf16Length(text string) int {
	length := 0
	for _, current := range text {
		width := utf16.RuneLen(current)
		if width < 1 {
			width = 1
		}
		length += width
	}
	return length
}

// tokenKind gives compiler-provided position facts precedence over lexical
// categories so a hard token may receive its parser-confirmed contextual role
// without changing the lexer's global keyword inventory.
//
// Rules:
//   - rules/tooling/lsp.md — "Semantic tokens"
//   - rules/foundations/lexical_structure.md — §§10, 10.1, 22, and 23
func tokenKind(token lexer.Token, names map[string]string) (string, uint32) {
	if classification := names[ClassificationKey(token.File, token.Line, token.Column)]; classification != "" {
		return decodeClassification(classification)
	}
	switch token.Type {
	case lexer.COMMENT:
		if strings.HasPrefix(token.Lexeme, "/**") {
			return "comment", tokenModifierIndex["documentation"]
		}
		return "comment", 0
	case lexer.STRING, lexer.CHAR, lexer.RAW_STRING, lexer.INTERPSTRING:
		return "string", 0
	case lexer.INT, lexer.FLOAT:
		return "number", 0
	case lexer.IDENT, lexer.SELF:
		if classification := names[token.Lexeme]; classification != "" {
			return decodeClassification(classification)
		}
		return "variable", 0
	case lexer.ASSIGN, lexer.DECLARE, lexer.MOVE_ASSIGN, lexer.MOVE_DECLARE, lexer.ARROW, lexer.CONSUME_ARROW, lexer.PLUS, lexer.MINUS, lexer.ASTERISK, lexer.SLASH, lexer.PERCENT, lexer.INCREMENT, lexer.DECREMENT, lexer.PLUS_ASSIGN, lexer.MINUS_ASSIGN, lexer.ASTERISK_ASSIGN, lexer.SLASH_ASSIGN, lexer.PERCENT_ASSIGN, lexer.EQ, lexer.NEQ, lexer.LT, lexer.LTE, lexer.GT, lexer.GTE, lexer.AND, lexer.OR, lexer.NOT, lexer.BIT_AND, lexer.BIT_OR, lexer.BIT_XOR, lexer.BIT_NOT, lexer.SHIFT_LEFT, lexer.SHIFT_RIGHT, lexer.BIT_AND_ASSIGN, lexer.BIT_OR_ASSIGN, lexer.BIT_XOR_ASSIGN, lexer.SHIFT_LEFT_ASSIGN, lexer.SHIFT_RIGHT_ASSIGN, lexer.DOT, lexer.RANGE, lexer.RANGE_EXCLUSIVE, lexer.SPREAD, lexer.COLON:
		return "operator", 0
	case lexer.UNDERSCORE:
		return "keyword", 0
	case lexer.COMMA, lexer.SEMICOLON, lexer.QUESTION, lexer.AT, lexer.HASH, lexer.LPAREN, lexer.RPAREN, lexer.LBRACE, lexer.RBRACE, lexer.LBRACKET, lexer.RBRACKET:
		return "", 0
	default:
		return "keyword", 0
	}
}

func decodeClassification(classification string) (string, uint32) {
	parts := strings.Fields(classification)
	if len(parts) == 0 {
		return "", 0
	}
	var modifiers uint32
	for _, modifier := range parts[1:] {
		modifiers |= tokenModifierIndex[modifier]
	}
	return parts[0], modifiers
}

// ClassificationKey identifies one token independently of same-named symbols
// in other scopes.
func ClassificationKey(file string, line int, column int) string {
	return fmt.Sprintf("%s:%d:%d", file, line, column)
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
