package sema

import (
	"sec/internal/ast"
)

// MD-070 RawPtr-to-safe-reference conversions. `RawPtr[T].AsRef()` and
// `RawPtr[T].AsRefMut()` are the only spellings that form a safe reference
// from a raw pointer; both are unsafe compiler-known methods of RawPtr[T]
// alone, add no lifetime syntax, and never convert implicitly. The result is
// an ordinary Sec reference: copying, reborrowing, escape analysis and borrow
// ends follow the usual rules. Obligations Sema cannot prove remain the
// caller's unsafe responsibility; obligations Sema can disprove, such as a
// proven-null pointer, are compile-time errors even inside `unsafe`.
//
// Rules:
//   - rules/memory/raw_pointers.md — § 18A "Forming safe references" (MD-070)
//   - rules/compiler/compiler_known_members.md — "RawPtr[T] compiler-known methods"

// inferRawPointerReferenceConversion types `pointer.AsRef()` and
// `pointer.AsRefMut()`.
func (a *Analyzer) inferRawPointerReferenceConversion(expr *ast.CallExpression, member *ast.MemberExpression, pointer Type) (Type, expressionValue, bool) {
	invalid := expressionValue{Display: expr.String()}
	operation := "RawPtr." + member.Property.Value
	mutable := member.Property.Value == "AsRefMut"
	if !a.callUnsafeAuthorized(expr) {
		reference := "ref T"
		if mutable {
			reference = "ref mut T"
		}
		a.addErrorAtToken(member.Property.Token, "%s requires unsafe because it asserts that the raw pointer is a valid %s for the reference's whole live range", operation, reference)
		return Type{Kind: InvalidType}, invalid, true
	}
	if !a.checkCompilerKnownCallArity(expr, operation, 0, 0) {
		return Type{Kind: InvalidType}, invalid, true
	}
	element := compilerKnownRawPointerElement(pointer)
	if element.Kind == InvalidType || element.Kind == VoidType {
		a.addErrorAtToken(member.Property.Token, "RawPtr[void].%s cannot form a safe reference; interpret the pointer as a concrete RawPtr[T] first", member.Property.Value)
		return Type{Kind: InvalidType}, invalid, true
	}
	if a.rejectProvenNullDereference(member, member.Property.Value) {
		return Type{Kind: InvalidType}, invalid, true
	}
	if a.rejectKnownRawOriginConflict(member, mutable) {
		return Type{Kind: InvalidType}, invalid, true
	}
	return Type{Name: referenceTypeName(element, mutable), Kind: ReferenceType, Element: &element, ReferenceMutable: mutable}, invalid, true
}

// recordRawPointerOriginBinding remembers the Place an immutable RawPtr[T]
// binding takes its address from (`place.Ptr`, `r.Ptr` or `(*r).Ptr`; the
// latter two address the referent).
func (a *Analyzer) recordRawPointerOriginBinding(name string, value ast.Expression) {
	symbol, ok := a.symbols[name]
	member, isMember := value.(*ast.MemberExpression)
	if !ok || symbol.Mutable || symbol.Volatile || symbol.Type.Kind != RawPtrType || !isMember || member.Property == nil || member.Property.Value != "Ptr" {
		return
	}
	place, ok := a.resolvePlace(member.Object)
	if !ok {
		return
	}
	if place.Type.Kind == ReferenceType && place.Type.Element != nil {
		place = a.canonicalDereferencePlace(member.Object, place, place.Type)
	}
	origin := clonePlace(place)
	symbol.RawPointerOrigin = &origin
	a.symbols[name] = symbol
}

// rejectKnownRawOriginConflict applies MD-070 § 5(3) where Sema knows the
// pointer's origin: forming `ref mut T` while any live borrow overlaps the
// origin, or `ref T` while a mutable one does, is a proven authority
// conflict, and an origin whose owner no longer holds a value is a proven
// dangling address. Both are errors even inside unsafe.
func (a *Analyzer) rejectKnownRawOriginConflict(member *ast.MemberExpression, mutable bool) bool {
	ident, ok := member.Object.(*ast.Identifier)
	if !ok {
		return false
	}
	symbol, ok := a.symbols[ident.Value]
	if !ok || symbol.Mutable || symbol.RawPointerOrigin == nil {
		return false
	}
	origin := *symbol.RawPointerOrigin
	if _, _, _, unavailable := a.unavailablePlace(origin); unavailable {
		a.addErrorAtToken(member.Property.Token, "RawPtr.%s through %s would form a reference to %s, which no longer holds a value; unsafe does not make a known dangling address valid", member.Property.Value, ident.Value, origin.String())
		return true
	}
	action := "form a shared reference through RawPtr.AsRef to"
	if mutable {
		action = "form a mutable reference through RawPtr.AsRefMut to"
		return a.checkBorrowedMutationPlaceForAction(origin, member.Property.Token, action)
	}
	return a.checkBorrowedReadPlaceForAction(origin, member.Property.Token, action)
}
