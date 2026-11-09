package cst

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// applyOwnershipSpacingRoles attaches consuming parameters and move captures
// to their parsed binding and retains the capture/lambda boundary.
// It grants no role to legacy or unparsed markers.
// Rules: rules/tooling/formatter.md — §§21(2–7),31(8);
// rules/corrections/applied/formatter-ownership-v2-correction-20260826.md — "Correction".
func (d *Document) applyOwnershipSpacingRoles(node any, indexes map[concreteTokenKey]int, mark func(lexer.Token, Role)) {
	switch node := node.(type) {
	case *ast.ReturnStatement:
		// rules/declarations/functions.md §14; rules/tooling/formatter.md
		// §21(5): preserve the optional marker and attach it to its operand.
		if node.Value != nil && node.MoveToken.Type == lexer.MOVE_ASSIGN {
			mark(node.MoveToken, PrefixOperator)
		}
	case *ast.Parameter:
		if !node.Consuming || node.Name == nil || node.Type == nil || node.Type.Invalid {
			return
		}
		index, ok := indexes[keyForToken(node.Name.Token)]
		if !ok {
			return
		}
		for previous := index - 1; previous >= 0; previous-- {
			element := d.Elements[previous]
			if element.Kind == Whitespace {
				continue
			}
			if element.Kind == Token && element.Token.Type == lexer.CONSUME_ARROW {
				mark(element.Token, PrefixOperator)
			}
			break
		}
	case *ast.RefExpression:
		// `ref value` and `ref mut value` keep one space between their words
		// (rules/tooling/formatter.md §§ 6(6), 21; borrowing syntax).
		if node.Token.Type == lexer.REF {
			d.markBorrowKeywords(node.Token, indexes, mark)
		}
	case *ast.MatchPatternBinding:
		if node.Mode != ast.MatchBindingValue && node.Token.Type == lexer.REF {
			d.markBorrowKeywords(node.Token, indexes, mark)
		}
	case *ast.LambdaExpression:
		// A capture list stays attached to capture and separated from fn;
		// the lambda return type follows its parameter list with one space.
		// Rules: rules/tooling/formatter.md — §16(10–13).
		if node.CaptureOpen.Type == lexer.LPAREN {
			mark(node.Token, SpacedBefore)
		}
		if node.ReturnType != nil && !node.ReturnType.Invalid {
			mark(typeReferenceFirstToken(node.ReturnType), SpacedBefore)
		}
		for _, capture := range node.Captures {
			if capture.Mode == ast.LambdaCaptureMove && capture.Name != nil && capture.Token.Type == lexer.MOVE_ASSIGN {
				mark(capture.Token, PrefixOperator)
			}
			// `ref name` and `ref mut name` keep exactly one space between
			// their words (lambda-functions.md §15; formatter.md §16(11)).
			if capture.IsBorrow() && capture.Name != nil && capture.Token.Type == lexer.REF {
				mark(capture.Token, SpacedKeyword)
				if capture.MutToken.Type == lexer.MUT {
					mark(capture.MutToken, SpacedKeyword)
				}
			}
		}
	}
}

// markBorrowKeywords marks a parsed `ref` keyword and a directly following
// `mut` on the same line as spaced keywords, so `ref   mut  x` formats as
// `ref mut x`. Comments between the words keep their source spacing.
func (d *Document) markBorrowKeywords(ref lexer.Token, indexes map[concreteTokenKey]int, mark func(lexer.Token, Role)) {
	index, ok := indexes[keyForToken(ref)]
	if !ok {
		return
	}
	mark(ref, SpacedKeyword)
	for next := index + 1; next < len(d.Elements); next++ {
		element := d.Elements[next]
		if element.Kind == Whitespace {
			continue
		}
		if element.Kind == Token && element.Token.Type == lexer.MUT && element.Token.Line == ref.Line {
			mark(element.Token, SpacedKeyword)
		}
		return
	}
}
