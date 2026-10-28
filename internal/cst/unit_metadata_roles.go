package cst

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// applyUnitMetadataRoles projects the parser's line-bounded metadata tokens
// onto concrete spacing roles. Qualified unit impls need no local declaration;
// legacy impl Name retains the existing same-file unit-name check. No metadata
// values, enum aliases or exact numeric expressions are evaluated or rewritten.
// Rules: rules/foundations/grammar.md — "Unit metadata inside impl";
// rules/tooling/formatter.md §§6(4–6),22(11–13); rules/types/units.md — "Formatter requirements".
func (d *Document) applyUnitMetadataRoles(program *ast.Program, indexes map[concreteTokenKey]int, mark func(lexer.Token, Role)) {
	unitNames := map[string]bool{}
	for _, statement := range program.Statements {
		if declaration, ok := statement.(*ast.UnitDeclStatement); ok && declaration.Name != nil {
			unitNames[declaration.Name.Value] = true
		}
	}
	for _, statement := range program.Statements {
		implementation, ok := statement.(*ast.ImplStatement)
		if !ok || implementation.Target == nil {
			continue
		}
		target := implementation.Target
		namedUnit := target.UnitOnly && target.UnitExpression != nil && target.UnitExpression.Kind == ast.UnitExpressionName
		if !namedUnit && !unitNames[target.Name] {
			continue
		}
		for _, member := range implementation.Members {
			metadata, ok := member.(*ast.UnitMetadataDeclaration)
			if !ok {
				continue
			}
			mark(metadata.Token, UnitMetadataName)
			if index, ok := indexes[keyForToken(metadata.Token)]; ok {
				for next := index + 1; next < len(d.Elements); next++ {
					element := d.Elements[next]
					if element.Kind == Whitespace || element.Kind == Comment {
						continue
					}
					if element.Kind == Token && element.Token.Type == lexer.COLON {
						mark(element.Token, DeclarationColon)
					}
					break
				}
			}
			key := strings.ReplaceAll(strings.ToLower(metadata.Name), "_", "")
			values := make([]lexer.Token, 0, len(metadata.Value))
			for _, token := range metadata.Value {
				if token.Type != lexer.COMMENT {
					values = append(values, token)
				}
			}
			for i, token := range values {
				if key == "dimension" && token.Type == lexer.BIT_XOR {
					mark(token, UnitMetadataCompactToken)
				}
				// Only identifier qualification can be compacted. In particular,
				// an invalid numeric token sequence 1 . 5 must never become 1.5.
				if token.Type == lexer.DOT && i > 0 && i+1 < len(values) && values[i-1].Type == lexer.IDENT && values[i+1].Type == lexer.IDENT {
					switch key {
					case "kind", "system", "origin", "transform", "status", "dimension":
						mark(token, UnitMetadataCompactToken)
					}
				}
			}
			switch key {
			case "scale", "offset", "logbase", "logfactor", "reference":
				// The Reference suffix is a named unit, never a comparison expression.
				if n := len(values); key == "reference" && n >= 4 && values[n-3].Type == lexer.LT && values[n-2].Type == lexer.IDENT && values[n-1].Type == lexer.GT {
					mark(values[n-3], UnitMetadataCompactToken)
					mark(values[n-1], UnitMetadataCompactToken)
					values = values[:n-3]
				}
				markUnitMetadataNumericSpacing(values, mark)
			case "dimension":
				for _, token := range values {
					if token.Type == lexer.MINUS || token.Type == lexer.PLUS {
						mark(token, PrefixOperator)
					}
				}
			}
		}
	}
}

// markUnitMetadataNumericSpacing distinguishes sign attachment from binary
// whitespace in the parser-owned numeric token list, without interpreting
// precedence, computing a value, folding parentheses or changing any token.
// Rules: rules/types/units.md — "Unit metadata" exact numeric expressions;
// rules/tooling/formatter.md §§6(5–6),22(11–13); MD-062 §12(10).
func markUnitMetadataNumericSpacing(tokens []lexer.Token, mark func(lexer.Token, Role)) {
	operand := true
	for _, token := range tokens {
		switch token.Type {
		case lexer.PLUS, lexer.MINUS:
			if operand {
				mark(token, PrefixOperator)
			} else {
				mark(token, BinaryOperator)
			}
			operand = true
		case lexer.ASTERISK, lexer.SLASH, lexer.BIT_XOR:
			mark(token, BinaryOperator)
			operand = true
		case lexer.LPAREN:
			operand = true
		case lexer.INT, lexer.FLOAT, lexer.RPAREN:
			operand = false
		}
	}
}
