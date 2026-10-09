package sema

import (
	"fmt"
	"sec/internal/ast"
	"strings"
)

// canInitialize preserves nominal identity for typed values while permitting
// canonical contextual literals, references and specialized unit/error widening.
// Rules: rules/types/types.md — Type identity, Assignability, Untyped literals versus typed values.
func canInitialize(target Type, value Type, expr ast.Expression) bool {
	if target.Kind == InvalidType || value.Kind == InvalidType {
		return true
	}
	// A never expression has no continuing path on which a value would need to
	// inhabit the target type.
	if value.Kind == NeverType {
		return true
	}
	if target.Kind == AnyType {
		return value.Kind != VoidType && value.Kind != NeverType
	}
	if value.Kind == AnyType {
		return target.Kind == AnyType
	}
	if hasUnitSemantics(target) || hasUnitSemantics(value) {
		return canInitializeUnitQuantity(target, value, expr)
	}
	if allowed, foreign := canInitializeForeignCScalar(target, value, isNumericLiteral(expr) || isBooleanLiteral(expr)); foreign {
		return allowed
	}
	// rules/errors/errorhandling.md defines a one-way, error-specific widening
	// relation. It is not general interface inheritance and never permits
	// implicit narrowing from error to one concrete error family.
	if target.Kind == ErrorRootType || value.Kind == ErrorRootType {
		return target.Kind == ErrorRootType && (value.Kind == ErrorRootType || value.ErrorAssignable)
	}

	if target.Kind == FunctionType || value.Kind == FunctionType {
		return sameConcreteType(target, value)
	}

	if target.Kind == ReferenceType {
		if target.Element == nil {
			return false
		}
		if value.Kind == ReferenceType {
			return sameConcreteType(target, value)
		}
		return canInitialize(*target.Element, value, expr)
	}

	if target.Kind == ArrayType || value.Kind == ArrayType || target.Kind == SliceType || value.Kind == SliceType {
		// rules/collections/collections.md; correction26.md: T[] and T[N]
		// are distinct owning representations. Dynamic owners initialize only
		// from the same dynamic owner type; fixed arrays require exact extent.
		if target.Kind == ArrayType && value.Kind == ArrayType &&
			arrayShapeOf(target) == ArrayShapeDynamic && arrayShapeOf(value) == ArrayShapeDynamic &&
			target.Element != nil && value.Element != nil {
			return sameConcreteType(target, value)
		}
		return sameConcreteType(target, value)
	}

	if target.Kind == EnumType || value.Kind == EnumType {
		return target.Kind == EnumType && value.Kind == EnumType && sameConcreteType(target, value)
	}

	if target.Kind == StructType || value.Kind == StructType {
		return target.Kind == StructType && value.Kind == StructType && sameConcreteType(target, value)
	}

	if target.Kind == RegisterType || value.Kind == RegisterType {
		return target.Kind == RegisterType && value.Kind == RegisterType && sameConcreteType(target, value)
	}

	if target.Kind == UnionType || value.Kind == UnionType {
		return target.Kind == UnionType && value.Kind == UnionType && sameConcreteType(target, value)
	}

	if len(target.TypeArgs) > 0 || len(value.TypeArgs) > 0 {
		return sameConcreteType(target, value)
	}

	if (target.Named || value.Named) && !sameConcreteType(target, value) {
		if target.Kind == value.Kind {
			switch expr.(type) {
			case *ast.StringLiteral:
				return target.Kind == StringType
			case *ast.BooleanLiteral:
				return target.Kind == BoolType
			case *ast.CharLiteral:
				return target.Kind == CharType || target.Kind == RuneType
			case *ast.IntegerLiteral:
				literal := expr.(*ast.IntegerLiteral)
				return target.Kind == CharType && literal.Suffix() == "t" || target.Kind == RuneType && literal.Suffix() == "r" || canUntypedNumericInitializeNominal(target, value, expr)
			}
		}
		return canUntypedNumericInitializeNominal(target, value, expr)
	}

	if target.Kind == value.Kind {
		if isNumericType(target) && isNumericType(value) {
			if isNumericLiteral(expr) {
				return true
			}
			return sameConcreteType(target, value)
		}
		return true
	}

	if target.Kind == UintType && value.Kind == IntType {
		return isNumericLiteral(expr)
	}

	if target.Kind == DecimalType && isNumericLiteral(expr) {
		_, ok := decimalLiteralValue(expr)
		return ok
	}

	if target.Kind == FloatType && value.Kind == DecimalType && isNumericLiteral(expr) {
		return true
	}

	return false
}

