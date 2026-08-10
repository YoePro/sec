// Package features contains protocol-independent LSP feature implementations.
package features

import (
	"fmt"
	"strings"

	"sec/internal/lexer"
)

var SemanticTokenTypes = []string{"namespace", "type", "class", "enum", "interface", "struct", "typeParameter", "parameter", "variable", "property", "enumMember", "event", "function", "method", "keyword", "modifier", "comment", "string", "number", "operator"}
var SemanticTokenModifiers = []string{"declaration", "static", "readonly"}

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

// SemanticTokens encodes lexical tokens using compiler-provided semantic names.
func SemanticTokens(text, file string, classification map[string]string) []int {
	l := lexer.NewWithFile(text, file)
	data := []int{}
	previousLine, previousStart := 0, 0
	previousToken := lexer.Token{}
	for {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			break
		}
		kind, modifiers := tokenKind(token, classification)
		if token.Type == lexer.IDENT && previousToken.Type == lexer.AT && compilerKnownAttributes[token.Lexeme] {
			kind = "modifier"
			modifiers = 0
		}
		previousToken = token
		index, ok := tokenTypeIndex[kind]
		if !ok {
			continue
		}
		line, start := max(token.Line-1, 0), max(token.Column-1, 0)
		length := tokenLength(token)
		if length <= 0 {
			continue
		}
		deltaStart := start
		if line == previousLine {
			deltaStart = start - previousStart
		}
		data = append(data, line-previousLine, deltaStart, length, index, int(modifiers))
		previousLine, previousStart = line, start
	}
	return data
}

func tokenKind(token lexer.Token, names map[string]string) (string, uint32) {
	switch token.Type {
	case lexer.COMMENT:
		return "comment", 0
	case lexer.STRING, lexer.CHAR, lexer.RAW_STRING, lexer.INTERPSTRING:
		return "string", 0
	case lexer.INT, lexer.FLOAT:
		return "number", 0
	case lexer.IDENT, lexer.SELF:
		if classification := names[ClassificationKey(token.File, token.Line, token.Column)]; classification != "" {
			return decodeClassification(classification)
		}
		if classification := names[token.Lexeme]; classification != "" {
			return decodeClassification(classification)
		}
		return "variable", 0
	case lexer.ASSIGN, lexer.DECLARE, lexer.MOVE_ASSIGN, lexer.MOVE_DECLARE, lexer.ARROW, lexer.PLUS, lexer.MINUS, lexer.ASTERISK, lexer.SLASH, lexer.PERCENT, lexer.PLUS_ASSIGN, lexer.MINUS_ASSIGN, lexer.ASTERISK_ASSIGN, lexer.SLASH_ASSIGN, lexer.PERCENT_ASSIGN, lexer.EQ, lexer.NEQ, lexer.LT, lexer.LTE, lexer.GT, lexer.GTE, lexer.AND, lexer.OR, lexer.NOT, lexer.BIT_AND, lexer.BIT_OR, lexer.BIT_XOR, lexer.BIT_NOT, lexer.SHIFT_LEFT, lexer.SHIFT_RIGHT, lexer.BIT_AND_ASSIGN, lexer.BIT_OR_ASSIGN, lexer.BIT_XOR_ASSIGN, lexer.SHIFT_LEFT_ASSIGN, lexer.SHIFT_RIGHT_ASSIGN, lexer.DOT, lexer.RANGE, lexer.RANGE_EXCLUSIVE, lexer.SPREAD, lexer.COLON:
		return "operator", 0
	case lexer.COMMA, lexer.SEMICOLON, lexer.QUESTION, lexer.UNDERSCORE, lexer.AT, lexer.HASH, lexer.LPAREN, lexer.RPAREN, lexer.LBRACE, lexer.RBRACE, lexer.LBRACKET, lexer.RBRACKET:
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
func tokenLength(t lexer.Token) int {
	if strings.Contains(t.Lexeme, "\n") {
		return len([]rune(strings.Split(t.Lexeme, "\n")[0]))
	}
	return len([]rune(t.Lexeme))
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
