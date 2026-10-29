package sema

import (
	"strings"

	"sec/internal/ast"
)

// Decimal core methods of decimal and decimal128 (core-library.md §§ 10.2,
// 10.4; MD-066 § 8, MD-068 § 9(2)). Normalize and the integral roundings are
// total; Rescale and RoundTo report PrecisionError, Abs OverflowError and
// Clamp RangeError. Formatting and conversions keep their own members.

// decimalCoreParameter is one parameter of a decimal core method: the
// receiver type itself, int32 scale, or the core RoundingMode.
type decimalCoreParameter string

const (
	decimalParameterSelf     decimalCoreParameter = "self"
	decimalParameterScale    decimalCoreParameter = "scale"
	decimalParameterRounding decimalCoreParameter = "rounding"
)

// decimalCoreMethod describes one method: its required parameters, optional
// trailing parameters and the error type of a Result return ("" for a plain
// receiver-typed result).
type decimalCoreMethod struct {
	required []decimalCoreParameter
	optional []decimalCoreParameter
	err      string
}

var decimalCoreMethods = map[string]decimalCoreMethod{
	"Normalize": {},
	"Floor":     {},
	"Ceiling":   {},
	"Truncate":  {},
	"Round":     {},
	"Min":       {required: []decimalCoreParameter{decimalParameterSelf}},
	"Max":       {required: []decimalCoreParameter{decimalParameterSelf}},
	"Abs":       {err: "OverflowError"},
	"Clamp":     {required: []decimalCoreParameter{decimalParameterSelf, decimalParameterSelf}, err: "RangeError"},
	"Rescale":   {required: []decimalCoreParameter{decimalParameterScale}, err: "PrecisionError"},
	"RoundTo":   {required: []decimalCoreParameter{decimalParameterScale}, optional: []decimalCoreParameter{decimalParameterRounding}, err: "PrecisionError"},
}

var decimalCoreMethodOrder = []string{"Normalize", "Floor", "Ceiling", "Truncate", "Round", "Min", "Max", "Abs", "Clamp", "Rescale", "RoundTo"}

// decimalCoreReceiver reports a primitive decimal or decimal128; named and
// unit-carrying decimals keep their own identity.
func decimalCoreReceiver(typ Type) bool {
	return typ.Kind == DecimalType && !typ.Named && !typ.Declared && !hasUnitSemantics(typ)
}

func compilerKnownDecimalMembers(typ Type) []CompilerKnownMember {
	if !decimalCoreReceiver(typ) {
		return nil
	}
	prefix := "CKM-DECIMAL-"
	if typ.Name == "decimal128" {
		prefix = "CKM-DECIMAL128-"
	}
	members := make([]CompilerKnownMember, 0, len(decimalCoreMethodOrder))
	for _, name := range decimalCoreMethodOrder {
		method := decimalCoreMethods[name]
		result := typ
		if method.err != "" {
			result = compilerKnownResult(typ, decimalCoreErrorType(method.err))
		}
		members = append(members, CompilerKnownMember{ID: prefix + strings.ToUpper(name), Name: name, Kind: CompilerKnownMethod, Result: result})
	}
	return members
}

// decimalCoreErrorType names a core error enum of a decimal method result;
// the call resolves it against the loaded core declaration.
func decimalCoreErrorType(name string) Type {
	if builtin, ok := sharedBuiltinTypes()[name]; ok {
		return builtin
	}
	return Type{Name: name, Kind: EnumType, Module: "core", Named: true, Declared: true, ErrorAssignable: true}
}

// inferDecimalCoreCall checks the arity and argument types of a decimal core
// method call and returns its result with the loaded core error type.
func (a *Analyzer) inferDecimalCoreCall(expr *ast.CallExpression, receiver Type, member CompilerKnownMember) (Type, expressionValue, bool) {
	method, ok := decimalCoreMethods[member.Name]
	if !ok {
		return Type{}, expressionValue{}, false
	}
	display := typeDisplayName(receiver) + "." + member.Name
	if !a.checkCompilerKnownCallArity(expr, display, len(method.required), len(method.required)+len(method.optional)) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	parameters := append(append([]decimalCoreParameter{}, method.required...), method.optional...)
	for index, argument := range expr.Arguments {
		var expected Type
		switch parameters[index] {
		case decimalParameterSelf:
			expected = receiver
		case decimalParameterScale:
			expected = builtinType("int32")
		case decimalParameterRounding:
			rounding, declared := a.types["RoundingMode"]
			if !declared {
				a.addErrorAtToken(expressionToken(argument), "%s needs the core RoundingMode type", display)
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
			expected = rounding
		}
		argumentType, _ := a.inferExpressionWithExpected(argument, expected)
		if argumentType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !sameConcreteType(expected, argumentType) && !a.canInitialize(expected, argumentType, argument) {
			a.addErrorAtToken(expressionToken(argument), "%s argument %d must be %s, got %s", display, index+1, typeDisplayName(expected), typeDisplayName(argumentType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
	}
	result := member.Result
	if method.err != "" {
		if declared, ok := a.types[method.err]; ok {
			result = compilerKnownResult(receiver, declared)
		}
	}
	return a.refinedCompilerKnownType(result), expressionValue{Display: expr.String()}, true
}