// canInitializeUnitQuantity applies the implicit fixed-conversion policy from
// rules/types/units.md before nominal numeric initialization is considered.
func canInitializeUnitQuantity(target Type, value Type, expr ast.Expression) bool {
	if isUntypedNumericExpression(expr) && hasUnitSemantics(target) && !hasUnitSemantics(value) &&
		isNumericType(target) && isNumericType(value) {
		return target.Kind == value.Kind ||
			(target.Kind == UintType && value.Kind == IntType) ||
			(target.Kind == DecimalType && (value.Kind == IntType || value.Kind == UintType || value.Kind == DecimalType || value.Kind == FloatType)) ||
			(target.Kind == FloatType && (value.Kind == IntType || value.Kind == UintType || value.Kind == DecimalType || value.Kind == FloatType))
	}
	if !isNumericType(target) || !isNumericType(value) {
		return false
	}
	// rules/types/units.md, "Same named unit": no unit conversion is needed.
	if sameConcreteType(target, value) {
		return true
	}
	if !sameNumericCarrier(target, value) || !target.Dimension.Equal(value.Dimension) {
		return false
	}
	to, from := effectiveUnitSemantics(target), effectiveUnitSemantics(value)
	if !unitKindCompatible(to, from) || !unitOriginCompatible(to, from) || to.Role != from.Role {
		return false
	}
	return exactImplicitUnitConversion(from, to, target.Kind)
}

// canUntypedNumericInitializeNominal shapes numeric literals by the resolved
// family, including chained named types, while typed runtime values remain nominal.
// Rules: rules/types/types.md — Untyped literals versus typed values, Named types.
func canUntypedNumericInitializeNominal(target Type, value Type, expr ast.Expression) bool {
	if !isUntypedNumericExpression(expr) {
		return false
	}
	switch target.Kind {
	case IntType:
		return value.Kind == IntType || value.Kind == UintType
	case UintType:
		return value.Kind == IntType || value.Kind == UintType
	case FloatType:
		return value.Kind == IntType || value.Kind == UintType || value.Kind == DecimalType || value.Kind == FloatType
	case DecimalType:
		return value.Kind == IntType || value.Kind == UintType || value.Kind == DecimalType || value.Kind == FloatType
	default:
		return false
	}
}

// canExplicitConvert checks parameterized representation identity for aggregate
// conversions and follows named wrappers only through their declared base chain.
// Equal struct fields or equal ABI layout introduce no conversion relation.
// Rules: rules/types/types.md — Named types, Explicit conversions;
// rules/declarations/struct.md — Identity and nominal semantics.
func (a *Analyzer) canExplicitConvert(target, value Type) bool {
	if target.Kind == RawPtrType || value.Kind == RawPtrType {
		return canExplicitConvert(target, value)
	}
	if (target.Kind == EnumType && value.Kind == EnumType) || (target.Kind == RegisterType && value.Kind == RegisterType) || (target.Kind == UnionType && value.Kind == UnionType) {
		return sameConcreteType(a.namedRepresentation(target), a.namedRepresentation(value))
	}
	if target.Kind == AnyType || value.Kind == AnyType {
		return canExplicitConvert(target, value)
	}
	if target.Kind == StructType || value.Kind == StructType || target.Kind == InterfaceType || value.Kind == InterfaceType || target.Kind == ResultType || value.Kind == ResultType {
		if target.Kind != value.Kind {
			return false
		}
		if sameConcreteType(target, value) {
			return true
		}
		return sameConcreteType(a.namedRepresentation(target), a.namedRepresentation(value))
	}
	if target.Kind == ArrayType || value.Kind == ArrayType || target.Kind == SliceType || value.Kind == SliceType || target.Kind == ReferenceType || value.Kind == ReferenceType || target.Kind == FunctionType || value.Kind == FunctionType {
		if target.Kind != value.Kind {
			return false
		}
		target = a.namedRepresentation(target)
		value = a.namedRepresentation(value)
		return sameConcreteType(target, value)
	}
	return canExplicitConvert(target, value)
}

