package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// statementToken projects statement source identity for semantic passes and
// diagnostics, including test invocations. Tests retain ordinary file privacy
// and loader-proven core authority rather than an anonymous source identity.
// Rules: rules/tooling/testing.md — §§4,7.1–7.4;
// rules/foundations/names_scopes_visibility.md — §§12.2–12.3;
// rules/library/core-library.md — §1.2.
func statementToken(stmt ast.Statement) lexer.Token {
	switch stmt := stmt.(type) {
	case *ast.CAliasDeclaration:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.HardwareDeclaration:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ModuleStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ImportStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.TypeDeclStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.EnumDeclaration:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ImplStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.TestDeclaration:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.FunctionDeclaration:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.StructStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.LetStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.LetGroupStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.AssignmentStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.TryAssignmentStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.DeferStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.DiscardStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.AssertStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.PanicStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.UnreachableStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.JoinStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.DetachStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.CancelStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ExpressionStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ReturnStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.IfStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ForStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.WhileStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.SwitchStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.SelectStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.FallthroughStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.BreakStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ContinueStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.UnsafeStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.AsmStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.MatchStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.CommentStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.InvalidStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.InvalidDeclaration:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	default:
		return lexer.Token{}
	}
}
