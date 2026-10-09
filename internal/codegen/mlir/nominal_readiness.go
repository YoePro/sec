package mlir

import (
	"sec/internal/ast"
	"sec/internal/ir/semantic"
)

// validateNominalSignature stops unresolved source value types from using the
// legacy mapper's void sentinel as a physical representation. A void return is
// permitted only for an actual void signature. Explicit reference parameters
// retain their separately supported pointer representation.
// Rules: rules/types/types.md — Type identity, void;
// rules/compiler/compiler_pipeline.md — lowering prerequisites;
// rules/compiler/semantic_ir.md — source validity versus unsupported lowering.
func (g *Generator) validateNominalSignature(fn *ast.FunctionDeclaration) error {
	for _, parameter := range fn.Parameters {
		if parameter.Type != nil && g.mlirParameterType(parameter) == "void" {
			token := parameter.Type.Token
			return &semantic.UnsupportedFeatureError{Feature: "legacy MLIR parameter representation for " + parameter.Type.Name, Package: 3, Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}}
		}
	}
	if fn.ReturnType != nil && fn.ReturnType.Name != "void" && g.mlirType(fn.ReturnType) == "void" {
		token := fn.ReturnType.Token
		return &semantic.UnsupportedFeatureError{Feature: "legacy MLIR return representation for " + fn.ReturnType.Name, Package: 3, Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}}
	}
	return nil
}