// namedRepresentation follows canonical named-type ancestry without inspecting
// fields, machine layout or same-sized carriers. Concrete arguments remain facts.
// Rules: rules/types/types.md — Named types, Generic and parameterized types.
func (a *Analyzer) namedRepresentation(typ Type) Type {
	seen := map[string]bool{}
	for typ.Named && typ.Underlying != "" && !seen[typ.Name] {
		seen[typ.Name] = true
		var base Type
		if typ.NamedBase != nil {
			base = *typ.NamedBase
		} else {
			var found bool
			base, found = a.types[typ.Underlying]
			if !found || base.Kind != typ.Kind {
				break
			}
		}
		if base.Kind != typ.Kind {
			break
		}
		typ = base
	}
	if typ.Named && (typ.Kind == ArrayType || typ.Kind == SliceType || typ.Kind == ReferenceType || typ.Kind == FunctionType) {
		// These families carry complete component/shape facts directly; their
		// anonymous base is not a nominal declaration in the type table.
		typ.Named = false
		typ.Name = typ.Underlying
		typ.Module = ""
	}
	return typ
}

// canExplicitConvert checks scalar-family conversion permissions; it does not
// implement runtime domain checking. The legacy numeric-to-string exception
// remains unresolved under MD-051 rather than claiming normative conformance.
//
// Rules:
//   - rules/declarations/enums.md — "Conversions"
//   - rules/types/types.md — explicit conversions
func canExplicitConvert(target Type, value Type) bool {

	if target.Kind == InvalidType || value.Kind == InvalidType {
		return false
	}
	if target.Kind == AnyType {
		return value.Kind != VoidType && value.Kind != NeverType
	}
	if value.Kind == AnyType {
		return false
	}

	// rules/concurrency/mutex.md §13 and temporal.md §4: opaque monotonic
	// points cannot be reinterpreted as wall-clock or other struct values.
	if target.MonotonicPoint || value.MonotonicPoint {
		return target.MonotonicPoint && value.MonotonicPoint
	}

	if target.Kind == EnumType && isIntegerType(value) {
		return true
	}
	if target.Kind == EnumType && target.Underlying == "string" && value.Kind == StringType && !value.Named {
		return true
	}
	if target.Kind == RegisterType && isIntegerType(value) {
		return true
	}

	// rules/types/types.md defines char and rune as distinct scalar types whose
	// represented values cross the integer boundary only through an explicit
	// conversion. This grants the conversion spelling, not implicit arithmetic
	// or assignability between the families.
	if isIntegerType(target) && (isIntegerType(value) || value.Kind == DecimalType || value.Kind == CharType || value.Kind == RuneType) {
		return true
	}

	if isIntegerType(target) && value.Kind == EnumType {
		return true
	}
	if target.Kind == StringType && !target.Named && value.Kind == EnumType && value.Underlying == "string" {
		return true
	}

	if target.Kind == CharType && (isIntegerType(value) || value.Kind == RuneType) {
		return true
	}

	if target.Kind == RuneType && (isIntegerType(value) || value.Kind == CharType) {
		return true
	}

	if target.Kind == BoolType && isNumericType(value) {
		return true
	}

	if target.Kind == RawPtrType && (value.Kind == UintType || value.Kind == RawPtrType || value.Kind == ReferenceType) {
		return true
	}
	if target.Kind == UintType && (value.Kind == RawPtrType || value.Kind == ReferenceType) {
		return true
	}

	// Legacy permission only; MD-051 records the missing normative constructor
	// relation. ToString is a separate operation, not a rule for this cast.
	if target.Kind == StringType && isNumericType(value) {
		return true
	}

	if target.Kind == UnionType || value.Kind == UnionType {
		return target.Kind == UnionType && value.Kind == UnionType && sameConcreteType(target, value)
	}

	if target.Kind == value.Kind {
		return true
	}

	if target.Kind == FloatType && isNumericType(value) {
		return true
	}

	if target.Kind == DecimalType && isNumericType(value) {
		return true
	}

	return false
}

// hasContracts reports attached source contracts.
// Rules: rules/types/contracts.md — Contract attachment.
func hasContracts(typ Type) bool {
	return len(typ.Contracts) > 0
}

// isNominal identifies the contract/unit/enum-sensitive subset used by legacy scalar conversion checks.
// Rules: rules/types/types.md — Named types; rules/types/contracts.md — Type contracts.
func isNominal(typ Type) bool {
	return typ.Named && (typ.Kind == EnumType || typ.Kind == UnionType || hasContracts(typ) || !typ.Dimension.IsZero())
}

// isIntegerType classifies signed and unsigned integer families.
// Rules: rules/types/types.md — int and uint.
func isIntegerType(typ Type) bool {
	return typ.Kind == IntType || typ.Kind == UintType
}

// isNumericType classifies integer, floating and decimal scalar carriers.
// Rules: rules/types/types.md — Numeric types.
func isNumericType(typ Type) bool {
	return typ.Kind == IntType || typ.Kind == UintType || typ.Kind == FloatType || typ.Kind == DecimalType
}

