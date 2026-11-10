package cst

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// applyForBindingSpacingRoles assigns whitespace ownership to parser-confirmed
// binding modes and separators; it does not classify expressions or parse a
// second for grammar. Comments and line breaks remain owned by the trivia tape.
// Rules: rules/control-flow/flowcontrol_for.md — §§4,6–7;
// rules/tooling/formatter.md — §§3(3–4),6,31(2,8).
func (d *Document) applyForBindingSpacingRoles(stmt *ast.ForStatement, indexes map[concreteTokenKey]int, mark func(lexer.Token, Role)) {
	for i, binding := range stmt.Bindings {
		first := binding.Token
		if binding.ModeToken.Type == lexer.REF {
			first = binding.ModeToken
			d.markBorrowKeywords(first, indexes, mark)
		}
		if i == 0 {
			continue
		}
		index, ok := indexes[keyForToken(first)]
		if !ok {
			continue
		}
		for previous := index - 1; previous >= 0; previous-- {
			element := d.Elements[previous]
			if element.Kind == Whitespace {
				continue
			}
			if element.Kind == Token && element.Token.Type == lexer.COMMA {
				mark(element.Token, SingleLineComma)
			}
			break
		}
	}
}
