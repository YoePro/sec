package cst

import (
	"reflect"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// ExpressionContinuation connects the first concrete element of a continuation
// line to its parser-owned return/if keyword. Delimiter nesting is resolved here,
// so consumers need not reconstruct expressions from line spellings.
// Rules: rules/tooling/formatter.md — §5, §6(3), §9(15).
type ExpressionContinuation struct{ Keyword, Element int }

// applyExpressionContinuations records inline-start return/if continuations outside
// groups opened on the keyword line, or within groups opened by a later
// continuation. Existing delimiter indentation is thereby counted only once.
// Rules: rules/tooling/formatter.md — §6(3), §9(15), §11(3), §25.
func (d *Document) applyExpressionContinuations(anchor lexer.Token, expression ast.Expression, indexes map[concreteTokenKey]int) {
	keyword, ok := indexes[keyForToken(anchor)]
	if !ok || expression == nil {
		return
	}
	first := keyword + 1
	for first < len(d.Elements) && (d.Elements[first].Kind == Whitespace || d.Elements[first].Kind == Comment) {
		first++
	}
	if first == len(d.Elements) || d.Elements[first].Kind != Token || d.Elements[first].Token.Line != anchor.Line {
		return
	}
	end := continuationExpressionTokenEnd(reflect.ValueOf(expression))
	// AST expressions do not all retain their real closing delimiters. The
	// lossless hierarchy supplies them, excluding enclosing statement blocks.
	last := first
	for _, group := range d.Groups {
		if group.Close >= 0 && group.Open >= first && d.Elements[group.Open].Span.Start < end && d.Elements[group.Close].Span.End > end {
			end = d.Elements[group.Close].Span.End
		}
	}
	for last+1 < len(d.Elements) && d.Elements[last+1].Span.Start < end {
		last++
	}
	line := anchor.Line
	for index := first; index <= last; index++ {
		element := d.Elements[index]
		if element.Kind != Token && element.Kind != Comment {
			continue
		}
		startsLine := element.Token.Line > line
		endLine, _ := element.Token.EndPosition()
		if endLine > line {
			line = endLine
		}
		if !startsLine {
			continue
		}
		outer := -1
		for g, group := range d.Groups {
			if group.Open >= first && group.Open <= index && group.Close >= index && (outer < 0 || group.Open < d.Groups[outer].Open) {
				outer = g
			}
		}
		if outer >= 0 && d.Elements[d.Groups[outer].Open].Token.Line == anchor.Line {
			continue
		}
		d.ExpressionContinuations = append(d.ExpressionContinuations, ExpressionContinuation{keyword, index})
	}
}

// continuationExpressionTokenEnd finds the final real AST-owned token in an expression;
// delimiter completion remains owned by the lossless CST hierarchy.
// Rules: rules/tooling/formatter.md — §5(2–6), §6(3).
func continuationExpressionTokenEnd(value reflect.Value) int {
	if !value.IsValid() {
		return 0
	}
	if value.Type() == reflect.TypeOf(lexer.Token{}) {
		return value.Interface().(lexer.Token).ByteEnd
	}
	end := 0
	visit := func(child reflect.Value) {
		if next := continuationExpressionTokenEnd(child); next > end {
			end = next
		}
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if !value.IsNil() {
			visit(value.Elem())
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).IsExported() {
				visit(value.Field(i))
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			visit(value.Index(i))
		}
	}
	return end
}
