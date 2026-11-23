package cst

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// AttributeArgumentListOpen identifies a parser-owned attribute argument list
// whose layout follows ordinary calls (formatter.md §16(16)).
const AttributeArgumentListOpen Role = "attribute-argument-list-open"

// applyAttributeRoles projects every supported attribute attachment onto real
// tokens, retaining order, complete argument groups and the actual declaration
// prefix (including unsafe/extern/static). Incomplete lists get no layout roles.
// Rules: rules/tooling/formatter.md §§5,16(14–17),22(5),25;
// rules/foundations/attributes.md — Attribute attachment, Comments and whitespace.
func (d *Document) applyAttributeRoles(node any, indexes map[concreteTokenKey]int) {
	var attributes []*ast.Attribute
	standalone := false
	if statement, ok := node.(ast.Statement); ok {
		attributes = ast.StatementAttributes(statement)
	}
	switch n := node.(type) {
	case *ast.InitDeclaration:
		attributes = n.Attributes
	case *ast.FreeDeclaration:
		attributes = n.Attributes
	case *ast.PropertyDeclaration:
		attributes = n.GetterAttributes
	case *ast.PropertySetter:
		attributes = n.Attributes
	case *ast.LetStatement:
		if lambda, ok := n.Value.(*ast.LambdaExpression); ok {
			attributes = append(append([]*ast.Attribute(nil), attributes...), lambda.Attributes...)
		}
	case *ast.TargetDirective:
		if n.Canonical && n.Attribute != nil {
			attributes = []*ast.Attribute{n.Attribute}
			standalone = true
		}
	}
	attributes = append([]*ast.Attribute(nil), attributes...)
	sort.SliceStable(attributes, func(i, j int) bool {
		if attributes[i] == nil {
			return false
		}
		if attributes[j] == nil {
			return true
		}
		return attributes[i].Token.ByteStart < attributes[j].Token.ByteStart
	})
	mark := func(index int, role Role) { d.Elements[index].Roles = appendUniqueRole(d.Elements[index].Roles, role) }
	for _, attribute := range attributes {
		if attribute == nil || attribute.Name == nil {
			continue
		}
		end, found := indexes[keyForToken(attribute.Name.Token)]
		if !found {
			continue
		}
		if attribute.ArgumentsOpen.Type != lexer.LPAREN {
			// A rejected argument list on an argument-free attribute has no
			// parser-owned delimiters; it must not be split after the name.
			unexpected := false
			for next := end + 1; next < len(d.Elements); next++ {
				if d.Elements[next].Kind == Token {
					unexpected = d.Elements[next].Token.Type == lexer.LPAREN
					break
				}
			}
			if unexpected {
				continue
			}
		}
		if attribute.ArgumentsOpen.Type == lexer.LPAREN {
			open, found := indexes[keyForToken(attribute.ArgumentsOpen)]
			close, complete := indexes[keyForToken(attribute.ArgumentsClose)]
			if !found || !complete || attribute.ArgumentsClose.Type != lexer.RPAREN {
				continue
			}
			// Both real parser tokens must belong to the same lossless group.
			matched := false
			for _, group := range d.Groups {
				if group.Open == open && group.Close == close {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			mark(open, AttributeArgumentListOpen)
			mark(open, CommaListOpen)
			mark(close, CommaListClose)
			end = close
		}
		if standalone {
			continue
		}
		for next := end + 1; next < len(d.Elements); next++ {
			if d.Elements[next].Kind != Token {
				continue
			}
			mark(end, AttachedAttributeEnd)
			mark(next, AttributedDeclarationStart)
			break
		}
	}
}
