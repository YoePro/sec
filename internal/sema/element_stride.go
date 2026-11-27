package sema

import (
	"math/big"

	"sec/internal/ast"
)

// ResolvedElementStrideOf is the canonical layout stride in bytes of element
// under the analyzer's target plan: AlignUp(Size(T), Align(T)), zero for a
// zero-sized element. It reports false when no plan is selected or the
// element's layout is incomplete (for example a descriptor type whose
// canonical layout P24 has not defined yet); it never uses host widths.
// Rules: rules/memory/layout.md — § 9; rules/collections/collections.md — § 3.4.
func (a *Analyzer) ResolvedElementStrideOf(element Type) (*big.Int, bool) {
	if a == nil {
		return nil, false
	}
	model := a.parameterCostModel()
	if model.resolver == nil {
		return nil, false
	}
	resolved := model.resolver.Resolve(LayoutRequirementOf(element))
	if !resolved.Complete() || resolved.SizeBytes == nil || resolved.AlignBytes == nil {
		return nil, false
	}
	if resolved.SizeBytes.Sign() == 0 {
		return new(big.Int), true
	}
	if resolved.AlignBytes.Sign() <= 0 {
		return nil, false
	}
	// AlignUp(Size, Align); the resolver already bounded Size by the plan's
	// maximum byte extent (layout.md § 7(7)).
	stride := new(big.Int).Add(resolved.SizeBytes, new(big.Int).Sub(resolved.AlignBytes, big.NewInt(1)))
	stride.Div(stride, resolved.AlignBytes)
	return stride.Mul(stride, resolved.AlignBytes), true
}

// ResolvedTypeSizeOf is the canonical layout size in bytes of a statically
// sized type under the analyzer's target plan (`T.SizeOf`, and value-form
// SizeOf of a statically sized value). It reports false without a plan or
// when the type's layout is incomplete; it never uses host widths.
// Rules: rules/compiler/compiler_known_members.md — §§ 14(4), 15(4), 16(4);
// rules/memory/layout.md — § 7.
func (a *Analyzer) ResolvedTypeSizeOf(typ Type) (*big.Int, bool) {
	if a == nil {
		return nil, false
	}
	model := a.parameterCostModel()
	if model.resolver == nil {
		return nil, false
	}
	resolved := model.resolver.Resolve(LayoutRequirementOf(typ))
	if !resolved.Complete() || resolved.SizeBytes == nil {
		return nil, false
	}
	return new(big.Int).Set(resolved.SizeBytes), true
}

// SizeOfTypeSubject reads the resolved type receiver of a `T.SizeOf` member
// expression (CKM-SIZEOF-TYPE); the result is detached.
// Rules: rules/compiler/compiler_known_members.md — § 15(4).
func (a *Analyzer) SizeOfTypeSubject(member *ast.MemberExpression) (Type, bool) {
	if a == nil || member == nil || member.Property == nil {
		return Type{}, false
	}
	typ, ok := a.sizeOfTypeSubjects[sourceTokenLocation(member.Property.Token)]
	if !ok {
		return Type{}, false
	}
	return semanticSnapshotType(typ), true
}
