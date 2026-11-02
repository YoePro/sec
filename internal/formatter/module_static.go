package formatter

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// formatModuleStatic removes only the real static modifier of a parser-confirmed
// top-level let declaration. Scope comes from the AST rather than indentation;
// comments, literals and required impl/local modifiers are never candidates.
// Any parse error retains the source's modifiers for safe recovery formatting.
// Rules: rules/declarations/static.md — §§3,25;
// rules/tooling/formatter.md — §§6,25,29 (trivia and malformed-source preservation).
func formatModuleStatic(text string) string {
	if !strings.Contains(text, "static") {
		return text
	}
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 || program == nil {
		return text
	}
	removals := []formatterReplacement{}
	for _, statement := range program.Statements {
		var token lexer.Token
		switch statement := statement.(type) {
		case *ast.LetStatement:
			if !statement.Static {
				continue
			}
			token = statement.Token
		case *ast.LetGroupStatement:
			token = statement.Token
		default:
			continue
		}
		if token.Type != lexer.STATIC || token.ByteStart < 0 || token.ByteEnd > len(text) || token.ByteEnd <= token.ByteStart || text[token.ByteStart:token.ByteEnd] != "static" {
			continue
		}
		end := token.ByteEnd
		for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
			end++
		}
		removals = append(removals, formatterReplacement{start: token.ByteStart, end: end})
	}
	sort.Slice(removals, func(i, j int) bool { return removals[i].start > removals[j].start })
	for _, removal := range removals {
		text = text[:removal.start] + text[removal.end:]
	}
	return text
}
