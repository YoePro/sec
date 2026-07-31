// Package features contains protocol-independent LSP feature implementations.
package features

import (
	"strings"

	"sec/internal/lexer"
)

var SemanticTokenTypes = []string{"namespace", "type", "class", "enum", "interface", "struct", "typeParameter", "parameter", "variable", "property", "enumMember", "event", "function", "method", "keyword", "modifier", "comment", "string", "number", "operator"}
var SemanticTokenModifiers = []string{"declaration", "static"}

var tokenTypeIndex = func() map[string]int {
	result := map[string]int{}
	for i, name := range SemanticTokenTypes {
		result[name] = i
	}
	return result
}()

// SemanticTokens encodes lexical tokens using compiler-provided semantic names.
func SemanticTokens(text, file string, classification map[string]string) []int {
	l := lexer.NewWithFile(text, file)
	data := []int{}
	previousLine, previousStart := 0, 0
	for {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			break
		}
		kind := tokenKind(token, classification)
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
		data = append(data, line-previousLine, deltaStart, length, index, 0)
		previousLine, previousStart = line, start
	}
	return data
}

func tokenKind(token lexer.Token, names map[string]string) string {
	switch token.Type {
	case lexer.COMMENT:
		return "comment"
	case lexer.STRING, lexer.CHAR, lexer.RAW_STRING, lexer.INTERPSTRING:
		return "string"
	case lexer.INT, lexer.FLOAT:
		return "number"
	case lexer.IDENT, lexer.SELF:
		if kind := names[token.Lexeme]; kind != "" {
			return kind
		}
		return "variable"
	case lexer.ASSIGN, lexer.DECLARE, lexer.MOVE_ASSIGN, lexer.MOVE_DECLARE, lexer.ARROW, lexer.PLUS, lexer.MINUS, lexer.ASTERISK, lexer.SLASH, lexer.PERCENT, lexer.PLUS_ASSIGN, lexer.MINUS_ASSIGN, lexer.ASTERISK_ASSIGN, lexer.SLASH_ASSIGN, lexer.PERCENT_ASSIGN, lexer.EQ, lexer.NEQ, lexer.LT, lexer.LTE, lexer.GT, lexer.GTE, lexer.AND, lexer.OR, lexer.NOT, lexer.BIT_AND, lexer.BIT_OR, lexer.BIT_XOR, lexer.BIT_NOT, lexer.SHIFT_LEFT, lexer.SHIFT_RIGHT, lexer.BIT_AND_ASSIGN, lexer.BIT_OR_ASSIGN, lexer.BIT_XOR_ASSIGN, lexer.SHIFT_LEFT_ASSIGN, lexer.SHIFT_RIGHT_ASSIGN, lexer.DOT, lexer.RANGE, lexer.RANGE_EXCLUSIVE, lexer.SPREAD, lexer.COLON:
		return "operator"
	case lexer.COMMA, lexer.SEMICOLON, lexer.QUESTION, lexer.UNDERSCORE, lexer.AT, lexer.HASH, lexer.LPAREN, lexer.RPAREN, lexer.LBRACE, lexer.RBRACE, lexer.LBRACKET, lexer.RBRACKET:
		return ""
	default:
		return "keyword"
	}
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
