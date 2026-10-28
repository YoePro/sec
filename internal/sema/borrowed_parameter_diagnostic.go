package sema

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/diagnostics"
)

// reportMoveIntoBorrowedParameter explains the selected borrow contract without
// validating or committing a move. The suggested call removes only this marker
// in a detached AST copy; it is advice, never an automatic API signature edit.
// Real parameter declarations are related when available; builtin-only analysis
// has no source declaration to invent.
// Rules: rules/declarations/functions.md — §§7,9–10;
// rules/memory/borrowing.md — §15; rules/memory/copy_move.md — §28.1;
// rules/tooling/diagnostics.md — §§8–9.
func (a *Analyzer) reportMoveIntoBorrowedParameter(move *ast.PrefixExpression, parameter FunctionParameter, function Function, call *ast.CallExpression) {
	source := move.Right.String()
	borrowedType := typeDisplayName(parameter.Type)
	if parameter.Type.Kind != ReferenceType {
		if parameter.MutableRef {
			borrowedType = "ref mut " + borrowedType
		} else {
			borrowedType = "ref " + borrowedType
		}
	}
	suggestion := function.Name + "(" + source + ")"
	if call != nil {
		replacement := *call
		replacement.Arguments = append([]ast.Expression(nil), call.Arguments...)
		for i, arg := range replacement.Arguments {
			if arg == move {
				replacement.Arguments[i] = move.Right
			}
		}
		suggestion = replacement.String()
	}
	token := move.Token
	endLine, endColumn := token.EndPosition()
	_ = walk.Inspect(move.Right, func(node any) error {
		if expr, ok := node.(ast.Expression); ok {
			t := expressionToken(expr)
			line, column := t.EndPosition()
			if t.File == token.File && (line > endLine || line == endLine && column > endColumn) {
				endLine, endColumn = line, column
			}
		}
		return nil
	})
	err := Error{ID: diagnostics.MoveIntoBorrowedParameter, Severity: diagnostics.SeverityError,
		Message: fmt.Sprintf("Cannot move '%s' into borrowed parameter '%s'.\n\nParameter '%s' requires '%s', which borrows the value without transferring ownership.", source, parameter.Name, parameter.Name, borrowedType),
		Help:    fmt.Sprintf("Use '%s' to borrow the value, or change the parameter to an owning type if ownership transfer is intended.", suggestion),
		File:    token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn,
	}
	declaration := parameter.Token
	if declaration.Line <= 0 {
		declaration = function.Token
	}
	if declaration.Line > 0 {
		line, column := declaration.EndPosition()
		err.Related = []RelatedLocation{{File: declaration.File, Line: declaration.Line, Column: declaration.Column, EndLine: line, EndColumn: column, Label: "borrowed parameter declared here"}}
	}
	a.appendError(err)
}
