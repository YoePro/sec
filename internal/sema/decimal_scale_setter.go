package sema

import (
	"fmt"

	"sec/internal/ast"
)

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
//   - rules/corrections/applied/MD-049-decimal-range-scale-inference-capacity-correction-rev1-2-2026-11-21-20261122.md — § 12(2)
func (a *Analyzer) checkDecimalScaleWrite(stmt *ast.AssignmentStatement, member *ast.MemberExpression, objectType Type, allowFallible bool) bool {
	// MD-049 § 12(2): a fixed-scale named type keeps the Scale getter but has
	// no setter at all, with or without try and even for its own scale.
	if receiver := dereferenceType(objectType); receiver.Named {
		if scale, fixed := fixedDecimalScaleContract(receiver); fixed {
			a.addErrorAtTokenWithMetadata(member.Property.Token, "",
				fmt.Sprintf("%s has the fixed scale %d; use Rescale or RoundTo for a decimal value, then a checked %s(...) conversion", typeDisplayName(receiver), scale.Value, typeDisplayName(receiver)),
				"compiler-known property %s of %s is read-only", member.Property.Value, typeDisplayName(receiver))
			return false
		}
	}
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
