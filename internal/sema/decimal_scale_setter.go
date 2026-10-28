package sema

import "sec/internal/ast"

// decimalScaleMember reports whether member names the compiler-known Scale
// property of a decimal or decimal128 receiver, rather than a struct field
// of the same spelling.
// Rules: rules/library/core-library.md — §§ 10.1, 10.4 (MD-066/MD-068).
func decimalScaleMember(objectType Type, member *ast.MemberExpression) bool {
	receiver := dereferenceType(objectType)
	if member.Property == nil || receiver.Kind != DecimalType {
		return false
	}
	known, ok := compilerKnownMember(receiver, member.Property.Value, false)
	return ok && (known.ID == "CKM-DECIMAL-SCALE" || known.ID == "CKM-DECIMAL128-SCALE")
}

// checkDecimalScaleWrite validates `try value.Scale = scale`: the setter is
// fallible, so it requires try, and it rescales the receiver in place, so the
// receiver must be a mutable, assignable place that is not borrowed. The
// accepted assignment is recorded for its PrecisionError failure type.
//
// Rules:
//   - rules/library/core-library.md — § 10.1 (MD-066)
//   - rules/errors/errorhandling.md — § 24 "Fallible property setters"
func (a *Analyzer) checkDecimalScaleWrite(stmt *ast.AssignmentStatement, member *ast.MemberExpression, allowFallible bool) bool {
	if !allowFallible {
		a.addErrorAtTokenWithMetadata(member.Property.Token, "",
			"Scale has a fallible setter: write `try value.Scale = scale` and handle PrecisionError, or use Rescale/RoundTo for a new value",
			"assigning fallible property %s requires try", member.Property.Value)
		return false
	}
	place, ok := a.resolvePlace(member.Object)
	if !ok {
		a.addErrorAtToken(member.Property.Token, "%s can only be set on an assignable decimal variable or field; use Rescale for a new value", member.Property.Value)
		return false
	}
	if !place.Mutable {
		a.addErrorAtToken(member.Property.Token, "cannot assign to immutable place %s", place.String())
		return false
	}
	if a.checkBorrowedMutationPlace(place, member.Property.Token) {
		return false
	}
	if a.decimalScaleWrites == nil {
		a.decimalScaleWrites = map[*ast.AssignmentStatement]bool{}
	}
	a.decimalScaleWrites[stmt] = true
	return true
}

// IsDecimalScaleWrite reports whether stmt assigns the fallible Scale
// setter of a decimal value, for later phases that must lower or reject it.
// Rules: rules/library/core-library.md — § 10.1 (MD-066).
func (a *Analyzer) IsDecimalScaleWrite(stmt *ast.AssignmentStatement) bool {
	return a != nil && a.decimalScaleWrites[stmt]
}
