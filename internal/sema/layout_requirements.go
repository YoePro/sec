package sema

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/layout"
)

// LayoutRequirementOf maps a concrete Sema type to the plan-independent
// layout requirement the canonical resolver consumes (P24-10/16-21). The type
// identity is canonicalTypeIdentity, so nominal declarations and generic
// substitutions stay distinct and display names are never identity. The
// mapping selects no size or alignment; the resolver decides those under one
// CompilationPlan.
//
// Rules:
//   - rules/memory/layout.md — §§ 4, 11–13, 18, 19, 25, 39, 47.6
//   - rules/types/types.md — §§ 9.2–9.4, Named types
//   - rules/mlir/packages/sec-mlir-dialect_package24.md — §§ 7–16
func LayoutRequirementOf(typ Type) layout.LayoutRequirement {
	return layoutRequirementOf(typ, map[string]bool{})
}

func layoutRequirementOf(typ Type, visiting map[string]bool) layout.LayoutRequirement {
	identity := canonicalTypeIdentity(typ)
	requirement := layout.LayoutRequirement{TypeIdentity: identity}
	if visiting[identity] {
		// A by-value revisit: hand the resolver the bare identity so its
		// in-progress stack reports the concrete cycle path.
		requirement.Kind = layout.LayoutStruct
		return requirement
	}
	visiting[identity] = true
	defer delete(visiting, identity)

	if isAtomicType(typ) {
		// MD-095: Atomic[T] is atomic storage over T, never a fieldless
		// struct; the resolver applies the target's atomic minimum
		// (layout.md § 47.6).
		contained := layoutRequirementOf(typ.TypeArgs[0], visiting)
		requirement.Kind, requirement.Carrier = layout.LayoutAtomic, &contained
		return requirement
	}
	if typ.Named && typ.NamedBase != nil {
		carrier := layoutRequirementOf(*typ.NamedBase, visiting)
		requirement.Kind = layout.LayoutNamed
		requirement.Carrier = &carrier
		requirement.Validity = namedLayoutValidity(identity, typ)
		return requirement
	}
	switch typ.Kind {
	case BoolType:
		requirement.Kind, requirement.Scalar = layout.LayoutScalar, layout.ScalarBool
	case CharType:
		requirement.Kind, requirement.Scalar, requirement.WidthBits = layout.LayoutScalar, layout.ScalarChar, 8
	case RuneType:
		requirement.Kind, requirement.Scalar = layout.LayoutScalar, layout.ScalarRune
	case IntType, UintType:
		requirement.Kind = layout.LayoutScalar
		requirement.Scalar = layout.ScalarSigned
		if typ.Kind == UintType {
			requirement.Scalar = layout.ScalarUnsigned
		}
		switch {
		case typ.Name == "byte":
			requirement.Scalar, requirement.WidthBits = layout.ScalarByte, 8
		case typ.Name == "int" || typ.Name == "uint":
			requirement.TargetSized = true
		default:
			requirement.WidthBits = integerLayoutWidth(typ)
		}
	case FloatType:
		requirement.Kind, requirement.Scalar = layout.LayoutScalar, layout.ScalarFloat
		if typ.Name == "float" {
			requirement.TargetSized = true
		} else {
			requirement.WidthBits = uint16(typ.FloatBits)
		}
	case DecimalType:
		requirement.Kind, requirement.Scalar, requirement.WidthBits = layout.LayoutScalar, layout.ScalarDecimal, 64
		if typ.Name == "decimal128" || typ.Underlying == "decimal128" {
			requirement.WidthBits = 128
		}
	case EnumType:
		requirement.Kind = layout.LayoutEnum
		carrier := builtinType(typ.Underlying)
		if typ.AssociatedEnum || typ.BitWidth != 0 || (carrier.Kind != IntType && carrier.Kind != UintType) {
			return requirement
		}
		underlying := layoutRequirementOf(carrier, visiting)
		requirement.Carrier = &underlying
		for _, name := range typ.EnumValues {
			member := typ.EnumConsts[name]
			value := member.Value
			if value != nil {
				value = new(big.Int).Set(value)
			}
			requirement.EnumMembers = append(requirement.EnumMembers, layout.EnumMemberRequirement{Name: name, Value: value})
		}
	case StructType:
		requirement.Kind = layout.LayoutStruct
		for _, field := range typ.Fields {
			requirement.Fields = append(requirement.Fields, layout.FieldRequirement{Name: field.Name, Requirement: layoutRequirementOf(field.Type, visiting)})
		}
	case ArrayType:
		count, fixed := exactFixedArrayLength(typ)
		if !fixed || typ.Element == nil {
			// An owning dynamic array is a descriptor over separate backing.
			requirement.Kind = layout.LayoutDescriptor
			return requirement
		}
		element := layoutRequirementOf(*typ.Element, visiting)
		requirement.Kind, requirement.Element, requirement.Count = layout.LayoutArray, &element, count
	case ReferenceType, SliceType, StringType, StringViewType, RawPtrType, FunctionType:
		// Indirection: never a by-value containment edge (layout.md § 13(7)).
		requirement.Kind = layout.LayoutDescriptor
	default:
		// Unions, Result/Option and the remaining families belong to
		// later P24 stages; the resolver reports them unsupported.
		requirement.Kind = layout.LayoutKind(typ.Kind)
	}
	return requirement
}

// integerLayoutWidth is the declared bit width of a fixed-width integer,
// derived from its exact range when no width is recorded.
func integerLayoutWidth(typ Type) uint16 {
	if typ.BitWidth > 0 {
		return uint16(typ.BitWidth)
	}
	if typ.MaxInteger == nil && !typ.Named {
		// A detached snapshot drops the integer domain; a fixed-width builtin
		// is recovered from its canonical declaration, never guessed.
		if builtin := builtinType(typ.Name); builtin.Kind == typ.Kind && (builtin.BitWidth > 0 || builtin.MaxInteger != nil) {
			typ = builtin
			if typ.BitWidth > 0 {
				return uint16(typ.BitWidth)
			}
		}
	}
	if typ.MaxInteger == nil {
		return 0
	}
	width := typ.MaxInteger.BitLen()
	if typ.Kind == IntType {
		width++
	}
	return uint16(width)
}

// namedLayoutValidity states the nominal and contract validity a named or
// constrained type adds over its carrier (layout.md § 31).
func namedLayoutValidity(identity string, typ Type) string {
	if len(typ.Contracts) == 0 {
		return ""
	}
	kinds := make([]string, 0, len(typ.Contracts))
	for _, contract := range typ.Contracts {
		kinds = append(kinds, strings.TrimPrefix(fmt.Sprintf("%T", contract), "sema."))
	}
	return identity + " contracts: " + strings.Join(kinds, ", ")
}
