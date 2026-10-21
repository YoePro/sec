package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// deferredSliceCapability names unsupported operations using the receiver's
// resolved type and compiler-known identity, before generic expression lowering
// can report an unrelated missing member/call. It never grants representation
// authority or interprets a user method from its spelling.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§71,73–74,142;
// rules/compiler/compiler_pipeline.md — §§32–34.
func (b *builder) deferredSliceCapability(expr ast.Expression) (string, bool) {
	var member *ast.MemberExpression
	switch expr := expr.(type) {
	case *ast.MemberExpression:
		member = expr
	case *ast.CallExpression:
		member, _ = expr.Callee.(*ast.MemberExpression)
	case *ast.InfixExpression:
		if typ, known := b.analyzer.ResolvedTypeOf(expr.Right); known && sourceSliceType(typ) {
			if operation, known := b.analyzer.ResolvedOperatorOf(expr); known && (operation.Kind == sema.ResolvedMembershipCompare || operation.Kind == sema.ResolvedNegatedMembershipCompare) {
				return "slice membership lowering", true
			}
		}
	}
	if member == nil || member.Property == nil {
		return "", false
	}
	typ, known := b.analyzer.ResolvedTypeOf(member.Object)
	if !known || !sourceSliceType(typ) {
		return "", false
	}
	token := member.Property.Token
	fact, known := b.analyzer.CompilerKnownMemberAt(token.File, token.Line, token.Column)
	if known {
		switch fact.ID {
		case "CKM-PTR-VALUE":
			return "slice physical pointer extraction", true
		case "CKM-MUTABLE-SLICE-REVERSE":
			return "mutable slice Reverse lowering", true
		}
	}
	return "", false
}
