package readiness

import (
	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
)

// RejectPlatformFloat rejects native float consumers when a legacy backend
// cannot preserve the selected platform's binary floating-point semantics.
// All nested type sites and family literals are checked before publishing IR.
// Explicit float32/float64 signatures remain available; family literals require
// resolved context facts which the AST-only readiness check does not possess.
// Rules: rules/types/types.md — "Binary floating-point types";
// rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md §6.
func RejectPlatformFloat(program *ast.Program, backend string) error {
	return astwalk.Inspect(program, func(node any) error {
		var token lexer.Token
		feature := ""
		switch node := node.(type) {
		case *ast.TypeReference:
			if node.Name == "float" {
				token, feature = node.Token, "platform-sized float is unsupported by legacy "+backend+" lowering"
			}
		case *ast.Identifier:
			// A primitive conversion can be parsed as a call without a type reference.
			if node.Value == "float" {
				token, feature = node.Token, "platform-sized float is unsupported by legacy "+backend+" lowering"
			}
		case *ast.FloatLiteral:
			if node.Suffix() == "g" {
				token, feature = node.Token, "binary float literals require resolved scalar facts; legacy "+backend+" lowering"
			}
		case *ast.IntegerLiteral:
			if node.Suffix() == "g" {
				token, feature = node.Token, "binary float literals require resolved scalar facts; legacy "+backend+" lowering"
			}
		}
		if feature != "" {
			return &semantic.UnsupportedFeatureError{Feature: feature, Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}}
		}
		return nil
	})
}