// sameConcreteType compares source identities and all family-specific parameters;
// a matching backend carrier never establishes nominal equality.
// Rules: rules/types/types.md — Type identity, Generic and parameterized types,
// Arrays, References, Function types.
func sameConcreteType(left Type, right Type) bool {
	if left.Kind != right.Kind {
		// Declaration prepass placeholders already carry canonical names and
		// modules, including recursive references and forward property types.
		// Their identity is retained while bodies acquire complete type facts.
		return (left.Kind == InvalidType || right.Kind == InvalidType) && left.Name != "" && left.Name == right.Name && left.Module == right.Module && sameTypeArguments(left.TypeArgs, right.TypeArgs) && sameConstArguments(left.ConstArgs, right.ConstArgs)
	}
	if (left.Named || right.Named) && (left.Named != right.Named || left.Name != right.Name || left.Module != right.Module || !sameTypeArguments(left.TypeArgs, right.TypeArgs) || !sameConstArguments(left.ConstArgs, right.ConstArgs)) {
		return false
	}
	if left.Kind == ReferenceType {
		if left.ReferenceMutable != right.ReferenceMutable || left.Element == nil || right.Element == nil {
			return false
		}
		return sameConcreteType(referenceReferent(*left.Element), referenceReferent(*right.Element))
	}
	if left.Kind == SliceType {
		return left.ReferenceMutable == right.ReferenceMutable && left.Element != nil && right.Element != nil && sameConcreteType(*left.Element, *right.Element)
	}
	if left.Kind == FunctionType || right.Kind == FunctionType {
		return sameFunctionType(left, right)
	}
	if left.Kind == ArrayType || right.Kind == ArrayType {
		return left.Kind == ArrayType && right.Kind == ArrayType &&
			sameArrayShape(left, right) && left.Element != nil && right.Element != nil &&
			sameConcreteType(*left.Element, *right.Element)
	}
	if left.Kind == VariadicPackType || right.Kind == VariadicPackType {
		return left.Kind == VariadicPackType && right.Kind == VariadicPackType &&
			left.Element != nil && right.Element != nil && sameConcreteType(*left.Element, *right.Element)
	}
	if left.Unit != "" || right.Unit != "" {
		return left.Kind == right.Kind &&
			left.Name == right.Name &&
			left.Unit == right.Unit &&
			sameTypeArguments(left.TypeArgs, right.TypeArgs) &&
			sameConstArguments(left.ConstArgs, right.ConstArgs)
	}
	if !left.Dimension.IsZero() || !right.Dimension.IsZero() {
		return left.Kind == right.Kind &&
			left.Name == right.Name &&
			left.Dimension.Equal(right.Dimension) &&
			sameTypeArguments(left.TypeArgs, right.TypeArgs) &&
			sameConstArguments(left.ConstArgs, right.ConstArgs)
	}
	if left.Name != "" || right.Name != "" {
		return left.Name == right.Name &&
			(!isEventFamilyName(left.Name) || eventCapacity(left) == eventCapacity(right)) &&
			sameTypeArguments(left.TypeArgs, right.TypeArgs) &&
			sameConstArguments(left.ConstArgs, right.ConstArgs)
	}
	return left.Kind == right.Kind
}

// sameTypeArguments retains ordered concrete generic argument identities.
// Rules: rules/types/types.md — Generic and parameterized types.
func sameTypeArguments(left []Type, right []Type) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !sameConcreteType(left[i], right[i]) {
			return false
		}
	}
	return true
}

