package cst

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// applyHeaderSpacingRoles completes header spacing from parser-owned module,
// callable-modifier and register-modifier facts, preserving comments and line
// boundaries. Ordinary expressions and unaccepted modifiers receive no roles.
// Rules: rules/tooling/formatter.md — §5(2–6),§6,§16(18–19),§22(1);
// rules/declarations/interfaces.md — §3 receiver/static requirements;
// rules/declarations/registers.md — register allocation and byte-order modifiers.
func (d *Document) applyHeaderSpacingRoles(node any, indexes map[concreteTokenKey]int, mark func(lexer.Token, Role)) {
	switch node := node.(type) {
	case *ast.ModuleStatement:
		if node.Path != "" && node.Path == node.NameToken.Lexeme {
			mark(node.Token, SpacedKeyword)
		}
	case *ast.LetStatement:
		if !node.Static || node.Name == nil || node.Token.Type != lexer.STATIC {
			return
		}
		start, ok := indexes[keyForToken(node.Token)]
		if !ok {
			return
		}
		for index := start; index < len(d.Elements); index++ {
			element := d.Elements[index]
			if element.Kind == Whitespace {
				continue
			}
			if element.Kind != Token || (element.Token.Type != lexer.STATIC && element.Token.Type != lexer.LET && element.Token.Type != lexer.MUT) {
				break
			}
			mark(element.Token, SpacedKeyword)
		}
	case *ast.FunctionDeclaration:
		if node.Name == nil {
			return
		}
		start, ok := indexes[keyForToken(node.Token)]
		if !ok {
			return
		}
		end, ok := indexes[keyForToken(node.Name.Token)]
		if !ok {
			return
		}
		// Prefixes kept separately as receiver/static facts still belong to
		// this exact function, not to a neighboring declaration.
		for previous := start - 1; previous >= 0; previous-- {
			element := d.Elements[previous]
			if element.Kind == Whitespace {
				continue
			}
			if element.Kind == Token && ((node.Static && element.Token.Type == lexer.STATIC) ||
				(node.ReceiverCapability == ast.ReceiverMutable && element.Token.Type == lexer.MUT) ||
				(node.ReceiverCapability == ast.ReceiverConsuming && element.Token.Type == lexer.CONSUME_ARROW)) {
				mark(element.Token, SpacedKeyword)
			}
			break
		}
		for index := start; index < end; index++ {
			element := d.Elements[index]
			if element.Kind == Token {
				mark(element.Token, SpacedKeyword)
			}
		}
	case *ast.RegisterType:
		for _, modifier := range node.ModifierTokens {
			start, foundStart := indexes[keyForToken(modifier.Start)]
			end, foundEnd := indexes[keyForToken(modifier.End)]
			if !foundStart || !foundEnd || end <= start {
				continue
			}
			mark(modifier.Start, SpacedBefore)
			mark(modifier.End, SpacedKeyword)
			for index := start + 1; index < end; index++ {
				element := d.Elements[index]
				if element.Kind == Token && element.Token.Type == lexer.MINUS {
					mark(element.Token, HeaderCompoundHyphen)
				}
			}
		}
	}
}
