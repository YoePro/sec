package parser

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// recordSourceToken retains each lexed source token once even when speculative
// parsing restores and rereads lexer state.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §5.5 "Comment preservation"
//   - rules/tooling/formatter.md — Appendix A.5 "Build lossless syntax and trivia support"
func (p *Parser) recordSourceToken(token lexer.Token) {
	key := fmt.Sprintf("%s:%d:%d:%s:%s", token.File, token.Line, token.Column, token.Type, token.Lexeme)
	if _, exists := p.sourceTokenKeys[key]; exists {
		return
	}
	p.sourceTokenKeys[key] = struct{}{}
	p.sourceTokens = append(p.sourceTokens, token)
}

// buildCommentAttachments classifies preserved comment groups from exact token
// positions and blank-line boundaries. It never derives attachment from
// formatter-trimmed lines.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§5.4–5.5
//   - rules/tooling/formatter.md — "Comment attachment"
func buildCommentAttachments(source []lexer.Token, program *ast.Program) []ast.CommentAttachment {
	tokens := append([]lexer.Token(nil), source...)
	sort.SliceStable(tokens, func(left, right int) bool {
		if tokens[left].File != tokens[right].File {
			return tokens[left].File < tokens[right].File
		}
		if tokens[left].Line != tokens[right].Line {
			return tokens[left].Line < tokens[right].Line
		}
		return tokens[left].Column < tokens[right].Column
	})
	targets := commentTargetsByStart(program)
	attachments := []ast.CommentAttachment{}
	for index := 0; index < len(tokens); {
		if tokens[index].Type != lexer.COMMENT {
			index++
			continue
		}
		comment := tokens[index]
		previous := previousNonCommentToken(tokens, index)
		if previous.Type != "" && previous.Line == comment.Line {
			placement := ast.CommentInline
			if strings.HasPrefix(comment.Lexeme, "//") {
				placement = ast.CommentTrailing
			}
			attachments = append(attachments, ast.CommentAttachment{
				Comments:  []lexer.Token{comment},
				Placement: placement,
				Anchor:    previous,
			})
			index++
			continue
		}

		end := index + 1
		for end < len(tokens) && tokens[end].Type == lexer.COMMENT && tokens[end].Line > comment.Line {
			end++
		}
		group := append([]lexer.Token(nil), tokens[index:end]...)
		next := lexer.Token{}
		if end < len(tokens) {
			next = tokens[end]
		}
		placement := ast.CommentDetached
		var target ast.Node
		if next.Type != lexer.EOF && next.Type != "" && next.Line == tokenEndLine(group[len(group)-1])+1 {
			placement = ast.CommentLeading
			if allDocumentationComments(group) {
				placement = ast.CommentDocumentation
			}
			target = targets[sourceTokenKey(next)]
		}
		attachments = append(attachments, ast.CommentAttachment{
			Comments:  group,
			Placement: placement,
			Anchor:    next,
			Target:    target,
		})
		index = end
	}
	return attachments
}

// previousNonCommentToken returns the nearest lexical anchor before a comment.
func previousNonCommentToken(tokens []lexer.Token, before int) lexer.Token {
	for index := before - 1; index >= 0; index-- {
		if tokens[index].Type != lexer.COMMENT {
			return tokens[index]
		}
	}
	return lexer.Token{}
}

// allDocumentationComments reports whether every comment in an attached group
// has the canonical documentation prefix.
func allDocumentationComments(comments []lexer.Token) bool {
	if len(comments) == 0 {
		return false
	}
	for _, comment := range comments {
		if !strings.HasPrefix(comment.Lexeme, "/**") {
			return false
		}
	}
	return true
}

// commentTargetsByStart indexes declaration and statement nodes by their
// parser-owned starting token for source-to-source consumers.
func commentTargetsByStart(program *ast.Program) map[string]ast.Node {
	targets := map[string]ast.Node{}
	var visit func(reflect.Value)
	visit = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		if value.Kind() == reflect.Interface {
			if !value.IsNil() {
				visit(value.Elem())
			}
			return
		}
		if value.Kind() == reflect.Pointer {
			if value.IsNil() || value.Type().Elem().PkgPath() != "sec/internal/ast" {
				return
			}
			if node, ok := value.Interface().(ast.Node); ok {
				_, statement := node.(ast.Statement)
				if !statement && !isDocumentableDeclaration(node) {
					visit(value.Elem())
					return
				}
				field := value.Elem().FieldByName("Token")
				if field.IsValid() && field.CanInterface() {
					if token, ok := field.Interface().(lexer.Token); ok && token.Type != "" {
						targets[sourceTokenKey(token)] = node
					}
				}
			}
			visit(value.Elem())
			return
		}
		switch value.Kind() {
		case reflect.Struct:
			if value.Type().PkgPath() != "sec/internal/ast" {
				return
			}
			for index := 0; index < value.NumField(); index++ {
				visit(value.Field(index))
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				visit(value.Index(index))
			}
		}
	}
	visit(reflect.ValueOf(program))
	return targets
}

func sourceTokenKey(token lexer.Token) string {
	return fmt.Sprintf("%s:%d:%d", token.File, token.Line, token.Column)
}