// sameConstArguments retains ordered constant parameters in type identity.
// Rules: rules/types/types.md — Generic and parameterized types, Arrays.
func sameConstArguments(left []int64, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// sameFunctionType retains capability, variadic shape, ordered parameter and
// result identity without making nominal callable wrappers interchangeable.
// Rules: rules/types/types.md — Function types, Named types.
func sameFunctionType(left Type, right Type) bool {
	if (left.Named || right.Named) && (left.Named != right.Named || left.Name != right.Name || left.Module != right.Module) {
		return false
	}
	if left.Kind != FunctionType || right.Kind != FunctionType {
		return false
	}
	if left.FunctionReturnType == nil || right.FunctionReturnType == nil {
		return false
	}
	if normalizedCallableCapability(left.FunctionCapability) != normalizedCallableCapability(right.FunctionCapability) {
		return false
	}
	if left.FunctionVariadic != right.FunctionVariadic {
		return false
	}
	if len(left.FunctionParameterTypes) != len(right.FunctionParameterTypes) {
		return false
	}
	for i := range left.FunctionParameterTypes {
		if !sameConcreteType(left.FunctionParameterTypes[i], right.FunctionParameterTypes[i]) {
			return false
		}
	}
	return sameConcreteType(*left.FunctionReturnType, *right.FunctionReturnType)
}

// referenceReferent gives explicit borrowing of an anonymous owning T[] the
// canonical borrowed-slice identity. Fixed arrays and named owners retain their
// exact identity; provenance remains on the original reference fact.
// Rules: rules/collections/collections.md — §§7.1–7.2 Slice types and sources;
// rules/types/types.md — References.
func referenceReferent(typ Type) Type {
	if typ.Kind == ArrayType && !typ.Named && arrayShapeOf(typ) == ArrayShapeDynamic {
		return Type{Name: typ.Name, Kind: SliceType, Element: typ.Element}
	}
	return typ
}

// typeDeclarationIdentity retains module, source family and declared name.
// Rules: rules/types/types.md — Type identity.
func typeDeclarationIdentity(typ Type) string {
	return typ.Module + ":" + string(typ.Kind) + ":" + typ.Name
}

// canonicalTypeArgumentsKey retains complete ordered argument identities in cache keys.
// Rules: rules/types/types.md — Generic and parameterized types.
func canonicalTypeArgumentsKey(args []Type) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, canonicalTypeIdentity(arg))
	}
	return strings.Join(parts, ";")
}

// canonicalTypeIdentity uses nominal declarations before structural family facts;
// it never merges named array/function wrappers because their carriers match.
// Rules: rules/types/types.md — Type identity, Arrays, References, Function types.
func canonicalTypeIdentity(typ Type) string {
	if typ.Named {
		identity := typeDeclarationIdentity(typ)
		if len(typ.TypeArgs) > 0 {
			identity += "[" + canonicalTypeArgumentsKey(typ.TypeArgs) + "]"
		}
		for _, arg := range typ.ConstArgs {
			identity += fmt.Sprintf("[%d]", arg)
		}
		return identity
	}
	switch typ.Kind {
	case ReferenceType:
		mode := "ref"
		if typ.ReferenceMutable {
			mode = "ref-mut"
		}
		if typ.Element == nil {
			return mode + ":<nil>"
		}
		return mode + ":" + canonicalTypeIdentity(referenceReferent(*typ.Element))
	case ArrayType:
		shape := string(arrayShapeOf(typ))
		if length, ok := exactFixedArrayLength(typ); ok {
			shape += ":" + length.String()
		}
		if typ.Element == nil {
			return "array:" + shape + ":<nil>"
		}
		return "array:" + shape + ":" + canonicalTypeIdentity(*typ.Element)
	case SliceType:
		if typ.Element == nil {
			return "slice:<nil>"
		}
		return "slice:" + canonicalTypeIdentity(*typ.Element)
	case VariadicPackType:
		if typ.Element == nil {
			return "variadic-pack:<nil>"
		}
		return "variadic-pack:" + canonicalTypeIdentity(*typ.Element)
	case FunctionType:
		params := make([]string, 0, len(typ.FunctionParameterTypes))
		for _, param := range typ.FunctionParameterTypes {
			params = append(params, canonicalTypeIdentity(param))
		}
		returnType := "<nil>"
		if typ.FunctionReturnType != nil {
			returnType = canonicalTypeIdentity(*typ.FunctionReturnType)
		}
		shape := "fixed"
		if typ.FunctionVariadic {
			shape = "variadic"
		}
		return string(normalizedCallableCapability(typ.FunctionCapability)) + ":fn:" + shape + ":(" + strings.Join(params, ",") + ")->" + returnType
	default:
		identity := typeDeclarationIdentity(typ)
		if identity == "::" || typ.Name == "" {
			identity = string(typ.Kind)
		}
		if len(typ.TypeArgs) > 0 {
			identity += "[" + canonicalTypeArgumentsKey(typ.TypeArgs) + "]"
		}
		if len(typ.ConstArgs) > 0 {
			parts := make([]string, 0, len(typ.ConstArgs))
			for _, arg := range typ.ConstArgs {
				parts = append(parts, fmt.Sprintf("%d", arg))
			}
			identity += "[" + strings.Join(parts, ";") + "]"
		}
		return identity
	}
}

func genericFunctionDisplayName(name string, function Function) string {
	if len(function.Parameters) == 0 {
		return name
	}
	return name
}
