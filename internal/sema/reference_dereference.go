package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// MD-063 explicit whole-value dereference. A safe reference holder `r` has
// reference type; `*r` names its referent Place. Reading `*r` is an ordinary
// by-value read of that Place, and `*r = value` / `*r op= value` replace it.
// Nothing here dereferences a holder implicitly.
//
// Rules:
//   - rules/memory/references.md — "Explicit whole-value dereference"
//   - rules/foundations/operators.md — prefix dereference
//   - rules/compiler/semantic_ir.md — § 45(8) referent Place resolution

// dereferenceExpression reports whether expr is the source prefix `*operand`.
func dereferenceExpression(expr ast.Expression) (*ast.PrefixExpression, bool) {
	prefix, ok := expr.(*ast.PrefixExpression)
	if !ok || prefix == nil || prefix.Operator != "*" || prefix.Right == nil {
		return nil, false
	}
	return prefix, true
}

// dereferencedReferenceType validates the operand of `*` and returns its safe
// reference type. Option[ref T], RawPtr[T], slices and plain values are not
// safe references and are rejected without a speculative fix.
func (a *Analyzer) dereferencedReferenceType(expr *ast.PrefixExpression, operandType Type) (Type, bool) {
	if operandType.Kind == ReferenceType && operandType.Element != nil && operandType.Element.Kind != SliceType {
		return operandType, true
	}
	reason := "it is not a safe ref T or ref mut T"
	switch {
	case operandType.Kind == SliceType || operandType.Element != nil && operandType.Element.Kind == SliceType:
		reason = "a slice view has no whole referent value; index, iterate or subslice it instead"
	case isOptionType(operandType):
		reason = "an Option is not a reference; obtain the contained reference through ordinary Option handling first"
	case operandType.Kind == RawPtrType:
		reason = "RawPtr is not a safe reference and has no dereference operator"
	}
	a.addErrorAtToken(expr.Token, "operator * requires a safe reference operand, got %s: %s", typeDisplayName(operandType), reason)
	return Type{}, false
}

// inferDereferenceExpression types a value use of `*r` as the referent type.
// The read follows the referent's copy classification: a move-only referent
// cannot be copied out, and a borrow never transfers ownership. As the direct
// base of a longer Place path (`(*r).Field`) no whole-value read occurs.
func (a *Analyzer) inferDereferenceExpression(expr *ast.PrefixExpression) (Type, expressionValue) {
	invalid := expressionValue{Display: expr.String()}
	operandType, _ := a.inferPlaceBase(expr.Right)
	if operandType.Kind == InvalidType {
		return Type{Kind: InvalidType}, invalid
	}
	referenceType, ok := a.dereferencedReferenceType(expr, operandType)
	if !ok {
		return Type{Kind: InvalidType}, invalid
	}
	referent := *referenceType.Element
	if !a.isPlaceBase(expr) {
		switch CopyClassificationOf(referent) {
		case CopyTrivial, CopySemantic:
		default:
			a.addErrorAtTokenWithMetadata(
				expr.Token,
				diagnostics.ImplicitMoveDisallowed,
				"Project the needed fields through `"+expr.Right.String()+".Field`, or pass the reference itself; a borrowed referent can be neither copied nor moved out.",
				"cannot copy referent *%s of move-only type %s through a reference",
				expr.Right.String(),
				typeDisplayName(referent),
			)
			return Type{Kind: InvalidType}, invalid
		}
	}
	return referent, invalid
}

// rejectDereferenceMove keeps explicit `<-` / `:<-` from extracting a value out
// of a borrowed referent (MD-063 § 11(2)). It reports whether it rejected.
func (a *Analyzer) rejectDereferenceMove(mode ast.OwnershipMode, value ast.Expression) bool {
	prefix, ok := dereferenceExpression(value)
	if !ok || mode != ast.OwnershipMove {
		return false
	}
	a.addErrorAtTokenWithMetadata(
		prefix.Token,
		diagnostics.ImplicitMoveDisallowed,
		"A reference grants access, not ownership: copy a copyable referent with `*r`, or move the owner itself.",
		"cannot move out of borrowed referent *%s",
		prefix.Right.String(),
	)
	return true
}

// resolveDereferencePlace resolves `*r` to the canonical referent Place of the
// holder's reference type.
func (a *Analyzer) resolveDereferencePlace(expr *ast.PrefixExpression) (Place, bool) {
	holder, ok := a.resolvePlace(expr.Right)
	if !ok || holder.Type.Kind != ReferenceType || holder.Type.Element == nil {
		return Place{}, false
	}
	return a.canonicalDereferencePlace(expr.Right, holder, holder.Type), true
}

