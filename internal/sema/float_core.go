package sema

import (
	"strings"

	"sec/internal/ast"
)

// Floating core methods of float, float32 and float64 (core-library.md § 9.3;
// sec-mlir-dialect_package20.md §§ 29–36). Abs, NaN-propagating Min/Max,
// classification and rounding are total; Clamp is Result[T, RangeError].
// SquareRoot stays outside this surface until MathError and its core-math
// package exist (§ 37); ToString keeps its general member.

// floatCoreMethods maps a method name to its parameter count.
var floatCoreMethods = map[string]int{
	"Abs": 0, "Min": 1, "Max": 1, "Clamp": 2,
	"IsFinite": 0, "IsInfinite": 0, "IsNaN": 0, "IsNormal": 0, "IsSubnormal": 0,
	"Floor": 0, "Ceiling": 0, "Truncate": 0, "Round": 0,
}

// floatCoreReceiver reports whether typ is one of the primitive binary float
// types; named and unit-carrying floats keep their own identity.
func floatCoreReceiver(typ Type) bool {
	return typ.Kind == FloatType && !typ.Named && !typ.Declared && !hasUnitSemantics(typ)
}

func compilerKnownFloatMembers(typ Type) []CompilerKnownMember {
	if !floatCoreReceiver(typ) {
		return nil
	}
	boolType := builtinType("bool")
	members := []CompilerKnownMember{}
	for _, name := range []string{"Abs", "Min", "Max", "Clamp", "IsFinite", "IsInfinite", "IsNaN", "IsNormal", "IsSubnormal", "Floor", "Ceiling", "Truncate", "Round"} {
		result := typ
		switch {
		case strings.HasPrefix(name, "Is"):
			result = boolType
		case name == "Clamp":
			result = compilerKnownResult(typ, builtinType("RangeError"))
		}
		members = append(members, CompilerKnownMember{ID: "CKM-FLOAT-" + strings.ToUpper(name), Name: name, Kind: CompilerKnownMethod, Result: result})
	}
	return members
}

// inferFloatCoreCall checks the arity and the same-type arguments of a float
// core method call (Min(other: T), Clamp(minimum: T, maximum: T)).
func (a *Analyzer) inferFloatCoreCall(expr *ast.CallExpression, receiver Type, member CompilerKnownMember) (Type, expressionValue, bool) {
	count := floatCoreMethods[member.Name]
	if !a.checkCompilerKnownCallArity(expr, typeDisplayName(receiver)+"."+member.Name, count, count) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	for _, argument := range expr.Arguments {
		argumentType, _ := a.inferExpressionWithExpected(argument, receiver)
		if argumentType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !sameConcreteType(receiver, argumentType) && !a.canInitialize(receiver, argumentType, argument) {
			a.addErrorAtToken(expressionToken(argument), "%s.%s argument must be %s, got %s", typeDisplayName(receiver), member.Name, typeDisplayName(receiver), typeDisplayName(argumentType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
	}
	return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
}

// StaticMemberOwnerOf returns the type whose associated member `T.Member`
// names, as resolved for compiler-known associated properties such as
// int32.Bits or float64.Max.
func (a *Analyzer) StaticMemberOwnerOf(member *ast.MemberExpression) (Type, bool) {
	if a == nil || member == nil || member.Property == nil {
		return Type{}, false
	}
	if _, known := a.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)]; !known {
		return Type{}, false
	}
	path, ok := typePathFromExpression(member.Object)
	if !ok {
		return Type{}, false
	}
	typ, ok := a.types[a.resolveTypeName(path)]
	return typ, ok
}

// staticCompilerKnownPropertyReceiver reports whether a call receiver such as
// `float64.NegativeInfinity` in `float64.NegativeInfinity.Abs()` is a
// compiler-known associated property value rather than a type path.
func (a *Analyzer) staticCompilerKnownPropertyReceiver(receiver ast.Expression) bool {
	member, ok := receiver.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return false
	}
	path, ok := typePathFromExpression(member.Object)
	if !ok {
		return false
	}
	owner, ok := a.types[a.resolveTypeName(path)]
	if !ok {
		return false
	}
	known, ok := compilerKnownMember(owner, member.Property.Value, true)
	return ok && known.Kind == CompilerKnownProperty
}

// reportMisspelledNumericProperty diagnoses a numeric associated property
// written in another case, such as float64.max for float64.Max: the
// PascalCase spelling is the only one (core-library.md § 7.1, decided
// 2026-10-26).
func (a *Analyzer) reportMisspelledNumericProperty(typ Type, expr *ast.MemberExpression) bool {
	if !isIntegerType(typ) && typ.Kind != FloatType {
		return false
	}
	for _, member := range compilerKnownStaticMembers(typ) {
		if member.Kind == CompilerKnownProperty && member.Name != expr.Property.Value && strings.EqualFold(member.Name, expr.Property.Value) {
			a.addErrorAtTokenWithMetadata(expr.Property.Token, "",
				"Numeric associated properties are spelled in PascalCase: use "+typeDisplayName(typ)+"."+member.Name+".",
				"%s has no associated property %s; did you mean %s?", typeDisplayName(typ), expr.Property.Value, member.Name)
			return true
		}
	}
	return false
}
