package sema

import (
	"math/big"

	"sec/internal/ast"
)

// registerValueField returns the multi-bit (or unit-annotated) numeric field
// of a register that member writes. One-bit plain fields are bool, and a
// nested register field keeps its own nominal write rules.
func (a *Analyzer) registerValueField(member *ast.MemberExpression) (RegisterField, bool) {
	if member == nil || member.Property == nil {
		return RegisterField{}, false
	}
	objectType := dereferenceType(a.expressionTypes[member.Object])
	if objectType.Kind != RegisterType {
		return RegisterField{}, false
	}
	for _, field := range objectType.RegisterFields {
		if field.Name == member.Property.Value {
			if field.Type.Kind != UintType {
				return RegisterField{}, false
			}
			return field, true
		}
	}
	return RegisterField{}, false
}

// checkRegisterFieldWrite enforces MD-046 for a write to a bit[N] or
// bit[N]<U> field: the value carries exactly the field's unit identity (no
// unit for a plain field) and is proven to lie in 0..2^N-1. A wider or
// unknown value is never truncated and the write never becomes fallible; the
// program narrows it explicitly first, typically with bit[N](value) or
// bit[N](value)<U>.
//
// Rules:
//   - rules/declarations/registers.md — § 12 (field reads and writes)
//   - rules/types/units.md — "Interaction with registers and FFI"
func (a *Analyzer) checkRegisterFieldWrite(stmt *ast.AssignmentStatement, field RegisterField, valueType Type) bool {
	token := expressionToken(stmt.Value)
	fieldName := field.Name
	spelling := registerFieldSpelling(field)
	if stmt.Operator != "=" {
		a.addErrorAtTokenWithMetadata(stmt.Token, "",
			"Compute the new value, narrow it with "+registerFieldNarrowing(field, "value")+", and assign it with =.",
			"compound assignment %s to register field %s (%s) cannot prove the result fits; a register write never truncates", stmt.Operator, fieldName, spelling)
		return false
	}
	if !isIntegerType(valueType) {
		a.addErrorAtToken(token, "cannot assign %s to register field %s (%s)", typeDisplayName(valueType), fieldName, spelling)
		return false
	}
	// An untyped integer literal takes the field's unit, as it does for any
	// unit-bearing declaration (`let d: decimal<m> := 10`).
	if isUntypedNumericExpression(stmt.Value) {
		valueType = field.Type
	}
	switch {
	case field.Unit == "" && hasUnitSemantics(valueType):
		a.addErrorAtTokenWithMetadata(token, "",
			"bit[N](value) explicitly discards the unit when raw bits are intended.",
			"cannot assign %s to plain register field %s (%s): the value has a unit", typeDisplayName(valueType), fieldName, spelling)
		return false
	case field.Unit != "" && !hasUnitSemantics(valueType):
		a.addErrorAtTokenWithMetadata(token, "",
			"Label raw bits explicitly with "+registerFieldNarrowing(field, "raw")+"; a register write never creates a unit implicitly.",
			"cannot assign %s to register field %s (%s): missing unit identity <%s>", typeDisplayName(valueType), fieldName, spelling, field.Unit)
		return false
	case field.Unit != "" && !sameSemanticUnit(field.Type, valueType):
		a.addErrorAtTokenWithMetadata(token, "",
			"Convert the value with a declared unit conversion such as <"+field.Unit+">(value) first; bit extraction never changes a unit.",
			"cannot assign %s to register field %s (%s): the unit is not <%s>", typeDisplayName(valueType), fieldName, spelling, field.Unit)
		return false
	}
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(field.Width)), big.NewInt(1))
	// Checked arithmetic never yields a value outside its type, so the
	// proven interval is clamped to the value type's domain.
	if value, constant := a.integerConstantValue(stmt.Value); constant {
		if value.Sign() >= 0 && value.Cmp(maximum) <= 0 {
			a.recordRegisterWrite(stmt, ResolvedRegisterWriteProof{Basis: RegisterWriteConstant, Low: value, High: new(big.Int).Set(value)})
			return true
		}
		a.addErrorAtToken(token, "value %s does not fit register field %s (%s, 0..%s)", value.String(), fieldName, spelling, maximum.String())
		return false
	}
	if low, high, known := a.scalarValueInterval(stmt.Value, valueType); known && low.Sign() >= 0 && high.Cmp(maximum) <= 0 {
		a.recordRegisterWrite(stmt, ResolvedRegisterWriteProof{Basis: RegisterWriteInterval, Low: low, High: high})
		return true
	}
	a.addErrorAtTokenWithMetadata(token, "",
		"Narrow the value explicitly with "+registerFieldNarrowing(field, "value")+", or prove its range first (for example with an if condition).",
		"cannot assign %s to register field %s (%s): the value is not proven to fit 0..%s", typeDisplayName(valueType), fieldName, spelling, maximum.String())
	return false
}

func registerFieldSpelling(field RegisterField) string {
	spelling := "bit[" + big.NewInt(field.Width).String() + "]"
	if field.Unit != "" {
		spelling += "<" + field.Unit + ">"
	}
	return spelling
}

func registerFieldNarrowing(field RegisterField, operand string) string {
	return "bit[" + big.NewInt(field.Width).String() + "](" + operand + ")" + unitSuffix(field.Unit)
}

func unitSuffix(unit string) string {
	if unit == "" {
		return ""
	}
	return "<" + unit + ">"
}

// sameSemanticUnit reports two unit-bearing types with the same unit
// identity: the same named unit, or the same structural factors, with
// identical coordinates. A matching dimension and carrier alone is not the
// same unit (MD-046 § 4.5).
func sameSemanticUnit(left, right Type) bool {
	if !hasUnitSemantics(left) || !hasUnitSemantics(right) {
		return false
	}
	l, r := effectiveUnitSemantics(left), effectiveUnitSemantics(right)
	if l.Identity != r.Identity || l.Named != r.Named || len(l.Factors) != len(r.Factors) {
		return false
	}
	for name, exponent := range l.Factors {
		if r.Factors[name] != exponent {
			return false
		}
	}
	factor, offset, ok := unitConversionCoordinates(r, l)
	return ok && factor.Cmp(big.NewRat(1, 1)) == 0 && offset.Sign() == 0
}