// analyzeDereferenceAssignmentStatement validates `*r = value` and
// `*r op= value`: the holder must grant mutable authority, the value must
// initialize the referent type, and the replacement follows ordinary
// assignment ownership. The holder binding itself need not be mutable.
func (a *Analyzer) analyzeDereferenceAssignmentStatement(stmt *ast.AssignmentStatement, target *ast.PrefixExpression, allowFallible bool) {
	operandType, _ := a.inferPlaceBase(target.Right)
	if operandType.Kind == InvalidType {
		return
	}
	referenceType, ok := a.dereferencedReferenceType(target, operandType)
	if !ok {
		return
	}
	referent := *referenceType.Element
	if !referenceType.ReferenceMutable {
		a.addErrorAtTokenWithMetadata(
			target.Token,
			diagnostics.BorrowMutableAuthority,
			"A shared reference never grants mutation authority; take `ref mut` from the owner to replace the referent.",
			"cannot assign through shared reference %s: %s grants no write authority",
			target.Right.String(),
			typeDisplayName(referenceType),
		)
		return
	}
	place, placeOK := a.resolveDereferencePlace(target)
	if placeOK && a.checkBorrowedMutationPlace(place, target.Token) {
		return
	}
	if hasContracts(referent) && !allowFallible {
		start := len(a.errors)
		a.addErrorAtTokenWithMetadata(target.Token, diagnostics.ConstrainedAssignmentRequiresTry, "use try assignment with an Err handler", "assigning referent *%s requires try because %s has contracts", target.Right.String(), typeDisplayName(referent))
		a.relateConstrainedAssignment(start, referent)
		return
	}
	if !a.validateMoveAssignmentTarget(stmt) {
		return
	}
	valueType, _ := a.inferExpressionWithExpected(stmt.Value, referent)
	if valueType.Kind == InvalidType {
		return
	}
	if a.checkStorageDomainStore(stmt.Target, referent, stmt.Value, valueType) {
		return
	}
	if a.checkAssignmentEscapesLocalReference(stmt.Target, stmt.Value) {
		return
	}
	if a.checkCompileTimeContractExpression(referent, stmt.Value) {
		return
	}
	assignmentType := valueType
	if stmt.Operator != "=" {
		var valid bool
		assignmentType, valid = a.inferCompoundAssignmentType(stmt.Operator, referent, valueType, stmt.Value)
		if !valid {
			return
		}
		a.recordResolvedCompoundAssignment(stmt, referent, valueType)
	}
	if a.rejectImplicitBorrow(referent, assignmentType, stmt.Value) {
		return
	}
	if !a.canInitialize(referent, assignmentType, stmt.Value) {
		a.addErrorAtToken(expressionToken(stmt.Value), "cannot assign %s to %s", typeDisplayName(valueType), typeDisplayName(referent))
		return
	}
	if !a.validateAssignmentOwnership(stmt) {
		return
	}
	a.applyAssignmentOwnership(stmt)
	if placeOK {
		a.markPlaceAvailable(place)
	}
}

// referenceHolderAssignmentHelp explains an assignment whose target is a
// reference holder but whose operation needs the referent (MD-063 § 15).
func referenceHolderAssignmentHelp(holder string) string {
	return "`" + holder + "` is a reference holder; write `*" + holder + "` to read or replace the referenced value, or assign another reference to rebind the holder."
}

// referenceOperandHelp suggests `*` when a numeric operator received a safe
// reference whose referent would satisfy it (MD-063 § 15(2)).
func referenceOperandHelp(left Type, right Type, leftExpr ast.Expression, rightExpr ast.Expression) string {
	for _, operand := range []struct {
		typ  Type
		expr ast.Expression
	}{{left, leftExpr}, {right, rightExpr}} {
		if operand.typ.Kind == ReferenceType && operand.typ.Element != nil && isNumericType(*operand.typ.Element) && operand.expr != nil {
			return "`" + operand.expr.String() + "` is a reference of type " + typeDisplayName(operand.typ) + "; write `*" + operand.expr.String() + "` to read the referenced value."
		}
	}
	return ""
}

// referenceValueHelp suggests `*` when a by-value T position received a safe
// reference to T. A reference never satisfies an owned/value parameter
// implicitly (MD-063 § 8(3)); the suggestion is omitted for move-only T,
// whose referent cannot be copied out.
func referenceValueHelp(expected Type, actual Type, expr ast.Expression) string {
	if expr != nil && expected.Kind == ReferenceType && expected.Element != nil && actual.Kind == RawPtrType {
		// MD-070: a raw pointer never converts to a safe reference implicitly.
		method := "AsRef"
		if expected.ReferenceMutable {
			method = "AsRefMut"
		}
		return "A raw pointer never converts to a safe reference implicitly; write `unsafe " + expr.String() + "." + method + "()` and uphold its safety contract."
	}
	if expr == nil || expected.Kind == ReferenceType || actual.Kind != ReferenceType || actual.Element == nil || !sameConcreteType(expected, *actual.Element) {
		return ""
	}
	switch CopyClassificationOf(expected) {
	case CopyTrivial, CopySemantic:
		return "`" + expr.String() + "` is a reference of type " + typeDisplayName(actual) + "; write `*" + expr.String() + "` to pass a copy of the referenced value."
	}
	return ""
}
