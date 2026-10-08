// Package readiness shares prerequisites for the legacy AST backends.
package readiness

import (
	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
)

// RejectInstant prevents legacy lowering from inventing storage for the opaque
// monotonic identity, including nested and unused source declarations.
// Rules: rules/concurrency/mutex.md §13; rules/types/temporal.md §4;
// rules/compiler/compiler_pipeline.md — lowering prerequisites.
func RejectInstant(program *ast.Program) error {
	declarations := map[*ast.Identifier]bool{}
	_ = astwalk.Inspect(program, func(node any) error {
		if decl, ok := node.(*ast.TypeDeclStatement); ok {
			declarations[decl.Name] = true
		}
		return nil
	})
	return astwalk.Inspect(program, func(node any) error {
		var token lexer.Token
		found := false
		switch node := node.(type) {
		case *ast.TypeReference:
			token, found = node.Token, node.Name == "Instant"
		case *ast.Identifier:
			token, found = node.Token, node.Value == "Instant" && !declarations[node]
		}
		if found {
			return &semantic.UnsupportedFeatureError{Feature: "Instant runtime representation is unsupported by legacy lowering", Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}}
		}
		return nil
	})
}
