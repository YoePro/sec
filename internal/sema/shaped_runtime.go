package sema

import "sec/internal/lexer"

// runtimeShapedTensorRank recognizes the owning runtime-shaped tensor identity
// without treating it as tensor_view or as a statically shaped tensor. The
// Shape argument has already passed ordinary compiler-known type validation,
// so its one retained constant is the compile-time-known rank.
//
// Rules:
//   - rules/collections/shaped-types.md — §3.4 "Runtime-shaped owning tensor"
//   - rules/collections/shaped-types.md — §5 "Rank, Shape, and Len"
func runtimeShapedTensorRank(typ Type) (int64, bool) {
	if typ.Name != "tensor" || len(typ.TypeArgs) != 2 || len(typ.ConstArgs) != 0 {
		return 0, false
	}
	shape := typ.TypeArgs[1]
	if shape.Name != "Shape" || len(shape.TypeArgs) != 0 || len(shape.ConstArgs) != 1 || shape.ConstArgs[0] < 0 {
		return 0, false
	}
	return shape.ConstArgs[0], true
}

// validateTensorType accepts exactly one of the two canonical owning tensor
// forms: tensor[T, D0, ...] or tensor[T, Shape[Rank]]. Runtime-shaped tensors
// deliberately do not receive a StaticElementCount because their extents and
// Len are runtime values.
//
// Rules:
//   - rules/collections/shaped-types.md — §§3.3–3.4 "tensor"
func (a *Analyzer) validateTensorType(token lexer.Token, typ *Type) bool {
	if typ == nil {
		return false
	}
	if len(typ.TypeArgs) == 2 {
		if _, ok := runtimeShapedTensorRank(*typ); !ok {
			a.addErrorAtToken(token, "runtime-shaped tensor requires exactly tensor[T, Shape[Rank]] with no static extents")
			return false
		}
		typ.StaticElementCount = nil
		return true
	}
	return a.validateShapedStaticType(token, typ, 1, -1)
}

// acceptsExtendedCompilerKnownTypeArguments lets the tensor-specific validator
// inspect its second compiler-known Shape argument. Ordinary generic types keep
// the declared generic-parameter arity rules.
//
// Rules:
//   - rules/collections/shaped-types.md — §3.4 "Runtime-shaped owning tensor"
func acceptsExtendedCompilerKnownTypeArguments(typ Type, argumentCount int) bool {
	return typ.Name == "tensor" && argumentCount == 2
}
