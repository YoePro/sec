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
	return Type{Name: referenceTypeName(element, mutable), Kind: ReferenceType, Element: &element, ReferenceMutable: mutable}, invalid, true
}
